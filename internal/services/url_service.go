package services

import (
	"URLShortener/internal/models"
	"URLShortener/internal/repositories"
	"URLShortener/pkg/cache"
	"URLShortener/pkg/database"
	"encoding/json"
	"github.com/google/uuid"
	"log"
	"net/http"
	"time"
)

var prefix = "http://localhost:8080/"

type UrlService struct {
	urlRepository *repositories.ShortenURLRepository
	cache         *cache.Redis
}

func NewUrlService(db *database.DB, cache *cache.Redis) *UrlService {
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
	urlShorten.SetClicks(0)
	log.Println(urlShorten)

	// Save to database
	err := us.urlRepository.Save(urlShorten)
	if err != nil {
		log.Printf("Error while saving url: %v", err)
		return err
	}

	return nil
}

func (us *UrlService) GetUrl(id string) (*models.ShortenedUrl, error) {
	// Get from cache
	data, e := us.cache.Get(id)

	if e == nil && data != "" {
		log.Printf("Get from cache: %v", data)
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
	log.Println("Get from database")
	findByID, err := us.urlRepository.FindByID(id)
	if err != nil {
		log.Printf("Error while getting FindByID: %v", err)
		return &models.ShortenedUrl{}, err
	}
	// Convert to json
	byteData, e := json.Marshal(findByID)
	if e != nil {
		log.Printf("Error while marshalling: %v", e)
	}
	// Save to cache
	err = us.cache.Set(id, string(byteData), 72*time.Hour)
	if err != nil {
		log.Printf("Error while setting cache: %v", err)
	}
	return findByID, nil
}

func (us *UrlService) IsValidUrl(urlChecking string) bool {
	response, err := http.Head(urlChecking)
	if err != nil {
		return false
	}

	if response.StatusCode != http.StatusOK {
		return false
	}

	return true
}

func (us *UrlService) DeleteUrl(id string) error {
	var shortenedUrl models.ShortenedUrl

	// Get from cache
	data, err := us.cache.Get(id)
	if err == nil && data != "" {
		log.Println("Get from cache")
		err = json.Unmarshal([]byte(data), &shortenedUrl)
		if err != nil {
			return err
		}
		// Delete from cache
		err = us.cache.Delete(id)
		if err != nil {
			log.Printf("Error while deleting cache: %v", err)
			return err
		}
	} else {
		// Get from database
		findById, e := us.urlRepository.FindByID(id)
		log.Println("Get from database")
		if e != nil {
			log.Printf("Error while getting url: %v", err)
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
		log.Printf("Error while deleting url: %v", err)
		return err
	}

	return nil
}

func (us *UrlService) UpdateUrl(url *models.ShortenedUrl) error {
	err := us.urlRepository.UpdateClicks(url)
	if err != nil {
		log.Printf("Error while updating url: %v", err)
		return err
	}
	return nil
}
