package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/ducanng/URLShortener/docs"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/metrics"
	"github.com/ducanng/URLShortener/internal/repository/postgres"
	"github.com/ducanng/URLShortener/internal/repository/redis"
	"github.com/ducanng/URLShortener/internal/service/urlservice"
	grpctransport "github.com/ducanng/URLShortener/internal/transport/grpc"
	httptransport "github.com/ducanng/URLShortener/internal/transport/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

// @title URL Shortener API
// @description This is a server for URL Shortener API.
// @version 1.5.0
// @BasePath /
// @schemes http https
// @host localhost:8080
// @securityDefinitions.basic  BasicAuth
func RunServer() {
	// Initialise the structured logger first; everything else depends on it
	// for error reporting. Failure here is fatal — no point continuing
	// without observability.
	log, err := logger.New(logger.Config{Level: zapcore.InfoLevel})
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// Redirect the stdlib log package so any third-party library that uses
	// `log.Printf` also emits structured JSON.
	zap.RedirectStdLog(log.Logger)

	// Context that cancels on SIGINT / SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Storage. Constructors return errors so main owns the fatal decision.
	cache, err := redis.NewCache(log)
	if err != nil {
		log.Fatalf("init redis cache: %v", err)
	}
	counter, err := redis.NewCounter(log)
	if err != nil {
		log.Fatalf("init redis counter: %v", err)
	}
	pgRepo, err := postgres.New(log)
	if err != nil {
		log.Fatalf("init postgres: %v", err)
	}

	// Seed the Redis global counter from PG MAX(id) — uses SET NX so it is a
	// no-op when the counter already exists (normal restart). Fail fast when
	// Redis is unreachable because ID generation depends on it.
	maxID, err := pgRepo.MaxID(ctx)
	if err != nil {
		log.Fatalf("read MAX(id) from DB: %v", err)
	}
	if err := counter.InitCounter(ctx, maxID+1); err != nil {
		log.Fatalf("init Redis counter: %v", err)
	}

	// Service layer — business logic is isolated here; transport adapters
	// (gRPC, HTTP) call into it via the URLService methods.
	svc := urlservice.New(log, pgRepo, cache, counter)

	// gRPC server
	grpcServer, err := grpctransport.NewServer(log, svc)
	if err != nil {
		log.Fatalf("init grpc server: %v", err)
	}

	// gRPC client (used by HTTP gateway + redirect handler)
	grpcClient, cleanup, err := grpctransport.NewClient("localhost:50051", log)
	if err != nil {
		log.Fatalf("init grpc client: %v", err)
	}
	defer cleanup()

	// HTTP server (Gin + grpc-gateway + Swagger + Prometheus middleware + trace_id)
	httpSrv := httptransport.NewServer(ctx, log, grpcClient, cache, pgRepo)

	// Dedicated metrics server on :7070 — isolated from public :8080.
	metricsSrv := metrics.NewServer(metrics.DefaultAddr)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error { return grpcServer.ListenAndServe() })

	g.Go(func() error {
		log.Info("HTTP gateway starting on :8080")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		log.Infof("Metrics server starting on %s", metrics.DefaultAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	// Graceful shutdown
	g.Go(func() error {
		<-gCtx.Done()
		log.Info("Shutting down servers...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Errorf("HTTP shutdown error: %v", err)
		}
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			log.Errorf("Metrics shutdown error: %v", err)
		}

		grpcShutdownCtx, grpcCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer grpcCancel()
		grpcServer.Shutdown(grpcShutdownCtx)

		log.Info("All servers stopped gracefully")
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}

func main() {
	RunServer()
}
