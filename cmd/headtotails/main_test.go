package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldPrintVersion(t *testing.T) {
	assert.False(t, shouldPrintVersion([]string{"headtotails"}))
	assert.True(t, shouldPrintVersion([]string{"headtotails", "--version"}))
	assert.True(t, shouldPrintVersion([]string{"headtotails", "version"}))
	assert.False(t, shouldPrintVersion([]string{"headtotails", "serve"}))
}

func TestPrintVersion(t *testing.T) {
	originalVersion := version
	version = "test-version"
	defer func() { version = originalVersion }()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	printVersion()
	_ = w.Close()
	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	assert.Contains(t, out, "headtotails version: test-version")
	assert.Contains(t, out, "target tailscale api: 0.28.0")
}
