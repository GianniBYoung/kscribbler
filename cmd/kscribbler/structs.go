package main

import (
	"database/sql"

	"github.com/GianniBYoung/simpleISBN"
)

type PrivacyLevel int

const (
	PrivacyPublic    PrivacyLevel = 1
	PrivacyFollowers PrivacyLevel = 2
	PrivacyPrivate   PrivacyLevel = 3
)

type Hardcover struct {
	BookID       int
	EditionID    int
	PrivacyLevel PrivacyLevel
}

// Represents a book entry from KoboReader.sqlite
type Book struct {
	BookID string         `db:"book_id"`
	Title  sql.NullString `db:"title"`
	//TODO: check this
	ISBN      simpleISBN.ISBN `db:"isbn"`
	Bookmarks []Bookmark
	Hardcover Hardcover
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
