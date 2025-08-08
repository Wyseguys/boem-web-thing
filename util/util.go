package util

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, os.ModePerm)
}

// SanitizeFilename takes a string and makes it safe for filesystem usage.
func SanitizeFilename(name string) string {
	// Replace invalid characters with underscores
	invalidChars := regexp.MustCompile(`[<>:"/\\|?*]+`)
	safe := invalidChars.ReplaceAllString(name, "_")
	safe = strings.TrimSpace(safe)
	if safe == "" {
		safe = "_"
	}
	return safe
}

// URLToFilePath maps a URL to a local file path for saving HTML/content.
// If the URL ends in '/', it becomes index.html
func URLToFilePath(baseDir string, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		// fallback to hashed filename
		hash := sha1.Sum([]byte(rawURL))
		return filepath.Join(baseDir, hex.EncodeToString(hash[:])+".html")
	}

	hostPath := SanitizeFilename(parsed.Host)
	path := parsed.Path
	if strings.HasSuffix(path, "/") || path == "" {
		path += "index.html"
	}

	// Remove query strings for filesystem naming, but keep uniqueness by hashing
	if parsed.RawQuery != "" {
		hash := sha1.Sum([]byte(parsed.RawQuery))
		path += "_" + hex.EncodeToString(hash[:])
	}

	return filepath.Join(baseDir, hostPath, SanitizeFilename(path))
}
