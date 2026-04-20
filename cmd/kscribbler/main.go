package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
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
var stopAfterInit bool
var markAllAsUploaded bool
var showVersion bool
var uploadAnnotations bool

// koboToHardcover fleshes out struct and assocites book to hardcover.
func (book *Book) koboToHardcover() {
	// TODO: Think about a more efficient query so i don't hammer the api

	// this also assumes a valid isbn already
	if book.SimpleISBN.ISBN10Number == "" && book.SimpleISBN.ISBN13Number == "" {
		log.Printf("Book %s has no valid ISBN to query Hardcover", book.BookID)
		return
	}

	ctx := context.Background()
	client := newHTTPClient()

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

	req := newHardcoverRequest(ctx, bodyBytes)

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
	fmt.Printf("Hardcover response: %+v\n", findBookResp)

	if len(findBookResp.Data.Books) < 1 || len(findBookResp.Data.Books[0].Editions) < 1 {
		log.Printf(
			"Unable to ID Books from ISBN\nISBN10: %s\nISBN13: %s",
			book.SimpleISBN.ISBN10Number,
			book.SimpleISBN.ISBN13Number,
		)
	} else {

		// set the hardcover info in the book struct for later use
		book.HardcoverID = findBookResp.Data.Books[0].ID
		book.HardcoverEdition = findBookResp.Data.Books[0].Editions[0].ID
	}

}

// hasBeenUploaded checks if the bookmark has already been uploaded to Hardcover by querying the kscribblerDB.
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

// markAsUploaded updates the kscribblerDB to mark the quote as uploaded.
func (bm Bookmark) markAsUploaded() {
	log.Printf("Marking bookmark %s as uploaded", bm.BookmarkID)
	_, err := kscribblerDB.Exec(`
		UPDATE quote
		SET kscribbler_uploaded = 1
		WHERE bookmark_id = ?;
	`, bm.BookmarkID)

	if err != nil {
		log.Fatalf("failed to mark bookmark as uploaded: %v", err)
	}
	log.Printf("Marked bookmark %s as uploaded", bm.BookmarkID)
}

// postEntry uploads the bookmark (quote or annotation) to Hardcover using their GraphQL API.
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
		if !uploadAnnotations {
			log.Printf("Skipping annotation (UPLOAD_ANNOTATIONS is not enabled): %s", entry.BookmarkID)
			return nil
		}
		entryText = fmt.Sprintf("%s\n\n---\n\n%s", quote, annotation)
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
	req := newHardcoverRequest(ctx, bodyBytes)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error with post response %s", err)
		return err
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return err
	}
	fmt.Println("Hardcover response:", string(rawResp))

	// Parse the response to check for errors
	var response Response
	err = json.Unmarshal(rawResp, &response)
	if err != nil {
		log.Printf("Error unmarshaling response: %v", err)
		return err
	}

	// Check for top-level GraphQL errors first
	if len(response.Errors) > 0 {
		log.Printf("Hardcover API returned GraphQL error: %s", response.Errors[0].Message)
		return fmt.Errorf("hardcover GraphQL error: %s", response.Errors[0].Message)
	}

	// Check if there were errors from Hardcover API
	if response.Data.InsertReadingJournal.Errors != nil &&
		*response.Data.InsertReadingJournal.Errors != "" {
		log.Printf("Hardcover API returned error: %s", *response.Data.InsertReadingJournal.Errors)
		return fmt.Errorf("hardcover API error: %s", *response.Data.InsertReadingJournal.Errors)
	}

	// Only mark as uploaded if there were no errors
	entry.markAsUploaded()

	return nil
}

// Initializes the environment, database, and retrieves the last opened book and its bookmarks.
func init() {
	flag.BoolVar(&stopAfterInit, "init", false, "Stop execution after init() runs")
	flag.BoolVar(
		&markAllAsUploaded,
		"mark-all-as-uploaded",
		false,
		"Mark all quotes in the database as uploaded (useful for migration)",
	)
	flag.BoolVar(&showVersion, "version", false, "Show version information and exit")
	flag.Parse()
	if showVersion {
		fmt.Printf("v%s", version.Version)
		os.Exit(0)
	}
	log.Printf("Starting Kscribbler v%s\n", version.Version)

	godotenv.Load("/mnt/onboard/.adds/kscribbler/config.env")
	authToken = os.Getenv("HARDCOVER_API_TOKEN")
	uploadAnnotations = strings.ToLower(os.Getenv("UPLOAD_ANNOTATIONS")) == "true"
	if authToken == "" {
		log.Fatalf(
			"HARDCOVER_API_TOKEN is not set.\nPlease set it in /mnt/onboard/.kobo/.adds/kscribbler/config.env\n",
		)
	}

	if devDBPath := os.Getenv("KSCRIBBLER_DB_PATH"); devDBPath != "" {
		koboDBPath = devDBPath + "/KoboReader.sqlite"
		kscribblerDBPath = devDBPath + "/kscribbler.sqlite"
	}

	// create kscribblerDB and populate it with relevant data from KoboReader.sqlite
	createKscribblerTables()
	populateBookTable()
	populateQuoteTable()

	// Supplement book entries with ISBNs and Hardcover info
	kscribblerDB = connectKscribblerDB()
	syncISBNsFromKoboDB()
	updateDBWithISBNs()
	updateDBWithHardcoverInfo()
	kscribblerDB.Close()

	log.Println("kscribblerDB initialized. Ready to upload quotes")
}

func main() {
	if stopAfterInit {
		log.Println(
			"The init flag was set; stopping execution after database initialization. Quotes will not be uploaded.",
		)
		os.Exit(0)
	}

	if markAllAsUploaded {
		log.Println(
			"The mark-all-as-uploaded flag was set; marking all quotes in kscribblerDB as uploaded. Quotes will not be uploaded.",
		)
		kscribblerDB = connectKscribblerDB()
		_, err := kscribblerDB.Exec(` UPDATE quote SET kscribbler_uploaded = 1;`)

		if err != nil {
			log.Fatalf("failed to mark all quotes as uploaded in kscribblerDB: %v", err)
		}
		log.Println("All quotes marked as uploaded. Exiting.")
		os.Exit(0)
	}

	ctx := context.Background()
	client := newHTTPClient()
	verifyHardcoverConnection(client, ctx)

	kscribblerDB = connectKscribblerDB()
	defer kscribblerDB.Close()
	books := loadBooksFromDB()

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

			if err != nil {
				log.Printf("There was an error uploading quote to reading journal: %s\n", err)
			} else {
				log.Printf("Uploaded bookmark: %s\n", bm.BookmarkID)
			}
		}
		log.Printf("Finished uploading bookmarks for book: %s\n", currentBook.Title.String)
	}
	log.Println("Job done!")
}
