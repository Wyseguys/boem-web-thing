package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"boem-web-thing/util"

	_ "modernc.org/sqlite"
)

// Storage wraps the database connection.
type Storage struct {
	db *sql.DB
}

type Pages struct {
	Id           int
	Url          string
	Status_code  int
	Content_type string
	File_path    string
	Fetched_at   time.Time
	Scan_results string
}

type Links struct {
	Id       int
	From_url string
	To_url   string
}

// New opens (or creates) the SQLite database at the given path.
func New(dbPath string) (*Storage, error) {

	filemode := int(755) //if we are creating te database file, better make it writable
	if err := util.EnsureFile(dbPath, os.FileMode(filemode)); err != nil {
		return nil, fmt.Errorf("failed to ensure db file: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-threaded, so we limit to 1 connection

	// Create tables if they don't exist
	schema := `
	CREATE TABLE IF NOT EXISTS pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL UNIQUE,
		status_code INTEGER,
		content_type TEXT,
		file_path TEXT,
		fetched_at DATETIME,
		scan_results TEXT
	);
	CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_url TEXT NOT NULL,
		to_url TEXT NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return &Storage{db: db}, nil
}

// SavePage inserts or updates a page record.
func (s *Storage) SavePage(url string, status int, contentType string, filePath string) error {

	_, err := s.db.Exec(`
	INSERT INTO pages (url, status_code, content_type, file_path, fetched_at, scan_results)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(url) DO UPDATE SET
		status_code=excluded.status_code,
		content_type=excluded.content_type,
		file_path=excluded.file_path,
		fetched_at=excluded.fetched_at
	`,
		url, status, contentType, filePath, time.Now(), "",
	)

	return err
}

// SaveScan updates a page record with it's scan result.
func (s *Storage) SaveScan(filePath string, result string) error {

	fmt.Println(filePath, result)

	_, err := s.db.Exec(`
	UPDATE pages
	SET scan_results = ?
	WHERE file_path = ?
	`,
		result, filePath)

	if err != nil {
		log.Printf("Error updating scan result for %s: %v", filePath, err)
	}

	return err
}

// SaveLink records a link found on a page.
func (s *Storage) SaveLink(fromURL, toURL string) error {
	_, err := s.db.Exec(`
	INSERT INTO links (from_url, to_url)
	VALUES (?, ?)
	`, fromURL, toURL)
	return err
}

// Get all the output paths for the pages stored based on the configuration files
// list of allowed hosts.
func (s *Storage) GetPagesByAllowedHosts(allowedHost []string) ([]Pages, error) {

	filePaths := make([]Pages, 0)
	query := "SELECT id, url, status_code, content_type, file_path, fetched_at, scan_results FROM pages WHERE file_path like ?"
	allowed_hosts := "%" + strings.Join(allowedHost, "','") + "%"
	rows, err := s.db.Query(query, allowed_hosts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item := Pages{}
		err2 := rows.Scan(&item.Id, &item.Url, &item.Status_code, &item.Content_type, &item.File_path, &item.Fetched_at, &item.Scan_results)
		if err2 != nil {
			panic(err2)
		}
		filePaths = append(filePaths, item)
	}

	return filePaths, nil
}

// Close closes the database connection.
func (s *Storage) Close() error {
	return s.db.Close()
}
