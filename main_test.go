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
	writeTestFile(t, filepath.Join(root, markerName), "# Guarded\nArtifact: out.txt\nCommand: printf ok > out.txt\n")
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
	form := url.Values{"jade": {"."}, "file": {markerName}}

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

func TestGuardAllowsSameOriginRun(t *testing.T) {
	application := testApp(t)
	handler := application.handler()
	form := url.Values{"jade": {"."}, "file": {markerName}}
	response := postForm(handler, "/run", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusOK {
		t.Fatalf("same-origin POST = %d: %s", response.Code, response.Body.String())
	}
	contents, err := os.ReadFile(filepath.Join(application.root, "out.txt"))
	if err != nil || string(contents) != "ok" {
		t.Fatalf("artifact = %q, %v", contents, err)
	}
}
