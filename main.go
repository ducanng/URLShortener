package main

import (
	"URLShortener-gRPC-Swagger/client"
	_ "URLShortener-gRPC-Swagger/docs"
	"URLShortener-gRPC-Swagger/model"
	"URLShortener-gRPC-Swagger/proto/urlshortenerpb"
	"URLShortener-gRPC-Swagger/server"
	"URLShortener-gRPC-Swagger/storage"
	"google.golang.org/grpc/credentials/insecure"
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
	urlshortenerpb.CreateURLResponse
}
type Client struct {
	CC client.Client
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
	// Init sql
	sqlStore := storage.SQLStore{}
	sqlStore.Init()
	var entry model.URLEntry
	// register server
	urlshortenerpb.RegisterURLShortenerServiceServer(s, &server.Server{Redis: &redis, DB: &sqlStore, UrlEntry: entry})

	go func() {
		log.Println("Starting server ...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
			return
		}
		wg.Done()
	}()

	go func() {
		// create gin engine
		router := gin.Default()
		// create routes
		var c Client
		cc, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("err while dial %v", err)
		}
		defer cc.Close()
		c.CC.CC = urlshortenerpb.NewURLShortenerServiceClient(cc)
		router.POST("/shorted", c.ShortenedURL())
		router.GET("/:id", c.Redirect())
		// docs route
		router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		// run the Gin server
		router.Run()
		wg.Done()
	}()
	wg.Wait()
}

func main() {
	RunServer()
	//test grpc

}

// ShortenedURL
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
func (cli *Client) ShortenedURL() gin.HandlerFunc {
	//check originalURL is valid
	return func(c *gin.Context) {
		var body shortenBody
		if err := c.BindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, response{
				CreateURLResponse: urlshortenerpb.CreateURLResponse{
					Status:  "Invalid URL",
					Message: err.Error(),
					Url: &urlshortenerpb.ShortenedURL{
						OriginalURL:  body.OriginalURL,
						ShortenedURL: "",
						Clicks:       0,
					},
				},
			})
			return
		}
		_, urlErr := url.ParseRequestURI(body.OriginalURL)
		if urlErr != nil {
			c.JSON(http.StatusBadRequest, response{
				CreateURLResponse: urlshortenerpb.CreateURLResponse{
					Status:  "Invalid URL",
					Message: urlErr.Error(),
					Url: &urlshortenerpb.ShortenedURL{
						OriginalURL:  body.OriginalURL,
						ShortenedURL: "",
						Clicks:       0,
					},
				},
			})
			return
		}
		res := cli.CC.CallCreateURL(body.OriginalURL)
		c.JSON(http.StatusOK, response{
			CreateURLResponse: urlshortenerpb.CreateURLResponse{
				Status:  res.GetStatus(),
				Message: res.Message,
				Url: &urlshortenerpb.ShortenedURL{
					OriginalURL:  res.GetUrl().GetOriginalURL(),
					ShortenedURL: res.GetUrl().GetShortenedURL(),
					Clicks:       res.GetUrl().GetClicks(),
				},
			},
		})
	}
	// return response
	//c.JSON(http.StatusOK, response{message{Message: "Success"}, *res.GetUrl()})
}

// Redirect
// @Summary Redirect to original URL
// @ID redirect-url
// @Description Redirect to original URL
// @Tags redirect
// @Accept  json
// @Produce  json
// @Param id path string true "Shortened URL"
// @Router /{id} [get]
func (cli *Client) Redirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, message{Message: "Invalid ID"})
			return
		}
		res := cli.CC.CallGetURL(id)
		// redirect to original url
		c.Redirect(http.StatusMovedPermanently, res.GetUrl().GetOriginalURL())
	}
}
