package storage

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"boem-web-thing/util"

	_ "modernc.org/sqlite"
)

// Storage wraps the database connection.
type Storage struct {
	db *sql.DB
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
		fetched_at DATETIME
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
	INSERT INTO pages (url, status_code, content_type, file_path, fetched_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(url) DO UPDATE SET
		status_code=excluded.status_code,
		content_type=excluded.content_type,
		file_path=excluded.file_path,
		fetched_at=excluded.fetched_at
	`,
		url, status, contentType, filePath, time.Now(),
	)

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

// Close closes the database connection.
func (s *Storage) Close() error {
	return s.db.Close()
}
