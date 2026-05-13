package cache

import (
	"log"
	"time"

	"github.com/ducanng/URLShortener/internal/config"
	"github.com/go-redis/redis"
)

type Redis struct {
	*redis.Client
}

func (r *Redis) Connect(cfg *config.Config) error {
	r.Client = redis.NewClient(cfg.Redis)
	_, err := r.Client.Ping().Result()
	if err != nil {
		log.Println("Failed to connect to Redis")
		return err
	}
	log.Println("Connected to Redis")
	return nil
}

func (r *Redis) Get(key string) (string, error) {
	log.Println("Getting from Redis")
	return r.Client.Get(key).Result()
}

func (r *Redis) Set(key string, value string, expire time.Duration) error {
	log.Println("Setting to Redis")
	return r.Client.Set(key, value, expire).Err()
}

func (r *Redis) Update(key string, value string, expire time.Duration) error {
	log.Println("Updating Redis")
	return r.Client.Set(key, value, expire).Err()
}

func (r *Redis) Delete(key string) error {
	log.Println("Deleting from Redis")
	return r.Client.Del(key).Err()
}

func (r *Redis) LPush(key string, value string) error {
	return r.Client.LPush(key, value).Err()
}

func (r *Redis) LRange(key string, start, stop int64) ([]string, error) {
	return r.Client.LRange(key, start, stop).Result()
}

func (r *Redis) Expire(key string, ttl time.Duration) error {
	return r.Client.Expire(key, ttl).Err()
}
