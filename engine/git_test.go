package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
)

func TestBranchesListsAndSwitchesLocalBranches(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n")
	initializeRepository(t, root)
	runGit(t, root, "branch", "feature")
	application, err := newApp(root, 7333)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/git/branches?jade=.", nil)
	request.Host = "127.0.0.1:7333"
	response := httptest.NewRecorder()
	application.handler().ServeHTTP(response, request)
	var state branchState
	if err = json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || state.Current != "main" || len(state.Branches) != 2 || state.Branches[0] != "feature" || state.Branches[1] != "main" {
		t.Fatalf("branches = %d %#v", response.Code, state)
	}

	form := url.Values{"jade": {"."}, "branch": {"feature"}}
	switched := postForm(application.handler(), "/git/switch", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if switched.Code != http.StatusOK {
		t.Fatalf("switch = %d: %s", switched.Code, switched.Body.String())
	}
	if current := runGit(t, root, "branch", "--show-current"); current != "feature" {
		t.Fatalf("current branch = %q", current)
	}
}
