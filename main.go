package main

import (
	"URLShortener-gRPC-Swagger/proto/urlshortenerpb"
	"context"
	"log"
	"net/http"
	"net/url"

	_ "URLShortener-gRPC-Swagger/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type shortenBody struct {
	OriginalURL string `json:"original_url"`
}
type message struct {
	Message string `json:"message"`
}
type response struct {
	message
	urlshortenerpb.ShortenedURL
}

// @title URL Shortener API
// @description This is a server for URL Shortener API.
// @version 1.0.0
// @BasePath /
// @schemes http https
// @host localhost:8080
// @securityDefinitions.basic  BasicAuth
func main() {
	// create gin engine
	router := gin.Default()
	// create routes
	router.POST("/shorted", ShortenedURL)
	router.GET("/:id", Redirect)

	// docs route
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// run the Gin server
	router.Run()
}

// @Summary Shorten URL
// @ID shorten-url
// @Description Create a shortened URL
// @Tags shorten
// @Accept  json
// @Produce  json
// @Param shorten body shortenBody true "Original URL"
// @Success 200 {object} response
// @Failure 400 {object} message
// @Router /shorted [post]
func ShortenedURL(c *gin.Context) {
	//check originalURL is valid
	var body shortenBody
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, message{Message: "Invalid JSON " + err.Error()})
		return
	}
	_, urlErr := url.ParseRequestURI(body.OriginalURL)
	if urlErr != nil {
		c.JSON(http.StatusBadRequest, message{Message: "Invalid URL " + urlErr.Error()})
		return
	}
	// connect to gRPC server
	conn, err := grpc.Dial(":50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Could not connect: %v", err)
	}
	defer conn.Close()
	// create gRPC client
	client := urlshortenerpb.NewURLShortenerServiceClient(conn)
	// create gRPC request
	req := &urlshortenerpb.CreateURLRequest{
		Url: body.OriginalURL,
	}
	// call gRPC server
	res, err := client.CreateURL(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling CreateURL RPC: %v", err)
	}
	// return response
	c.JSON(http.StatusOK, response{message{Message: "Success"}, *res.GetUrl()})
}

var prefixLink string = "http://localhost:8080/"

// @Summary Redirect to original URL
// @ID redirect-url
// @Description Redirect to original URL
// @Tags redirect
// @Accept  json
// @Produce  json
// @Param id path string true "Shortened URL"
// @Router /{id} [get]
func Redirect(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, message{Message: "Invalid ID"})
		return
	}
	// connect to gRPC server
	conn, err := grpc.Dial(":50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Could not connect: %v", err)
	}
	defer conn.Close()
	// create gRPC client
	client := urlshortenerpb.NewURLShortenerServiceClient(conn)
	// create gRPC request
	req := &urlshortenerpb.GetURLRequest{
		URL: prefixLink + id,
	}
	// call gRPC server
	res, err := client.GetURL(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling CreateURL RPC: %v", err)
	}
	// redirect to original url
	c.Redirect(http.StatusMovedPermanently, res.GetUrl().GetOriginalURL())
}
