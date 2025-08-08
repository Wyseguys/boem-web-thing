package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds all user-configurable settings loaded from JSON.
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

// LoadConfig reads JSON from the given path and applies defaults where needed.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	// Apply defaults if missing
	if cfg.OutputDir == "" {
		cfg.OutputDir = "output"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "boem.db"
	}
	if cfg.LogPath == "" {
		cfg.LogPath = "boem.log"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 5
	}
	if cfg.MaxDepth < 0 {
		cfg.MaxDepth = 5
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "boem-web-thing/1.0 (+https://boem.gov)"
	}
	if cfg.RateMs <= 0 {
		cfg.RateMs = 200
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = int((30 * time.Second).Seconds())
	}

	return &cfg, nil
}
