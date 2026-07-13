// Package server implements the gRPC URLShortenerService and the lifecycle
// (Listen, Serve, GracefulStop) of the gRPC server. The package depends on
// storage and logger; nothing here calls os.Exit / log.Fatal — failures are
// returned to main.
package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"time"

	"github.com/ducanng/URLShortener/internal/domain"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/shortcode"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"
	"github.com/ducanng/URLShortener/internal/repository/postgres"
	"github.com/ducanng/URLShortener/internal/repository/redis"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	grpcAddr   = ":50051"
	prefixLink = "http://localhost:8080/"

	// traceMetadataKey is the gRPC metadata key used to propagate the
	// HTTP X-Trace-Id header across the grpc-gateway → gRPC boundary.
	// gRPC normalises metadata keys to lower-case.
	traceMetadataKey = "x-trace-id"

	// defaultURLTTL is applied when CreateURL receives neither expires_at
	// nor no_expire. Tunable later via env var without changing call sites.
	defaultURLTTL = 30 * 24 * time.Hour
)

// service implements the gRPC URLShortenerService handlers. The embedded
// *Logger gives every handler structured logging without an extra field.
type service struct {
	*logger.Logger
	urlshortenerpb.URLShortenerServiceServer
	Cache   *redis.Cache
	Counter *redis.Counter
	DB      *postgres.Repo
}

// GRPCServer wraps the gRPC server lifecycle (listen, serve, shutdown) and
// holds the process logger for startup/shutdown messages.
type GRPCServer struct {
	*logger.Logger
	srv *grpc.Server
	lis net.Listener
}

// NewGRPCServer binds :50051, wires interceptors (trace → logging →
// recovery), registers the URL service and the standard gRPC health
// service. The listener bind is the only failure point so the only
// returned error is from net.Listen.
func NewGRPCServer(log *logger.Logger, cache *redis.Cache, counter *redis.Counter, db *postgres.Repo) (*GRPCServer, error) {
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", grpcAddr, err)
	}

	// Order matters: trace must run first so logging/recovery can read
	// the trace_id from ctx.
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			traceInterceptor(),
			loggingInterceptor(log),
			recoveryInterceptor(log),
		),
	)

	urlshortenerpb.RegisterURLShortenerServiceServer(srv, &service{
		Logger:  log,
		Cache:   cache,
		Counter: counter,
		DB:      db,
	})

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("urlshortener.URLShortenerService", healthpb.HealthCheckResponse_SERVING)

	return &GRPCServer{Logger: log, srv: srv, lis: lis}, nil
}

// ListenAndServe blocks until the gRPC server stops. Returns the error from
// grpc.Server.Serve, or nil on graceful shutdown.
func (s *GRPCServer) ListenAndServe() error {
	s.Infof("gRPC server starting on %s", grpcAddr)
	return s.srv.Serve(s.lis)
}

// Shutdown gracefully stops the gRPC server, falling back to a forced Stop
// if GracefulStop has not finished by the time ctx is cancelled. Drains
// in-flight RPCs without losing requests under normal load.
func (s *GRPCServer) Shutdown(ctx context.Context) {
	stopped := make(chan struct{})
	go func() {
		s.srv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
		s.Info("gRPC server stopped gracefully")
	case <-ctx.Done():
		s.Warn("gRPC GracefulStop timed out, forcing Stop")
		s.srv.Stop()
	}
}

// traceInterceptor reads x-trace-id from incoming metadata (forwarded by
// grpc-gateway from the HTTP X-Trace-Id header) and stores it in ctx. When
// no metadata is present (direct gRPC client without trace), it generates a
// fresh UUID so every RPC has a trace_id.
func traceInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		traceID := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if v := md.Get(traceMetadataKey); len(v) > 0 {
				traceID = v[0]
			}
		}
		if traceID == "" {
			traceID = logger.NewTraceID()
		}
		ctx = logger.WithTraceID(ctx, traceID)
		return handler(ctx, req)
	}
}

// loggingInterceptor emits one structured log line per unary RPC with the
// method, latency, and gRPC status code. The trace_id from ctx is included
// automatically via WithContext.
func loggingInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.WithContext(ctx).Infow("grpc call",
			"method", info.FullMethod,
			"duration", time.Since(start),
			"code", status.Code(err).String(),
		)
		return resp, err
	}
}

// recoveryInterceptor catches panics in handlers, logs them with stack
// trace + trace_id, and returns codes.Internal so the client receives a
// well-formed gRPC error instead of a connection drop.
func recoveryInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.WithContext(ctx).Errorf("grpc panic recovered: %v\n%s", r, debug.Stack())
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// resolveExpiry derives the effective expiry time from the request.
// Returns nil for "never expires". The four cases are:
//
//	(no_expire=true, expires_at=nil)   → nil  (vĩnh viễn)
//	(no_expire=false, expires_at=nil)  → now + defaultURLTTL
//	(no_expire=false, expires_at=t)    → t (must be in the future)
//	(no_expire=true, expires_at=t)     → InvalidArgument (mâu thuẫn)
func resolveExpiry(req *urlshortenerpb.CreateURLRequest, now time.Time) (*time.Time, error) {
	hasExplicit := req.GetExpiresAt() != nil
	if req.GetNoExpire() && hasExplicit {
		return nil, status.Error(codes.InvalidArgument, "no_expire and expires_at are mutually exclusive")
	}
	if req.GetNoExpire() {
		return nil, nil
	}
	if hasExplicit {
		t := req.GetExpiresAt().AsTime()
		if !t.After(now) {
			return nil, status.Error(codes.InvalidArgument, "expires_at must be in the future")
		}
		return &t, nil
	}
	t := now.Add(defaultURLTTL)
	return &t, nil
}

// expiresAtPB converts domain.URLEntry.ExpiresAt to a protobuf Timestamp.
// Returns nil for entries that never expire.
func expiresAtPB(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// CreateURL implements the gRPC RPC — generates an ID, persists the entry,
// and best-effort caches it.
//
//goland:noinspection GoUnreachableCode
func (s *service) CreateURL(ctx context.Context, req *urlshortenerpb.CreateURLRequest) (*urlshortenerpb.Response, error) {
	log := s.WithContext(ctx)

	expiresAt, err := resolveExpiry(req, time.Now())
	if err != nil {
		return nil, err
	}

	id, err := s.Counter.NextID(ctx)
	if err != nil {
		log.Errorf("getting next ID from Redis: %v", err)
		return nil, status.Errorf(codes.Unavailable, "ID generation unavailable: %v", err)
	}
	url := domain.URLEntry{
		Id:          id,
		OriginalURL: req.GetUrl(),
		ShortedURL:  shortcode.ShortPathFromID(id),
		Clicks:      0,
		ExpiresAt:   expiresAt,
	}
	if err := s.DB.Save(ctx, url); err != nil {
		log.Errorf("saving to db: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to save URL: %v", err)
	}
	if err := s.Cache.Set(ctx, url); err != nil {
		log.Warnf("failed to cache in redis, continuing: %v", err)
	}
	return &urlshortenerpb.Response{
		Message: "Create short url",
		Status:  "Success",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  url.OriginalURL,
			ShortenedURL: prefixLink + url.ShortedURL,
			Clicks:       url.Clicks,
			ExpiresAt:    expiresAtPB(url.ExpiresAt),
		},
	}, nil
}

// GetURL implements the gRPC RPC — cache-aside: try Redis first, fall back
// to PostgreSQL on miss, repopulate cache best-effort. Returns
// FailedPrecondition when the URL has expired so the HTTP layer can map
// it to 410 Gone (distinct from a real NotFound).
//
//goland:noinspection ALL
func (s *service) GetURL(ctx context.Context, req *urlshortenerpb.GetURLRequest) (*urlshortenerpb.Response, error) {
	log := s.WithContext(ctx)

	url, err := s.Cache.Get(ctx, req.GetURL())
	if err != nil {
		url, err = s.DB.Load(ctx, req.GetURL())
		if err != nil {
			log.Errorf("loading from db: %v", err)
			if errors.Is(err, sql.ErrNoRows) {
				return nil, status.Errorf(codes.NotFound, "URL %q not found", req.GetURL())
			}
			return nil, status.Errorf(codes.Internal, "failed to load URL: %v", err)
		}
		if cacheErr := s.Cache.Set(ctx, url); cacheErr != nil {
			log.Warnf("failed to re-cache in redis: %v", cacheErr)
		}
	}

	if url.ExpiresAt != nil && !time.Now().Before(*url.ExpiresAt) {
		return nil, status.Errorf(codes.FailedPrecondition, "URL %q has expired", req.GetURL())
	}

	return &urlshortenerpb.Response{
		Message: "Get short url",
		Status:  "Success",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  url.OriginalURL,
			ShortenedURL: url.ShortedURL,
			Clicks:       url.Clicks,
			ExpiresAt:    expiresAtPB(url.ExpiresAt),
		},
	}, nil
}
