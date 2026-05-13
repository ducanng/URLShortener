package router

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/ducanng/URLShortener/internal/controllers"
	"github.com/ducanng/URLShortener/internal/models"
	"github.com/ducanng/URLShortener/pkg/cache"
	"github.com/ducanng/URLShortener/pkg/database"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func resolveHistory(c *gin.Context, cache *cache.Redis, db *database.DB) []models.ShortenedUrl {
	var result []models.ShortenedUrl
	sid, _ := c.Get("sessionId")
	if sid == nil {
		log.Println("[history] sessionId is nil")
		return result
	}
	key := historyKeyPrefix + sid.(string)
	log.Printf("[history] loading for session=%s key=%s", sid, key)
	ids, err := cache.LRange(key, 0, -1)
	if err != nil {
		log.Printf("[history] LRange error: %v", err)
		return result
	}
	log.Printf("[history] found %d items", len(ids))
	for _, item := range ids {
		var urlID string
		// Backward-compatible: detect old format (full JSON) vs new format (plain ID)
		if len(item) > 0 && item[0] == '{' {
			var legacy models.ShortenedUrl
			if err := json.Unmarshal([]byte(item), &legacy); err == nil {
				urlID = legacy.Id
			}
		} else {
			urlID = item
		}
		if urlID == "" {
			continue
		}
		// Try URL cache first (always up-to-date clicks)
		if data, err := cache.Get(urlID); err == nil {
			var u models.ShortenedUrl
			if err := json.Unmarshal([]byte(data), &u); err == nil {
				result = append(result, u)
				continue
			}
		}
		// Fallback: query DB directly
		row := db.QueryRow("SELECT id, original_url, short_url, created_at, clicks FROM shortened_urls WHERE id = ?", urlID)
		var u models.ShortenedUrl
		if err := row.Scan(&u.Id, &u.OriginalUrl, &u.ShortUrl, &u.CreatedAt, &u.Clicks); err == nil {
			result = append(result, u)
		}
	}
	return result
}

const sessionCookieName = "anon_session"
const historyKeyPrefix = "history:"
const sessionTTL = 30 * 24 * time.Hour

func sessionMiddleware(c *gin.Context) {
	sid, err := c.Cookie(sessionCookieName)
	if err != nil || sid == "" {
		sid = uuid.New().String()
		c.SetCookie(sessionCookieName, sid, int(sessionTTL.Seconds()), "/", "", false, true)
	}
	c.Set("sessionId", sid)
	c.Next()
}

func InitRouter(db *database.DB, cache *cache.Redis) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Static("/templates/assets", "./templates/assets/")
	//check
	files := []string{
		"templates/views/index.html",
	}
	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		//log.Fatalf("Error while parsing template: %v", err)
		panic(err)
	}
	r.SetHTMLTemplate(tmpl)
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	store := cookie.NewStore([]byte("secret"))

	r.Use(sessions.Sessions("mysession", store))
	r.Use(sessionMiddleware)
	// curl http://localhost:8080/ping
	r.GET("/ping", func(c *gin.Context) {
		log.Println("pong")
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// Initialize controllers
	urlController := controllers.NewController(db, cache)
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/home")
	})

	r.GET("/home", func(c *gin.Context) {
		shortenedUrls := resolveHistory(c, cache, db)
		c.HTML(http.StatusOK, "index.html", gin.H{
			"Title":         "Home Page",
			"Content":       "URL Shortener",
			"ShortenedUrls": shortenedUrls,
			"Date":          time.Now().Format("January 2, 2006"),
		})
	})

	r.GET("/api/history", func(c *gin.Context) {
		c.JSON(http.StatusOK, resolveHistory(c, cache, db))
	})

	// curl http://localhost:8080/16c6de36
	r.GET("/:id", urlController.Redirect)
	// Define API routes
	api := r.Group("/api")
	{
		// curl -X POST -H "Content-Type: application/json" -d '{"originalUrl":"https://www.google.com"}' http://localhost:8080/api/url
		// curl -X POST -H "Content-Type: application/json" -d '{"originalUrl":"https://www.google.com", "alias":"testingurl"}' http://localhost:8080/api/shorten
		api.POST("shorten", urlController.Create)

		// curl -X DELETE http://localhost:8080/api/url/16c6de36
		api.DELETE("url/:id", urlController.Delete)
	}

	return r
}
