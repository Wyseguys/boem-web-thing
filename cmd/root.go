package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "webcrawler",
	Short: "Webcrawler is a tool to crawl and save websites",
	Long:  `A simple CLI tool to crawl websites and save HTML files to disk.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available commands: crawl, pa11y")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
