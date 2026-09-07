package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProjectRootsMustBeExplicitDirectories(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "note.txt")
	writeTestFile(t, file, "note")
	for _, path := range []string{"", ".", "../other", file, filepath.Join(root, "missing")} {
		if _, err := projectRoot(path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	actual, err := projectRoot(root)
	if err != nil || actual == "" {
		t.Fatal(actual, err)
	}
}

func TestDesktopProjectsStayIsolatedReuseServersAndSkipSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry := newProjectRegistry(ctx)
	registry.recentFile = filepath.Join(t.TempDir(), "recent.json")
	roots := []string{t.TempDir(), t.TempDir()}
	writeTestFile(t, filepath.Join(roots[0], "notes.txt"), "first project")
	writeTestFile(t, filepath.Join(roots[1], "notes.txt"), "second project")
	// A broken sync config would prevent normal startup. Editor-only projects
	// must not inspect it or create sync state, even when the config is present.
	writeTestFile(t, filepath.Join(roots[1], ".jade-sync", "config.json"), "invalid json")
	first, err := registry.open(roots[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.open(roots[1])
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("roots share a listener")
	}
	client := &http.Client{Timeout: time.Second}
	for index, url := range []string{first, second} {
		response, err := client.Get(url + "/file?jade=.&file=notes.txt")
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || !strings.Contains(string(data), []string{"first project", "second project"}[index]) {
			t.Fatalf("wrong root: %s", data)
		}
	}
	response, err := client.Get(second + "/sync")
	if err != nil {
		t.Fatal(err)
	}
	var state struct{ Enabled bool }
	json.NewDecoder(response.Body).Decode(&state)
	response.Body.Close()
	if state.Enabled {
		t.Fatal("child started sync")
	}
	var wait sync.WaitGroup
	for index := 0; index < 5; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			url, err := registry.open(roots[0])
			if err != nil || url != first {
				t.Errorf("server not reused: %s %v", url, err)
			}
		}()
	}
	wait.Wait()
	if err := registry.remember(roots[1]); err != nil {
		t.Fatal(err)
	}
	if err := registry.remember(roots[0]); err != nil {
		t.Fatal(err)
	}
	restored := newProjectRegistry(ctx)
	restored.recentFile = registry.recentFile
	if paths := restored.recent(); len(paths) != 2 || paths[0] != roots[0] {
		t.Fatal(paths)
	}
	cancel()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		registry.mu.Lock()
		remaining := len(registry.urls)
		registry.mu.Unlock()
		if remaining == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child listeners did not stop")
}

func TestProjectEndpointRejectsCrossOriginAndNonJSON(t *testing.T) {
	application, err := newApp(t.TempDir(), 45678)
	if err != nil {
		t.Fatal(err)
	}
	application.projects = newProjectRegistry(context.Background())
	application.projects.recentFile = filepath.Join(t.TempDir(), "recent.json")
	for _, test := range []struct {
		method, origin, content string
		status                  int
	}{
		{"POST", "https://example.com", "application/json", 403},
		{"POST", "", "text/plain", 415},
		{"DELETE", "", "", 405},
		{"POST", "", "application/json", 400},
	} {
		req := httptest.NewRequest(test.method, "http://127.0.0.1:45678/projects", strings.NewReader(`{"path":"relative"}`))
		req.Header.Set("Origin", test.origin)
		req.Header.Set("Content-Type", test.content)
		out := httptest.NewRecorder()
		application.handler().ServeHTTP(out, req)
		if out.Code != test.status {
			t.Fatalf("status %d want %d: %s", out.Code, test.status, out.Body.String())
		}
	}
	if _, err := os.Stat(application.projects.recentFile); !os.IsNotExist(err) {
		t.Fatal("rejected request altered recents")
	}
}

func TestPrimaryServerWaitsForChildShutdown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan string, 1)
	done := make(chan error, 1)
	root := t.TempDir()
	go func() { done <- Serve(ctx, root, "127.0.0.1:0", func(url string) { ready <- url }) }()
	var primary string
	select {
	case primary = <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("startup timeout")
	}
	// Open the same root reuses the primary listener; an unrelated root creates a child.
	childRoot := t.TempDir()
	payload, _ := json.Marshal(map[string]string{"path": childRoot})
	client := &http.Client{Timeout: time.Second}
	response, err := client.Post(primary+"/projects", "application/json", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ URL string }
	json.NewDecoder(response.Body).Decode(&result)
	response.Body.Close()
	if result.URL == "" {
		t.Fatal("child not opened")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("shutdown timeout")
	}
	if response, err := client.Get(result.URL); err == nil {
		response.Body.Close()
		t.Fatal("child listener survived primary shutdown")
	}
}
