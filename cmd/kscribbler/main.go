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
	"slices"
	"strings"

	"github.com/GianniBYoung/kscribbler/version"
	"github.com/GianniBYoung/simpleISBN"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

var db *sqlx.DB
var currentBook Book
var authToken string
var dbPath = "/mnt/onboard/.kobo/KoboReader.sqlite"

// Print info about the book and its bookmarks
func (b Book) String() string {
	var result string

	result += "\n========== Book ==========\n"
	result += fmt.Sprintf("Title: %s\n", b.Title.String)
	result += fmt.Sprintf("ContentID: %s\n", b.ContentID)
	result += fmt.Sprintf("ISBN: %s", b.ISBN)

	result += "\n===== Hardcover Info =====\n"
	result += fmt.Sprintf("BookID: %d\n", b.Hardcover.BookID)
	result += fmt.Sprintf("EditionID: %d\n", b.Hardcover.EditionID)
	result += fmt.Sprintf("PrivacyLevel: %d\n", b.Hardcover.PrivacyLevel)

	result += "\n======== Bookmarks ========\n"
	for i, bm := range b.Bookmarks {
		result += fmt.Sprintf("[%d]\n", i+1)
		result += fmt.Sprintf("Chapter Title: %s\n", bm.ChapterTitle.String)
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

// Modifies the database to ensure the KscribblerUploaded column exists in the Bookmark table.
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

// Sets the ISBN field of the Book struct using the KoboISBN field.
func (b *Book) SetIsbn() error {
	isbn, err := simpleISBN.NewISBN(b.KoboISBN.String)
	if err != nil {
		return err
	}
	b.ISBN = *isbn
	return nil
}

// Attempts to extract an ISBN from the book's highlights or notes beginning with `kscrib:`.
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

		//potentially handle the fatal by removing problem quote?
		isbn, err = simpleISBN.NewISBN(match)
		if err != nil {
			log.Fatalf("ISBN matched from highlight/note but failed to parse:\n%s\n%s", match, err)
		}
		b.ISBN = *isbn

		// Update the content table with the found ISBN as isbn-13
		updateString := "UPDATE content SET ISBN = ? WHERE ContentID LIKE ?;"
		_, err = db.Exec(updateString, isbn.ISBN13Number, "%"+b.ContentID+"%")
		log.Println("Updating content table with ISBN ->", isbn.ISBN13Number)
		log.Println("ContentID:", b.ContentID)
		if err != nil {
			log.Printf("Failed to update ISBN for book: %v", err)
			continue
		}

		markAsUploadedErr := b.Bookmarks[i].markAsUploaded()
		if markAsUploadedErr != nil {
			log.Printf(
				"Failed to mark isbn highlight as uploaded: %v\ncontinuing...",
				markAsUploadedErr,
			)
		}
		// delete the bookmark from the list so we don't upload it
		b.Bookmarks = slices.Delete(b.Bookmarks, i, i+1)

		return err, true

	}
	return nil, false
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
			"Unable to ID Books from ISBN\n ISBN10: %s\nISBN13: %s",
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
		SELECT c.ContentID, c.ISBN, c.Title
		FROM content c
		WHERE c.ContentType = 6
			AND c.DateLastRead IS NOT NULL
		ORDER BY c.DateLastRead DESC
		LIMIT 1;
	`
	err = db.Get(&currentBook, cidquery)
	if strings.HasPrefix(currentBook.ContentID, "file://") {
		currentBook.ContentID = currentBook.ContentID[len("file://"):]
		log.Println("Stripped file:// from ContentID")
		log.Println("Current Book ContentID:", currentBook.ContentID)
	}

	if err != nil {
		log.Fatal("Error getting last opened ContentID:", err)
	}

	err = db.Select(&currentBook.Bookmarks, `
	SELECT
	b.BookmarkID,
	b.ContentID,
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

	if !currentBook.KoboISBN.Valid {
		log.Println("Attempting to set isbn from highlights and notes")
		err, isbnFound := currentBook.SetIsbnFromBook()
		if err != nil || !isbnFound {
			log.Println(err)
			log.Fatal(
				"ISBN is missing. Please highlight a valid isbn within the book or create a new annotation containing `kscrib:isbn-xxxxxx`",
			)
		}
	} else {
		err = currentBook.SetIsbn()
		if err != nil {
			fmt.Printf("Error setting ISBN: %v", err)
		}
	}

	currentBook.Hardcover.PrivacyLevel = 1 // public by default
	fmt.Println(currentBook)
}

func main() {
	defer db.Close()
	ctx := context.Background()

	client, err := newHTTPClient()
	if err != nil {
		log.Fatalf("Failed to create HTTP client: %v", err)
	}

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
	log.Printf("Finished uploading bookmarks for %s to hardcover", currentBook.ContentID)
}
