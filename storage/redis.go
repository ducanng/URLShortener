package storage

import (
	"URLShortener-gRPC-Swagger/model"
	"URLShortener-gRPC-Swagger/shorten/base62"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/go-redis/redis"
)

type Redis struct {
	Client *redis.Client
}

// Init NewRedisClient creates a new redis client
func (s *Redis) Init() {
	s.Client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
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
func (s *Redis) Set(entry model.URLEntry) error {
	marshal, e := json.Marshal(entry)
	if e != nil {
		log.Fatal(e)
	}
	_, err := s.Client.Set(strconv.FormatUint(uint64(entry.Id), 10), marshal, 0).Result()
	if err != nil {
		log.Fatal("Error when set key-value to redis: ", err)
	}
	return err
}

// Get gets a value from redis
func (s *Redis) Get(key string) (model.URLEntry, error) {
	id := base62.Decode(key)
	val, err := s.Client.Get(strconv.FormatUint(id, 10)).Result()

	if err != nil {
		log.Fatal("Error when get key-value from redis: ", err)
	}
	var entry model.URLEntry
	_ = json.Unmarshal([]byte(val), &entry)
	return entry, err
}
