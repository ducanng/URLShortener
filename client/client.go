package client

import (
	"URLShortener-gRPC-Swagger/proto/urlshortenerpb"
	"context"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func CallCreateURL(url string) *urlshortenerpb.CreateURLResponse {
	// create connection
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Error when connect to server: %v", err)
	}
	defer conn.Close()
	// create client
	client := urlshortenerpb.NewURLShortenerServiceClient(conn)
	// create gRPC request
	req := &urlshortenerpb.CreateURLRequest{
		Url: url,
	}
	// call gRPC server
	res, err := client.CreateURL(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling CreateURL RPC: %v", err)
	}
	return res
}

func CallGetURL(url string) *urlshortenerpb.GetURLResponse {
	// create connection
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Error when connect to server: %v", err)
	}
	defer conn.Close()
	// create client
	client := urlshortenerpb.NewURLShortenerServiceClient(conn)
	// create gRPC request
	req := &urlshortenerpb.GetURLRequest{
		URL: url,
	}
	// call gRPC server
	res, err := client.GetURL(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling CreateURL RPC: %v", err)
	}
	return res
}
