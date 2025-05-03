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
	"regexp"
	"strings"

	"github.com/GianniBYoung/simpleISBN"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var db *sqlx.DB
var currentBook Book
var authToken string
var dbPath = "/mnt/onboard/.kobo/KoboReader.sqlite"


type PrivacyLevel int

const (
	PrivacyPublic    PrivacyLevel = 1
	PrivacyFollowers PrivacyLevel = 2
	PrivacyPrivate   PrivacyLevel = 3
	apiURL                        = "https://api.hardcover.app/v1/graphql"
)

type Book struct {
	ContentID string         `db:"ContentID"`
	KoboISBN  sql.NullString `db:"ISBN"`
	ISBN      simpleISBN.ISBN
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

func (b Book) String() string {
	var result string

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

	result += "========== Book ==========\n"
	result += fmt.Sprintf("ContentID: %s\n", b.ContentID)

	result += fmt.Sprintf("ISBN: %s\n", b.ISBN)

	result += "\n-- Hardcover Info --\n"
	result += fmt.Sprintf("BookID: %d\n", b.Hardcover.BookID)
	result += fmt.Sprintf("EditionID: %d\n", b.Hardcover.EditionID)
	result += fmt.Sprintf("PrivacyLevel: %d\n", b.Hardcover.PrivacyLevel)

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

func (b *Book) SetIsbn() error {
	isbn, err := simpleISBN.NewISBN(b.KoboISBN.String)
	if err != nil {
		return err
	}
	b.ISBN = *isbn
	return nil
}

func (b *Book) SetIsbnFromHighlight() (error, bool) {
	isbn10Regex := regexp.MustCompile(`[0-9][-0-9]{8,12}[0-9Xx]`)
	isbn13Regex := regexp.MustCompile(`97[89][-0-9]{10,16}`)

	for _, bm := range b.Bookmarks {
		if !bm.Quote.Valid {
			continue
		}

		text := strings.TrimSpace(bm.Quote.String)

		// Ignore if the highlight is very long (user probably highlighted a sentence)
		if len(text) > 45 {
			continue
		}

		var isbn *simpleISBN.ISBN
		var err error
		var match string
		if isbn13Regex.MatchString(text) {
			match = isbn13Regex.FindString(text)
		} else if isbn10Regex.MatchString(text) {
			match = isbn10Regex.FindString(text)
		} else {
			continue
		}
		log.Println("ISBN FOUND")
		log.Println(match)

		//potentially handle the fatal by removing problem quote?
		isbn, err = simpleISBN.NewISBN(match)
		if err != nil {
			log.Fatalf("ISBN matched from highlight but failed to parse:\n%s\n%s", match, err)
		}
		b.ISBN = *isbn

		// Update the content table with the found ISBN as isbn-13
		_, err = db.Exec(`
			UPDATE content
			SET ISBN = ?
			WHERE ContentID = ?
		`, isbn.ISBN13Number, b.ContentID)
		if err != nil {
			log.Printf("Failed to update ISBN for book: %v", err)
			continue
		}

		// Delete the bookmark after updating
		//TODO: DELETE THIS FROM THE STRUCT
		_, err = db.Exec(`
			DELETE FROM Bookmark
			WHERE BookmarkID = ?
		`, bm.BookmarkID)
		if err != nil {
			log.Printf("Failed to delete Bookmark %s: %v", bm.BookmarkID, err)
		} else {
			log.Printf("Deleted BookmarkID %s after extracting ISBN", bm.BookmarkID)
		}

		return err, true

	}
	return nil, false
}

func init() {

	authToken = os.Getenv("HARDCOVER_API_TOKEN")
	if authToken == "" {
		log.Fatal("HARDCOVER_API_TOKEN is not set")
	}

	if devDBPath := os.Getenv("KSCRIBBLER_DB_PATH"); devDBPath != "" {
		dbPath = devDBPath
	}

	var err error
	db, err = sqlx.Open("sqlite", dbPath)

	if err != nil {
		log.Print("Error opening database")
		log.Fatal(err)
	}

	err = ensureKscribblerUploadedColumn(db)
	if err != nil {
		log.Println("error creating KscribblerUploaded column")
		log.Fatal(err)
	}

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
	if len(currentBook.Bookmarks) == 0 {
		log.Println("Exiting. No highlights found")
		os.Exit(0)
	}

	if currentBook.KoboISBN.Valid == false {
		log.Println("Attempting to set isbn from highlights")
		err, isbnFound := currentBook.SetIsbnFromHighlight()
		if err != nil || isbnFound == false {
			log.Println(err)
			log.Fatal(
				"ISBN is missing. Please highlight a valid isbn within the book or create a new annotation containing `kscribbler:config:ISBN-xxxxxx`",
			)
		}
	} else {
		err = currentBook.SetIsbn()
		if err != nil {
			fmt.Printf("Error setting ISBN: %v", err)
		}
		fmt.Println(currentBook.ISBN)
	}

	currentBook.Hardcover.PrivacyLevel = 1 // public by default
	// fmt.Println(currentBook)
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
	if book.ISBN.ISBN13Number != "" {
		filters = append(filters, fmt.Sprintf(`{isbn_13: {_eq: "%s"}}`, book.ISBN.ISBN13Number))
	}
	if book.ISBN.ISBN10Number != "" {
		filters = append(filters, fmt.Sprintf(`{isbn_10: {_eq: "%s"}}`, book.ISBN.ISBN10Number))
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

	// log.Println("Final Query:\n", query)

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
			"Unable to ID Books from ISBN.ISBN10Number: %s, ISBN.ISBN13Number: %s",
			book.ISBN.ISBN10Number,
			book.ISBN.ISBN13Number,
		)
	}

	book.Hardcover.BookID = findBookResp.Data.Books[0].ID
	book.Hardcover.EditionID = findBookResp.Data.Books[0].Editions[0].ID
}

func (bm Bookmark) hasBeenUploaded(db *sqlx.DB) (bool, error) {
	var isUploaded int

	err := db.Get(&isUploaded, `
		SELECT KscribblerUploaded
		FROM Bookmark
		WHERE BookmarkID = ?
	`, bm.BookmarkID)
	if err != nil {
		return false, err
	}

	return isUploaded != 0, nil
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
	client *http.Client,
	ctx context.Context,
	hardcoverID int,
	spoiler bool,
	privacyLevel PrivacyLevel,
) error {
	isUploaded, err := entry.hasBeenUploaded(db)
	if err != nil {
		log.Printf("failed to check if entry has been uploaded: %v", err)
		log.Printf(
			"------\nBookMarkID: %s\nBookmarkType %s\n------\n",
			entry.BookmarkID,
			entry.Type,
		)
		return err
	}
	if isUploaded {
		log.Printf("Entry has already been uploaded, skipping: %s", entry.BookmarkID)
		return nil
	}

	quote := strings.TrimSpace(entry.Quote.String)
	annotation := strings.TrimSpace(entry.Annotation.String)

	entryText := quote

	hardcoverType := "quote"
	if entry.Type == "annotation" {
		hardcoverType = "note"
		entryText = fmt.Sprintf("%s\n\n============\n\n%s", quote, annotation)
		return nil // skip for now because hardcover api has multiline formatting issues
	}

	entryText = strings.ReplaceAll(entryText, `"""`, `\"\"\"`)
	mutation := fmt.Sprintf(`
	mutation postquote {
    insert_reading_journal(
    object: {book_id: %d, event: "%s", tags: {spoiler: %t, category: "%s", tag: ""}, entry: """%s""", privacy_setting_id: %d}
     ) {
    errors
  }
}`,
		hardcoverID, hardcoverType, spoiler,
		hardcoverType, entryText, privacyLevel)

	reqBody := map[string]string{"query": mutation}
	bodyBytes, _ := json.Marshal(reqBody)
	req, err := newHardcoverRequest(ctx, bodyBytes)

	// resp, err := client.Do(req)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error with post response %s", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)
	fmt.Println("Hardcover response:", string(rawResp))
	entry.markAsUploaded()
	return nil

}

func main() {

	defer db.Close()
	ctx := context.Background()

	client := &http.Client{}

	currentBook.koboToHardcover(client, ctx)

	for _, bm := range currentBook.Bookmarks {
		err := bm.postEntry(
			client,
			ctx,
			currentBook.Hardcover.BookID,
			false,
			currentBook.Hardcover.PrivacyLevel,
		)

		if err != nil {
			log.Printf("There was an error uploading quote to reading journal: %s\n", err)
		}
	}
}

// next steps
// maybe parse annotations starting with kscribbler.config - <directive>
// long term logging
// how to trigger the program
// actually write tests (maybe)
// organize this mess
// validate that the isbn matched an existing book in hardcover
// better marking of uploaded
