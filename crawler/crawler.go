package crawler

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"boem-web-thing/config"
	"boem-web-thing/logger"
	"boem-web-thing/storage"
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

func (c *Crawler) worker(urlCh <-chan string, doneCh <-chan struct{}) {
	defer c.wg.Done()

	for {
		select {
		case <-doneCh:
			return
		case u, ok := <-urlCh:
			if !ok {
				return
			}
			c.processURL(u, urlCh)
		}
	}
}

func (c *Crawler) processURL(u string, urlCh chan<- string) {
	// Check if visited already
	c.mu.Lock()
	if c.visited[u] {
		c.mu.Unlock()
		return
	}
	c.visited[u] = true
	c.mu.Unlock()

	// Fetch
	status, contentType, links, filePath, err := c.fetchAndSave(u)
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
