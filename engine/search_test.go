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

func TestSearchTextScopeAndBounds(t *testing.T) {
	a := testApp(t)
	writeTestFile(t, filepath.Join(a.root, "Café.md"), "# Café\n\nFind CHELLAM here.\n")
	writeTestFile(t, filepath.Join(a.root, "inner", "private.txt"), "inner phrase")
	writeTestFile(t, filepath.Join(a.root, "node_modules", "hidden.txt"), "CHELLAM")
	writeTestFile(t, filepath.Join(a.root, "binary.bin"), "CHELLAM\x00")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	writeTestFile(t, outside, "CHELLAM")
	if err := os.Symlink(outside, filepath.Join(a.root, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	get := func(target string) (int, []searchResult, bool) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, target, nil)
		r.Host = "127.0.0.1:7333"
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		var data struct {
			Results    []searchResult
			Incomplete bool
		}
		if w.Code == 200 {
			if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
		}
		return w.Code, data.Results, data.Incomplete
	}
	code, results, incomplete := get("/search?q=chellam")
	if code != 200 || incomplete || len(results) != 1 || results[0].File != "Café.md" || results[0].Line != 3 {
		t.Fatalf("search: %d %#v %v", code, results, incomplete)
	}
	_, results, _ = get("/search?q=CAF%C3%89")
	if len(results) != 2 || results[0].Line != 0 {
		t.Fatalf("filename and Unicode content: %#v", results)
	}
	_, results, _ = get("/search?jade=inner&q=chellam")
	if len(results) != 0 {
		t.Fatalf("escaped inner workspace: %#v", results)
	}
	for _, target := range []string{"/search?q=x&jade=..", "/search?q=", "/search?q=" + strings.Repeat("x", 201)} {
		if code, _, _ := get(target); code != 400 {
			t.Fatalf("%s = %d", target, code)
		}
	}
	writeTestFile(t, filepath.Join(a.root, "many.txt"), strings.Repeat("CHELLAM\n", 120))
	_, results, incomplete = get("/search?q=chellam")
	if len(results) != 100 || !incomplete {
		t.Fatalf("result cap: %d %v", len(results), incomplete)
	}
}

func TestOrdinaryMarkdownPreviewWithoutMarker(t *testing.T) {
	a := testApp(t)
	if err := os.Remove(filepath.Join(a.root, homepageName)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(a.root, "draft.MD"), "# My draft\n")
	for _, view := range []string{"", "missing.md"} {
		data, err := a.pageData(".", "draft.MD", view, false)
		if err != nil || !data.Markdown || data.ViewURL != "/view?jade=.&file=draft.MD" {
			t.Fatalf("preview: %#v %v", data, err)
		}
	}
}
