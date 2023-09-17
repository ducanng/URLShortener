package shorten

import (
	"math/rand"
	"time"

	base62 "github.com/alextanhongpin/base62"
)

// GenerateShortLink generates a short link
func GenerateShortLink() string {
	id := rand.New(rand.NewSource(time.Now().UnixNano()))
	shortPath := base62.Encode(id.Uint64())
	return shortPath[:5]
}
