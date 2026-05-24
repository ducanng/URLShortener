package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/ducanng/URLShortener/client"
	_ "github.com/ducanng/URLShortener/docs"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"
	"github.com/ducanng/URLShortener/server"
	"github.com/ducanng/URLShortener/storage"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ShortenRequest represents the JSON body for creating a short URL.
type ShortenRequest struct {
	URL string `json:"url" example:"https://example.com"`
}

// URLResponse represents the JSON response from grpc-gateway.
type URLResponse struct {
	Message string        `json:"message" example:"Create short url"`
	Status  string        `json:"status" example:"Success"`
	URL     *ShortenedURL `json:"url,omitempty"`
}

// ShortenedURL represents URL details in the response.
type ShortenedURL struct {
	OriginalURL  string `json:"originalURL" example:"https://example.com"`
	ShortenedURL string `json:"shortenedURL" example:"http://localhost:8080/abc123"`
	Clicks       int32  `json:"clicks" example:"0"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Message string `json:"message" example:"URL not found"`
}

// newHTTPServer sets up the Gin router with grpc-gateway, Swagger, Prometheus,
// and the redirect handler, then returns a configured *http.Server.
func newHTTPServer(ctx context.Context, grpcClient *client.Client) *http.Server {
	// grpc-gateway: reverse proxy REST → gRPC (auto-generated from proto annotations)
	gwMux := runtime.NewServeMux()
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := urlshortenerpb.RegisterURLShortenerServiceHandlerFromEndpoint(
		ctx, gwMux, "localhost:50051", dialOpts,
	); err != nil {
		log.Fatalf("Failed to register grpc-gateway: %v", err)
	}

	router := gin.Default()

	// Prometheus metrics — exposes GET /metrics
	p := ginprometheus.NewPrometheus("gin")
	p.Use(router)

	// grpc-gateway handles JSON API
	router.POST("/shorted", handleCreateURL(gwMux))
	router.GET("/info/:path", handleGetURL(gwMux))

	// Swagger docs
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
	})
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Redirect
	router.GET("/:path", handleRedirect(grpcClient))

	return &http.Server{
		Addr:    ":8080",
		Handler: router.Handler(),
	}
}

// handleCreateURL godoc
// @Summary      Create shortened URL
// @Description  Create a shortened URL from the original URL via grpc-gateway
// @Tags         shorten
// @Accept       json
// @Produce      json
// @Param        body body ShortenRequest true "Original URL"
// @Success      200 {object} URLResponse
// @Failure      500 {object} ErrorResponse
// @Router       /shorted [post]
func handleCreateURL(gwMux *runtime.ServeMux) gin.HandlerFunc {
	return gin.WrapH(gwMux)
}

// handleGetURL godoc
// @Summary      Get URL info
// @Description  Get info of a shortened URL (original URL, clicks count)
// @Tags         info
// @Produce      json
// @Param        path path string true "Short URL path" example("abc123")
// @Success      200 {object} URLResponse
// @Failure      404 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /info/{path} [get]
func handleGetURL(gwMux *runtime.ServeMux) gin.HandlerFunc {
	return gin.WrapH(gwMux)
}

// handleRedirect godoc
// @Summary      Redirect to original URL
// @Description  Redirect (HTTP 301) to the original URL using the short path
// @Tags         redirect
// @Param        path path string true "Short URL path" example("abc123")
// @Success      301 "Moved Permanently"
// @Failure      404 {object} ErrorResponse
// @Router       /{path} [get]
func handleRedirect(grpcClient *client.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		res, err := grpcClient.CC.GetURL(c.Request.Context(), &urlshortenerpb.GetURLRequest{URL: path})
		if err != nil || res.GetStatus() == "Failed" {
			c.JSON(http.StatusNotFound, gin.H{"message": "URL not found"})
			return
		}
		c.Redirect(http.StatusMovedPermanently, res.GetUrl().GetOriginalURL())
	}
}

// @title URL Shortener API
// @description This is a server for URL Shortener API.
// @version 1.5.0
// @BasePath /
// @schemes http https
// @host localhost:8080
// @securityDefinitions.basic  BasicAuth
func RunServer() {
	// Context that cancels on SIGINT / SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Init storage
	redisStore := storage.Redis{}
	redisStore.Init()
	sqlStore := storage.SQLStore{}
	sqlStore.Init()

	// gRPC server
	grpcServer := server.NewGRPCServer(&redisStore, &sqlStore)

	// gRPC client (used by HTTP gateway + redirect handler)
	grpcClient, cleanup, err := client.NewClient("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer cleanup()

	// HTTP server (Gin + grpc-gateway + Swagger + Prometheus)
	httpSrv := newHTTPServer(ctx, grpcClient)

	// Start servers with errgroup
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error { return grpcServer.ListenAndServe() })

	g.Go(func() error {
		log.Println("HTTP gateway starting on :8080")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	// Graceful shutdown
	g.Go(func() error {
		<-gCtx.Done()
		log.Println("Shutting down servers...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP shutdown error: %v", err)
		}

		grpcShutdownCtx, grpcCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer grpcCancel()
		grpcServer.Shutdown(grpcShutdownCtx)

		log.Println("All servers stopped gracefully")
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}

func main() {
	RunServer()
}
