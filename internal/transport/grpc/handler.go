package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/ducanng/URLShortener/internal/domain"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/service/urlservice"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// prefixLink is the base URL prepended to the generated short path when
// building the response ShortenedURL field.
const prefixLink = "http://localhost:8080/"

// service is the thin gRPC adapter. It converts proto ↔ domain types and
// delegates all business decisions to the injected URLService.
type service struct {
	*logger.Logger
	urlshortenerpb.URLShortenerServiceServer
	svc *urlservice.URLService
}

// CreateURL decodes the proto request → domain params, delegates to
// URLService, then encodes the domain result → proto response.
//
//goland:noinspection GoUnreachableCode
func (s *service) CreateURL(ctx context.Context, req *urlshortenerpb.CreateURLRequest) (*urlshortenerpb.Response, error) {
	log := s.WithContext(ctx)

	var explicit *time.Time
	if req.GetExpiresAt() != nil {
		t := req.GetExpiresAt().AsTime()
		explicit = &t
	}

	entry, err := s.svc.CreateURL(ctx, urlservice.CreateParams{
		OriginalURL:    req.GetUrl(),
		NoExpire:       req.GetNoExpire(),
		ExplicitExpiry: explicit,
	}, time.Now())
	if err != nil {
		if errors.Is(err, urlservice.ErrNoExpireConflict) || errors.Is(err, urlservice.ErrExpiresAtPast) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		log.Errorf("create URL: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create URL: %v", err)
	}

	return &urlshortenerpb.Response{
		Message: "Create short url",
		Status:  "Success",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  entry.OriginalURL,
			ShortenedURL: prefixLink + entry.ShortedURL,
			Clicks:       entry.Clicks,
			ExpiresAt:    expiresAtPB(entry.ExpiresAt),
		},
	}, nil
}

// GetURL decodes the proto request, delegates to URLService (cache-aside
// + expiry check), then encodes the result.
//
//goland:noinspection ALL
func (s *service) GetURL(ctx context.Context, req *urlshortenerpb.GetURLRequest) (*urlshortenerpb.Response, error) {
	log := s.WithContext(ctx)

	entry, err := s.svc.GetURL(ctx, req.GetURL())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			return nil, status.Errorf(codes.NotFound, "URL %q not found", req.GetURL())
		case errors.Is(err, domain.ErrExpired):
			return nil, status.Errorf(codes.FailedPrecondition, "URL %q has expired", req.GetURL())
		default:
			log.Errorf("get URL: %v", err)
			return nil, status.Errorf(codes.Internal, "failed to get URL: %v", err)
		}
	}

	return &urlshortenerpb.Response{
		Message: "Get short url",
		Status:  "Success",
		Url: &urlshortenerpb.ShortenedURL{
			OriginalURL:  entry.OriginalURL,
			ShortenedURL: entry.ShortedURL,
			Clicks:       entry.Clicks,
			ExpiresAt:    expiresAtPB(entry.ExpiresAt),
		},
	}, nil
}

// expiresAtPB converts a *time.Time to a proto Timestamp.
// Returns nil for entries that never expire (nil input).
func expiresAtPB(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
