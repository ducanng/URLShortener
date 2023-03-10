package repositories

import (
	"URLShortener/internal/models"
	"URLShortener/pkg/cache"
	"URLShortener/pkg/database"
	"encoding/json"
	"log"
	"time"
	_ "time"
)

type ShortenedUrlRepository interface {
	FindByID(shortUrl string) (*models.ShortenedUrl, error)
	Save(shortUrl *models.ShortenedUrl) error
	Delete(shortUrl *models.ShortenedUrl) error
}

type ShortenURLRepository struct {
	db    database.DB
	cache cache.Redis
}

func NewShortenURLRepository(db database.DB, cache cache.Redis) *ShortenURLRepository {
	return &ShortenURLRepository{
		db:    db,
		cache: cache,
	}
}

func (s *ShortenURLRepository) FindByID(shortUrl string) (*models.ShortenedUrl, error) {
	res := s.db.QueryRow("SELECT * FROM shortened_urls WHERE id = ?", shortUrl)
	var shortenedUrl models.ShortenedUrl
	err := res.Scan(&shortenedUrl.Id, &shortenedUrl.OriginalUrl, &shortenedUrl.ShortUrl, &shortenedUrl.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &shortenedUrl, nil
}

func (s *ShortenURLRepository) Save(shortUrl *models.ShortenedUrl) error {
	_, err := s.db.Exec("INSERT INTO shortened_urls (id, original_url, short_url, created_at) VALUES (?, ?, ?, ?)",
		shortUrl.GetId(), shortUrl.GetOriginalUrl(), shortUrl.GetShortUrl(), shortUrl.GetCreatedAt())
	if err != nil {
		return err
	}
	// convert to string
	byteData, e := json.Marshal(shortUrl)
	if e != nil {
		return e
	}
	data := string(byteData)
	// Set cache
	err = s.cache.Set(shortUrl.GetId(), data, 72*time.Hour)
	if err != nil {
		log.Printf("Error while setting cache: %v", err)
		return err
	}

	return nil
}

func (s *ShortenURLRepository) Delete(shortUrl *models.ShortenedUrl) error {
	_, err := s.db.Exec("DELETE FROM shortened_urls WHERE id = ?", shortUrl.GetId())
	if err != nil {
		return err
	}
	return nil
}
