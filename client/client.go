package client

import (
	"context"
	"fmt"
	"log"

	"github.com/ducanng/URLShortener/proto/urlshortenerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	CC urlshortenerpb.URLShortenerServiceClient
}

// NewClient creates a gRPC client connected to the given target.
// Returns the client, a cleanup function to close the connection, and any error.
func NewClient(target string) (*Client, func(), error) {
	cc, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	c := &Client{CC: urlshortenerpb.NewURLShortenerServiceClient(cc)}
	cleanup := func() { cc.Close() }
	return c, cleanup, nil
}

//goland:noinspection ALL
func (client *Client) CallCreateURL(ctx context.Context, url string) (*urlshortenerpb.Response, error) {
	// create gRPC request
	req := &urlshortenerpb.CreateURLRequest{
		Url: url,
	}
	// call gRPC server
	res, err := client.CC.CreateURL(ctx, req)
	if err != nil {
		log.Printf("Error while calling CreateURL RPC: %v", err)
		return nil, err
	}
	return res, nil
}

//goland:noinspection ALL
func (client *Client) CallGetURL(ctx context.Context, url string) (*urlshortenerpb.Response, error) {
	// create gRPC request
	req := &urlshortenerpb.GetURLRequest{
		URL: url,
	}
	// call gRPC server
	res, err := client.CC.GetURL(ctx, req)
	if err != nil {
		log.Printf("Error while calling GetURL RPC: %v", err)
		return nil, err
	}
	return res, nil
}
