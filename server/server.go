package server

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"runtime/debug"
	"time"

	"github.com/ducanng/URLShortener/model"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"
	"github.com/ducanng/URLShortener/shorten"
	"github.com/ducanng/URLShortener/storage"

	base62 "github.com/alextanhongpin/base62"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

var prefixLink = "http://localhost:8080/"

// service implements the gRPC URLShortenerService handlers.
type service struct {
	urlshortenerpb.URLShortenerServiceServer
	Redis *storage.Redis
	DB    *storage.SQLStore
}

// GRPCServer wraps the gRPC server lifecycle (listen, serve, shutdown).
type GRPCServer struct {
	srv *grpc.Server
	lis net.Listener
}

// NewGRPCServer creates a gRPC server with interceptors, health check,
// and the URLShortenerService registered.
func NewGRPCServer(redis *storage.Redis, db *storage.SQLStore) *GRPCServer {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(loggingInterceptor, recoveryInterceptor),
	)

	urlshortenerpb.RegisterURLShortenerServiceServer(srv, &service{
		Redis: redis,
		DB:    db,
	})

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("urlshortener.URLShortenerService", healthpb.HealthCheckResponse_SERVING)

	return &GRPCServer{srv: srv, lis: lis}
}

// ListenAndServe starts the gRPC server. Blocks until the server stops.
func (s *GRPCServer) ListenAndServe() error {
	log.Println("gRPC server starting on :50051")
	return s.srv.Serve(s.lis)
}

// Shutdown gracefully stops the gRPC server. Falls back to forced Stop
// if GracefulStop doesn't finish before ctx is cancelled.
func (s *GRPCServer) Shutdown(ctx context.Context) {
	stopped := make(chan struct{})
	go func() { s.srv.GracefulStop(); close(stopped) }()
	select {
	case <-stopped:
		log.Println("gRPC server stopped gracefully")
	case <-ctx.Done():
		log.Println("gRPC GracefulStop timed out, forcing Stop")
		s.srv.Stop()
	}
}

// loggingInterceptor logs method, duration, and gRPC status code for every unary RPC.
func loggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	log.Printf("grpc method=%s duration=%s code=%s", info.FullMethod, time.Since(start), status.Code(err))
	return resp, err
}

// recoveryInterceptor catches panics in handlers and returns codes.Internal.
func recoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("grpc panic recovered: %v\n%s", r, debug.Stack())
			err = status.Errorf(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}

//goland:noinspection GoUnreachableCode
func (s *service) CreateURL(_ context.Context, req *urlshortenerpb.CreateURLRequest) (*urlshortenerpb.Response, error) {
	//log.Printf("CreateURL call...")
	// create short url
	shorPath := shorten.GenerateShortLink()
	url := model.URLEntry{
		Id:          int64(base62.Decode(shorPath)),
		OriginalURL: req.GetUrl(),
		ShortedURL:  shorPath,
		Clicks:      0,
	}
	// save to redis (best effort — not critical)
	if err := s.Redis.Set(url); err != nil {
		log.Printf("warn: Failed to cache in redis, continuing: %v", err)
	}
	// save to db
	if err := s.DB.Save(url); err != nil {
		log.Printf("Error saving to db: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to save URL: %v", err)
	}
	return &urlshortenerpb.Response{
		Message: "Create short url",
		Status:  "Success",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  url.OriginalURL,
			ShortenedURL: prefixLink + url.ShortedURL,
			Clicks:       url.Clicks,
		},
	}, nil
}

//goland:noinspection ALL
func (s *service) GetURL(_ context.Context, req *urlshortenerpb.GetURLRequest) (*urlshortenerpb.Response, error) {
	//log.Printf("GetURL call...")
	// get data from redis (cache-aside: miss → fallback to db)
	url, err := s.Redis.Get(req.GetURL())
	if err != nil {
		url, err = s.DB.Load(req.GetURL())
		if err != nil {
			log.Printf("Error loading from db: %v", err)
			if errors.Is(err, sql.ErrNoRows) {
				return nil, status.Errorf(codes.NotFound, "URL %q not found", req.GetURL())
			}
			return nil, status.Errorf(codes.Internal, "failed to load URL: %v", err)
		}
		// re-populate cache (best effort)
		if cacheErr := s.Redis.Set(url); cacheErr != nil {
			log.Printf("warn: Failed to re-cache in redis: %v", cacheErr)
		}
	}
	// return response
	return &urlshortenerpb.Response{
		Message: "Get short url",
		Status:  "Success",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  url.OriginalURL,
			ShortenedURL: url.ShortedURL,
			Clicks:       url.Clicks,
		},
	}, nil
}
