package main

import (
	"github.com/ducanng/URLShortener/internal/config"
	"github.com/ducanng/URLShortener/pkg/cache"
	"github.com/ducanng/URLShortener/pkg/database"
	"github.com/ducanng/URLShortener/router"
	"log"
	"os"
)

func LoadConnect(cfg *config.Config) (*database.DB, *cache.Redis) {
	db := database.DB{}
	err := db.Connect(cfg)
	if err != nil {
		log.Printf("Error while connecting to database: %v", err)
		return nil, nil
	}

	r := cache.Redis{}
	err = r.Connect(cfg)
	if err != nil {
		log.Printf("Error while connecting to redis: %v", err)
		return &db, nil
	}
	return &db, &r
}

func main() {
	log.SetOutput(os.Stdout)

	cfg, err := config.LoadConfig(".env")
	if err != nil {
		log.Fatalf("Error while loading config: %v", err)
		return
	}

	db, redis := LoadConnect(cfg)

	r := router.InitRouter(db, redis)
	log.Println("Server start on port 8080")
	err = r.Run(":8080")
	if err != nil {
		log.Fatalf("Error while starting server: %v", err)
		return
	}
}
