package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Each explicitly opened project keeps its own immutable root and loopback
// listener. Opening a desktop project never starts another Notes sync worker.
type projectRegistry struct {
	mu         sync.Mutex
	children   sync.WaitGroup
	ctx        context.Context
	urls       map[string]string
	recentFile string
}

func newProjectRegistry(ctx context.Context) *projectRegistry {
	config, err := os.UserConfigDir()
	filename := ""
	if err == nil {
		filename = filepath.Join(config, "JaDE", "recent-projects.json")
	}
	return &projectRegistry{ctx: ctx, urls: map[string]string{}, recentFile: filename}
}

func projectRoot(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("Enter an absolute folder path on this Mac (or start with ~/).")
	}
	root, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("Choose a folder, not a file.")
	}
	directory, err := os.Open(root)
	if err != nil {
		return "", err
	}
	defer directory.Close()
	if _, err = directory.Readdirnames(1); err != nil && err != io.EOF {
		return "", err
	}
	return filepath.Clean(root), nil
}

func (p *projectRegistry) recent() []string {
	var paths []string
	data, err := os.ReadFile(p.recentFile)
	if err == nil && len(data) <= 65536 {
		_ = json.Unmarshal(data, &paths)
	}
	if len(paths) > 12 {
		paths = paths[:12]
	}
	if paths == nil {
		paths = []string{}
	}
	return paths
}

func (p *projectRegistry) remember(root string) error {
	paths := []string{root}
	for _, path := range p.recent() {
		if path != root && len(paths) < 12 {
			paths = append(paths, path)
		}
	}
	if p.recentFile == "" {
		return errors.New("Recent projects storage is unavailable.")
	}
	if err := os.MkdirAll(filepath.Dir(p.recentFile), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(p.recentFile), "recent-projects-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), p.recentFile)
}

func (p *projectRegistry) open(root string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ctx.Err(); err != nil {
		return "", err
	}
	if url := p.urls[root]; url != "" {
		return url, nil
	}
	ready := make(chan string, 1)
	finished := make(chan error, 1)
	p.children.Add(1)
	go func() {
		defer p.children.Done()
		finished <- serveProject(p.ctx, root, "127.0.0.1:0", func(url string) { ready <- url }, p, false)
	}()
	select {
	case err := <-finished:
		return "", err
	case <-p.ctx.Done():
		return "", p.ctx.Err()
	case url := <-ready:
		p.urls[root] = url
		go func() {
			<-finished
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.urls[root] == url {
				delete(p.urls, root)
			}
		}()
		return url, nil
	}
}

func (a *app) projectsHTTP(response http.ResponseWriter, request *http.Request) {
	if a.projects == nil {
		http.Error(response, "Project switching is unavailable in this server.", http.StatusServiceUnavailable)
		return
	}
	switch request.Method {
	case http.MethodGet:
		a.projects.mu.Lock()
		recent := a.projects.recent()
		a.projects.mu.Unlock()
		writeJSON(response, http.StatusOK, map[string]any{"current": a.root, "recent": recent})
	case http.MethodPost:
		media, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if media != "application/json" {
			http.Error(response, "Expected application/json", http.StatusUnsupportedMediaType)
			return
		}
		var input struct {
			Path string `json:"path"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 8192))
		if err := decoder.Decode(&input); err != nil {
			http.Error(response, "Invalid project request", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(response, "Invalid project request", http.StatusBadRequest)
			return
		}
		root, err := projectRoot(input.Path)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		url, err := a.projects.open(root)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		a.projects.mu.Lock()
		recentErr := a.projects.remember(a.root)
		if recentErr == nil {
			recentErr = a.projects.remember(root)
		}
		a.projects.mu.Unlock()
		if recentErr != nil {
			url += "?projects-warning=recents"
		}
		writeJSON(response, http.StatusOK, map[string]string{"url": url})
	default:
		response.Header().Set("Allow", "GET, POST")
		http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
