package shorten

import (
<<<<<<< HEAD
	"math/rand"
	"time"
	"url-shortener/base62"
)

type URLEntry struct {
	OriginalURL string    `json:"long_url"`
	ShortenURL  string    `json:"short_url"`
	Id          uint64    `json:"id"`
	Clicks      uint      `json:"click"`
	CreateAt    time.Time `json:"create_at"`
	UpdateAt    time.Time `json:"update_at"`
}

=======
	"URLShortener-gRPC-Swagger/shorten/base62"
	"math/rand"
	"time"
)

// GenerateShortLink generates a short link
>>>>>>> 65b11c0454d2c48ca6981909bce446dbcc75b0fa
func GenerateShortLink() string {
	id := rand.New(rand.NewSource(time.Now().UnixNano()))
	shortPath := base62.Encode(id.Uint64())
	return shortPath[:5]
}
