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

func (b Book) String() string {
	var result string

	result += "========== Book ==========\n"
	result += fmt.Sprintf("ContentID: %s\n", b.ContentID)

	if b.ISBN.Valid {
		result += fmt.Sprintf("ISBN: %s\n", b.ISBN.String)
	} else {
		result += "ISBN: (none)\n"
	}

	if b.ISBN10 != "" {
		result += fmt.Sprintf("ISBN-10: %s\n", b.ISBN10)
	}

	if b.ISBN13 != "" {
		result += fmt.Sprintf("ISBN-13: %s\n", b.ISBN13)
	}

	if b.ASIN != "" {
		result += fmt.Sprintf("ASIN: %s\n", b.ASIN)
	}

	result += "\n-- Hardcover Info --\n"
	result += fmt.Sprintf("BookID: %d\n", b.Hardcover.BookID)
	result += fmt.Sprintf("EditionID: %d\n", b.Hardcover.EditionID)
	result += fmt.Sprintf("PrivacyLevel: %d\n", b.Hardcover.PrivacyLevel)

	result += "\n-- Bookmarks --\n"
	for i, bm := range b.Bookmarks {
		result += fmt.Sprintf("[%d]\n", i+1)
		result += fmt.Sprintf("Chapter Title: %s\n", bm.ChapterTitle.String)
		result += fmt.Sprintf("BookmarkID: %s\n", bm.BookmarkID)
		result += fmt.Sprintf("Chapter Progress: %.2f%%\n", bm.ChapterProgress*100)

		if bm.Quote.Valid {
			result += fmt.Sprintf("Quote: %s\n", bm.Quote.String)
		} else {
			result += "Quote: (none)\n"
		}

		if bm.Annotation.Valid {
			result += fmt.Sprintf("Annotation: %s\n", bm.Annotation.String)
		} else {
			result += "Annotation: (none)\n"
		}

		result += fmt.Sprintf("Type: %s\n", bm.Type)
		result += "--------------------------\n"
	}

	return result
}

func ensureKscribblerUploadedColumn(db *sqlx.DB) error {
	var count int
	err := db.Get(&count, `
		SELECT COUNT(*)
		FROM pragma_table_info('Bookmark')
		WHERE name = 'KscribblerUploaded';
	`)
	if err != nil {
		return fmt.Errorf("error checking columns: %w", err)
	}

	if count == 0 {
		// Column doesn't exist, create it
		_, err := db.Exec(`ALTER TABLE Bookmark ADD COLUMN KscribblerUploaded INTEGER DEFAULT 0;`)
		if err != nil {
			return fmt.Errorf("error adding column: %w", err)
		}
		log.Println("Added missing column KscribblerUploaded to Bookmark table")
	} else {
		log.Println("KscribblerUploaded column already exists")
	}

	return nil
}

const apiURL = "https://api.hardcover.app/v1/graphql"

var dbPath string = "/home/gianni/go/src/kscribbler/KoboReader.sqlite"
var db *sqlx.DB
var currentBook Book
var authToken string

type PrivacyLevel int

const (
	PrivacyPublic    = 1
	PrivacyFollowers = 2
	PrivacyPrivate   = 3
)

type Book struct {
	ContentID string `db:"ContentID"`
	// new books have isbn 13 *always
	// 10 can be converted into 13
	ISBN      sql.NullString `db:"ISBN"`
	ISBN10    string
	ISBN13    string
	ASIN      string
	Bookmarks []Bookmark
	Hardcover Hardcover
}

type Hardcover struct {
	BookID       int
	EditionID    int
	PrivacyLevel PrivacyLevel
}

// a single book will have multiple bookmarks(quotes|notes) with unique BookmarkIDs
type Bookmark struct {
	BookmarkID         string         `db:"BookmarkID"`
	ContentID          string         `db:"ContentID"`
	ChapterProgress    float64        `db:"ChapterProgress"`
	Quote              sql.NullString `db:"Text"`
	Annotation         sql.NullString `db:"Annotation"`
	Type               string         `db:"Type"`
	ChapterTitle       sql.NullString `db:"ChapterTitle"`
	KscribblerUploaded bool           `db:"KscribblerUploaded"`
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
	var err error
	db, err = sqlx.Open("sqlite", dbPath)

	if err != nil {
		log.Print("Error opening database")
		log.Fatal(err)
	}

	ensureKscribblerUploadedColumn(db)

	cidquery := `
		SELECT c.ContentID, c.ISBN
		FROM content c
		WHERE c.ContentType = 6
			AND c.DateLastRead IS NOT NULL
		ORDER BY c.DateLastRead DESC
		LIMIT 1;
	`
	err = db.Get(&currentBook, cidquery)

	if err != nil {
		log.Fatal("Error getting last opened ContentID:", err)
	}

	err = db.Select(&currentBook.Bookmarks, `
	SELECT
	b.BookmarkID,
	b.ContentID,
	b.ChapterProgress,
	b.Text,
	b.Annotation,
	b.Type,
	c.Title AS ChapterTitle
	FROM Bookmark b
	LEFT JOIN content c ON b.ContentID = c.ContentID
	WHERE b.ContentID LIKE ?
	AND b.Type != 'dogear'
	AND b.Text IS NOT NULL;
	`, currentBook.ContentID+"%")

	if err != nil {
		log.Fatal("Error getting bookmarks:", err)
	}

	currentBook.Hardcover.PrivacyLevel = 1 // public by default
}

func newHardcoverRequest(ctx context.Context, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authToken)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// flesh out struct and associte book to hardcover
func (book *Book) koboToHardcover(client *http.Client, ctx context.Context) {

	var filters []string
	if book.ISBN13 != "" {
		filters = append(filters, fmt.Sprintf(`{isbn_13: {_eq: "%s"}}`, book.ISBN13))
	}
	if book.ISBN10 != "" {
		filters = append(filters, fmt.Sprintf(`{isbn_10: {_eq: "%s"}}`, book.ISBN10))
	}
	if book.ASIN != "" {
		filters = append(filters, fmt.Sprintf(`{asin: {_eq: "%s"}}`, book.ASIN))
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

	req, err := newHardcoverRequest(ctx, bodyBytes)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)
	var findBookResp Response
	if err := json.Unmarshal(rawResp, &findBookResp); err != nil {
		log.Println("Raw response:\n", string(rawResp))
		log.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(findBookResp.Data.Books) < 1 || len(findBookResp.Data.Books[0].Editions) < 1 {
		log.Fatalf(
			"Unable to ID Books from ISBN10: %s, ISBN13: %s, ASIN: %s",
			book.ISBN10,
			book.ISBN13,
			book.ASIN,
		)
	}

	book.Hardcover.BookID = findBookResp.Data.Books[0].ID
	book.Hardcover.EditionID = findBookResp.Data.Books[0].Editions[0].ID
}

func (bm Bookmark) hasBeenUploaded(db *sqlx.DB) (error, bool) {
	var isUploaded int

	err := db.Get(&isUploaded, `
		SELECT KscribblerUploaded
		FROM Bookmark
		WHERE BookmarkID = ?
	`, bm.BookmarkID)
	if err != nil {
		return err, false
	}

	return nil, isUploaded == 1
}

func (bm Bookmark) markAsUploaded() error {
	_, err := db.Exec(`
		UPDATE Bookmark
		SET KscribblerUploaded = 1
		WHERE BookmarkID = ?
	`, bm.BookmarkID)
	if err != nil {
		return fmt.Errorf("failed to mark bookmark as uploaded: %w", err)
	}
	return nil
}

func (entry Bookmark) postEntry(
	client *graphql.Client,
	ctx context.Context,
	hardcoverID int,
	spoiler bool,
	privacyLevel PrivacyLevel,
) error {
	err, isUploaded := entry.hasBeenUploaded(db)
	if err != nil {
		log.Fatalf("failed to check if entry has been uploaded: %v", err)
	}
	if isUploaded {
		log.Printf("Entry has already been uploaded, skipping: %s", entry.BookmarkID)
		return nil
	}

	hardcoverType := "quote"
	if entry.Type == "annotation" {
		hardcoverType = "note"
	}

	mutation := fmt.Sprintf(`
	mutation postquote {
    insert_reading_journal(
    object: {book_id: %d, event: "%s", tags: {spoiler: %t, category: "%s", tag: ""}, entry: """%s""", privacy_setting_id: %d}
     ) {
    errors
  }
}`,
		hardcoverID, hardcoverType, spoiler,
		hardcoverType, entry.Quote.String, privacyLevel)

	req := graphql.NewRequest(mutation)
	req.Header.Set("Authorization", authToken)

	var resp Response

	if err := client.Run(ctx, req, &resp); err != nil {
		log.Printf("Error making GraphQL request %v ", err)
	} else {
		err := entry.markAsUploaded()
		return err
	}

	return nil

}

func main() {

	defer db.Close()
	graphClient := graphql.NewClient(apiURL)
	ctx := context.Background()

	// testmark.ISBN10 = "081257558X"
	// testmark.ISBN13 = "9780812575583"

	client := &http.Client{}
	currentBook.ISBN13 = "9780812575583" // still need to deal with isbn
	currentBook.koboToHardcover(client, ctx)

	err := currentBook.Bookmarks[0].postEntry(
		graphClient,
		ctx,
		currentBook.Hardcover.BookID,
		false,
		currentBook.Hardcover.PrivacyLevel,
	)

	if err != nil {
		log.Printf("There was an error uploading quote to reading journal: %s\n", err)
	}
}
