package main

import (
	"errors"
	"fmt"
	"log"

	"os"

	"github.com/GianniBYoung/simpleISBN"

	"github.com/jmoiron/sqlx"
)

var kscribblerDB *sqlx.DB
var koboDBPath = "/mnt/onboard/.kobo/KoboReader.sqlite"
var kscribblerDBPath = "/mnt/onboard/.adds/kscribbler.sqlite"

// connectKscribblerDB connects to the kscribbler SQLite database and creates it if it doesn't exist.
func connectKscribblerDB() *sqlx.DB {
	dbErrMsg := "failed to open database at %s: %w"

	kscribblerDB, err := sqlx.Open("sqlite", kscribblerDBPath)
	if err != nil {
		err := fmt.Errorf(dbErrMsg, kscribblerDBPath, err)
		log.Fatal(err.Error())
	}
	return kscribblerDB
}

// connectDatabases attaches kscribblerDB to KoboReaderDB in order to populate kscribblerDB with relevant data.
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

// createKscribblerTables creates the SQLite database if it doesn't exist.
func createKscribblerTables() {
	if _, err := os.Stat(kscribblerDBPath); err == nil {
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("Error while trying to create/open kscribblerDB: %v", err)
	}

	kscribblerDB := connectKscribblerDB()
	defer kscribblerDB.Close()

	_, err := kscribblerDB.Exec(`
    CREATE TABLE IF NOT EXISTS book (
        book_id TEXT PRIMARY KEY NOT NULL,
        book_title TEXT NOT NULL,
		isbn TEXT,
		hardcover_id INTEGER DEFAULT -1,
		hardcover_edition INTEGER Default -1
    )
`)
	if err != nil {
		log.Fatalf("failed to create book table in kscribblerDB: %v", err)
	}

	// Quotes table
	_, err = kscribblerDB.Exec(`
    CREATE TABLE IF NOT EXISTS quote (
        book_id INTEGER NOT NULL,
		bookmark_id TEXT PRIMARY KEY NOT NULL,
        quote TEXT NOT NULL,
        annotation TEXT,
        page INTEGER,
		type TEXT,
		kscribbler_uploaded INTEGER DEFAULT 0,
        FOREIGN KEY(book_id) REFERENCES book(book_id),
		CONSTRAINT unique_trimmed_quote UNIQUE (quote)
    )
`)
	if err != nil {
		log.Fatalf("failed to create quotes table in kscribblerDB: %v", err)
	}

}

// populateQuoteTable populates the quote table in kscribblerDB with quotes and annotations from KoboReader.sqlite.
func populateQuoteTable() {
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
		log.Fatalf("failed to populate kscribblerDB book Table: %v", err)
	}
}

// populateBookTable populates the book table in kscribblerDB with book identifiers from KoboReader.sqlite.
func populateBookTable() {
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
		log.Fatalf("failed to populate kscribblerDB book Table : %v", err)
	}
}

// updateDBWithHardcoverInfo updates the kscribblerDB with missing hardcover info from Hardcover API.
func updateDBWithHardcoverInfo() {

	var books []Book
	err := kscribblerDB.Select(
		&books,
		`SELECT isbn FROM book WHERE (hardcover_id = -1 OR hardcover_edition = -1) AND isbn IS NOT NULL;`,
	)

	if err != nil {
		log.Printf("failed to load books with missing hardcover info: %v", err)
		return
	}

	log.Printf("Found %d books with missing hardcover info", len(books))
	for _, book := range books {
		isbn, err := simpleISBN.NewISBN(book.FoundISBN.String)
		if err != nil {
			log.Printf("failed to parse isbn %s: %v", book.FoundISBN.String, err)
			continue
		}
		book.SimpleISBN = *isbn

		book.koboToHardcover()

		log.Printf(
			"Updating book %s with hardcover info: %d, %d, %s, %s",
			book.Title.String,
			book.HardcoverID,
			book.HardcoverEdition,
			book.SimpleISBN.ISBN13Number,
			book.SimpleISBN.ISBN10Number,
		)
		_, err = kscribblerDB.Exec(
			`UPDATE book SET hardcover_id = ?, hardcover_edition = ? WHERE isbn = ? OR isbn = ?;`,
			book.HardcoverID,
			book.HardcoverEdition,
			book.SimpleISBN.ISBN13Number,
			book.SimpleISBN.ISBN10Number,
		)
		if err != nil {
			log.Printf("failed to update book %s with hardcover info: %v", book.Title.String, err)
		}
	}

	log.Println("Updated missing Hardcover info in book table")
}

// updateDBWithISBNs loops through all books with missing isbns and tries to populate them from their quotes and annotations.
func updateDBWithISBNs() {

	var books []Book
	err := kscribblerDB.Select(&books, `SELECT book_id, isbn FROM book WHERE isbn IS NULL;`)

	if err != nil {
		log.Printf("failed to load books with missing isbns: %v", err)
		return
	}

	for _, book := range books {
		var quotes []Bookmark
		err := kscribblerDB.Select(&quotes, `
			SELECT 
				bookmark_id,
				book_id,
				quote,
				annotation,
				type,
				kscribbler_uploaded
			FROM quote
			WHERE book_id = ?;
		`, book.BookID)
		if err != nil {
			log.Printf("failed to load quotes for book %s: %v", book.BookID, err)
			continue
		}
		book.Bookmarks = quotes
		if !book.SetIsbnFromBook() {
			continue
		}
	}

	log.Println("Updated missing ISBNs in book table")
	// TODO: figure out overriding precedence - 1. annotation, 2. highlights 3. isbn from KoboDB
	// current approach is only a passthrough of things missing isbn
	// also want to make sure isbn 13 is stored
}

// loadBooksFromDB loads books with pending quotes from the kscribbler database.
func loadBooksFromDB() []Book {
	var books []Book

	err := kscribblerDB.Select(&books, `
		SELECT 
			b.book_id,
			b.book_title,
			b.isbn,
			b.hardcover_id,
			b.hardcover_edition,
			(SELECT COUNT(*) FROM quote q WHERE q.book_id = b.book_id AND q.kscribbler_uploaded = 0) AS pending_quotes
		FROM book b
		WHERE (SELECT COUNT(*) FROM quote q WHERE q.book_id = b.book_id AND q.kscribbler_uploaded = 0) > 0
		AND b.hardcover_id IS NOT NULL
		AND b.hardcover_edition IS NOT NULL
		ORDER BY b.book_id;
		`)
	if err != nil {
		log.Fatalf("failed to load books: %v", err)
	}

	for i := range books {
		err := kscribblerDB.Select(&books[i].Bookmarks, `
			SELECT 
				bookmark_id,
				book_id,
				quote,
				annotation,
				type,
				kscribbler_uploaded
			FROM quote
			WHERE book_id = ? AND kscribbler_uploaded = 0;
		`, books[i].BookID)
		if err != nil {
			log.Fatalf("failed to load bookmarks for book %s: %v", books[i].BookID, err)
		}
	}

	return books
}
