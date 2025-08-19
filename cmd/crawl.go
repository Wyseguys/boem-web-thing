package cmd

import (
	"boem-web-thing/config"
	"boem-web-thing/crawler"
	"boem-web-thing/logger"
	"boem-web-thing/storage"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var crawlCmd = &cobra.Command{
	Use:   "crawl [config.site.json]",
	Short: "Crawl a website and save HTML files",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		// url := args[0]
		// fmt.Printf("Crawling %s...\n", url)

		configPath := "config.json" // default

		if len(args) == 1 {
			configPath = args[0]
		}

		// 1. Load config
		cfg, err := config.LoadConfig(configPath)
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

		appLogger.Debug("Configuration initialized")
		appLogger.Debug("Logger initialized")

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
		appLogger.Debug("Database opened")

		// 4. Create crawler
		c := crawler.New(cfg, appLogger, store)
		appLogger.Debug("Crawler successfully initialized")

		// 5. Start crawling
		appLogger.Info("Beginning crawl")
		c.Crawl()

		// 6. Tell me the crawl is done
		appLogger.Info("Finished crawl")

		// 7. Tidy up
		store.Close()
		appLogger.Close()

	},
}

func init() {
	rootCmd.AddCommand(crawlCmd)
}
