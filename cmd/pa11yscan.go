package cmd

import (
	"boem-web-thing/scanner"
	"fmt"

	"github.com/spf13/cobra"
)

var pa11yCmd = &cobra.Command{
	Use:   "pa11y [path/to/html]",
	Short: "Run a pa11y scan (pa11y must be installed with NPM)",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {

		// url := args[0]
		// fmt.Printf("Crawling %s...\n", url)

		htmlpath := "" // default
		pa11yCmd := "" // no default commands

		if len(args) >= 1 {
			htmlpath = args[0]
		}
		if len(args) == 2 {
			pa11yCmd = args[1]
		}
		result, err := RunPa11y(htmlpath, pa11yCmd)
		if err != nil {
			fmt.Printf("ERROR %s...\n", err.Error())
			//} else {
		}
		fmt.Printf("RESULT %s...\n", result)
	},
}

func init() {
	rootCmd.AddCommand(pa11yCmd)
}

// RunPa11y runs pa11y CLI on a given HTML file and returns the output
func RunPa11y(filePath string, pa11yCmd string) (string, error) {

	results, err := scanner.ScanWithPa11y(filePath, pa11yCmd)
	return results, err

}
