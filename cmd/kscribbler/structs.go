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
	ContentID string         `db:"ContentID"`
	Title     sql.NullString `db:"Title"`
	KoboISBN  sql.NullString `db:"ISBN"`
	ISBN      simpleISBN.ISBN
	Bookmarks []Bookmark
	Hardcover Hardcover
}

// Represents the KoboReader.sqlite for a quote or annotation.
type Bookmark struct {
	BookmarkID         string         `db:"BookmarkID"`
	ContentID          string         `db:"ContentID"`
	Quote              sql.NullString `db:"Text"`
	Annotation         sql.NullString `db:"Annotation"`
	Type               string         `db:"Type"`
	ChapterTitle       sql.NullString `db:"ChapterTitle"`
	KscribblerUploaded bool           `db:"KscribblerUploaded"`
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
