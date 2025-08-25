package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/GianniBYoung/kscribbler/version"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

var authToken string

// fleshes out struct and assocites book to hardcover
// TODO: Think about a more efficient query so i don't hammer the api
// TODO: this also assumes a valid isbn already
func (book *Book) koboToHardcover() {

	if book.SimpleISBN.ISBN10Number == "" && book.SimpleISBN.ISBN13Number == "" {
		log.Printf("Book %s has no valid ISBN to query Hardcover", book.BookID)
		return
	}

	ctx := context.Background()

	client, err := newHTTPClient()
	if err != nil {
		log.Fatalf("Failed to create HTTP client: %v", err)
	}

	var filters []string
	if book.SimpleISBN.ISBN13Number != "" {
		filters = append(
			filters,
			fmt.Sprintf(`{isbn_13: {_eq: "%s"}}`, book.SimpleISBN.ISBN13Number),
		)
	}
	if book.SimpleISBN.ISBN10Number != "" {
		filters = append(
			filters,
			fmt.Sprintf(`{isbn_10: {_eq: "%s"}}`, book.SimpleISBN.ISBN10Number),
		)
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

	// TODO: If i am populating the db first and then uploading quotes later, i don't need to persist this to structs???
	if len(findBookResp.Data.Books) < 1 || len(findBookResp.Data.Books[0].Editions) < 1 {
		log.Printf(
			"Unable to ID Books from ISBN\nISBN10: %s\nISBN13: %s",
			book.SimpleISBN.ISBN10Number,
			book.SimpleISBN.ISBN13Number,
		)
	} else {

		// set the hardcover info in the book struct for later use
		book.HardcoverID = findBookResp.Data.Books[0].ID
		book.HardcoverID = findBookResp.Data.Books[0].Editions[0].ID
	}

}

func (bm Bookmark) hasBeenUploaded() bool {
	var isUploaded int

	err := kscribblerDB.Get(&isUploaded, `
		SELECT kscribbler_uploaded
		FROM quote
		WHERE bookmark_id = ?
	`, bm.BookmarkID)
	if err != nil {
		log.Printf("failed to check if bookmark has been uploaded: %v", err)
		return true
	}

	return isUploaded != 0
}

func (bm Bookmark) markAsUploaded() error {
	log.Printf("Marking bookmark %s as uploaded", bm.BookmarkID)
	_, err := kscribblerDB.Exec(`
		UPDATE quote
		SET kscribbler_uploaded = 1
		WHERE bookmark_id = ?;
	`, bm.BookmarkID)
	log.Printf("Marked bookmark %s as uploaded", bm.BookmarkID)
	if err != nil {
		return fmt.Errorf("failed to mark bookmark as uploaded: %w", err)
	}
	return nil
}

func (entry Bookmark) postEntry(
	client *http.Client,
	ctx context.Context,
	hardcoverID int,
	hardcoverEdition int,
	spoiler bool,
) error {

	if entry.hasBeenUploaded() {
		return nil
	}

	quote := strings.TrimSpace(entry.Quote.String)
	annotation := strings.TrimSpace(entry.Annotation.String)

	entryText := quote

	hardcoverType := "quote"
	if entry.Type == "note" {
		hardcoverType = "annotation"
		entryText = fmt.Sprintf("%s\n\n============\n\n%s", quote, annotation)
		log.Println(
			"Skipping annotation upload until hardcover's api has better multiline support",
			entryText,
		)
		return nil // skip for now because hardcover api has multiline formatting issues
	}

	entryText = strings.ReplaceAll(entryText, `"""`, `\"\"\"`)
	mutation := fmt.Sprintf(`
	mutation postquote {
    insert_reading_journal(
		object: {privacy_setting_id: 1, book_id: %d, edition_id: %d, event: "%s", tags: {spoiler: %t, category: "%s", tag: ""}, entry: """%s""" }
     ) {
    errors
  }
}`,
		hardcoverID, hardcoverEdition, hardcoverType, spoiler,
		hardcoverType, entryText)

	reqBody := map[string]string{"query": mutation}
	bodyBytes, _ := json.Marshal(reqBody)
	req, err := newHardcoverRequest(ctx, bodyBytes)
	if err != nil {
		log.Printf("Error with request creation %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error with post response %s", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)
	fmt.Println("Hardcover response:", string(rawResp))
	err = entry.markAsUploaded()
	if err != nil {
		log.Printf("Failed to mark entry as uploaded: %v", err)
	}

	return nil
}

// Initializes the environment, database, and retrieves the last opened book and its bookmarks.
// This should now prepare the database and the main function will re-open the db connection and upload quotes.
func init() {
	log.Printf("Starting Kscribbler v%s\n", version.Version)

	godotenv.Load("/mnt/onboard/.adds/kscribbler/config.env")
	authToken = os.Getenv("HARDCOVER_API_TOKEN")
	if authToken == "" {
		log.Fatalf(
			"HARDCOVER_API_TOKEN is not set.\nPlease set it in /mnt/onboard/.kobo/.adds/kscribbler/config.env\n",
		)
	}

	if devDBPath := os.Getenv("KSCRIBBLER_DB_PATH"); devDBPath != "" {
		koboDBPath = devDBPath + "/KoboReader.sqlite"
		kscribblerDBPath = devDBPath + "/kscribbler.sqlite"
	}

	if err := createKscribblerTables(); err != nil {
		log.Fatalf("Failed to create kscribbler database: %v", err)
	}

	if err := populateBookTable(); err != nil {
		log.Fatalf("Failed to populate kscribbler book table: %v", err)
	}

	if err := populateQuoteTable(); err != nil {
		log.Fatalf("Failed to populate kscribbler quote table: %v", err)
	}

	kscribblerDB = connectKscribblerDB()
	updateDBWithISBNs()
	log.Println("Updated missing ISBNs in book table")

	updateDBWithHardcoverInfo()
	log.Println("Updated missing Hardcover info in book table")

	kscribblerDB.Close()
	log.Println("kscribblerDB initialized. Ready to upload quotes")
}

func main() {
	ctx := context.Background()

	client, err := newHTTPClient()
	if err != nil {
		log.Fatalf("Failed to create HTTP client: %v", err)
	}

	kscribblerDB = connectKscribblerDB()

	books, err := loadBooksFromDB()

	if err != nil {
		log.Fatalf("Failed to load books from database: %v", err)
	}

	for _, currentBook := range books {
		log.Printf("Processing book: %s\n", currentBook)
		for _, bm := range currentBook.Bookmarks {
			err := bm.postEntry(
				client,
				ctx,
				currentBook.HardcoverID,
				currentBook.HardcoverEdition,
				false,
			)
			log.Printf("Uploaded bookmark: %s\n", bm.BookmarkID)

			if err != nil {
				log.Printf("There was an error uploading quote to reading journal: %s\n", err)
			}
		}
		log.Printf("Finished uploading bookmarks for book: %s\n", currentBook.Title.String)
	}

}
