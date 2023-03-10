package router

import (
	"URLShortener/internal/controllers"
	"URLShortener/pkg/cache"
	"URLShortener/pkg/database"
	"github.com/gin-gonic/gin"
	"log"
)

func InitRouter(db database.DB, cache cache.Redis) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.GET("/ping", func(c *gin.Context) {
		log.Println("pong")
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// Initialize controllers
	urlController := controllers.NewController(db, cache)

	// Define API routes
	api := r.Group("/api")
	{
		// curl -X POST -H "Content-Type: application/json" -d '{"originalUrl":"https://www.google.com"}' http://localhost:8080/api/url
		// curl -X POST -H "Content-Type: application/json" -d '{"originalUrl":"https://www.google.com", "alias":"testingurl"}' http://localhost:8080/api/url
		api.POST("url", urlController.Create)
		api.DELETE("url/:id", urlController.Delete)
	}
	// curl http://localhost:8080/16c6de36
	r.GET("/:id", urlController.Redirect)
	return r
}
