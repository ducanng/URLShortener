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
	ShortedURL  string `json:"shorted_url"`
	OriginalURL string `json:"original_url"`
}
type message struct {
	Message string `json:"message"`
}
type response struct {
	urlshortenerpb.CreateURLResponse
}

type Client struct {
	CC   client.Client
	DB   *storage.SQLStore
	body shortenBody
}

// @title URL Shortener API
// @description This is a server for URL Shortener API.
// @version 1.4.0
// @BasePath /
// @schemes http https
// @host localhost:8080
// @securityDefinitions.basic  BasicAuth
var wg = sync.WaitGroup{}

func RunServer() {
	wg.Add(2)

	// Init redis
	redis := storage.Redis{}
	redis.Init()
	// Init sql
	sqlStore := storage.SQLStore{}
	sqlStore.Init()
	var entry model.URLEntry

	log.Println("Server is running...")
	// create server grpc
	s := grpc.NewServer()
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
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
	// create server http
	go func() {

		var c Client
		cc, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("err while dial %v", err)
		}
		defer cc.Close()
		c.CC.CC = urlshortenerpb.NewURLShortenerServiceClient(cc)

		// create gin engine
		router := gin.Default()
		// create routes
		router.GET("/getinfo/:path", c.GetInfoURL())
		router.POST("/shorted", c.ShortenedURL())
		router.GET("/:path", c.Redirect())
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
		if err := c.BindJSON(&cli.body.OriginalURL); err != nil {
			c.JSON(http.StatusBadRequest, response{
				CreateURLResponse: urlshortenerpb.CreateURLResponse{
					Status:  "Invalid URL",
					Message: err.Error(),
				},
			})
			return
		}
		_, urlErr := url.ParseRequestURI(cli.body.OriginalURL)
		if urlErr != nil {
			c.JSON(http.StatusBadRequest, response{
				CreateURLResponse: urlshortenerpb.CreateURLResponse{
					Status:  "Invalid URL",
					Message: urlErr.Error(),
				},
			})
			return
		}
		res := cli.CC.CallCreateURL(cli.body.OriginalURL)
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
// @Param path path string true "Shortened URL"
// @Router /{path} [get]
func (cli *Client) Redirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		if path == "" {
			c.JSON(http.StatusBadRequest, message{Message: "Invalid path"})
			return
		}
		res := cli.CC.CallGetURL(path)
		// redirect to original url
		click := res.GetUrl().GetClicks() + 1
		err := cli.DB.UpdateClicks(path, click)
		if err != nil {
			return
		}
		c.Redirect(http.StatusMovedPermanently, res.GetUrl().GetOriginalURL())
	}
}

// GetInfoURL
// @Summary Get info of URL
// @ID get-info-url
// @Description Get info of URL
// @Tags getinfo
// @Accept  json
// @Produce  json
// @Param path path string true "Info URL"
// @Success 200 {object} response
// @Failure 400 {object} message
// @Router /getinfo/{path} [get]
func (cli *Client) GetInfoURL() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		if path == "" {
			c.JSON(http.StatusBadRequest, message{Message: "Invalid ID"})
			return
		}
		res := cli.CC.CallGetURL(path)
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
}
