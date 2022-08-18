package server

import (
	"URLShortener-gRPC-Swagger/model"
	"URLShortener-gRPC-Swagger/proto/urlshortenerpb"
	"URLShortener-gRPC-Swagger/shorten"
	"URLShortener-gRPC-Swagger/storage"
	"context"
	"log"
)

var prefixLink string = "http://localhost:8080/"

type Server struct {
	urlshortenerpb.URLShortenerServiceServer
	Redis *storage.Redis
}

func (s *Server) CreateURL(ctx context.Context, req *urlshortenerpb.CreateURLRequest) (*urlshortenerpb.CreateURLResponse, error) {
	//log.Printf("CreateURL call...")
	// create short url
	url := model.URLEntry{
		OriginalURL: req.GetUrl(),
		ShortedURL:  prefixLink + shorten.GenerateShortLink(),
	}
	// save to redis
	_, err := s.Redis.Set(url.ShortedURL, url.OriginalURL)
	// return response
	if err != nil {
		log.Fatalf("Error when set key-value to redis: %v", err)
		return &urlshortenerpb.CreateURLResponse{
			Message: err.Error(),
			Status:  "Error",
		}, err
	}
	return &urlshortenerpb.CreateURLResponse{
		Message: "URL shortened",
		Status:  "Ok",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  req.Url,
			ShortenedURL: url.ShortedURL,
		},
	}, nil
}

func (s *Server) GetURL(ctx context.Context, req *urlshortenerpb.GetURLRequest) (*urlshortenerpb.GetURLResponse, error) {
	//log.Printf("GetURL call...")
	// get original url from redis
	originalURL, err := s.Redis.Get(req.GetURL())

	if err != nil {
		log.Fatalf("Error when get key-value from redis: %v", err)
	}
	// return response
	return &urlshortenerpb.GetURLResponse{
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  originalURL,
			ShortenedURL: req.GetURL(),
		},
	}, nil
}

// func startgRPCServer() {
// 	log.Println("Server is running...")
// 	// create server grpc
// 	s := grpc.NewServer()
// 	lis, err := net.Listen("tcp", ":50051")
// 	if err != nil {
// 		log.Fatalf("Failed to listen: %v", err)
// 	}
// 	// Init redis
// 	redis := storage.Redis{}
// 	redis.Init()
// 	// register server
// 	urlshortenerpb.RegisterURLShortenerServiceServer(s, &Server{redis: &redis})

// 	log.Println("Starting server ...")
// 	if err := s.Serve(lis); err != nil {
// 		log.Fatalf("failed to serve: %v", err)
// 		return
// 	}
// }

// func main() {
// 	startgRPCServer()
// }
