package controllers

import (
	"URLShortener/internal/models"
	"URLShortener/internal/services"
	"URLShortener/pkg/cache"
	"URLShortener/pkg/database"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"net/url"
)

type UrlController struct {
	urlService *services.UrlService
}

func NewController(db *database.DB, cache *cache.Redis) *UrlController {
	return &UrlController{
		urlService: services.NewUrlService(db, cache),
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
	// Set cookies history of shortened urls
	var shortenedURLs []string
	if cookie, err := c.Request.Cookie("shortenedURLs"); err == nil {
		// Giải mã cookie
		if value, err := url.QueryUnescape(cookie.Value); err == nil {
			// Chuyển đổi giá trị cookie từ JSON sang danh sách các URL
			if err := json.Unmarshal([]byte(value), &shortenedURLs); err != nil {
				log.Printf("Error while unmarshalling cookie: %v", err)
			}
		} else {
			log.Printf("Error while unescaping cookie: %v", err)
		}
	}
	byteData, e := json.Marshal(shortenedUrl)
	if e != nil {
		log.Printf("Error while marshalling cookie: %v", e)
	}
	data := string(byteData)
	// Thêm shortenedURL vào danh sách
	shortenedURLs = append(shortenedURLs, data)

	// Chuyển đổi danh sách shortenedURLs sang JSON và lưu vào cookie
	if jsonValue, err := json.Marshal(shortenedURLs); err == nil {
		encodedValue := url.QueryEscape(string(jsonValue))
		c.SetCookie("shortenedURLs", encodedValue, 86400, "", "", false, true)
	} else {
		// Xử lý lỗi
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
