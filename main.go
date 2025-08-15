package main

import (
	"log"
	"os"

	"github.com/wyseguys/boem-web-thing/config"
	"github.com/wyseguys/boem-web-thing/crawler"
	"github.com/wyseguys/boem-web-thing/logger"
	"github.com/wyseguys/boem-web-thing/storage"
)

func main() {

	// 1. Load config
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	// 2. Init logger
	logDir := cfg.LogPath
	if logDir == "" {
		log.Fatal("Error with logging configuration:", err)
	}
	appLogger, err := logger.New(logDir, cfg.LogLevel)
	if err != nil {
		log.Fatal("Error initializing logger:", err)
	}
	defer appLogger.Close()

	if cfg.Verbose {
		appLogger.Info("Configuration initialized")
		appLogger.Info("Logger initialized")
	}

	// 3. Open SQLite storage
	dbPath := cfg.DBFilePath
	if dbPath == "" {
		log.Fatal("Error with database configuration:", err)
	}

	store, err := storage.New(dbPath)
	if err != nil {
		appLogger.Error("Error opening database:", err)
		os.Exit(1)
	}
	defer store.Close()
	if cfg.Verbose {
		appLogger.Info("Database opened")
	}

	// 4. Create crawler
	c := crawler.New(cfg, appLogger, store)
	if cfg.Verbose {
		appLogger.Info("Crawler initialized")
	}

	// 5. Start crawling
	appLogger.Info("Beginning crawl")
	c.Crawl()

	// 6. Tell me the crawl is done
	appLogger.Info("Finished crawl")
}
