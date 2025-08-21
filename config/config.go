package config

import (
	"boem-web-thing/logger"
	"boem-web-thing/storage"
	"encoding/json"
	"log"
	"os"
	"time"
)

// Config holds all user-configurable settings loaded from JSON.
type Config struct {
	StartURL      string   `json:"start_url"`
	OutputDir     string   `json:"output_dir"`
	DBFilePath    string   `json:"db_file_path"`
	LogPath       string   `json:"log_path"`
	LogLevel      string   `json:"log_level"`
	Concurrency   int      `json:"concurrency"`
	MaxDepth      int      `json:"max_depth"`
	RespectRobots bool     `json:"respect_robots"`
	UserAgent     string   `json:"user_agent"`
	AllowedHosts  []string `json:"allowed_hosts"`
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
		cfg.OutputDir = "./_output"
	}
	if cfg.DBFilePath == "" {
		cfg.DBFilePath = "./_db/webthing.db"
	}
	if cfg.LogPath == "" {
		cfg.LogPath = "./_logs"
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

func (c *Config) InitializeApp() (*logger.Logger, *storage.Storage, error) {

	// 2. Init logger
	logDir := c.LogPath
	if logDir == "" {
		log.Fatal("Error with logging configuration:")
		return nil, nil, nil
	}
	appLogger, err := logger.New(logDir, c.LogLevel)
	if err != nil {
		log.Fatal("Error initializing logger:", err)
		return nil, nil, err
	}
	defer appLogger.Close()

	appLogger.Debug("Configuration initialized")
	appLogger.Debug("Logger initialized")

	// 3. Open SQLite storage
	dbPath := c.DBFilePath
	if dbPath == "" {
		log.Fatal("Error with database configuration:", err)
		return nil, nil, err
	}

	store, err := storage.New(dbPath)
	if err != nil {
		appLogger.Error("Error opening database:", err)
		os.Exit(1)
		return nil, nil, err
	}
	appLogger.Debug("Database opened")

	return appLogger, store, nil
}
