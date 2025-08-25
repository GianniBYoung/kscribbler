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
	"regexp"
	"strings"

	"github.com/GianniBYoung/kscribbler/version"
	"github.com/GianniBYoung/simpleISBN"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

var currentBook Book
var authToken string
var kscribblerDB *sqlx.DB

// Print info about the book and its bookmarks
func (book Book) String() string {
	var result string

	result += "\n========== Book ==========\n"
	result += fmt.Sprintf("Title: %s\n", book.Title.String)
	result += fmt.Sprintf("ContentID: %s\n", book.BookID)
	result += fmt.Sprintf("ISBN: %s", book.ISBN)

	result += "\n===== Hardcover Info =====\n"
	result += fmt.Sprintf("BookID: %d\n", book.Hardcover.BookID)
	result += fmt.Sprintf("EditionID: %d\n", book.Hardcover.EditionID)

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

// // Sets the ISBN field of the Book struct using the KoboISBN field.
// // TODO: can i unmarshal directly to simpleISBN.ISBN?
// func (b *Book) SetIsbn() error {
// 	isbn, err := simpleISBN.NewISBN(b.ISBN)
// 	if err != nil {
// 		return err
// 	}
// 	b.ISBN = *isbn
// 	return nil
// }

// Attempts to extract an ISBN from the book's highlights (if it is a highlighted ISBN) or notes beginning with `kscrib:`.
func (b *Book) SetIsbnFromBook() (error, bool) {
	isbn10Regex := regexp.MustCompile(`[0-9][-0-9]{8,12}[0-9Xx]`)
	isbn13Regex := regexp.MustCompile(`97[89][-0-9]{10,16}`)

	for i, bm := range b.Bookmarks {
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
			log.Fatalf("ISBN matched from highlight/note but failed to parse:\n%s\n%s", match, err)
		}
		b.ISBN = *isbn

		// update the book table with the new isbn
		updateString := "UPDATE book SET isbn = ? WHERE id LIKE ?;"
		_, err = koboDB.Exec(updateString, isbn.ISBN13Number, "%"+b.BookID+"%")
		log.Println("Updating content table with ISBN ->", isbn.ISBN13Number)

		if err != nil {
			log.Printf("Failed to update ISBN for book: %v", err)
			return err, true
		}

	}
	return nil, false
}

// fleshes out struct and associte book to hardcover
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
			"Unable to ID Books from ISBN\nISBN10: %s\nISBN13: %s",
			book.ISBN.ISBN10Number,
			book.ISBN.ISBN13Number,
		)
	}

	book.Hardcover.BookID = findBookResp.Data.Books[0].ID
	book.Hardcover.EditionID = findBookResp.Data.Books[0].Editions[0].ID
	if book.updateHardcoverInfo() != nil {
		log.Fatalf("Failed to update hardcover info in book table %v", err)
	}

}

func (book Book) updateHardcoverInfo() error {
	if book.Hardcover.BookID != -1 || book.Hardcover.EditionID != -1 {
		updateString := "UPDATE book SET hardcover_id = ?, hardcover_edition = ? WHERE id LIKE ?;"
		_, err := koboDB.Exec(
			updateString,
			book.Hardcover.BookID,
			book.Hardcover.EditionID,
			"%"+book.BookID+"%",
		)
		if err != nil {
			log.Printf(
				"Failed to update hardcover_id and hardcover_edition for %s: %v",
				book.Title.String,
				err,
			)
		}
	}
	return nil
}

func (bm Bookmark) hasBeenUploaded(db *sqlx.DB) (bool, error) {
	var isUploaded int

	err := kscribblerDB.Get(&isUploaded, `
		SELECT kscribbler_uploaded
		FROM quote
		WHERE bookmark_id = ?
	`, bm.BookmarkID)
	if err != nil {
		return false, err
	}

	return isUploaded != 0, nil
}

func (bm Bookmark) markAsUploaded() error {
	_, err := koboDB.Exec(`
		UPDATE quote
		SET kscribbler_uploaded = 1
		WHERE bookmark_id = ?
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
) error {
	isUploaded, err := entry.hasBeenUploaded(koboDB)
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
	if entry.Type == "note" {
		hardcoverType = "annotation"
		entryText = fmt.Sprintf("%s\n\n============\n\n%s", quote, annotation)
		fmt.Println(
			"Skipping annotation upload until hardcover's api has better multiline support",
			entryText,
		)
		return nil // skip for now because hardcover api has multiline formatting issues
	}

	entryText = strings.ReplaceAll(entryText, `"""`, `\"\"\"`)
	mutation := fmt.Sprintf(`
	mutation postquote {
    insert_reading_journal(
    object: {book_id: %d, event: "%s", tags: {spoiler: %t, category: "%s", tag: ""}, entry: """%s""" }
     ) {
    errors
  }
}`,
		hardcoverID, hardcoverType, spoiler,
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
// This has a timing issue since the last opened book depends on when the KoboReader.sqlite database was last updated which may not be immediate after a book is opened.
func init() {
	log.Printf("Kscribbler v%s\n", version.Version)

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
	log.Println("Book population done")

	if err := populateQuoteTable(); err != nil {
		log.Fatalf("Failed to populate kscribbler quote table: %v", err)
	}
	log.Println("Quote population done")
	log.Println("Kscribbler init done")
}

func main() {
	ctx := context.Background()

	client, err := newHTTPClient()
	if err != nil {
		log.Fatalf("Failed to create HTTP client: %v", err)
	}

	kscribblerDB = connectKscribblerDB()

	currentBook.koboToHardcover(client, ctx)

	// for _, bm := range currentBook.Bookmarks {
	// 	err := bm.postEntry(
	// 		client,
	// 		ctx,
	// 		currentBook.Hardcover.BookID,
	// 		false,
	// 	)
	//
	// 	if err != nil {
	// 		log.Printf("There was an error uploading quote to reading journal: %s\n", err)
	// 	}
	// }
	// log.Printf("Finished uploading bookmarks for %s to hardcover", currentBook.ContentID)
}
