package store

import (
	"database/sql"
	"fmt"

	"url-shortener/base62"
	"url-shortener/shorten"

	_ "github.com/lib/pq"
)

// const (
// 	host     = "localhost"
// 	port     = 2345
// 	user     = "postgres"
// 	password = "secret"
// 	dbname   = "postgres"
// )

// Connect postgresql
func InitalizeStore() *sql.DB {
	// psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
	// 	"password=%s dbname=%s sslmode=disable",
	// 	host, port, user, password, dbname)
	connStr := "postgresql://postgres:secret@host.docker.internal:2345/postgres?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}
	err = db.Ping() // verifies a connection to the database
	if err != nil {
		panic(err)
	}
	return db
}

// Save id, url short, url long to db
func SaveURL(entry shorten.URLEntry) {
	db := InitalizeStore()
	sqlStatement := `INSERT INTO urlshortener (id, urloriginal, urlshort, clicks, create_at, update_at) 
						VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := db.Exec(sqlStatement, entry.Id, entry.OriginalURL, entry.ShortenURL, entry.Clicks, entry.CreateAt, entry.UpdateAt)
	if err != nil {
		panic(fmt.Sprintf("Failed saving url | Error: %v - shortUrl: %s - originalUrl: %s\n", err, entry.ShortenURL, entry.OriginalURL))
	}
	fmt.Printf("Saved shortUrl: %s - originalUrl: %s\n", entry.ShortenURL, entry.OriginalURL)
	defer db.Close()
}

// Get data from db
func GetURLEntry(longurl string) shorten.URLEntry {
	var urlentry shorten.URLEntry
	db := InitalizeStore()
	rows := db.QueryRow("SELECT * FROM urlshortener WHERE urloriginal = $1", longurl)
	//Scan copies the columns from the matched row into the values pointed at by dest
	err := rows.Scan(&urlentry.Id, &urlentry.OriginalURL, &urlentry.ShortenURL, &urlentry.Clicks, &urlentry.CreateAt, &urlentry.UpdateAt)
	if err != nil {
		panic(fmt.Sprintf("Failed Retrieve Initial Url | Error: %v\n", err))
	}
	defer db.Close()
	return urlentry
}

// Get long url from db, input short url
func GetLongURL(shorturl string) string {
	var long string = ""
	key := base62.Decode(shorturl)
	db := InitalizeStore()
	rows := db.QueryRow("SELECT urloriginal FROM urlshortener WHERE id = $1", key)
	err := rows.Scan(&long) //Scan copies the columns from the matched row into the values pointed at by dest
	if err != nil {
		fmt.Printf("Failed Retrieve Url | Error: %v - shortUrl: %s\n", err, shorturl)
	}
	defer db.Close()
	return long
}

//Check long url exists in db
func CheckURLinDB(longurl string) bool {
	var check string
	db := InitalizeStore()
	rows := db.QueryRow("SELECT urloriginal FROM urlshortener WHERE urloriginal = $1", longurl)
	err := rows.Scan(&check)
	if err != nil {
		fmt.Printf("Don't Find Url | Error: %v\n", err)
		return false
	}
	fmt.Printf("Find Url | Error: %v\n", err)
	defer db.Close()
	return true
}

//Delete short url from database
func DeleteShortURL(key uint64) bool {
	db := InitalizeStore()
	sqlStatement := `DELETE FROM urlshortener WHERE id = $1`
	res, err := db.Exec(sqlStatement, key)
	if err != nil {
		panic(fmt.Sprintf("Don't find shortUrl to delete | Error: %v\n", err))
		return false
	}
	//Check deleted short url from database
	count, err := res.RowsAffected()
	if err != nil {
		panic(err)
		return false
	}
	fmt.Print(count) // 0 : not deleted , 1 : deleted
	if count == 0 {
		fmt.Println(" : Delete failed")
		return false
	}
	fmt.Println(" : Delete successful")
	defer db.Close()
	return true
}

func UpdateURL(updateUrlEntry shorten.URLEntry) bool {
	db := InitalizeStore()
	sqlStatement := `UPDATE urlshortener SET urloriginal = $2, clicks = $3, update_at = $4 WHERE id = $1`
	res, err := db.Exec(sqlStatement, updateUrlEntry.Id, updateUrlEntry.OriginalURL, updateUrlEntry.Clicks, updateUrlEntry.UpdateAt)
	if err != nil {
		panic(fmt.Sprintf("Failed Updating Url | Error: %v - shortUrl: %s - originalUrl: %s\n", err, updateUrlEntry.ShortenURL, updateUrlEntry.OriginalURL))
	}
	count, err := res.RowsAffected()
	if err != nil {
		panic(err)
	}
	fmt.Print(count) // 0 : update failed , 1 : update successful
	if count == 0 {
		fmt.Println(" : Update failed")
		return false
	}
	fmt.Println(" : Update successful")
	return true
}

func UpdateCounterLink(updateUrlEntry shorten.URLEntry) bool {
	db := InitalizeStore()
	sqlStatement := `UPDATE urlshortener SET clicks = $2 WHERE id = $1`
	res, err := db.Exec(sqlStatement, updateUrlEntry.Id, updateUrlEntry.Clicks)
	if err != nil {
		panic(fmt.Sprintf("Failed Updating Clicks | Error: %v", err))
	}
	count, err := res.RowsAffected()
	if err != nil {
		panic(err)
	}
	fmt.Print(count) // 0 : update failed , 1 : update successful
	if count == 0 {
		fmt.Println(" : Update failed in db")
		return false
	}
	fmt.Println(" : Update successful in db")
	return true
}
