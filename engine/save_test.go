package engine

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveRejectsStaleAndDeletedFiles(t *testing.T) {
	application := testApp(t)
	path := filepath.Join(application.root, "notes.go")
	revision := fileRevision("package notes\n")
	if err := os.WriteFile(path, []byte("// agent edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"jade": {"."}, "file": {"notes.go"}, "content": {"// editor edit\n"}, "revision": {revision}}
	response := postForm(application.handler(), "/save", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusConflict {
		t.Fatalf("stale save = %d: %s", response.Code, response.Body.String())
	}
	data, _ := os.ReadFile(path)
	if string(data) != "// agent edit\n" {
		t.Fatal("overwrote external edits")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	response = postForm(application.handler(), "/save", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusConflict {
		t.Fatalf("deleted save = %d", response.Code)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("recreated deleted file")
	}
}

func TestSavePreservesModeAndAllowsLostResponseRetry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "run.sh")
	if err := os.WriteFile(path, []byte("before\r\n"), 0755); err != nil {
		t.Fatal(err)
	}
	revision := fileRevision("before\r\n")
	for i := 0; i < 2; i++ {
		if err := updateWorkspaceFile(root, ".", "run.sh", "after\r\n", revision); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(data) != "after\r\n" || info.Mode().Perm() != 0755 {
		t.Fatalf("data=%q mode=%v", data, info.Mode())
	}
}
