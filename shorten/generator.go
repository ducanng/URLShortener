package shorten

import (
	"URLShortener-gRPC-Swagger/shorten/base62"
	"math/rand"
	"time"
)

// GenerateShortLink generates a short link
func GenerateShortLink() string {
	id := rand.New(rand.NewSource(time.Now().UnixNano()))
	shortPath := base62.Encode(id.Uint64())
	return shortPath[:5]
}
