package main

import (
	"URLShortener-gRPC-Swagger/client"
	_ "URLShortener-gRPC-Swagger/docs"
	"URLShortener-gRPC-Swagger/proto/urlshortenerpb"
	"URLShortener-gRPC-Swagger/server"
	"URLShortener-gRPC-Swagger/storage"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"google.golang.org/grpc"

	"github.com/gin-gonic/gin"
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
var wg = sync.WaitGroup{}

func RunServer() {
	wg.Add(2)
	log.Println("Server is running...")
	// create server grpc
	s := grpc.NewServer()
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	// Init redis
	redis := storage.Redis{}
	redis.Init()
	// register server
	urlshortenerpb.RegisterURLShortenerServiceServer(s, &server.Server{Redis: &redis})

	go func() {
		log.Println("Starting server ...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
			return
		}
		wg.Done()
	}()
	// create gin engine
	router := gin.Default()
	// create routes
	router.POST("/shorted", ShortenedURL)
	router.GET("/:id", Redirect)
	// docs route
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	go func() {
		// run the Gin server
		router.Run()
		wg.Done()
	}()
	wg.Wait()
}

func main() {
	RunServer()
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
	res := client.CallCreateURL(body.OriginalURL)
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
	url := prefixLink + id
	res := client.CallGetURL(url)
	// redirect to original url
	c.Redirect(http.StatusMovedPermanently, res.GetUrl().GetOriginalURL())
}
