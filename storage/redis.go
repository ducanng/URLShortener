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

// func main() {
// 	redis := Redis{}
// 	redis.Init()
// 	redis.Set("ducan123", "ducan")
// 	fmt.Print(redis.Get("ducan123"))
// }

// package main

// import (
// 	"encoding/json"
// 	"fmt"

// 	"github.com/go-redis/redis"
// )

// type Author struct {
// 	Name string `json:"name"`
// 	Age  int    `json:"age"`
// }

// func main() {
// 	client := redis.NewClient(&redis.Options{
// 		Addr:     "localhost:6379",
// 		Password: "",
// 		DB:       0,
// 	})

// 	json, err := json.Marshal(Author{Name: "Elliot", Age: 25})
// 	if err != nil {
// 		fmt.Println(err)
// 	}

// 	err = client.Set("id1234", json, 0).Err()
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	val, err := client.Get("id1234").Result()
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	fmt.Println(val)
// }
