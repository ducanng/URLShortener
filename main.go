package main

import (
	"URLShortener/internal/config"
	"URLShortener/internal/services"
	"URLShortener/router"
	"log"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg, err := config.LoadConfig(".env")
	if err != nil {
		log.Fatalf("Error while loading config: %v", err)
		return
	}

	db, redis := services.LoadConnect(cfg)

	r := router.InitRouter(db, redis)
	log.Println("Server start on port 8080")
	err = r.Run(":8080")
	if err != nil {
		log.Fatalf("Error while starting server: %v", err)
		return
	}
}
