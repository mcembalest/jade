package engine

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type searchResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Excerpt string `json:"excerpt"`
}

// Search reads the same bounded text files the editor exposes, without an index.
func (a *app) search(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("q"))
	if query == "" || utf8.RuneCountInString(query) > 200 {
		http.Error(response, "Enter between 1 and 200 characters", http.StatusBadRequest)
		return
	}
	jade := queryPath(request, "jade", ".")
	root, err := workspaceDirectory(a.root, jade)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	results := []searchResult{}
	incomplete := false
	remaining := int64(32_000_000)
	deadline := time.Now().Add(2 * time.Second)
	needle := strings.ToLower(query)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if request.Context().Err() != nil {
			return request.Context().Err()
		}
		if time.Now().After(deadline) || len(results) >= 100 {
			incomplete = true
			return fs.SkipAll
		}
		if walkErr != nil {
			incomplete = true
			return nil
		}
		if path != root && (entry.Name() == ".git" || (entry.IsDir() && ignoredDirectories[entry.Name()])) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			incomplete = true
			return nil
		}
		if !editableText(path, info) {
			return nil
		}
		file := relativeSlash(root, path)
		if strings.Contains(strings.ToLower(file), needle) {
			results = append(results, searchResult{File: file})
		}
		if info.Size() > remaining {
			incomplete = true
			return nil
		}
		remaining -= info.Size()
		contents, err := ReadWorkspaceFile(a.root, jade, file)
		if err != nil {
			incomplete = true
			return nil
		}
		for line, text := range strings.Split(contents, "\n") {
			if line%256 == 0 && (request.Context().Err() != nil || time.Now().After(deadline)) {
				incomplete = true
				return fs.SkipAll
			}
			if len(results) >= 100 {
				incomplete = true
				return fs.SkipAll
			}
			lower := strings.ToLower(text)
			offset := strings.Index(lower, needle)
			if offset < 0 {
				continue
			}
			runes := []rune(text)
			start := max(0, utf8.RuneCountInString(lower[:offset])-60)
			end := min(len(runes), start+240)
			excerpt := strings.TrimSpace(string(runes[start:end]))
			if start > 0 {
				excerpt = "…" + excerpt
			}
			if end < len(runes) {
				excerpt += "…"
			}
			results = append(results, searchResult{file, line + 1, excerpt})
		}
		return nil
	})
	if err != nil {
		incomplete = true
	}
	writeJSON(response, http.StatusOK, map[string]any{"results": results, "incomplete": incomplete})
}
