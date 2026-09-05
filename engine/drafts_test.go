package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestDraftPersistenceAndConditionalDeletion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := testApp(t)
	form := url.Values{"jade": {"."}, "file": {"notes.go"}, "id": {"12345678-1234-1234-1234-123456789012"}, "content": {"draft\r\n"}, "revision": {fileRevision("package notes\n")}}
	post := func() editorDraft {
		t.Helper()
		response := postForm(a.handler(), "/drafts", "127.0.0.1:7333", "", form)
		if response.Code != 200 {
			t.Fatalf("POST %d: %s", response.Code, response.Body.String())
		}
		var draft editorDraft
		if err := json.Unmarshal(response.Body.Bytes(), &draft); err != nil {
			t.Fatal(err)
		}
		return draft
	}
	first := post()
	// Reconstructing the app must preserve drafts, including when the file is deleted.
	var err error
	a, err = newApp(a.root, 7333)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(a.root, "notes.go")); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/drafts?jade=.&file=notes.go", nil)
	request.Host = "127.0.0.1:7333"
	response := httptest.NewRecorder()
	a.handler().ServeHTTP(response, request)
	var list struct {
		Drafts []editorDraft `json:"drafts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || len(list.Drafts) != 1 || list.Drafts[0].Content != "draft\r\n" {
		t.Fatalf("GET %d: %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/?jade=.&file=notes.go", nil)
	request.Host = "127.0.0.1:7333"
	response = httptest.NewRecorder()
	a.handler().ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("deleted-file recovery page = %d: %s", response.Code, response.Body.String())
	}
	form.Set("content", "newer draft")
	second := post()
	remove := func(token string) int {
		request := httptest.NewRequest(http.MethodDelete, "/drafts?jade=.&file=notes.go&id="+first.ID+"&token="+token, nil)
		request.Host = "127.0.0.1:7333"
		response := httptest.NewRecorder()
		a.handler().ServeHTTP(response, request)
		return response.Code
	}
	if code := remove(first.Token); code != 409 {
		t.Fatalf("stale deletion = %d", code)
	}
	if code := remove(second.Token); code != 200 {
		t.Fatalf("current deletion = %d", code)
	}
	form.Set("file", "../outside")
	if code := postForm(a.handler(), "/drafts", "127.0.0.1:7333", "", form).Code; code != 400 {
		t.Fatalf("traversal=%d", code)
	}
	form.Set("file", "notes.go")
	form.Set("id", "../escape")
	if code := postForm(a.handler(), "/drafts", "127.0.0.1:7333", "", form).Code; code != 400 {
		t.Fatalf("id traversal=%d", code)
	}
}

func TestDraftLockHonorsCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := testApp(t)
	directory, err := a.draftDirectory(".", "notes.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	lock := flock.New(filepath.Join(directory, ".lock"))
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/drafts?jade=.&file=notes.go", nil).WithContext(ctx)
	request.Host = "127.0.0.1:7333"
	response := httptest.NewRecorder()
	a.handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("held lock = %d", response.Code)
	}
	if err := lock.Unlock(); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/drafts?jade=.&file=notes.go", nil)
	request.Host = "127.0.0.1:7333"
	a.handler().ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("released lock=%d", response.Code)
	}
}

func TestDraftLimitPreservesExistingCopies(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := testApp(t)
	directory, err := a.draftDirectory(".", "notes.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	// An interrupted atomic write must not consume a recoverable-draft slot.
	if err := os.WriteFile(filepath.Join(directory, ".jade-save-interrupted"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"jade": {"."}, "file": {"notes.go"}, "content": {"draft"}, "revision": {fileRevision("package notes\n")}}
	for index := 0; index < 33; index++ {
		form.Set("id", fmt.Sprintf("12345678-1234-1234-1234-%012d", index))
		response := postForm(a.handler(), "/drafts", "127.0.0.1:7333", "", form)
		want := http.StatusOK
		if index == 32 {
			want = http.StatusConflict
		}
		if response.Code != want {
			t.Fatalf("draft %d: %d, want %d: %s", index, response.Code, want, response.Body.String())
		}
	}
	form.Set("id", "12345678-1234-1234-1234-000000000000")
	form.Set("content", "newer existing draft")
	if response := postForm(a.handler(), "/drafts", "127.0.0.1:7333", "", form); response.Code != http.StatusOK {
		t.Fatalf("existing draft update: %d: %s", response.Code, response.Body.String())
	}
}
