package router

import (
	"URLShortener/internal/controllers"
	"URLShortener/pkg/cache"
	"URLShortener/pkg/database"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func InitRouter(db *database.DB, cache *cache.Redis) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Static("/templates/assets", "./templates/assets/")
	//check
	files := []string{
		"templates/views/index.html",
		"templates/views/partials/header.html",
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

		c.HTML(http.StatusOK, "index.html", gin.H{
			"Title":   "Home Page",
			"Content": "URL Shortener",
		})
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
