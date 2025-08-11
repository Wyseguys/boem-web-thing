package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wyseguys/boem-web-thing/config"
	"github.com/wyseguys/boem-web-thing/logger"
	"github.com/wyseguys/boem-web-thing/storage"
	"github.com/wyseguys/boem-web-thing/util"
	"golang.org/x/net/html"
)

type Crawler struct {
	cfg     *config.Config
	log     *logger.Logger
	store   *storage.Storage
	client  *http.Client
	visited map[string]bool
	mu      sync.Mutex
	wg      sync.WaitGroup
}

func New(cfg *config.Config, log *logger.Logger, store *storage.Storage) *Crawler {
	return &Crawler{
		cfg:   cfg,
		log:   log,
		store: store,
		client: &http.Client{
			Timeout: time.Duration(cfg.HTTPTimeout) * time.Second,
		},
		visited: make(map[string]bool),
	}
}

// Crawl starts crawling from the StartURL
func (c *Crawler) Crawl() {
	startURL := c.cfg.StartURL
	c.log.Info("Starting crawl at", startURL)

	urlCh := make(chan string, c.cfg.Concurrency*2)
	doneCh := make(chan struct{})

	// Start worker goroutines
	for i := 0; i < c.cfg.Concurrency; i++ {
		c.wg.Add(1)
		go c.worker(urlCh, doneCh)
	}

	// Seed the queue
	urlCh <- startURL

	// Wait for all workers to finish
	c.wg.Wait()
	close(doneCh)
	close(urlCh)
}

func (c *Crawler) worker(urlCh chan string, doneCh <-chan struct{}) {
	defer c.wg.Done()

	for {
		select {
		case <-doneCh:
			return
		case u, ok := <-urlCh:
			if !ok {
				return
			}
			go c.processURL(u, urlCh)
		}
	}
}

func (c *Crawler) processURL(u string, urlCh chan string) {
	// Check if visited already
	c.mu.Lock()
	if c.visited[u] {
		c.mu.Unlock()
		return
	}
	c.visited[u] = true
	c.mu.Unlock()

	// Fetch
	status, contentType, filePath, links, err := c.fetchAndSave(u)
	if err != nil {
		c.log.Error("Error fetching", u, ":", err)
		return
	}

	// Save page record
	if err := c.store.SavePage(u, status, contentType, filePath); err != nil {
		c.log.Error("DB save error for", u, ":", err)
	}

	// Save links and enqueue new ones
	for _, link := range links {
		c.store.SaveLink(u, link)

		if c.shouldVisit(link) {
			urlCh <- link
		}
	}

	// Rate limiting
	time.Sleep(time.Duration(c.cfg.RateMs) * time.Millisecond)
}

func (c *Crawler) shouldVisit(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if len(c.cfg.AllowedHosts) > 0 {
		hostAllowed := false
		for _, allowed := range c.cfg.AllowedHosts {
			if strings.EqualFold(parsed.Host, allowed) {
				hostAllowed = true
				break
			}
		}
		if !hostAllowed {
			return false
		}
	}
	return true
}

func (c *Crawler) fetchAndSave(rawURL string) (status int, contentType string, filePath string, links []string, err error) {
	c.log.Debug("Fetching", rawURL)

	resp, err := c.client.Get(rawURL)
	if err != nil {
		return 0, "", "", nil, err
	}
	defer resp.Body.Close()

	status = resp.StatusCode
	contentType = resp.Header.Get("Content-Type")

	// Save only HTML and similar; still store others but don't parse links
	filePath = util.URLToFilePath(c.cfg.OutputDir, rawURL)
	if err := util.EnsureDir(filepath.Dir(filePath)); err != nil {
		return status, contentType, "", nil, fmt.Errorf("failed to create dir: %w", err)
	}

	// Write response to disk
	out, err := os.Create(filePath)
	if err != nil {
		return status, contentType, "", nil, err
	}
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return status, contentType, "", nil, err
	}

	// Re-fetch content for parsing (only if HTML)
	if strings.Contains(contentType, "text/html") && status == http.StatusOK {
		// We re-read from file to avoid touching the live network twice
		f, err := os.Open(filePath)
		if err != nil {
			return status, contentType, filePath, nil, err
		}
		defer f.Close()

		pageLinks, err := extractLinks(rawURL, f)
		if err != nil {
			c.log.Error("Link parse error for", rawURL, ":", err)
		} else {
			links = pageLinks
		}
	}

	return status, contentType, filePath, links, nil
}

// extractLinks parses HTML and returns all href values found on <a> tags.
func extractLinks(baseURL string, r io.Reader) ([]string, error) {
	var links []string
	tokenizer := html.NewTokenizer(r)

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				return links, nil
			}
			return links, tokenizer.Err()

		case html.StartTagToken, html.SelfClosingTagToken:
			t := tokenizer.Token()
			if t.DataAtom.String() == "a" {
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						href := strings.TrimSpace(attr.Val)
						if href == "" {
							continue
						}
						parsed, err := base.Parse(href)
						if err == nil {
							links = append(links, parsed.String())
						}
					}
				}
			}
		}
	}
}
