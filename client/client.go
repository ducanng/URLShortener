package client

import (
	"URLShortener-gRPC-Swagger/proto/urlshortenerpb"
	"context"
	"log"
)

type Client struct {
	CC urlshortenerpb.URLShortenerServiceClient
}

func (client *Client) CallCreateURL(url string) *urlshortenerpb.CreateURLResponse {
	// create gRPC request
	req := &urlshortenerpb.CreateURLRequest{
		Url: url,
	}
	// call gRPC server
	res, err := client.CC.CreateURL(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling CreateURL RPC: %v", err)
	}
	return res
}

func (client *Client) CallGetURL(url string) *urlshortenerpb.GetURLResponse {
	// create gRPC request
	req := &urlshortenerpb.GetURLRequest{
		URL: url,
	}
	// call gRPC server
	res, err := client.CC.GetURL(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling CreateURL RPC: %v", err)
	}
	return res
}

//func main() {
//	// create client
//	cc, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
//	if err != nil {
//		log.Fatalf("err while dial %v", err)
//	}
//	defer cc.Close()
//	client := &Client{}
//	client.CC = urlshortenerpb.NewURLShortenerServiceClient(cc)
//
//	// call gRPC server
//	//res := client.CallCreateURL("https://github.com/go-redis/redis/issues/739")
//	//log.Printf("CreateURL response: %v", res)
//	resGet := client.CallGetURL("CTT7w")
//	log.Printf("GetURL response: %v", resGet)
//}
