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

type Bookmark struct {
	BookmarkID      string         `db:"BookmarkID"`
	ContentID       string         `db:"ContentID"`
	ChapterProgress float64        `db:"ChapterProgress"`
	Quote           sql.NullString `db:"Text"`
	Annotation      sql.NullString `db:"Annotation"`
	Type            string         `db:"Type"`
}

// TODO:
type Response struct {
}

func init() {
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

// TODO:
func hardcoverUpload(graphqlQuery string) error {
	authToken := os.Getenv("HARDCOVER_API_TOKEN")
	if authToken == "" {
		log.Fatal("HARDCOVER_API_TOKEN is not set")
	}

	client := graphql.NewClient(apiURL)
	ctx := context.Background()

	req := graphql.NewRequest(graphqlQuery)
	req.Header.Set("Authorization", authToken)

	var resp Response

	if err := client.Run(ctx, req, &resp); err != nil {
		log.Printf("Error making GraphQL request %v ", err)
	}

	return nil

}

func main() {
	output, err := os.Create("/tmp/bookmarks.txt")

	if err != nil {
		log.Fatal("unable to open file")
	}
	defer output.Close()

	for _, bm := range bookmarks {
		log.Printf("Bookmark in %s: %s", bm.ContentID, bm.Quote.String)
		fmt.Fprintf(output, "Bookmark in %s: %s\n", bm.ContentID, bm.Quote.String)
	}
}
