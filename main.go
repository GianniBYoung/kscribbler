package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

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
	Spoiler         bool //idk how to find this yer
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
		Books []struct {
			ID       int    `json:"id"`
			Title    string `json:"title"`
			Editions []struct {
				ID int `json:"id"`
			} `json:"editions"`
		} `json:"books"`
		// InsertReadingJournal struct {
		// 	Errors *string `json:"errors"`
		// } `json:"insert_reading_journal"`
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

// flesh out struct and associte book to hardcover
func (bm *Bookmark) KoboToHardcover(ctx context.Context) {
	bm.Hardcover.Type = EntryQuote

	if bm.Type == "annotation" { // double check this
		// handle annotated note here?
		bm.Hardcover.Type = EntryNote
	}

	var filters []string
	if bm.ISBN13 != "" {
		filters = append(filters, fmt.Sprintf(`{isbn_13: {_eq: "%s"}}`, bm.ISBN13))
	}
	if bm.ISBN10 != "" {
		filters = append(filters, fmt.Sprintf(`{isbn_10: {_eq: "%s"}}`, bm.ISBN10))
	}
	if bm.ASIN != "" {
		filters = append(filters, fmt.Sprintf(`{asin: {_eq: "%s"}}`, bm.ASIN))
	}

	orBlock := strings.Join(filters, ", ")

	query := fmt.Sprintf(`
		query findById {
			books(
				where: {
					editions: {
						_or: [%s]
					}
				}
			) {
				id
				title
				editions(
					where: {
						_or: [%s]
					}
				) {
					id
				}
			}
		}`, orBlock, orBlock)

	fmt.Println("Final Query:\n", query)

	// Build JSON payload
	requestBody := map[string]string{"query": query}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Fatalf("Failed to encode GraphQL request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)
	fmt.Println("Raw response:\n", string(rawResp))

	var findBookResp Response
	if err := json.Unmarshal(rawResp, &findBookResp); err != nil {
		log.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(findBookResp.Data.Books) < 1 || len(findBookResp.Data.Books[0].Editions) < 1 {
		log.Printf("Response:\n%+v\n", findBookResp)
		log.Fatalf(
			"Unable to ID Books from ISBN10: %s, ISBN13: %s, ASIN: %s",
			bm.ISBN10,
			bm.ISBN13,
			bm.ASIN,
		)
	}

	bm.Hardcover.BookID = findBookResp.Data.Books[0].ID
	bm.Hardcover.EditionID = findBookResp.Data.Books[0].Editions[0].ID
}

func (entry Bookmark) postEntry(client *graphql.Client, ctx context.Context) error {

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

	ctx := context.Background()

	testmark := bookmarks[0]
	log.Printf("Test Bookmark is %v", testmark)

	// testmark.Hardcover.BookID = 428605
	testmark.ISBN10 = "081257558X"
	testmark.ISBN13 = "9780812575583"
	testmark.Hardcover.PrivacyLevel = PrivacyPrivate
	// testmark.Hardcover.Type = "quote"
	// err := testmark.postEntry(client, ctx)
	// if err != nil {
	// log.Printf("There was an error uploading quote to reading journal: %s\n", err)
	// }

	testmark.KoboToHardcover(ctx)

	log.Println("Execution done")

	// for _, bm := range bookmarks {
	// 	log.Printf("Bookmark in %s: %s", bm.ContentID, bm.Quote.String)
	// 	// fmt.Fprintf(output, "Bookmark in %s: %s\n", bm.ContentID, bm.Quote.String)
	// }

}
