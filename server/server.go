package server

import (
	"URLShortener-gRPC-Swagger/model"
	"URLShortener-gRPC-Swagger/proto/urlshortenerpb"
	"URLShortener-gRPC-Swagger/shorten"
	"URLShortener-gRPC-Swagger/storage"
	"context"
	"log"

	base62 "github.com/alextanhongpin/base62"
)

var prefixLink = "http://localhost:8080/"

type Server struct {
	urlshortenerpb.URLShortenerServiceServer
	Redis *storage.Redis
	DB    *storage.SQLStore
}

//goland:noinspection GoUnreachableCode
func (s *Server) CreateURL(_ context.Context, req *urlshortenerpb.CreateURLRequest) (*urlshortenerpb.Response, error) {
	//log.Printf("CreateURL call...")
	// create short url
	shorPath := shorten.GenerateShortLink()
	url := model.URLEntry{
		Id:          int64(base62.Decode(shorPath)),
		OriginalURL: req.GetUrl(),
		ShortedURL:  shorPath,
		Clicks:      0,
	}
	// save to redis
	err := s.Redis.Set(url)
	// return response
	if err != nil {
		log.Fatalf("Error when set key-value to redis: %v", err)
		return &urlshortenerpb.Response{
			Message: "Error when set key-value to redis",
			Status:  "Failed",
			Url:     nil,
		}, err
	}
	// save to db
	err = s.DB.Save(url)
	if err != nil {
		log.Fatalf("Error when save data to db: %v", err)
		return &urlshortenerpb.Response{
			Message: "Error when save data to database",
			Status:  "Failed",
			Url:     nil,
		}, err
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
func (s *Server) GetURL(_ context.Context, req *urlshortenerpb.GetURLRequest) (*urlshortenerpb.Response, error) {
	//log.Printf("GetURL call...")
	// get data from redis
	url, err := s.Redis.Get(req.GetURL())
	if err != nil {
		//get data from db
		url, err = s.DB.Load(req.GetURL())
		if err != nil {
			log.Fatalf("Error when get data from database: %v", err)
			return &urlshortenerpb.Response{
				Message: "Error when get data from database",
				Status:  "Failed",
				Url:     nil,
			}, err
		}
		err = s.Redis.Set(url)
		if err != nil {
			log.Fatalf("Error when set key-value to redis: %v", err)
			return &urlshortenerpb.Response{
				Message: "Error when set key-value to redis",
				Status:  "Failed",
				Url:     nil,
			}, err
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

//func startgRPCServer() {
//	log.Println("Server is running...")
//	// create server grpc
//	s := grpc.NewServer()
//	lis, err := net.Listen("tcp", ":50051")
//	if err != nil {
//		log.Fatalf("Failed to listen: %v", err)
//	}
//	// Init redis
//	redis := storage.Redis{}
//	redis.Init()
//	// Init db
//	db := storage.SQLStore{}
//	db.Init()
//	// register server
//	urlshortenerpb.RegisterURLShortenerServiceServer(s, &Server{Redis: &redis, DB: &db})
//
//	log.Println("Starting server ...")
//	if err := s.Serve(lis); err != nil {
//		log.Fatalf("failed to serve: %v", err)
//		return
//	}
//}
//
//func main() {
//	startgRPCServer()
//}
