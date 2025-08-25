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

// connectKscribblerDB connects to the kscribbler SQLite database and creates it if it doesn't exist.
func connectKscribblerDB() *sqlx.DB {
	dbErrMsG := "failed to open database at %s: %w"

	kscribblerDB, err := sqlx.Open("sqlite3", kscribblerDBPath)
	if err != nil {
		err := fmt.Errorf(dbErrMsG, kscribblerDBPath, err)
		log.Fatal(err.Error())
	}
	return kscribblerDB
}

// Read kobosqlite and populate kscribbler sqlite with relevant data
func connectDatabases() *sqlx.DB {
	kscribblerDB := connectKscribblerDB()
	// attach to kobo database also
	_, err := kscribblerDB.Exec("ATTACH DATABASE ? AS koboDB", koboDBPath)
	if err != nil {
		err := fmt.Errorf("failed to attach Kobo database: %w", err)
		log.Fatal(err.Error())
	}
	return kscribblerDB
}

// createKscribblerTables creates the SQLite database file if it doesn't exist
func createKscribblerTables() error {
	if _, err := os.Stat(kscribblerDBPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
		return err
	}

	kscribblerDB := connectKscribblerDB()
	defer kscribblerDB.Close()

	_, err := kscribblerDB.Exec(`
    CREATE TABLE IF NOT EXISTS book (
        book_id TEXT PRIMARY KEY NOT NULL,
        book_title TEXT NOT NULL,
		isbn TEXT,
		hardcover_id TEXT
    )
`)
	if err != nil {
		log.Fatalf("failed to create book table: %v", err)
	}

	// Quotes table
	_, err = kscribblerDB.Exec(`
    CREATE TABLE IF NOT EXISTS quote (
        book_id INTEGER NOT NULL,
		bookmark_id TEXT PRIMARY KEY NOT NULL,
        quote TEXT NOT NULL,
        annotation TEXT,
        page INTEGER,
		hardcover_id INTEGER Default -1,
		hardcover_edition INTEGER Default -1,
		type TEXT,
		kscribbler_uploaded INTEGER DEFAULT 0,
        FOREIGN KEY(book_id) REFERENCES book(book_id),
        FOREIGN KEY(hardcover_id) REFERENCES book(hardcover_id)
		CONSTRAINT unique_trimmed_quote UNIQUE (quote)
    )
`)
	if err != nil {
		log.Fatalf("failed to create quotes table: %v", err)
	}

	return nil
}

func populateQuoteTable() error {
	kscribblerDB := connectDatabases()
	defer kscribblerDB.Close()

	quoteQuery := `
		INSERT OR IGNORE INTO quote(book_id,bookmark_id, type, quote, annotation, kscribbler_uploaded)
	    SELECT b.VolumeID, b.BookmarkID, b.Type, TRIM(b.Text), b.Annotation, 
	    CASE
	        WHEN instr(lower(b.Annotation), 'kscrib') > 0 THEN 1
	        ELSE 0
	    END
	 	FROM koboDB.Bookmark b
	    WHERE b.Text IS NOT NULL AND TRIM(b.Text) != ''
   `
	log.Printf("Populating quote table...")
	_, err := kscribblerDB.Exec(quoteQuery)
	if err != nil {
		err := fmt.Errorf("failed to populate kscribblerDB book Table : %w", err)
		return err
	}
	return nil
}

func populateBookTable() error {
	kscribblerDB := connectDatabases()
	defer kscribblerDB.Close()

	bookQuery := `
		INSERT OR IGNORE INTO book(isbn, book_title, book_id)
	    SELECT DISTINCT c.ISBN, c.Title, b.VolumeID
		FROM koboDB.content c
		JOIN koboDB.Bookmark b
		ON c.ContentID = b.VolumeID
   `
	log.Printf("Populating book table...")
	_, err := kscribblerDB.Exec(bookQuery)
	if err != nil {
		err := fmt.Errorf("failed to populate kscribblerDB book Table : %w", err)
		return err
	}
	return nil
}
