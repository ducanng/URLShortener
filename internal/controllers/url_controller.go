package controllers

import (
	"log"
	"net/http"
	"time"

	"github.com/ducanng/URLShortener/internal/models"
	"github.com/ducanng/URLShortener/internal/services"
	"github.com/ducanng/URLShortener/pkg/cache"
	"github.com/ducanng/URLShortener/pkg/database"
	"github.com/gin-gonic/gin"
)

const historyKeyPrefix = "history:"
const sessionTTL = 30 * 24 * time.Hour

type UrlController struct {
	urlService *services.UrlService
	cache      *cache.Redis
}

func NewController(db *database.DB, cache *cache.Redis) *UrlController {
	return &UrlController{
		urlService: services.NewUrlService(db, cache),
		cache:      cache,
	}
}

func (u *UrlController) Redirect(c *gin.Context) {
	id := c.Param("id")
	log.Println(id)

	shortenedUrl, err := u.urlService.GetUrl(id)
	if err != nil {
		_ = c.AbortWithError(http.StatusNotFound, err)
		return
	}
	shortenedUrl.SetClicks(shortenedUrl.GetClicks() + 1)
	err = u.urlService.UpdateUrl(shortenedUrl)
	c.Redirect(http.StatusMovedPermanently, shortenedUrl.GetOriginalUrl())
}

func (u *UrlController) Create(c *gin.Context) {
	shortenedUrl := &models.ShortenedUrl{}
	log.Printf("URL Shortener Object:/n %+v", shortenedUrl)
	if err := c.ShouldBindJSON(&shortenedUrl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !u.urlService.IsValidUrl(shortenedUrl.GetOriginalUrl()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please enter a valid URL"})
		return
	}
	log.Println(shortenedUrl.GetOriginalUrl())
	err := u.urlService.CreateUrl(shortenedUrl)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	// Append URL ID to session history in Redis
	if sid, exists := c.Get("sessionId"); exists {
		key := historyKeyPrefix + sid.(string)
		log.Printf("[history] LPush session=%s id=%s", sid, shortenedUrl.GetId())
		if err := u.cache.LPush(key, shortenedUrl.GetId()); err != nil {
			log.Printf("Error while saving history: %v", err)
		}
		_ = u.cache.Expire(key, sessionTTL)
	} else {
		log.Println("[history] no sessionId in context for POST")
	}
	c.JSON(http.StatusCreated, shortenedUrl)
}

func (u *UrlController) Delete(c *gin.Context) {
	id := c.Param("id")
	err := u.urlService.DeleteUrl(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}
