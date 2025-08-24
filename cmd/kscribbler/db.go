package main

import (
	"errors"
	"fmt"
	"log"

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
	db, err := sqlx.Open("sqlite3", kscribblerDBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS book (
        id TEXT PRIMARY KEY NOT NULL,
        book_title TEXT NOT NULL,
		isbn TEXT,
		hardcover_id TEXT
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
		kscribbler_uploaded INTEGER DEFAULT 0,
        FOREIGN KEY(book_id) REFERENCES book(id),
        FOREIGN KEY(hardcover_id) REFERENCES book(hardcover_id)
    )
`)
	if err != nil {
		log.Fatalf("failed to create quotes table: %v", err)
	}

	return nil
}

// Read kobosqlite and populate kscribbler sqlite with relevant data
func connectDatabases() *sqlx.DB {
	dbErrMsG := "failed to open database at %s: %w"

	kscribblerDB, err := sqlx.Open("sqlite3", kscribblerDBPath)
	if err != nil {
		err := fmt.Errorf(dbErrMsG, kscribblerDBPath, err)
		log.Fatal(err.Error())
	}

	// attach to kobo database also
	_, err = kscribblerDB.Exec("ATTACH DATABASE ? AS koboDB", koboDBPath)
	if err != nil {
		err := fmt.Errorf("failed to attach Kobo database: %w", err)
		log.Fatal(err.Error())
	}
	return kscribblerDB
}

func populateKscribblerDBBook() error {
	kscribblerDB := connectDatabases()
	defer kscribblerDB.Close()

	bookQuery := `
		INSERT OR IGNORE INTO book(isbn, book_title, id)
	    SELECT DISTINCT c.ISBN, c.Title, b.VolumeID
		FROM koboDB.content c
		JOIN koboDB.Bookmark b
		ON c.ContentID = b.VolumeID
   `
	log.Printf("Populating kscribbler database from Kobo database...")
	_, err := kscribblerDB.Exec(bookQuery)
	if err != nil {
		err := fmt.Errorf("failed to populate kscribblerDB book Table : %w", err)
		return err
	}
	return nil
}
