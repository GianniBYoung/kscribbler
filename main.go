package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/machinebox/graphql"
	_ "modernc.org/sqlite"
)

const apiURL = "https://api.hardcover.app/v1/graphql"

var dbPath string = "/home/gianni/go/src/kscribbler/KoboReader.sqlite"
var bookmarks []Bookmark
var authToken string

type PrivacyLevel int
type EntryType string

const (
	PrivacyPublic    = 1
	PrivacyFollowers = 2
	PrivacyPrivate   = 3
	EntryQuote       = "quote"
	EntryNote        = "note"
)

type Bookmark struct {
	// info from kobo
	BookmarkID      string         `db:"BookmarkID"`
	ContentID       string         `db:"ContentID"`
	ChapterProgress float64        `db:"ChapterProgress"`
	Quote           sql.NullString `db:"Text"`
	Annotation      sql.NullString `db:"Annotation"`
	Type            string         `db:"Type"`
	ISBN10          string
	ISBN13          string
	ASIN            string
	Spoiler         bool //idk how to find this
	// location data?
	// hard cover info in a sep struct for unmarshalling eaze
	Hardcover struct {
		BookID       int
		EditionID    int
		PrivacyLevel PrivacyLevel
		Type         EntryType
	}
}

type Response struct {
	Data struct {
		InsertReadingJournal struct {
			Errors *string `json:"errors"`
		} `json:"insert_reading_journal"`
	} `json:"data"`
}

func init() {
	authToken = os.Getenv("HARDCOVER_API_TOKEN")
	if authToken == "" {
		log.Fatal("HARDCOVER_API_TOKEN is not set")
	}
	db, err := sqlx.Open("sqlite", dbPath)

	if err != nil {
		log.Print("Error opening database")
		log.Fatal(err)
	}
	defer db.Close()

	query := "SELECT BookmarkID, ContentID, ChapterProgress, Text, Annotation, Type FROM Bookmark"
	err = db.Select(&bookmarks, query)
	if err != nil {
		log.Print("Error with query")
		log.Fatalln(err)
	}
}

func (entry Bookmark) postEntry(client *graphql.Client, ctx context.Context) error {

	if authToken == "" {
		log.Fatal("HARDCOVER_API_TOKEN is not set in postEntry")
	}
	// need to figure out how to handle quote vs annotation
	mutation := fmt.Sprintf(`
	mutation postquote {
    insert_reading_journal(
    object: {book_id: %d, event: "%s", tags: {spoiler: %t, category: "%s", tag: ""}, entry: """%s""", privacy_setting_id: %d}
     ) {
    errors
  }
}`,
		entry.Hardcover.BookID, entry.Hardcover.Type, entry.Spoiler,
		entry.Type, entry.Quote.String, entry.Hardcover.PrivacyLevel)

	req := graphql.NewRequest(mutation)
	req.Header.Set("Authorization", authToken)

	var resp Response

	if err := client.Run(ctx, req, &resp); err != nil {
		log.Printf("Error making GraphQL request %v ", err)
	}
	fmt.Println(mutation)
	fmt.Println(resp)

	return nil

}

func main() {

	client := graphql.NewClient(apiURL)
	ctx := context.Background()

	testmark := bookmarks[0]
	log.Printf("Test Bookmark is %v", testmark)

	testmark.Hardcover.BookID = 428605
	testmark.Hardcover.PrivacyLevel = PrivacyPrivate
	testmark.Hardcover.Type = "quote"
	err := testmark.postEntry(client, ctx)

	if err != nil {
		log.Printf("There was an error uploading quote to reading journal: %s\n", err)
	}

	log.Println("Execution done")

	// for _, bm := range bookmarks {
	// 	log.Printf("Bookmark in %s: %s", bm.ContentID, bm.Quote.String)
	// 	// fmt.Fprintf(output, "Bookmark in %s: %s\n", bm.ContentID, bm.Quote.String)
	// }

}
