package services

import (
	"URLShortener/internal/models"
	"URLShortener/internal/repositories"
	"URLShortener/pkg/cache"
	"URLShortener/pkg/database"
	"encoding/json"
	"github.com/google/uuid"
	"log"
	"net/url"
	"time"
)

var prefix = "http://localhost:8080/"

type UrlService struct {
	urlRepository *repositories.ShortenURLRepository
	cache         cache.Redis
}

func NewUrlService(db database.DB, cache cache.Redis) *UrlService {
	return &UrlService{
		urlRepository: repositories.NewShortenURLRepository(db, cache),
		cache:         cache,
	}
}

func (us *UrlService) CreateUrl(urlShorten *models.ShortenedUrl) error {
	// Generate id
	id := uuid.New().String()[:8]
	urlShorten.SetId(id)
	urlShorten.SetShortUrl(prefix + id)
	urlShorten.SetCreatedAt(time.Now())
	log.Println(urlShorten)

	// Save to database
	err := us.urlRepository.Save(urlShorten)
	if err != nil {
		log.Fatalf("Error while saving url: %v", err)
		return err
	}

	return nil
}

func (us *UrlService) GetUrl(id string) (*models.ShortenedUrl, error) {
	// Get from cache
	data, e := us.cache.Get(id)
	if e == nil && data != "" {
		var shortenedUrl models.ShortenedUrl
		e = json.Unmarshal([]byte(data), &shortenedUrl)
		if e != nil {
			return nil, e
		}
		// Update cache
		us.cache.Expire(id, 72*time.Hour)
		return &shortenedUrl, nil
	}

	// Get from database
	findByID, err := us.urlRepository.FindByID(id)
	if err != nil {
		log.Fatalf("Error while getting FindByID: %v", err)
		return &models.ShortenedUrl{}, err
	}
	// Convert to json
	byteData, e := json.Marshal(findByID)
	if e != nil {
		return nil, e
	}
	// Save to cache
	err = us.cache.Set(id, string(byteData), 72*time.Hour)
	if err != nil {
		return nil, err
	}

	return findByID, nil
}

func (us *UrlService) IsValidUrl(urlChecking string) bool {
	if urlChecking == "" {
		return false
	}
	u, err := url.Parse(urlChecking)
	if err != nil {
		return false
	}
	if u.Scheme == "" || u.Host == "" {
		return false
	}
	return true
}

func (us *UrlService) DeleteUrl(id string) error {
	var shortenedUrl models.ShortenedUrl

	// Get from cache
	data, err := us.cache.Get(id)
	if err == nil && data != "" {
		err = json.Unmarshal([]byte(data), &shortenedUrl)
		if err != nil {
			return err
		}
		// Delete from cache
		err = us.cache.Delete(id)
		if err != nil {
			log.Fatalf("Error while deleting cache: %v", err)
			return err
		}
	} else {
		// Get from database
		findById, e := us.urlRepository.FindByID(id)
		if e != nil {
			log.Fatalf("Error while getting url: %v", err)
			return e
		}
		if findById == nil {
			log.Println("Url not found")
			return nil
		}
		shortenedUrl = *findById
	}

	// Delete from database
	err = us.urlRepository.Delete(&shortenedUrl)
	if err != nil {
		log.Fatalf("Error while deleting url: %v", err)
		return err
	}

	return nil
}
