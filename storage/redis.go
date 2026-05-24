package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/ducanng/URLShortener/model"

	base62 "github.com/alextanhongpin/base62"
	"github.com/joho/godotenv"

	"github.com/go-redis/redis"
)

type Redis struct {
	Client *redis.Client
}

// Init NewRedisClient creates a new redis client
func (s *Redis) Init() {
	if e := godotenv.Load(); e != nil {
		log.Printf("warn: .env not found, using environment variables: %v", e)
	}
	s.Client = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "",
		DB:       0,
	})
	pong, err := s.Client.Ping().Result()
	if err != nil {
		log.Printf("warn: Redis connection failed, will fallback to DB: %v", err)
		return
	}
	fmt.Println(pong, err)
}

// Set sets a key-value pair in redis
func (s *Redis) Set(entry model.URLEntry) error {
	marshal, e := json.Marshal(entry)
	if e != nil {
		log.Printf("Error marshaling entry for redis: %v", e)
		return e
	}
	_, err := s.Client.Set(strconv.FormatUint(uint64(entry.Id), 10), marshal, 0).Result()
	if err != nil {
		log.Printf("warn: Failed to set key-value to redis: %v", err)
	}
	return err
}

// Get gets a value from redis
func (s *Redis) Get(key string) (model.URLEntry, error) {
	id := base62.Decode(key)
	val, err := s.Client.Get(strconv.FormatUint(id, 10)).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("warn: Failed to get key-value from redis: %v", err)
		}
		return model.URLEntry{}, err
	}
	var entry model.URLEntry
	_ = json.Unmarshal([]byte(val), &entry)
	return entry, nil
}

func (s *Redis) Update(entry model.URLEntry) error {
	marshal, e := json.Marshal(entry)
	if e != nil {
		log.Printf("Error marshaling entry for redis: %v", e)
		return e
	}
	_, err := s.Client.Set(strconv.FormatUint(uint64(entry.Id), 10), marshal, 0).Result()
	if err != nil {
		log.Printf("warn: Failed to update key-value to redis: %v", err)
	}
	return err
}
