package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/GianniBYoung/simpleISBN"
)

// Represents a book entry from KoboReader.sqlite
type Book struct {
	BookID           string         `db:"book_id"`
	Title            sql.NullString `db:"book_title"`
	FoundISBN        sql.NullString `db:"isbn"`
	SimpleISBN       simpleISBN.ISBN
	Bookmarks        []Bookmark
	HardcoverID      int `db:"hardcover_id"`
	HardcoverEdition int `db:"hardcover_edition"`
	PendingQuotes    int `db:"pending_quotes"`
}

// Represents the KoboReader.sqlite for a quote or annotation.
type Bookmark struct {
	BookmarkID         string         `db:"bookmark_id"`
	BookID             string         `db:"book_id"`
	Quote              sql.NullString `db:"quote"`
	Annotation         sql.NullString `db:"annotation"`
	Type               string         `db:"type"`
	KscribblerUploaded bool           `db:"kscribbler_uploaded"`
}

// http response structure supporting books and reading journal insertions for hardcover.app
type Response struct {
	Data struct {
		Books []struct {
			ID       int    `json:"id"`
			Title    string `json:"title"`
			Editions []struct {
				ID int `json:"id"`
			} `json:"editions"`
		} `json:"books"`
		InsertReadingJournal struct {
			Errors *string `json:"errors"`
		} `json:"insert_reading_journal"`
	} `json:"data"`
}

// Attempts to extract an ISBN from the book's highlights (if it is a highlighted ISBN) or notes beginning with `kscrib:`. Returns true if an ISBN was found and set
func (book *Book) SetIsbnFromBook() bool {
	isbn10Regex := regexp.MustCompile(`[0-9][-0-9]{8,12}[0-9Xx]`)
	isbn13Regex := regexp.MustCompile(`97[89][-0-9]{10,16}`)

	for _, bm := range book.Bookmarks {
		if !bm.Annotation.Valid ||
			!strings.Contains(bm.Annotation.String, strings.ToLower("kscrib:")) {
			continue
		}

		var isbnCanidate string
		if bm.Type == "note" {
			isbnCanidate = strings.TrimSpace(bm.Annotation.String)
		} else {
			isbnCanidate = strings.TrimSpace(bm.Quote.String)
		}
		isbnCanidate = strings.ToLower(isbnCanidate)

		var isbnCleaner = strings.NewReplacer(
			" ", "",
			"-", "",
			"isbn", "",
			"(", "",
			")", "",
			"e-book", "",
			"ebook", "",
			"kscrib:", "",
			"electronic", "",
			"book", "",
		)
		isbnCanidate = isbnCleaner.Replace(isbnCanidate)

		// Ignore if the highlight is very long (user probably highlighted a sentence)
		if len(isbnCanidate) > 55 {
			continue
		}

		var isbn *simpleISBN.ISBN
		var err error
		var match string
		log.Println("Checking for ISBN in: ", isbnCanidate)
		if isbn13Regex.MatchString(isbnCanidate) {
			log.Println("Found ISBN-13")
			match = isbn13Regex.FindString(isbnCanidate)
		} else if isbn10Regex.MatchString(isbnCanidate) {
			log.Println("Found ISBN-10")
			match = isbn10Regex.FindString(isbnCanidate)
		} else {
			continue
		}
		log.Println(match)

		isbn, err = simpleISBN.NewISBN(match)
		if err != nil {
			log.Printf("ISBN matched from highlight/note but failed to parse:\n%s\n%s", match, err)
			return false
		}
		book.SimpleISBN = *isbn

		// update the book table with the new isbn
		updateString := "UPDATE book SET isbn = ? WHERE id LIKE ?;"
		_, err = koboDB.Exec(updateString, isbn.ISBN13Number, "%"+book.BookID+"%")
		log.Println("Updating content table with ISBN ->", isbn.ISBN13Number)

		if err != nil {
			log.Printf("Failed to update kscribblerDB ISBN for %s: %v", book.Title.String, err)
			return false
		}

	}
	return true
}

// Print info about the book and its bookmarks
func (book Book) String() string {
	var result string

	result += "\n========== Book ==========\n"
	result += fmt.Sprintf("Title: %s\n", book.Title.String)
	result += fmt.Sprintf("BookID: %s\n", book.BookID)
	result += fmt.Sprintf("ISBN: %s", book.SimpleISBN.String())

	result += "\n===== Hardcover Info =====\n"
	result += fmt.Sprintf("HardcoverID: %d\n", book.HardcoverID)
	result += fmt.Sprintf("EditionID: %d\n", book.HardcoverEdition)

	result += "\n======== Bookmarks ========\n"
	for i, bm := range book.Bookmarks {
		result += fmt.Sprintf("[%d]\n", i+1)
		result += fmt.Sprintf("BookmarkID: %s\n", bm.BookmarkID)
		result += fmt.Sprintf("Type: %s\n", bm.Type)

		if bm.Quote.Valid {
			result += fmt.Sprintf("Quote: %s\n", bm.Quote.String)
		}
		if bm.Annotation.Valid {
			result += fmt.Sprintf("Annotation: %s\n", bm.Annotation.String)
		}

		result += "--------------------------\n"
	}

	return result
}
