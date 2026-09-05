package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/gofrs/flock"
)

// Each browser page owns a separate draft, so one tab cannot replace another's work.
type editorDraft struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Revision string `json:"revision"`
	Updated  string `json:"updated"`
	Token    string `json:"token"`
}

var draftID = regexp.MustCompile(`^[a-f0-9-]{36}$`)
var draftRevision = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (a *app) draftDirectory(jade, file string) (string, error) {
	directory, err := workspaceDirectory(a.root, jade)
	if err != nil {
		return "", err
	}
	if file == "" {
		return "", errors.New("file is required")
	}
	path, err := safeJoin(directory, file)
	if err != nil {
		return "", err
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "JaDE", "drafts", fileRevision(path)), nil
}

func readDraft(path string) (editorDraft, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return editorDraft{}, err
	}
	var draft editorDraft
	err = json.Unmarshal(raw, &draft)
	draft.Token = fileRevision(string(raw))
	return draft, err
}

func (a *app) drafts(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodDelete {
		http.Error(response, "method not allowed", 405)
		return
	}
	if !parseForm(response, request) {
		return
	}
	directory, err := a.draftDirectory(request.FormValue("jade"), request.FormValue("file"))
	if err != nil {
		http.Error(response, err.Error(), 400)
		return
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		http.Error(response, err.Error(), 500)
		return
	}
	// OS locks coordinate separate JaDE processes and release automatically after a crash.
	lock := flock.New(filepath.Join(directory, ".lock"))
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil || !locked {
		http.Error(response, "Recovery drafts are busy; try again", http.StatusServiceUnavailable)
		return
	}
	defer lock.Close()
	if request.Method == http.MethodGet {
		entries, err := os.ReadDir(directory)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			http.Error(response, err.Error(), 500)
			return
		}
		drafts := []editorDraft{}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			draft, err := readDraft(filepath.Join(directory, entry.Name()))
			if err != nil {
				http.Error(response, "Cannot read a recovery draft: "+err.Error(), 500)
				return
			}
			drafts = append(drafts, draft)
		}
		sort.Slice(drafts, func(i, j int) bool { return drafts[i].Updated > drafts[j].Updated })
		writeJSON(response, 200, map[string]any{"drafts": drafts})
		return
	}
	id := request.FormValue("id")
	if !draftID.MatchString(id) {
		http.Error(response, "invalid draft id", 400)
		return
	}
	path := filepath.Join(directory, id+".json")
	if request.Method == http.MethodDelete {
		draft, err := readDraft(path)
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(response, 200, map[string]bool{"deleted": true})
			return
		}
		if err != nil {
			http.Error(response, err.Error(), 500)
			return
		}
		if draft.Token != request.FormValue("token") {
			http.Error(response, "Draft changed in another tab; refresh before discarding", 409)
			return
		}
		if err = os.Remove(path); err != nil {
			http.Error(response, err.Error(), 500)
			return
		}
		writeJSON(response, 200, map[string]bool{"deleted": true})
		return
	}
	contents, revision := request.FormValue("content"), request.FormValue("revision")
	if len(contents) > maximumTextBytes || !utf8.ValidString(contents) || !draftRevision.MatchString(revision) {
		http.Error(response, "draft must contain UTF-8 text within the size limit and a file revision", 400)
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		http.Error(response, err.Error(), 500)
		return
	}
	count := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			count++
		}
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) && count >= 32 {
		http.Error(response, "Too many recovery drafts for this file; download or discard old drafts first", 409)
		return
	}
	draft := editorDraft{ID: id, Content: contents, Revision: revision, Updated: time.Now().UTC().Format(time.RFC3339Nano)}
	raw, err := json.Marshal(draft)
	if err == nil {
		err = replaceFile(path, string(raw), 0600, nil)
	}
	if err != nil {
		http.Error(response, err.Error(), 500)
		return
	}
	draft.Token = fileRevision(string(raw))
	writeJSON(response, 200, draft)
}
