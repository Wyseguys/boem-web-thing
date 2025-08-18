package util

import "testing"

// TestSanitizePathSegment tests individual path segment sanitization
func TestSanitizePathSegment(t *testing.T) {
	tests := map[string]string{
		"normal":               "normal",
		"with/slash":           "with_slash",
		"../with/leadingslash": ".._with_leadingslash",
		"<>:\"/\\|?*":          "_",
		"   spaced   ":         "spaced",
		"":                     "_",
	}

	for input, expected := range tests {
		result := SanitizePathSegment(input)
		if result != expected {
			t.Errorf("SanitizePathSegment(%q) = %q; want %q", input, result, expected)
		}
	}
}

func TestStripHTMLFile(t *testing.T) {
	tests := map[string]string{
		"/test/site/path/":           "/test/site/path/",
		"/test/site/path/index.html": "/test/site/path/",
		"/test/site/path/page.htm":   "/test/site/path/",
		"/test/site/path/page":       "/test/site/path/page/",
		"/test/site/path/page.php":   "/test/site/path/",
		"/test/site/path/page.pdf":   "/test/site/path/",
		"/index.html":                "/",
		"index.html":                 "/",
		"plainfile":                  "plainfile/",
	}

	for input, expected := range tests {
		result := StripHTMLFile(input)
		if result != expected {
			t.Errorf("StripHTMLFile(%q) = %q; want %q", input, result, expected)
		}
	}
}
