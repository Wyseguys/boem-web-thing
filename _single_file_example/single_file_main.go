package single_file_main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/net/html"
)

type Config struct {
	StartURL      string   `json:"start_url"`
	OutputDir     string   `json:"output_dir"`
	DBPath        string   `json:"db_path"`
	LogPath       string   `json:"log_path"`
	Concurrency   int      `json:"concurrency"`
	MaxDepth      int      `json:"max_depth"`
	RespectRobots bool     `json:"respect_robots"`
	UserAgent     string   `json:"user_agent"`
	AllowedHosts  []string `json:"allowed_hosts"`
	Verbose       bool     `json:"verbose"`
	RateMs        int      `json:"rate_ms"`
	HTTPTimeout   int      `json:"http_timeout_seconds"`
}

type PageInfo struct {
	URL         string    `json:"url"`
	Status      int       `json:"status"`
	ContentType string    `json:"content_type"`
	LocalPath   string    `json:"local_path"`
	FetchedAt   time.Time `json:"fetched_at"`
}

var (
	infoLog  *log.Logger
	debugLog *log.Logger
	errorLog *log.Logger
)

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "output"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "db/boem.db"
	}
	if cfg.LogPath == "" {
		cfg.LogPath = "boem.log"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 5
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 3
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "boem-web-thing/1.0"
	}
	if cfg.RateMs <= 0 {
		cfg.RateMs = 200
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 30
	}
	return &cfg, nil
}

func initLogger(logPath string, verbose bool) error {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	mw := io.MultiWriter(os.Stdout, f)
	infoLog = log.New(mw, "INFO: ", log.LstdFlags)
	errorLog = log.New(mw, "ERROR: ", log.LstdFlags)
	if verbose {
		debugLog = log.New(mw, "DEBUG: ", log.LstdFlags)
	} else {
		debugLog = log.New(io.Discard, "", 0)
	}
	return nil
}

type Storage struct {
	db *bolt.DB
}

func newStorage(path string) (*Storage, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		if _, e := tx.CreateBucketIfNotExists([]byte("pages")); e != nil {
			return e
		}
		if _, e := tx.CreateBucketIfNotExists([]byte("links")); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Storage{db}, nil
}

func (s *Storage) savePage(pi PageInfo) {
	_ = s.db.Update(func(tx *bolt.Tx) error {
		b, _ := json.Marshal(pi)
		return tx.Bucket([]byte("pages")).Put([]byte(pi.URL), b)
	})
}

func (s *Storage) addLinks(from string, links []string) {
	_ = s.db.Update(func(tx *bolt.Tx) error {
		b, _ := json.Marshal(links)
		return tx.Bucket([]byte("links")).Put([]byte(from), b)
	})
}

func (s *Storage) Close() { _ = s.db.Close() }

type Crawler struct {
	cfg       *Config
	store     *Storage
	client    *http.Client
	robotsMap map[string]*robotstxt.RobotsData
	visited   sync.Map
	jobs      chan job
	wg        sync.WaitGroup
}

type job struct {
	u     *url.URL
	depth int
}

func newCrawler(cfg *Config, store *Storage) *Crawler {
	return &Crawler{
		cfg:       cfg,
		store:     store,
		client:    &http.Client{Timeout: time.Duration(cfg.HTTPTimeout) * time.Second},
		robotsMap: map[string]*robotstxt.RobotsData{},
		jobs:      make(chan job, 1000),
	}
}

func (c *Crawler) run(ctx context.Context) {
	for i := 0; i < c.cfg.Concurrency; i++ {
		c.wg.Add(1)
		go c.worker(ctx)
	}
	start, _ := url.Parse(c.cfg.StartURL)
	c.enqueue(start, 0)
	c.wg.Wait()
}

func (c *Crawler) worker(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-c.jobs:
			if j.depth > c.cfg.MaxDepth {
				continue
			}
			if _, seen := c.visited.LoadOrStore(j.u.String(), struct{}{}); seen {
				continue
			}
			c.fetch(ctx, j)
			time.Sleep(time.Duration(c.cfg.RateMs) * time.Millisecond)
		}
	}
}

func (c *Crawler) enqueue(u *url.URL, depth int) {
	select {
	case c.jobs <- job{u, depth}:
	default:
	}
}

func (c *Crawler) fetch(ctx context.Context, j job) {
	if c.cfg.RespectRobots && !c.allowedByRobots(j.u) {
		debugLog.Println("Disallowed by robots.txt:", j.u)
		return
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", j.u.String(), nil)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		errorLog.Println("Fetch error:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(body)
	}
	fsPath, localURL := c.mapURL(j.u)
	if err := os.MkdirAll(filepath.Dir(fsPath), 0o755); err != nil {
		errorLog.Println("mkdir error:", err)
		return
	}
	if strings.Contains(ct, "text/html") {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
		if err == nil {
			outgoing := c.rewriteLinks(doc, j.u, j.depth)
			var buf bytes.Buffer
			_ = html.Render(&buf, doc.Selection.Nodes[0])
			_ = os.WriteFile(fsPath, buf.Bytes(), 0o644)
			c.store.addLinks(j.u.String(), outgoing)
		} else {
			_ = os.WriteFile(fsPath, body, 0o644)
		}
	} else {
		_ = os.WriteFile(fsPath, body, 0o644)
	}
	c.store.savePage(PageInfo{
		URL:         j.u.String(),
		Status:      resp.StatusCode,
		ContentType: ct,
		LocalPath:   localURL,
		FetchedAt:   time.Now(),
	})
	infoLog.Println("Saved:", j.u)
}

func (c *Crawler) mapURL(u *url.URL) (string, string) {
	p := u.Path
	if p == "" || strings.HasSuffix(p, "/") {
		p = path.Join(p, "index.html")
	}
	if path.Ext(p) == "" {
		p = path.Join(p, "index.html")
	}
	if u.RawQuery != "" {
		q := strings.ReplaceAll(u.RawQuery, "&", "_")
		dir, file := path.Split(p)
		file = fmt.Sprintf("%s_%s", file, q)
		p = path.Join(dir, file)
	}
	fs := filepath.Join(c.cfg.OutputDir, u.Hostname(), filepath.FromSlash(path.Dir(p)))
	fp := filepath.Join(fs, filepath.Base(p))
	return fp, path.Join("/", u.Hostname(), p)
}

func (c *Crawler) rewriteLinks(doc *goquery.Document, base *url.URL, depth int) []string {
	var out []string
	doc.Find("[href],[src]").Each(func(_ int, s *goquery.Selection) {
		attr := "href"
		if _, ok := s.Attr("href"); !ok {
			attr = "src"
		}
		val, _ := s.Attr(attr)
		if val == "" {
			return
		}
		abs := toAbs(base, val)
		if abs == nil {
			return
		}
		out = append(out, abs.String())
		if !c.isAllowed(abs.Hostname()) {
			return
		}
		_, local := c.mapURL(abs)
		rel := relPath(path.Dir(base.Path), local)
		s.SetAttr(attr, rel)
		c.enqueue(abs, depth+1)
	})
	return out
}

func (c *Crawler) isAllowed(host string) bool {
	start, _ := url.Parse(c.cfg.StartURL)
	if host == start.Hostname() {
		return true
	}
	for _, h := range c.cfg.AllowedHosts {
		if h == host {
			return true
		}
	}
	return false
}

func relPath(from, to string) string {
	return strings.TrimPrefix(to, "/")
}

func toAbs(base *url.URL, href string) *url.URL {
	p, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return nil
	}
	return base.ResolveReference(p)
}

func (c *Crawler) allowedByRobots(u *url.URL) bool {
	key := u.Scheme + "://" + u.Host
	rd, ok := c.robotsMap[key]
	if !ok {
		resp, err := c.client.Get(key + "/robots.txt")
		if err != nil {
			return true
		}
		defer resp.Body.Close()
		data, _ := robotstxt.FromResponse(resp)
		rd = data
		c.robotsMap[key] = rd
	}
	return rd.TestAgent(u.Path, c.cfg.UserAgent)
}

func single_file_main() {
	cfgPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatal("Config error:", err)
	}
	if err := initLogger(cfg.LogPath, cfg.Verbose); err != nil {
		log.Fatal("Logger init error:", err)
	}
	store, err := newStorage(cfg.DBPath)
	if err != nil {
		log.Fatal("DB error:", err)
	}
	defer store.Close()

	ctx := context.Background()
	c := newCrawler(cfg, store)
	c.run(ctx)
}
