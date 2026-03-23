package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRepoRootWalksUpToGoMod(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(base, "go.mod"), []byte("module example"), 0o644))
	nested := filepath.Join(base, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(nested))
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	got, err := findRepoRoot()
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(base)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
