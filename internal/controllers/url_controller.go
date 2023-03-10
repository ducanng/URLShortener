package controllers

// Path: internal\controllers\url_controller.go
// Compare this snippet from internal\controllers\url_controller.go:
//
import (
	"URLShortener/internal/models"
	"URLShortener/internal/services"
	"URLShortener/pkg/cache"
	"URLShortener/pkg/database"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

type UrlController struct {
	urlService *services.UrlService
}

func NewController(db database.DB, cache cache.Redis) *UrlController {
	return &UrlController{
		urlService: services.NewUrlService(db, cache),
	}
}

func (u *UrlController) Redirect(c *gin.Context) {
	id := c.Param("id")
	log.Println(id)

	shortenedUrl, err := u.urlService.GetUrl(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Redirect(http.StatusMovedPermanently, shortenedUrl.GetOriginalUrl())
}

func (u *UrlController) Create(c *gin.Context) {
	shortenedUrl := &models.ShortenedUrl{}
	if err := c.ShouldBindJSON(&shortenedUrl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !u.urlService.IsValidUrl(shortenedUrl.GetOriginalUrl()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL"})
		return
	}
	log.Println(shortenedUrl.GetOriginalUrl())
	err := u.urlService.CreateUrl(shortenedUrl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
