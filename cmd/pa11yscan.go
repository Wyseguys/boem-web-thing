package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var pa11yCmd = &cobra.Command{
	Use:   "pa11y [path/to/html]",
	Short: "Run a pa11y scan (pa11y must be installed with NPM)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		// url := args[0]
		// fmt.Printf("Crawling %s...\n", url)

		htmlpath := "" // default

		if len(args) == 1 {
			htmlpath = args[0]
		}
		result, _ := RunPa11y(htmlpath)
		// if err != nil {
		// 	fmt.Printf("ERROR %s...\n", err.Error())
		// } else {
		fmt.Printf("RESULT %s...\n", result)
		//}

	},
}

func init() {
	rootCmd.AddCommand(pa11yCmd)
}

// RunPa11y runs pa11y CLI on a given HTML file and returns the output
func RunPa11y11(filePath string) (string, error) {
	//cmd := exec.Command("npx", "pa11y", "file://"+filePath, "--reporter", "json")
	//cmd := exec.Command("which", "npx")
	cmd := exec.Command("bash", "-c", "npx pa11y "+filePath+" --reporter json")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// RunPa11y runs pa11y CLI on a given HTML file and returns the output
func RunPa11y(filePath string) (string, error) {
	// Step 1: Check if npx is available
	npxPath, err := exec.LookPath("npx")
	if err != nil {

		// Try common fallback paths for nvm in WSL2
		homeDir, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(homeDir, ".nvm/versions/node/v22.18.0/bin/npx"), // adjust version as needed
			filepath.Join(homeDir, ".nvm/versions/node/current/bin/npx"),
			filepath.Join(homeDir, ".nvm/versions/node/bin/npx"),
		}

		for _, path := range candidates {
			if _, statErr := os.Stat(path); statErr == nil {
				npxPath = path
				break
			}
		}

		if npxPath == "" {
			return "", fmt.Errorf("npx not found in PATH or fallback locations. Make sure Node.js and npx are installed via nvm")
		}

	}

	// Step 2: Build the command
	fmt.Println("Running pa11y on:", filePath)
	// Use bash -c to allow shell features like sourcing nvm if needed
	command := fmt.Sprintf("npx pa11y %s --reporter json", filePath)

	// Optional: Source nvm if needed (you can make this conditional or configurable)
	shellCommand := fmt.Sprintf("source ~/.nvm/nvm.sh && %s", command)

	cmd := exec.Command("bash", "-c", shellCommand)

	// Step 3: Inherit environment
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH"))

	// Step 4: Capture combined output
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		return output, fmt.Errorf("pa11y failed: %w", err)
	}

	return output, nil
}
