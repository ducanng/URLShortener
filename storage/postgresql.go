package storage

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/ducanng/URLShortener/model"

	base62 "github.com/alextanhongpin/base62"

	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

type SQLStore struct {
	Client   *sql.DB
	URLEntry model.URLEntry
}

func (s *SQLStore) Init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("warn: .env not found, using environment variables: %v", err)
	}
	// Connect to the database
	var err error
	s.Client, err = sql.Open("postgres", os.Getenv("DB"))
	if err != nil {
		log.Fatalf("SQL can't not connect: %v", err)
	}

	// Connection pool config: (CPU×2)+spindle = (8×2)+1 = 17 for 8 vCPU SSD
	s.Client.SetMaxOpenConns(17)
	s.Client.SetMaxIdleConns(17)
	s.Client.SetConnMaxLifetime(5 * time.Minute)
	s.Client.SetConnMaxIdleTime(1 * time.Minute)
	//Read file .sql
	file, err := os.ReadFile("storage/init.sql")
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
		log.Printf("Error inserting to db: %v", err)
	}
	return err
}

func (s *SQLStore) Load(key string) (model.URLEntry, error) {
	id := int64(base62.Decode(key))
	var value model.URLEntry
	err := s.Client.QueryRow("SELECT * FROM url_list WHERE id = $1", id).
		Scan(&value.Id, &value.OriginalURL, &value.ShortedURL, &value.Clicks)
	if err != nil {
		log.Printf("Error loading from db: %v", err)
	}
	return value, err
}

func (s *SQLStore) UpdateClicks(path string, click int32) {
	id := int64(base62.Decode(path))
	_, err := s.Client.Exec(
		"UPDATE url_list SET clicks = $1 WHERE id = $2",
		click,
		id,
	)
	if err != nil {
		log.Printf("Error updating clicks in db: %v", err)
	}
}
