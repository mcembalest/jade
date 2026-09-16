package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectSyncStatusDistinguishesOptInAndStaleHelper(t *testing.T) {
	support := t.TempDir()
	root := t.TempDir()
	now := time.Now()
	put := func(name string, value any) {
		b, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(support, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	config := func(cloud bool) {
		put("remote.json", map[string]any{"agentToken": "never-expose", "roots": []any{map[string]any{"id": "p", "path": root, "cloud": cloud}}})
	}
	config(false)
	if v := projectSyncStatus(root, support, now); !v.Enabled || v.Mode != "project" || !strings.Contains(v.Message, "off") {
		t.Fatal(v)
	}
	config(true)
	put("cloud-status.json", map[string]any{"checkedAt": now.Add(-2 * time.Minute).Unix(), "projects": map[string]string{"p": "Everything checked"}})
	if v := projectSyncStatus(root, support, now); !strings.Contains(v.Message, "not checked recently") {
		t.Fatal(v)
	}
	put("cloud-status.json", map[string]any{"checkedAt": now.Unix(), "projects": map[string]string{"p": "Conflict: keep both"}})
	if v := projectSyncStatus(root, support, now); !strings.Contains(v.Message, "Conflict") {
		t.Fatal(v)
	}
	if v := projectSyncStatus(t.TempDir(), support, now); v.Enabled {
		t.Fatal(v)
	}
}
