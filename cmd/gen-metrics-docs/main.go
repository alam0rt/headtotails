package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/alam0rt/headtotails/internal/api"
)

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		log.Fatal(err)
	}

	outPath := filepath.Join(repoRoot, "docs", "metrics.md")
	content := api.RenderMetricsMarkdown()

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
