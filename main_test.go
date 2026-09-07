package main

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitFileLaunchOverridesRememberedSelection(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "a note & example.md")
	if err := os.WriteFile(filename, []byte("# Note"), 0600); err != nil {
		t.Fatal(err)
	}
	base := "http://127.0.0.1:1234"
	target, err := url.Parse(launchURL(base, filename))
	if err != nil || target.Query().Get("file") != "a note & example.md" {
		t.Fatalf("file selection %v: %v", target, err)
	}
	if got := launchURL(base, directory); got != base {
		t.Fatalf("folder should restore session: %s", got)
	}
}
