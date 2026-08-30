package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testApp(t *testing.T) *app {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, markerName), "# Guarded\n\n```sh\nprintf ok > out.txt\n```\n\nResult: [out.txt](out.txt)\n")
	application, err := newApp(root, 7333)
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func postForm(handler http.Handler, target, host, origin string, form url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	request.Host = host
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestGuardRejectsCrossOriginAndRebinding(t *testing.T) {
	handler := testApp(t).handler()
	form := url.Values{"jade": {"."}, "file": {markerName}, "command": {"printf ok > out.txt"}}

	if code := postForm(handler, "/run", "127.0.0.1:7333", "https://evil.example", form).Code; code != http.StatusForbidden {
		t.Fatalf("cross-origin POST = %d, want 403", code)
	}
	if code := postForm(handler, "/run", "evil.example:7333", "", form).Code; code != http.StatusForbidden {
		t.Fatalf("rebound Host POST = %d, want 403", code)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "evil.example:7333"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("rebound Host GET = %d, want 403", recorder.Code)
	}
}

func TestRunnableBlockAndAdhocCommand(t *testing.T) {
	application := testApp(t)
	handler := application.handler()

	// The sh block from jade.md appears as a runnable on the page.
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "127.0.0.1:7333"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Run: printf ok &gt; out.txt") {
		t.Fatalf("page = %d, runnable button missing:\n%s", recorder.Code, recorder.Body.String())
	}

	// Running it executes in the Jade and produces the file.
	form := url.Values{"jade": {"."}, "file": {markerName}, "command": {"printf ok > out.txt"}}
	response := postForm(handler, "/run", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusOK {
		t.Fatalf("same-origin POST = %d: %s", response.Code, response.Body.String())
	}
	contents, err := os.ReadFile(filepath.Join(application.root, "out.txt"))
	if err != nil || string(contents) != "ok" {
		t.Fatalf("run output file = %q, %v", contents, err)
	}

	// The first link in jade.md now resolves and becomes the default view.
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), "/view?jade=.&amp;file=out.txt") {
		t.Fatalf("default view missing:\n%s", recorder.Body.String())
	}

	// An ad-hoc command runs through the same machinery.
	form = url.Values{"jade": {"."}, "file": {markerName}, "command": {"printf adhoc > adhoc.txt"}}
	if code := postForm(handler, "/run", "127.0.0.1:7333", "http://127.0.0.1:7333", form).Code; code != http.StatusOK {
		t.Fatalf("adhoc POST = %d", code)
	}
	contents, err = os.ReadFile(filepath.Join(application.root, "adhoc.txt"))
	if err != nil || string(contents) != "adhoc" {
		t.Fatalf("adhoc file = %q, %v", contents, err)
	}

	// Empty command is rejected without executing anything.
	form = url.Values{"jade": {"."}, "file": {markerName}, "command": {"  "}}
	response = postForm(handler, "/run", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "command is empty") {
		t.Fatalf("empty command = %d:\n%s", response.Code, response.Body.String())
	}
}
