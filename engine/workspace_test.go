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

func TestHomepageTitle(t *testing.T) {
	if title := homepageTitle("```markdown\n# Example only\n```\n\n# A paper\n"); title != "A paper" {
		t.Fatalf("title = %q", title)
	}
	if title := homepageTitle("plain marker, no heading\n"); title != "Untitled JaDE" {
		t.Fatalf("fallback title = %q", title)
	}
}

func TestWorkspaceRecursionIsJustDirectories(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, homepageName), "# Project\n")
	writeTestFile(t, filepath.Join(root, "notes.md"), "ordinary note")
	writeTestFile(t, filepath.Join(root, "synthetic", homepageName), "# Synthetic data\n\n```sh\nprintf 'x\\n1\\n' > data.csv\n```\n")
	writeTestFile(t, filepath.Join(root, "synthetic", "generate.go"), "package main")
	writeTestFile(t, filepath.Join(root, ".git", "ignored.txt"), "ignored")
	writeTestFile(t, filepath.Join(root, ".build", "generated.swift"), "ignored")
	writeTestFile(t, filepath.Join(root, ".deps", "dependency.swift"), "ignored")
	writeTestFile(t, filepath.Join(root, ".tmp", "jade-engine.log"), "ignored")

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
	found, err := ResolveWorkspaceRoot(filepath.Join(root, "synthetic", "generate.go"))
	want, realErr := filepath.EvalSymlinks(filepath.Join(root, "synthetic"))
	if err != nil || realErr != nil || found != want {
		t.Fatalf("workspace root = %q, %v", found, err)
	}
}

func TestWorkspaceDoesNotRequireHomepage(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n")

	workspace, err := LoadWorkspace(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Homepage != "" || workspace.Title != filepath.Base(root) || len(workspace.Files) != 1 || workspace.Files[0] != "main.go" {
		t.Fatalf("plain workspace = %#v", workspace)
	}
}

func TestFilesStayInsideTheLaunchedFolder(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(root, homepageName), "# Safe\n")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	if err := CreateWorkspaceFile(root, ".", "notes/today.md", "hello"); err != nil {
		t.Fatal(err)
	}
	contents, err := ReadWorkspaceFile(root, ".", "notes/today.md")
	if err != nil || contents != "hello" {
		t.Fatalf("read = %q, %v", contents, err)
	}
	if err := CreateWorkspaceFile(root, ".", "../outside.md", "no"); err == nil {
		t.Fatal("expected traversal to fail")
	}
	if err := CreateWorkspaceFile(root, ".", "escape/outside.md", "no"); err == nil {
		t.Fatal("expected symlink escape to fail")
	}
	writeTestFile(t, filepath.Join(outside, "target.md"), "outside")
	if err := os.Symlink(filepath.Join(outside, "target.md"), filepath.Join(root, "linked.md")); err != nil {
		t.Fatal(err)
	}
	if err := CreateWorkspaceFile(root, ".", "linked.md", "no"); err == nil {
		t.Fatal("expected write through a file symlink to fail")
	}
}

func TestNestedFoldersShareFilesWithoutRequiringHomepages(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "shared.txt"), "original")
	writeTestFile(t, filepath.Join(root, "study", "report", "notes.txt"), "notes")
	workspace, err := LoadWorkspace(root, "study/report")
	if err != nil || workspace.Homepage != "" || workspace.Title != "report" {
		t.Fatalf("nested folder = %#v, %v", workspace, err)
	}
	contents, err := ReadWorkspaceFile(root, "study/report", "../../shared.txt")
	if err != nil || contents != "original" {
		t.Fatalf("shared read = %q, %v", contents, err)
	}
	if err := updateWorkspaceFile(root, "study/report", "../../shared.txt", "updated", fileRevision(contents)); err != nil {
		t.Fatal(err)
	}
	if contents, err := ReadWorkspaceFile(root, ".", "shared.txt"); err != nil || contents != "updated" {
		t.Fatalf("shared write = %q, %v", contents, err)
	}
	if err := CreateWorkspaceFile(root, "study/report", "../../other/new.md", "shared sibling"); err != nil {
		t.Fatal(err)
	}
	a, err := newApp(root, 7333)
	if err != nil {
		t.Fatal(err)
	}
	parentDraft, err := a.draftDirectory(".", "shared.txt")
	if err != nil {
		t.Fatal(err)
	}
	childDraft, err := a.draftDirectory("study/report", "../../shared.txt")
	if err != nil || childDraft != parentDraft {
		t.Fatalf("one file must share recovery across views: %q, %q, %v", parentDraft, childDraft, err)
	}
	if _, err := ReadWorkspaceFile(root, "study/report", "../../../outside.txt"); err == nil {
		t.Fatal("nested views must retain the launched folder boundary")
	}
}

func TestReadmeIsAnOrdinaryCaseInsensitiveHomepage(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "readme.MD"), "# Mixed case\n")
	writeTestFile(t, filepath.Join(root, "a.txt"), "first alphabetically")
	workspace, err := LoadWorkspace(root, ".")
	if err != nil || workspace.Homepage != "readme.MD" || workspace.Title != "Mixed case" || workspace.Files[0] != "readme.MD" {
		t.Fatalf("homepage = %#v, %v", workspace, err)
	}
}
