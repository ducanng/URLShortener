package main

import (
	"URLShortener-gRPC-Swagger/client"
	_ "URLShortener-gRPC-Swagger/docs"
	"URLShortener-gRPC-Swagger/proto/urlshortenerpb"
	"URLShortener-gRPC-Swagger/server"
	"URLShortener-gRPC-Swagger/storage"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"

	"google.golang.org/grpc/credentials/insecure"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"google.golang.org/grpc"

	"github.com/gin-gonic/gin"
)

//goland:noinspection ALL
type shortenBody struct {
	OriginalURL string `json:"original_url"`
}
type message struct {
	Message string `json:"message"`
}
type JSONReturn struct {
	OriginalURL  string `json:"original_url"`
	ShortenedURL string `json:"shortened_url"`
	Clicks       int32  `json:"clicks"`
}
type GRPCReturn struct {
	*urlshortenerpb.Response
}
type Response struct {
	Reply message    `json:"message"`
	GRPC  GRPCReturn `json:"grpc"`
	JSON  JSONReturn `json:"json"`
}

type IClient struct {
	CC   client.Client
	body shortenBody
}

// @title URL Shortener API
// @description This is a server for URL Shortener API.
// @version 1.5.0
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

	log.Println("Server is running...")
	// create server grpc
	s := grpc.NewServer()
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	// register server
	urlshortenerpb.RegisterURLShortenerServiceServer(s, &server.Server{Redis: &redis, DB: &sqlStore})

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

		var c IClient
		cc, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("err while dial %v", err)
		}
		defer cc.Close()
		c.CC.CC = urlshortenerpb.NewURLShortenerServiceClient(cc)

		// create gin engine
		router := gin.Default()
		// create routes
		router.GET("/info/:path", c.GetInfoURL())
		router.POST("/shorted", c.ShortenedURL())
		router.GET("/:path", c.Redirect()) //sqlStore, redis))
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
// @Description Create a shortened URL, choose between json or grpc response, default is json, if you want grpc, set return_type to grpc
// @Tags shorten
// @Accept  json
// @Produce  json
// @Param original-url body shortenBody true "Original URL"
// @Param return-type query string false "Return type" Enums(json, grpc) default(json)
// @Success 200 {object} Response
// @Failure 400 {object} message
// @Router /shorted [post]
func (cli *IClient) ShortenedURL() gin.HandlerFunc {
	//check originalURL is valid
	return func(c *gin.Context) {
		t := c.Query("return-type")
		fmt.Println(t)
		if err := c.BindJSON(&cli.body); err != nil {
			c.JSON(http.StatusBadRequest, message{Message: "Invalid body"})
			return
		}
		_, urlErr := url.ParseRequestURI(cli.body.OriginalURL)
		if urlErr != nil {
			c.JSON(http.StatusBadRequest, message{Message: "Invalid URL"})
			return
		}
		res := cli.CC.CallCreateURL(cli.body.OriginalURL)

		if t == "grpc" {
			c.JSON(http.StatusOK, Response{
				GRPC: GRPCReturn{
					res,
				},
			})
			return
		} else {
			c.JSON(http.StatusOK, Response{
				Reply: message{Message: res.GetMessage()},
				JSON: JSONReturn{
					OriginalURL:  res.GetUrl().GetOriginalURL(),
					ShortenedURL: res.GetUrl().GetShortenedURL(),
					Clicks:       res.GetUrl().GetClicks(),
				},
			})
			return
		}
	}
	// return response
	//c.JSON(http.StatusOK, response{message{Message: "Success"}, *res.GetUrl()})
}

// Redirect
// @Summary Redirect to original URL
// @ID redirect-url
// @Description Redirect to original URL
// @Tags redirect
// @Param pathShort path string true "Shortened URL"
// @Failure 400 {object} message
// @Router /{pathShort} [get]
func (cli *IClient) Redirect( /*sqlStore storage.SQLStore, redis storage.Redis*/ ) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		if path == "" {
			c.JSON(http.StatusBadRequest, message{Message: "Invalid path"})
			return
		}
		res := cli.CC.CallGetURL(path)
		if res.GetStatus() == "Failed" {
			c.JSON(http.StatusNotFound, message{Message: "URL not found"})
			return
		}

		//clicks := res.GetUrl().GetClicks() + 1
		//fmt.Println(clicks)
		//sqlStore.UpdateClicks(path, clicks)
		//entry := model.URLEntry{
		//	Id:          int64(base62.Decode(path)),
		//	OriginalURL: res.GetUrl().GetOriginalURL(),
		//	ShortedURL:  res.GetUrl().GetShortenedURL(),
		//	Clicks:      clicks,
		//}
		//redis.Set(entry)
		// redirect to original url
		c.Redirect(http.StatusMovedPermanently, res.GetUrl().GetOriginalURL())
	}
}

// GetInfoURL
// @Summary Get info of URL
// @ID get-info-url
// @Description Get info of URL, choose between json or grpc response, default is json, if you want grpc, set return_type to grpc
// @Tags getinfo
// @Accept  json
// @Produce  json
// @Param pathShort path string true "Info URL"
// @Param return-type query string false "Return type" Enums(json, grpc) default(json)
// @Success 200 {object} Response
// @Failure 400 {object} message
// @Router /info/{pathShort} [get]
func (cli *IClient) GetInfoURL() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := c.Query("return-type")
		path := c.Param("path")
		if path == "" {
			c.JSON(http.StatusBadRequest, message{Message: "Invalid ID"})
			return
		}
		res := cli.CC.CallGetURL(path)
		if t == "grpc" {
			c.JSON(http.StatusOK, Response{
				GRPC: GRPCReturn{
					res,
				},
			})
		} else {
			c.JSON(http.StatusOK, Response{
				Reply: message{Message: res.GetMessage()},
				JSON: JSONReturn{
					OriginalURL:  res.GetUrl().GetOriginalURL(),
					ShortenedURL: res.GetUrl().GetShortenedURL(),
					Clicks:       res.GetUrl().GetClicks(),
				},
			})
		}
	}
}
