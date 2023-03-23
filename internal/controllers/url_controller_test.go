package controllers

import (
	"URLShortener/internal/config"
	"URLShortener/internal/models"
	"URLShortener/internal/services"
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidUrl(t *testing.T) {
	var urlService *services.UrlService

	assert.True(t, urlService.IsValidUrl("https://www.google.com"))
	assert.True(t, urlService.IsValidUrl("https://www.google.com/"))
	assert.True(t, urlService.IsValidUrl("https://www.google.com/search?q=hello"))
	assert.True(t, urlService.IsValidUrl("https://www.google.com/search?q=hello&rlz=1C1CHBF_enVN921VN921&oq=hello&aqs=chrome..69i57j0l7.1001j0j7&sourceid=chrome&ie=UTF-8"))
	assert.False(t, urlService.IsValidUrl("https"))
	assert.False(t, urlService.IsValidUrl("https://"))
	assert.False(t, urlService.IsValidUrl("https://www"))
}

func TestNewController(t *testing.T) {
	cfg, err := config.LoadConfig("../../.env")
	if err != nil {
		log.Fatalf("Error while loading config: %v", err)
		return
	}
	db, redis := services.LoadConnect(cfg)

	// Set up the Gin engine
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	// Set up the URLController
	controller := NewController(db, redis)

	// Create a new URLShortened
	newUrl := &models.ShortenedUrl{
		OriginalUrl: "http://google.com",
	}

	// Marshal the new URLShortened into JSON
	payload, _ := json.Marshal(newUrl)

	// Create a new HTTP POST request to the Create API
	req, _ := http.NewRequest("POST", "/url", bytes.NewBuffer(payload))

	// Set the request headers
	req.Header.Set("Content-Type", "application/json")

	// Create a new HTTP recorder
	w := httptest.NewRecorder()

	// Perform the request
	r.POST("/url", controller.Create)
	r.ServeHTTP(w, req)

	// Check the response status code is 201 Created
	assert.Equal(t, http.StatusCreated, w.Code)

	// Decode the response JSON into a new URLShortened
	var responseUrl models.ShortenedUrl
	json.Unmarshal(w.Body.Bytes(), &responseUrl)

	// Check that the response URLShortened has been persisted
	assert.NotEmpty(t, responseUrl.Id)

	// Check that the response URLShortened has the correct Original URL
	assert.Equal(t, newUrl.OriginalUrl, responseUrl.OriginalUrl)

	// Check that the response URLShortened has the correct Shortened URL
	assert.Equal(t, "http://localhost:8080/"+responseUrl.Id, responseUrl.ShortUrl)

	// Check that the response URLShortened has the correct CreatedAt
	assert.NotEmpty(t, responseUrl.CreatedAt)

}
func isValidURL(u string) bool {
	parsedURL, err := url.ParseRequestURI(u)
	if err != nil {
		return false
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}

	if parsedURL.Host == "" {
		return false
	}

	return true
}

func TestValidateURL(t *testing.T) {
	assert.False(t, isValidURL("https://www.google"))
}
