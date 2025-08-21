package cmd

import (
	"boem-web-thing/config"
	"boem-web-thing/scanner"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var sitescanCmd = &cobra.Command{
	Use:   "sitescan [config.json]",
	Short: "Run a full site scan based on the site defined in the JSON configuration file(pa11y must be installed with NPM)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		configfile := "config.json" // default

		if len(args) == 1 {
			configfile = args[0]
		}
		result, err := runSiteScan(configfile)
		if err != nil {
			fmt.Printf("ERROR %s...\n", err.Error())
		} else {
			fmt.Printf("RESULT %s...\n", result)
		}

	},
}

func init() {
	rootCmd.AddCommand(sitescanCmd)
}

func runSiteScan(configPath string) (string, error) {

	// 1. Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	// 2, 3 initial the logger and the storage
	appLogger, store, err := cfg.InitializeApp()
	if err != nil {
		log.Fatal("Error initializing config:", err)
	}

	// 4. Create crawler
	s := scanner.New(cfg, appLogger, store)
	appLogger.Debug("Scanner successfully initialized")

	// 5. Start crawling
	appLogger.Info("Beginning scan")
	s.ScanSite()

	// 6. Tell me the crawl is done
	appLogger.Info("Finished scan")

	// 7. Tidy up
	store.Close()
	appLogger.Close()

	return "scan done", nil

}
