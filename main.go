package main

import (
	"log"
	"os"

	"boem-web-thing/config"
	"boem-web-thing/crawler"
	"boem-web-thing/logger"
	"boem-web-thing/storage"
)

func main() {
	// 1. Load config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	// 2. Init logger
	logDir := cfg.LogDir
	if logDir == "" {
		logDir = "./logs"
	}
	appLogger, err := logger.New(logDir, cfg.LogLevel)
	if err != nil {
		log.Fatal("Error initializing logger:", err)
	}
	defer appLogger.Close()

	// 3. Open SQLite storage
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = "./crawl.db"
	}
	store, err := storage.New(dbPath)
	if err != nil {
		appLogger.Error("Error opening database:", err)
		os.Exit(1)
	}
	defer store.Close()

	// 4. Create crawler
	c := crawler.New(cfg, appLogger, store)

	// 5. Start crawling
	c.Crawl()

	appLogger.Info("Crawl finished")
}
