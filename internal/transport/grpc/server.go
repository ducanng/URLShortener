// Package grpc implements the gRPC URLShortenerService transport: the server
// lifecycle (Listen, Serve, GracefulStop), the unary interceptors (trace →
// logging → recovery), the proto ↔ domain handler adapter, and the gRPC
// client used by the HTTP gateway. It is a thin transport layer: proto types
// are decoded into domain params, the service layer does the work, and domain
// results are re-encoded into proto responses. Nothing here calls os.Exit /
// log.Fatal — failures are returned to main.
package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/service/urlservice"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Server wraps the gRPC server lifecycle (listen, serve, shutdown) and holds
// the process logger for startup/shutdown messages.
type Server struct {
	*logger.Logger
	srv  *grpc.Server
	lis  net.Listener
	addr string
}

// NewServer binds addr (e.g. ":50051"), wires interceptors (trace → logging →
// recovery), registers the URL service and the standard gRPC health service.
// The listener bind is the only failure point so the only returned error is
// from net.Listen.
func NewServer(addr string, svc *urlservice.URLService, log *logger.Logger) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
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

	return &Server{Logger: log, srv: srv, lis: lis, addr: addr}, nil
}

// ListenAndServe blocks until the gRPC server stops.
func (s *Server) ListenAndServe() error {
	s.Infof("gRPC server starting on %s", s.addr)
	return s.srv.Serve(s.lis)
}

// Shutdown gracefully stops the gRPC server, falling back to a forced Stop
// if GracefulStop has not finished by the time ctx is cancelled.
func (s *Server) Shutdown(ctx context.Context) {
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
