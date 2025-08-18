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

	"boem-web-thing/config"
	"boem-web-thing/logger"
	"boem-web-thing/storage"
	"boem-web-thing/util"

	"github.com/temoto/robotstxt"
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
	ticker  *time.Ticker // NEW
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
		ticker:  time.NewTicker(time.Duration(cfg.RateMs) * time.Millisecond),
	}
}

// Crawl starts crawling from the StartURL
func (c *Crawler) Crawl() {
	startURL := c.cfg.StartURL
	c.log.Debug("Starting site crawl at", startURL)

	urlCh := make(chan string, c.cfg.Concurrency*2)

	// Download a copy of the robots.txt to refer to
	// Always download a new copy at the start of a job
	robots_url := startURL + "robots.txt"
	_, _, _, _, robots_err := c.fetchAndSave(robots_url)
	if robots_err != nil {
		c.log.Error("Error Downloading Robots.txt", robots_url, robots_err)
	}

	c.wg.Add(1) // adding a wait here to track the first URL and wait until all the subsequent processes happen
	// Seed the queue
	urlCh <- startURL

	// Start worker goroutines
	// for i := 0; i < c.cfg.Concurrency; i++ {
	//wg.Add(1)
	go c.worker(urlCh) // start a single worker for now
	// }

	c.log.Debug("Waiting to finish crawl of", startURL)
	c.wg.Wait() // Wait for all workers to finish processing
	c.log.Debug("Finished crawl of site", startURL)
}

// This worker is constantly looping and reading the urlCh channel
// once the channel is empty, then the worker loop should exit
// returning from this worker() to the Crawl() that called
// it and Crawl can stop waiting.
func (c *Crawler) worker(urlCh chan string) {
	c.log.Debug("Starting Worker")
	for urlToCheck := range urlCh {
		c.log.Debug("Starting to process a URL", urlToCheck)
		go func(url string) {
			defer c.wg.Done()
			c.processURL(url, urlCh)
		}(urlToCheck)
	}
	c.log.Debug("Ending Worker")
}

// processURL fetches the URL, saves the content, extracts links from the file on disk and enqueues new URLs
func (c *Crawler) processURL(u string, urlCh chan<- string) {

	c.log.Debug("Start of processURL...", u)

	if c.isAlreadyVisited(u) {
		c.log.Debug("Already visited", u)
		return
	}

	// 1. Fetch
	c.log.Debug("Sending to fetch and save", u)
	// 1.1 Rate limiting
	time.Sleep(time.Duration(c.cfg.RateMs) * time.Millisecond)
	// 1.2 Gather the info from the URL
	status, contentType, filePath, links, err := c.fetchAndSave(u)
	if err != nil {
		c.log.Error("Error fetching", u, ":", err)
		return
	}

	// 2. Save page record
	c.log.Debug("Saving the fetched URL")
	c.log.Info("Saving", u)
	if err := c.store.SavePage(u, status, contentType, filePath); err != nil {
		c.log.Error("DB save error for", u, ":", err)
	}

	// 3. Add this URL to the list so I don't check it again
	c.addToVisited(u)

	c.log.Debug("Extracting links from the fetched page")
	// 4. Save links and enqueue new ones
	for _, link := range links {
		c.store.SaveLink(u, link)
		if c.shouldVisit(link) {
			c.wg.Add(1) //add to the waitgroup to make sure this URL gets waited for to finish all the processing
			urlCh <- link
		}
	}

	c.log.Debug("End of processURL...")
}

// Keep track of which URLs have been visited so we don't try to access them
// more than necessary
func (c *Crawler) addToVisited(u string) {
	c.log.Debug("Add Url to Visited List", u)
	c.mu.Lock()
	c.visited[u] = true
	c.mu.Unlock()
}

// retrieve the contents from the URL, if it is HTML then save a file, save it to the database
func (c *Crawler) fetchAndSave(rawURL string) (status int, contentType string, filePath string, links []string, err error) {
	c.log.Debug("Start of fetchAndSave", rawURL)
	// Make a HEAD request to check the content type
	resp, err := c.client.Head(rawURL)
	if err != nil {
		return 0, "", "", nil, err
	}
	defer resp.Body.Close()
	status = resp.StatusCode
	contentType = resp.Header.Get("Content-Type")
	// Make a GET request since the content is HTML
	resp, err = c.client.Get(rawURL)
	if err != nil {
		return 0, "", "", nil, err
	}
	defer resp.Body.Close()
	// Save only HTML and similar; still store others but don't parse links
	filePath, err = util.URLToFilePath(c.cfg.OutputDir, rawURL)
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
	c.log.Debug("End of fetchAndSave", rawURL)
	return status, contentType, filePath, links, nil
}

// Validate the string as a possible URL, see if it is safe, in scope
// and formatted as a URL correctly.
func (c *Crawler) shouldVisit(raw string) bool {
	c.log.Debug("start shouldVisit")
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

	if c.isAlreadyVisited(parsed.String()) {
		return false
	}

	if !c.isRobotsTxtAllowed(parsed.Path) {
		c.log.Info("robots.txt blocks link path", parsed.Path)
		c.addToVisited(raw)
		return false
	}
	return true
}

// look in the slice of visited URLs to see if we can already visited
// the one in question. This can save us a trip for pulling a web page
// and processing related links on ones we do pull.
func (c *Crawler) isAlreadyVisited(u string) bool {
	// Check if visited already
	c.log.Debug("Start isAlreadyVisited")
	c.mu.Lock()
	if c.visited[u] {
		c.mu.Unlock()
		c.log.Debug("Already visited this URL.", u)
		return true
	}
	c.mu.Unlock()
	return false
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

// Download a copy of the
func readRobotsTxt(full_robots_path string) (*robotstxt.RobotsData, error) {
	robotsFile, err := os.ReadFile(full_robots_path)
	if err != nil {
		return nil, err
	}

	robotsData := string(robotsFile)
	robots, err := robotstxt.FromString(robotsData)
	if err != nil {
		return nil, err
	}

	return robots, nil
}

// Validate that the crawler is allowed to access the content as specified by the
// copy of the robots.txt that was downloaded at the beginning of the session
func (c *Crawler) isRobotsTxtAllowed(path string) bool {
	c.log.Debug("Starting isRobotsTxtAllowed")
	userAgent := c.cfg.UserAgent

	//if the start url ends in /
	assumedRobotsLocation := util.StripHTMLFile(c.cfg.StartURL) + "robots.txt"
	robotsFilePath, err := util.URLToFilePath(c.cfg.OutputDir, assumedRobotsLocation)
	if err != nil {
		c.log.Error("Unable to find robots.txt:", err)
		return true // If we can't read it, assume allowed
	}
	robots, err := readRobotsTxt(robotsFilePath)
	if err != nil {
		c.log.Error("Error reading robots.txt:", err)
		return true // If we can't read it, assume allowed
	}

	group := robots.FindGroup(userAgent)
	testResult := group.Test(path)

	c.log.Debug("Ending isRobotsTxtAllowed")
	return testResult

}
