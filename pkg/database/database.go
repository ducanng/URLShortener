package database

import (
	"URLShortener/internal/config"
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	*sql.DB
}

func (d *DB) Connect(cfg *config.Config) error {
	var err error
	d.DB, err = sql.Open("mysql", cfg.MySQL.FormatDSN())
	if err != nil {
		log.Println("Failed to connect to database")
		return err
	}
	if err = d.DB.Ping(); err != nil {
		log.Println("Failed to connect to database")
		err = d.Disconnect()
		return err
	}
	log.Println("Connected to database")
	return nil
}

func (d *DB) Disconnect() error {
	err := d.Close()
	if err != nil {
		log.Println("Failed to disconnect from database")
		return err
	}
	return nil
}
