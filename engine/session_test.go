package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolateEditorSessions(t *testing.T) {
	t.Helper()
	directory := t.TempDir()
	previous := editorSessionDirectory
	editorSessionDirectory = func() (string, error) { return directory, nil }
	t.Cleanup(func() { editorSessionDirectory = previous })
}
func postEditorSession(a *app, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/session?jade=.", strings.NewReader(body))
	request.Host = "127.0.0.1:7333"
	response := httptest.NewRecorder()
	a.handler().ServeHTTP(response, request)
	return response
}
func TestEditorSessionRestoresAcrossAppInstances(t *testing.T) {
	isolateEditorSessions(t)
	a := testApp(t)
	payload := `{"file":"notes.go","positions":{"notes.go":{"head":8,"scroll":24}},"filesOpen":true,"filesPinned":true,"folders":["inner"]}`
	if response := postEditorSession(a, payload); response.Code != http.StatusNoContent {
		t.Fatal(response.Code, response.Body.String())
	}
	reopened, err := newApp(a.root, 9444)
	if err != nil {
		t.Fatal(err)
	}
	data, err := reopened.restoredPageData(".", "", "", false)
	if err != nil || data.Selected != "notes.go" {
		t.Fatalf("restored %q: %v", data.Selected, err)
	}
	var state editorSession
	if err = json.Unmarshal([]byte(data.Session), &state); err != nil {
		t.Fatal(err)
	}
	if state.Positions["notes.go"].Head != 8 || !state.FilesOpen || !state.FilesPinned || len(state.Folders) != 1 {
		t.Fatalf("metadata: %+v", state)
	}
	explicit, err := reopened.restoredPageData(".", homepageName, "", true)
	if err != nil || explicit.Selected != homepageName {
		t.Fatalf("explicit selection %q: %v", explicit.Selected, err)
	}
	preview, err := reopened.restoredPageData(".", "", "README.md", true)
	if err != nil || preview.Selected != homepageName {
		t.Fatalf("explicit preview selection %q: %v", preview.Selected, err)
	}
	inner, err := reopened.restoredPageData("inner", "", "", false)
	if err != nil || inner.Selected != homepageName {
		t.Fatalf("workspace scope %q: %v", inner.Selected, err)
	}
	other := testApp(t)
	if other.readSession(".").File != "" {
		t.Fatal("leaked across roots")
	}
}
func TestEditorSessionMissingFileFallbackAndCorruptState(t *testing.T) {
	isolateEditorSessions(t)
	a := testApp(t)
	postEditorSession(a, `{"file":"missing.txt","positions":{},"folders":[]}`)
	data, err := a.restoredPageData(".", "", "", false)
	if err != nil || data.Selected != homepageName || data.SessionNotice == "" {
		t.Fatalf("fallback %+v: %v", data, err)
	}
	if _, err = a.restoredPageData(".", "missing.txt", "", true); err == nil {
		t.Fatal("explicit missing file should retain error")
	}
	path, _ := a.sessionPath(".")
	if err = os.WriteFile(path, []byte(`{"file":`), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err = a.restoredPageData(".", "", "", false); err != nil || data.Selected != homepageName {
		t.Fatalf("corrupt session blocked editor: %v", err)
	}
	if err = os.WriteFile(path, []byte(`{"positions":{"notes.go":{"head":-1}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if state := a.readSession("."); state.Positions != nil {
		t.Fatal("invalid position accepted")
	}
}
func TestEditorSessionRejectsUnboundedOrContentPayloads(t *testing.T) {
	isolateEditorSessions(t)
	a := testApp(t)
	for _, body := range []string{`{"content":"document text"}`, `{"positions":{"x":{"head":-1}}}`, `{"positions":{"x":{"scroll":-1}}}`, `{} {}`, `{"file":"` + strings.Repeat("x", 65536) + `"}`} {
		if response := postEditorSession(a, body); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid payload status %d", response.Code)
		}
	}
	positions := map[string]editorPosition{}
	for i := 0; i < 101; i++ {
		positions[string(rune(i+32))] = editorPosition{}
	}
	data, _ := json.Marshal(editorSession{Positions: positions})
	if response := postEditorSession(a, string(data)); response.Code != http.StatusBadRequest {
		t.Fatalf("too many positions: %d", response.Code)
	}
	request := httptest.NewRequest(http.MethodPost, "/session?jade=../outside", strings.NewReader(`{}`))
	request.Host = "127.0.0.1:7333"
	response := httptest.NewRecorder()
	a.handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("outside workspace: %d", response.Code)
	}
	postEditorSession(a, `{"file":"notes.go"}`)
	path, _ := a.sessionPath(".")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("session permissions %v", info.Mode())
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatal("temporary session files leaked")
	}
}
