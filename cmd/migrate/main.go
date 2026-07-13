// Package main provides a small CLI to run schema migrations against the
// configured database. It reuses the embedded migrations and runner from
// the storage package so the binary stays in lock-step with what the
// application would apply at startup.
//
// Usage:
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down [steps]
//	go run ./cmd/migrate version
//	go run ./cmd/migrate force <version>
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/repository/postgres"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"go.uber.org/zap/zapcore"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	log, err := logger.New(logger.Config{Level: zapcore.InfoLevel})
	if err != nil {
		fmt.Fprintln(os.Stderr, "init logger:", err)
		os.Exit(1)
	}
	defer log.Sync()

	if err := godotenv.Load(); err != nil {
		log.Warnf(".env not found, using environment variables: %v", err)
	}

	dsn := os.Getenv("DB")
	if dsn == "" {
		log.Fatal("DB env var is required")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("sql open: %v", err)
	}
	defer db.Close()

	switch os.Args[1] {
	case "up":
		if err := postgres.MigrateUp(db, log); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
	case "down":
		steps := 1
		if len(os.Args) >= 3 {
			n, err := strconv.Atoi(os.Args[2])
			if err != nil {
				log.Fatalf("invalid steps %q: %v", os.Args[2], err)
			}
			steps = n
		}
		if err := postgres.MigrateDown(db, steps); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
		log.Infof("migrate down %d step(s) ok", steps)
	case "version":
		v, dirty, err := postgres.MigrateVersion(db)
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
	case "force":
		if len(os.Args) < 3 {
			log.Fatal("force requires a version argument")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("invalid version %q: %v", os.Args[2], err)
		}
		if err := postgres.MigrateForce(db, v); err != nil {
			log.Fatalf("migrate force: %v", err)
		}
		log.Infof("migrate forced to version %d", v)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate <up|down [steps]|version|force <version>>")
}
