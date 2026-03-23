package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedMetricsDocsAreUpToDate(t *testing.T) {
	t.Helper()

	path := filepath.Join("..", "..", "docs", "metrics.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated metrics docs: %v", err)
	}

	want := RenderMetricsMarkdown()
	if string(got) != want {
		t.Fatalf("docs/metrics.md is stale; run `go generate ./...`")
	}
}
