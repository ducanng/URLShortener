package main

import (
	"URLShortener/internal/config"
	"URLShortener/pkg/cache"
	"URLShortener/pkg/database"
	"URLShortener/router"
	"log"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)

	db := database.DB{}
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error while loading config: %v", err)
		return
	}

	err = db.Connect(cfg)
	if err != nil {
		log.Fatalf("Error while connecting to database: %v", err)
		return
	}

	redis := cache.Redis{}
	err = redis.Connect(cfg)
	if err != nil {
		log.Fatalf("Error while connecting to redis: %v", err)
		return
	}

	r := router.InitRouter(db, redis)
	log.Println("Server start on port 8080")
	err = r.Run(":8080")
	if err != nil {
		log.Fatalf("Error while starting server: %v", err)
		return
	}
}
