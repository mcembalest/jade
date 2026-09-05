package engine

import (
	"encoding/json"
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
	writeTestFile(t, filepath.Join(root, markerName), "# Guarded\n\nResult: [out.txt](out.txt)\n")
	writeTestFile(t, filepath.Join(root, "notes.go"), "package notes\n")
	writeTestFile(t, filepath.Join(root, "inner", markerName), "# Inner\n")
	writeTestFile(t, filepath.Join(root, "inner", "code.py"), "print('inner')\n")
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
	form := url.Values{"jade": {"."}, "file": {markerName}, "content": {"# Changed\n"}}

	if code := postForm(handler, "/save", "127.0.0.1:7333", "https://evil.example", form).Code; code != http.StatusForbidden {
		t.Fatalf("cross-origin POST = %d, want 403", code)
	}
	if code := postForm(handler, "/save", "evil.example:7333", "", form).Code; code != http.StatusForbidden {
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

func TestIDEShellAndJadeResolution(t *testing.T) {
	application := testApp(t)
	handler := application.handler()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "127.0.0.1:7333"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	page := recorder.Body.String()
	for _, expected := range []string{
		`aria-label="Files"`,
		`id="terminal-toggle"`,
		`id="terminal-select"`,
		`id="document" class="jade-open"`,
		`data-jade="inner"`,
		`class="brand">JADE</span>`,
		`rel="icon"`,
		`🐉`,
	} {
		if recorder.Code != http.StatusOK || !strings.Contains(page, expected) {
			t.Fatalf("page = %d, missing %q:\n%s", recorder.Code, expected, page)
		}
	}
	for _, removed := range []string{`id="publish-open"`, `id="publish-dialog"`, `id="branch-select"`, "@xterm", `id="terminal-panel"`, "Run:", "sh command in this JaDE", ">Open</button>"} {
		if strings.Contains(page, removed) {
			t.Fatalf("obsolete control %q remains", removed)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/file?jade=.&file=notes.go", nil)
	request.Host = "127.0.0.1:7333"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var file struct {
		Selected string `json:"selected"`
		Contents string `json:"contents"`
		IsJade   bool   `json:"isJade"`
		ViewURL  string `json:"viewURL"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || file.Selected != "notes.go" || file.Contents != "package notes\n" || file.IsJade || file.ViewURL != "" {
		t.Fatalf("ordinary file response = %#v", file)
	}

	writeTestFile(t, filepath.Join(application.root, "out.txt"), "ready")
	request = httptest.NewRequest(http.MethodGet, "/file?jade=.&file=jade.md", nil)
	request.Host = "127.0.0.1:7333"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if err := json.NewDecoder(recorder.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if !file.IsJade || file.ViewURL != "/front?jade=." {
		t.Fatalf("jade.md response = %#v", file)
	}
	request = httptest.NewRequest(http.MethodGet, "/file?jade=.&file=jade.md&view=out.txt", nil)
	request.Host = "127.0.0.1:7333"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if err := json.NewDecoder(recorder.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if file.ViewURL != "/view?jade=.&file=out.txt" {
		t.Fatalf("explicit artifact view = %#v", file)
	}

}

func TestPlainRepositoryOpensWithoutJadeMarker(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n")
	application, err := newApp(root, 7333)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "127.0.0.1:7333"
	recorder := httptest.NewRecorder()
	application.handler().ServeHTTP(recorder, request)
	page := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(page, `data-file="main.go"`) || !strings.Contains(page, "package main") {
		t.Fatalf("plain repository = %d:\n%s", recorder.Code, page)
	}
}

func TestAppScriptUsesTerminalEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	request.Host = "127.0.0.1:7333"
	response := httptest.NewRecorder()
	testApp(t).handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"/terminal"`) || strings.Contains(response.Body.String(), `window.webkit`) {
		t.Fatalf("terminal endpoint = %d: %s", response.Code, response.Body.String())
	}
}

func TestSaveAndCreateUsePlainFiles(t *testing.T) {
	application := testApp(t)
	handler := application.handler()
	form := url.Values{"jade": {"."}, "file": {"notes.go"}, "content": {"package changed\n"}, "revision": {fileRevision("package notes\n")}}
	response := postForm(handler, "/save", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusOK {
		t.Fatalf("save = %d: %s", response.Code, response.Body.String())
	}
	contents, err := os.ReadFile(filepath.Join(application.root, "notes.go"))
	if err != nil || string(contents) != "package changed\n" {
		t.Fatalf("saved file = %q, %v", contents, err)
	}

	form = url.Values{"jade": {"."}, "path": {"notes/new.md"}}
	response = postForm(handler, "/new", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusOK || response.Body.String() != "/?jade=.&file=notes%2Fnew.md" {
		t.Fatalf("create = %d: %q", response.Code, response.Body.String())
	}
	if _, err = os.Stat(filepath.Join(application.root, "notes", "new.md")); err != nil {
		t.Fatal(err)
	}
}

func TestCrossSiteNavigationOnlyOpensShell(t *testing.T) {
	handler := testApp(t).handler()
	for _, tc := range []struct {
		name, method, path, mode, destination, origin, host string
		want                                                int
	}{
		{"preview link", "GET", "/", "navigate", "document", "", "127.0.0.1:7333", 200},
		{"embedded shell", "GET", "/", "navigate", "iframe", "", "127.0.0.1:7333", 403},
		{"shell fetch", "GET", "/", "cors", "empty", "", "127.0.0.1:7333", 403},
		{"file navigation", "GET", "/file", "navigate", "document", "", "127.0.0.1:7333", 403},
		{"draft navigation", "GET", "/drafts", "navigate", "document", "", "127.0.0.1:7333", 403},
		{"post navigation", "POST", "/save", "navigate", "document", "", "127.0.0.1:7333", 403},
		{"foreign origin", "GET", "/", "navigate", "document", "https://evil.example", "127.0.0.1:7333", 403},
		{"rebound host", "GET", "/", "navigate", "document", "", "evil.example:7333", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Host = tc.host
			request.Header.Set("Sec-Fetch-Site", "cross-site")
			request.Header.Set("Sec-Fetch-Mode", tc.mode)
			request.Header.Set("Sec-Fetch-Dest", tc.destination)
			request.Header.Set("Origin", tc.origin)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
			if tc.want == 200 && !strings.Contains(response.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Fatal("shell must not be embedded by another page")
			}
		})
	}
}
