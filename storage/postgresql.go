package storage

import (
	"URLShortener-gRPC-Swagger/model"
	"URLShortener-gRPC-Swagger/shorten/base62"
	"database/sql"
	"github.com/joho/godotenv"
	"log"
	"os"

	_ "github.com/lib/pq"
)

type SQLStore struct {
	Client   *sql.DB
	URLEntry model.URLEntry
}

func (s *SQLStore) Init() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("err loading: %v", err)
	}
	// Connect to the database
	s.Client, err = sql.Open("postgres", os.Getenv("DB"))
	if err != nil {
		log.Fatalf("SQL can't not connect: %v", err)
	}
	//Read file .sql
	file, err := os.ReadFile("C:\\Users\\HP\\Documents\\Go\\URLShortener-gRPC-Swagger\\storage\\init.sql")
	if err != nil {
		log.Fatalf("Can't read sql file: %v", err)
	}
	// Create table if not exist
	_, err = s.Client.Exec(string(file))
	if err != nil {
		log.Fatalf("Can't execute sql file: %v", err)
	}
}

func (s *SQLStore) Save(entry model.URLEntry) error {
	_, err := s.Client.Exec(
		"INSERT INTO url_list (id, original_url, shorted_url, clicks) VALUES ($1, $2, $3, $4)",
		entry.Id,
		entry.OriginalURL,
		entry.ShortedURL,
		entry.Clicks,
	)
	if err != nil {
		return err
	}
	return err
}

func (s *SQLStore) Load(key string) (model.URLEntry, error) {
	id := int64(base62.Decode(key))
	var value model.URLEntry
	err := s.Client.QueryRow("SELECT * FROM url_list WHERE id = $1", id).
		Scan(&value.Id, &value.OriginalURL, &value.ShortedURL, &value.Clicks)
	if err != nil {
		return value, err
	}
	return value, err
}
