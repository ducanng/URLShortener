package grpc

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/ducanng/URLShortener/internal/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// traceMetadataKey is the gRPC metadata key used to propagate the HTTP
// X-Trace-Id header across the grpc-gateway → gRPC boundary. gRPC normalises
// metadata keys to lower-case.
const traceMetadataKey = "x-trace-id"

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
