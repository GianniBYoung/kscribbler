package main

import (
	"errors"
	"log"

	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/jmoiron/sqlx"
)

var koboDB *sqlx.DB
var koboDBPath = "/mnt/onboard/.kobo/KoboReader.sqlite"
var kscribblerDBPath = "/mnt/onboard/.adds/kscribbler.sqlite"

// createKscribblerDB creates the SQLite database file if it doesn't exist
func createKscribblerDB() error {
	if _, err := os.Stat(kscribblerDBPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
		return err
	}

	// File does not exist, create the database
	db, err := sql.Open("sqlite3", kscribblerDBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS book (
        id TEXT PRIMARY KEY NOT NULL,
        book_title TEXT NOT NULL,
		isbn TEXT,
		hardcover_id TEXT, 
		kscribbler_uploaded INTEGER DEFAULT 0
    )
`)
	if err != nil {
		log.Fatalf("failed to create book table: %v", err)
	}

	// Quotes table
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS quote (
        book_id INTEGER NOT NULL,
        quote TEXT NOT NULL,
        page INTEGER,
		hardcover_id TEXT,
        FOREIGN KEY(book_id) REFERENCES book(id),
        FOREIGN KEY(hardcover_id) REFERENCES book(hardcover_id)
    )
`)
	if err != nil {
		log.Fatalf("failed to create quotes table: %v", err)
	}

	return nil
}

func populateKscribblerDB() error {
	ksdb, err := sql.Open("sqlite3", kscribblerDBPath)
	if err != nil {
		return err
	}
	defer ksdb.Close()

	return nil
}
