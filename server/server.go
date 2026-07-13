// Package server implements the gRPC URLShortenerService and the lifecycle
// (Listen, Serve, GracefulStop) of the gRPC server. It is a thin transport
// adapter: proto types are decoded into domain params, the service layer
// does the work, and domain results are re-encoded into proto responses.
// Nothing here calls os.Exit / log.Fatal — failures are returned to main.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"time"

	"github.com/ducanng/URLShortener/internal/domain"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/service/urlservice"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	grpcAddr = ":50051"

	// prefixLink is the base URL prepended to the generated short path
	// when building the response ShortenedURL field.
	prefixLink = "http://localhost:8080/"

	// traceMetadataKey is the gRPC metadata key used to propagate the
	// HTTP X-Trace-Id header across the grpc-gateway → gRPC boundary.
	// gRPC normalises metadata keys to lower-case.
	traceMetadataKey = "x-trace-id"
)

// service is the thin gRPC adapter. It converts proto ↔ domain types and
// delegates all business decisions to the injected URLService.
type service struct {
	*logger.Logger
	urlshortenerpb.URLShortenerServiceServer
	svc *urlservice.URLService
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
func NewGRPCServer(log *logger.Logger, svc *urlservice.URLService) (*GRPCServer, error) {
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
		Logger: log,
		svc:    svc,
	})

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("urlshortener.URLShortenerService", healthpb.HealthCheckResponse_SERVING)

	return &GRPCServer{Logger: log, srv: srv, lis: lis}, nil
}

// ListenAndServe blocks until the gRPC server stops.
func (s *GRPCServer) ListenAndServe() error {
	s.Infof("gRPC server starting on %s", grpcAddr)
	return s.srv.Serve(s.lis)
}

// Shutdown gracefully stops the gRPC server, falling back to a forced Stop
// if GracefulStop has not finished by the time ctx is cancelled.
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

// ── interceptors ─────────────────────────────────────────────────────────────

// traceInterceptor reads x-trace-id from incoming metadata (forwarded by
// grpc-gateway from the HTTP X-Trace-Id header) and stores it in ctx.
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

// loggingInterceptor emits one structured log line per unary RPC with
// method, latency, and gRPC status code.
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
// trace + trace_id, and returns codes.Internal.
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

// ── RPC handlers ─────────────────────────────────────────────────────────────

// CreateURL decodes the proto request → domain params, delegates to
// URLService, then encodes the domain result → proto response.
//
//goland:noinspection GoUnreachableCode
func (s *service) CreateURL(ctx context.Context, req *urlshortenerpb.CreateURLRequest) (*urlshortenerpb.Response, error) {
	log := s.WithContext(ctx)

	var explicit *time.Time
	if req.GetExpiresAt() != nil {
		t := req.GetExpiresAt().AsTime()
		explicit = &t
	}

	entry, err := s.svc.CreateURL(ctx, urlservice.CreateParams{
		OriginalURL:    req.GetUrl(),
		NoExpire:       req.GetNoExpire(),
		ExplicitExpiry: explicit,
	}, time.Now())
	if err != nil {
		if errors.Is(err, urlservice.ErrNoExpireConflict) || errors.Is(err, urlservice.ErrExpiresAtPast) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		log.Errorf("create URL: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create URL: %v", err)
	}

	return &urlshortenerpb.Response{
		Message: "Create short url",
		Status:  "Success",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  entry.OriginalURL,
			ShortenedURL: prefixLink + entry.ShortedURL,
			Clicks:       entry.Clicks,
			ExpiresAt:    expiresAtPB(entry.ExpiresAt),
		},
	}, nil
}

// GetURL decodes the proto request, delegates to URLService (cache-aside
// + expiry check), then encodes the result.
//
//goland:noinspection ALL
func (s *service) GetURL(ctx context.Context, req *urlshortenerpb.GetURLRequest) (*urlshortenerpb.Response, error) {
	log := s.WithContext(ctx)

	entry, err := s.svc.GetURL(ctx, req.GetURL())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			return nil, status.Errorf(codes.NotFound, "URL %q not found", req.GetURL())
		case errors.Is(err, domain.ErrExpired):
			return nil, status.Errorf(codes.FailedPrecondition, "URL %q has expired", req.GetURL())
		default:
			log.Errorf("get URL: %v", err)
			return nil, status.Errorf(codes.Internal, "failed to get URL: %v", err)
		}
	}

	return &urlshortenerpb.Response{
		Message: "Get short url",
		Status:  "Success",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  entry.OriginalURL,
			ShortenedURL: entry.ShortedURL,
			Clicks:       entry.Clicks,
			ExpiresAt:    expiresAtPB(entry.ExpiresAt),
		},
	}, nil
}

// expiresAtPB converts a *time.Time to a proto Timestamp.
// Returns nil for entries that never expire (nil input).
func expiresAtPB(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
