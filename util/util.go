package util

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, os.ModePerm)
}

// EnsureFile will make the file and the path if they don't already exist
// https://stackoverflow.com/a/74322748 for some hints on using syscall to make the permission
func EnsureFile(path string, perm os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		// file already exists
		return nil
	} else if !os.IsNotExist(err) {
		// some other error accessing file
		return err
	}

	// create parent dir
	if err := os.MkdirAll(filepath.Dir(path), perm); err != nil {
		return err
	}

	// create empty file
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
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

// SanitizePathSegment makes a single path segment safe for filesystem usage.
func SanitizePathSegment(segment string) string {
	invalidChars := regexp.MustCompile(`[<>:"/\\|?*]+`)
	safe := invalidChars.ReplaceAllString(segment, "_")
	safe = strings.TrimSpace(safe)
	if safe == "" {
		safe = "_"
	}
	return safe
}

// SanitizeFullPath sanitizes a full path by cleaning each segment individually.
func SanitizeFullPath(path string) string {

	if strings.HasPrefix(path, "/") || path == "" {
		path = path[1:]
	}

	segments := strings.Split(path, "/")
	for i, segment := range segments {
		segments[i] = SanitizePathSegment(segment)
	}
	return strings.Join(segments, "/")
}

func SafeJoin(baseDir, hostPath, unsafePath string) (string, error) {
	safePath := SanitizeFullPath(unsafePath)
	fullPath := filepath.Join(baseDir, hostPath, safePath)
	cleanPath := filepath.Clean(fullPath)

	// Ensure the final path is within the base directory
	if !strings.HasPrefix(cleanPath, filepath.Clean(baseDir)+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path: %s", unsafePath)
	}
	return cleanPath, nil
}

// URLToFilePath maps a URL to a local file path for saving HTML/content.
// If the URL ends in '/', it becomes index.html
func URLToFilePath(baseDir string, rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		// fallback to hashed filename
		hash := sha1.Sum([]byte(rawURL))
		return filepath.Join(baseDir, hex.EncodeToString(hash[:])+".html"), nil
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

	return SafeJoin(baseDir, "/"+hostPath, path)
}

// StripHTMLFile removes the HTML file from the path, if present.
func StripHTMLFile(p string) string {
	ext := path.Ext(p)
	if ext != "" {
		p = path.Dir(p)
	}
	if p == "." {
		p = ""
	}
	if !strings.HasSuffix(p, "/") {
		return p + "/"
	}
	return p
}
