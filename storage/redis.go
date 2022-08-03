package storage

import (
	"fmt"
	"log"

	"github.com/go-redis/redis"
)

type Redis struct {
	Client *redis.Client
}

// NewRedisClient creates a new redis client
func (s *Redis) Init() {
	s.Client = redis.NewClient(&redis.Options{
		Addr:     "redis-db:6379",
		Password: "",
		DB:       0,
	})
	pong, err := s.Client.Ping().Result()

	if err != nil {
		log.Fatalf("Redis can't not connect: %v", err)
	}
	fmt.Println(pong, err)
}

// Set sets a key-value pair in redis
func (s *Redis) Set(key string, value string) (string, error) {
	val, err := s.Client.Set(key, value, 0).Result()
	if err != nil {
		log.Fatal(err)
	}
	return val, err
}

// Get gets a value from redis
func (s *Redis) Get(key string) (string, error) {
	val, err := s.Client.Get(key).Result()
	if err != nil {
		log.Fatal(err)
	}
	return val, err
}
