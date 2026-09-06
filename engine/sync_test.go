package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyncPathsAndSymlinkBoundaries(t *testing.T) {
	for _, p := range []string{"../notes.md", ".jade-sync/key.txt", "/a.md", "a//b.md", "a/../b.md", "file.pdf", "a\\b.md"} {
		if validSyncPath(p) {
			t.Fatalf("accepted unsafe path %q", p)
		}
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	s := &workspaceSync{root: root}
	if err := s.writeLocal("escape/no.md", "private", nil); err == nil {
		t.Fatal("followed symlink directory")
	}
	if _, err := os.Stat(filepath.Join(outside, "no.md")); !os.IsNotExist(err) {
		t.Fatal("wrote outside workspace")
	}
	if err := s.writeLocal("notes/a.md", "first", nil); err != nil {
		t.Fatal(err)
	}
	wrong := "stale"
	if err := s.writeLocal("notes/a.md", "overwrite", &wrong); err == nil {
		t.Fatal("overwrote concurrent local edit")
	}
}

func TestCloudflareDesktopIntegration(t *testing.T) {
	endpoint := os.Getenv("JADE_TEST_SYNC_URL")
	if endpoint == "" {
		t.Skip("set JADE_TEST_SYNC_URL to the local test-server.mjs endpoint")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".jade-sync"), 0700); err != nil {
		t.Fatal(err)
	}
	s := &workspaceSync{root: root, config: syncConfig{Endpoint: endpoint, Token: "local-test-key-not-for-production-123456"}, client: &http.Client{Timeout: 5 * time.Second}, records: map[string]*syncRecord{}, status: syncView{Files: map[string]string{}}}
	ctx := context.Background()
	path := "desktop-" + mutationID() + ".md"
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cycle := func() {
		t.Helper()
		if err := s.exchange(ctx); err != nil {
			t.Fatal(err)
		}
	}
	write("from Mac")
	cycle()
	r := s.records[path]
	if r.BaseRevision == "" {
		t.Fatal("not uploaded")
	}
	if strings.Contains(s.status.Files[path], "Synced with") {
		t.Fatal("claimed iPhone receipt before acknowledgement")
	}
	var result struct {
		AcceptedRevision string      `json:"acceptedRevision"`
		File             *remoteFile `json:"file"`
	}
	phone := syncMutation{Path: path, Content: "from iPhone", BaseRevision: r.BaseRevision, MutationID: mutationID(), DeviceID: "iphone"}
	if _, err := s.request(ctx, "POST", "/v1/files", phone, &result); err != nil {
		t.Fatal(err)
	}
	cycle()
	got, _ := s.local(path)
	if got != "from iPhone" {
		t.Fatalf("download = %q", got)
	}
	// Divergent edits must leave the Mac's text intact and preserve incoming text.
	write("unsynced Mac edit")
	phone.BaseRevision = s.records[path].BaseRevision
	phone.Content = "simultaneous iPhone edit"
	phone.MutationID = mutationID()
	if _, err := s.request(ctx, "POST", "/v1/files", phone, &result); err != nil {
		t.Fatal(err)
	}
	cycle()
	if s.records[path].Conflict == nil {
		t.Fatal("did not detect conflict")
	}
	got, _ = s.local(path)
	if got != "unsynced Mac edit" {
		t.Fatal("discarded local conflict")
	}
	if err := s.keepBoth(path); err != nil {
		t.Fatal(err)
	}
	got, _ = s.local(path)
	if got != phone.Content {
		t.Fatal("incoming copy not installed")
	}
	copies, _ := filepath.Glob(filepath.Join(root, strings.TrimSuffix(path, ".md")+" (Mac conflict *).md"))
	if len(copies) != 1 {
		t.Fatalf("conflict copies: %v", copies)
	}
	raw, _ := os.ReadFile(copies[0])
	if string(raw) != "unsynced Mac edit" {
		t.Fatal("local copy lost")
	}
	cycle()
	// Crash after server commit but before response: replay the persisted operation.
	write("lost response edit")
	r = s.records[path]
	pending := &syncMutation{Path: path, Content: "lost response edit", BaseRevision: r.BaseRevision, MutationID: mutationID(), DeviceID: "mac"}
	r.Pending = pending
	if err := s.persist(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.request(ctx, "POST", "/v1/files", pending, &result); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(root, ".jade-sync", "state.json"))
	s.records = map[string]*syncRecord{}
	if err := json.Unmarshal(raw, &s.records); err != nil {
		t.Fatal(err)
	}
	cycle()
	if s.records[path].Pending != nil || s.records[path].Conflict != nil || s.records[path].BaseContent != "lost response edit" {
		t.Fatal("lost-response recovery failed")
	}
	// Deletion is deliberately local-only in the MVP; do not silently recreate it.
	if err := os.Remove(filepath.Join(root, path)); err != nil {
		t.Fatal(err)
	}
	cycle()
	if !strings.Contains(s.status.Files[path], "deletion sync not enabled") {
		t.Fatal("unreported deletion")
	}
}
