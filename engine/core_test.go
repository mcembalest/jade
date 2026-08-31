package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestJadeTitle(t *testing.T) {
	if title := jadeTitle("```markdown\n# Example only\n```\n\n# A paper\n"); title != "A paper" {
		t.Fatalf("title = %q", title)
	}
	if title := jadeTitle("plain marker, no heading\n"); title != "Untitled JaDE" {
		t.Fatalf("fallback title = %q", title)
	}
}

func TestWorkspaceRecursionIsJustDirectories(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, markerName), "# Project\n")
	writeTestFile(t, filepath.Join(root, "notes.md"), "ordinary note")
	writeTestFile(t, filepath.Join(root, "synthetic", markerName), "# Synthetic data\n\n```sh\nprintf 'x\\n1\\n' > data.csv\n```\n")
	writeTestFile(t, filepath.Join(root, "synthetic", "generate.go"), "package main")
	writeTestFile(t, filepath.Join(root, ".git", "ignored.txt"), "ignored")

	workspace, err := LoadWorkspace(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Files) != 4 {
		t.Fatalf("outer JaDE should still see ordinary files inside nested JaDEs: %#v", workspace.Files)
	}

	child, err := LoadWorkspace(root, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if child.Title != "Synthetic data" || child.Path != "synthetic" {
		t.Fatalf("unexpected child workspace: %#v", child)
	}
	found, err := FindJadeRoot(filepath.Join(root, "synthetic", "generate.go"))
	want, realErr := filepath.EvalSymlinks(filepath.Join(root, "synthetic"))
	if err != nil || realErr != nil || found != want {
		t.Fatalf("nearest JaDE root = %q, %v", found, err)
	}
}

func TestFilesStayInsideTheCurrentJade(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(root, markerName), "# Safe\n")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	if err := WriteWorkspaceFile(root, ".", "notes/today.md", "hello"); err != nil {
		t.Fatal(err)
	}
	contents, err := ReadWorkspaceFile(root, ".", "notes/today.md")
	if err != nil || contents != "hello" {
		t.Fatalf("read = %q, %v", contents, err)
	}
	if err := WriteWorkspaceFile(root, ".", "../outside.md", "no"); err == nil {
		t.Fatal("expected traversal to fail")
	}
	if err := WriteWorkspaceFile(root, ".", "escape/outside.md", "no"); err == nil {
		t.Fatal("expected symlink escape to fail")
	}
	writeTestFile(t, filepath.Join(outside, "target.md"), "outside")
	if err := os.Symlink(filepath.Join(outside, "target.md"), filepath.Join(root, "linked.md")); err != nil {
		t.Fatal(err)
	}
	if err := WriteWorkspaceFile(root, ".", "linked.md", "no"); err == nil {
		t.Fatal("expected write through a file symlink to fail")
	}
}
