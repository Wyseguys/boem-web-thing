package scanner

import (
	"boem-web-thing/config"
	"boem-web-thing/logger"
	"boem-web-thing/storage"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Scanner struct {
	cfg   *config.Config
	log   *logger.Logger
	store *storage.Storage
	wg    sync.WaitGroup
}

func New(cfg *config.Config, log *logger.Logger, store *storage.Storage) *Scanner {
	return &Scanner{
		cfg:   cfg,
		log:   log,
		store: store,
	}
}

func (s *Scanner) ScanSite() {

	pages, err := s.store.GetPagesByAllowedHosts(s.cfg.AllowedHosts)
	if err != nil {
		s.log.Error(err)
	}

	//For each URL, match it to the path on the disk
	for _, pg := range pages {

		//Pull the URLs from storge, add "./" so we are looking relatively
		filePath := "./" + pg.File_path //e.g. "./_output/doiboem.lndo.site/crawltest/index.html"

		//if the file exists on the disk, scan it
		_, err := os.Stat(filePath)
		if err != nil {
			s.log.Error("Cannot find filePath", filePath)
		}

		//Store the result of the scan next to the page
		scanResult, err := ScanWithPa11y(filePath, "")
		if err != nil {
			s.log.Error()
		}
		s.store.SaveScan(pg.File_path, scanResult)
	}

}

// Run the filepath through the pa11y scanner and output the result as a JSON string
func ScanWithPa11y(filePath string, pa11yCmds string) (string, error) {

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
	command := fmt.Sprintf("npx pa11y %s %s --reporter json", filePath, pa11yCmds)

	// Optional: Source nvm if needed (you can make this conditional or configurable)
	shellCommand := fmt.Sprintf("source ~/.nvm/nvm.sh && %s", command)

	cmd := exec.Command("bash", "-c", shellCommand)

	// Step 3: Inherit environment
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH"))

	// Step 4: Capture combined output
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		return output, fmt.Errorf("pa11y dectected errors: %w", err)
	}

	return output, nil
}
