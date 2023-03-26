package config

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	MySQL *mysql.Config
	Redis *redis.Options
}

func LoadConfig(path string) (*Config, error) {
	err := godotenv.Load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}

	mysqlCfg := mysql.Config{
		User:      os.Getenv("MYSQL_USER"),
		Passwd:    os.Getenv("MYSQL_ROOT_PASSWORD"),
		Net:       "tcp",
		Addr:      fmt.Sprintf("%s:%s", os.Getenv("MYSQL_HOST"), os.Getenv("MYSQL_PORT")),
		DBName:    os.Getenv("MYSQL_DATABASE"),
		ParseTime: true,
	}

	redisOpts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	}

	return &Config{
		MySQL: &mysqlCfg,
		Redis: redisOpts,
	}, nil
}
