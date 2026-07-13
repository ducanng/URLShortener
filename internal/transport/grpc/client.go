package grpc

import (
	"context"
	"fmt"

	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a thin wrapper around the generated gRPC stub plus the process
// logger. The embedded *Logger keeps log call sites short (c.Errorf vs
// c.log.Errorf).
type Client struct {
	*logger.Logger
	CC urlshortenerpb.URLShortenerServiceClient
}

// NewClient dials target with insecure credentials (loopback or in-cluster
// only) and returns the wrapper, a cleanup function to close the connection,
// and any dial error.
func NewClient(target string, log *logger.Logger) (*Client, func(), error) {
	cc, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("create gRPC client: %w", err)
	}
	c := &Client{
		Logger: log,
		CC:     urlshortenerpb.NewURLShortenerServiceClient(cc),
	}
	cleanup := func() { _ = cc.Close() }
	return c, cleanup, nil
}

// CallCreateURL invokes the CreateURL RPC. Errors are logged with the
// caller's trace_id (via WithContext) so a failed call can be correlated
// in Loki.
func (c *Client) CallCreateURL(ctx context.Context, url string) (*urlshortenerpb.Response, error) {
	res, err := c.CC.CreateURL(ctx, &urlshortenerpb.CreateURLRequest{Url: url})
	if err != nil {
		c.WithContext(ctx).Errorf("calling CreateURL RPC: %v", err)
		return nil, err
	}
	return res, nil
}

// CallGetURL invokes the GetURL RPC. Same logging contract as CallCreateURL.
func (c *Client) CallGetURL(ctx context.Context, url string) (*urlshortenerpb.Response, error) {
	res, err := c.CC.GetURL(ctx, &urlshortenerpb.GetURLRequest{URL: url})
	if err != nil {
		c.WithContext(ctx).Errorf("calling GetURL RPC: %v", err)
		return nil, err
	}
	return res, nil
}
