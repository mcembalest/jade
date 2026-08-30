package main

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

func TestParseJade(t *testing.T) {
	parsed, err := ParseJade("# A paper\n\nArtifact: out/paper.pdf\nCommand: typst compile paper.typ out/paper.pdf\n")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Title != "A paper" || parsed.Artifact != "out/paper.pdf" || parsed.Command == "" {
		t.Fatalf("unexpected declaration: %#v", parsed)
	}

	fenced, err := ParseJade("# Doc\n\n```markdown\nArtifact: fake.pdf\nCommand: make fake\n```\n\nArtifact: real.pdf\n")
	if err != nil {
		t.Fatal(err)
	}
	if fenced.Artifact != "real.pdf" || fenced.Command != "" {
		t.Fatalf("fenced declarations should be ignored: %#v", fenced)
	}

	bad := []string{
		"# Duplicate\nArtifact: one.pdf\nArtifact: two.pdf\n",
		"# Escape\nArtifact: ../outside.pdf\n",
		"# Hidden command\nCommand: make\n",
	}
	for _, markdown := range bad {
		if _, err := ParseJade(markdown); err == nil {
			t.Fatalf("expected %q to fail", markdown)
		}
	}
}

func TestWorkspaceRecursionIsJustDirectories(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, markerName), "# Project\n")
	writeTestFile(t, filepath.Join(root, "notes.md"), "ordinary note")
	writeTestFile(t, filepath.Join(root, "synthetic", markerName), "# Synthetic data\nArtifact: data.csv\nCommand: printf 'x\\n1\\n' > data.csv\n")
	writeTestFile(t, filepath.Join(root, "synthetic", "generate.go"), "package main")
	writeTestFile(t, filepath.Join(root, ".git", "ignored.txt"), "ignored")
	writeTestFile(t, filepath.Join(root, ".pixi", "ignored.txt"), "ignored")

	workspace, err := LoadWorkspace(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Children) != 1 || workspace.Children[0].Path != "synthetic" {
		t.Fatalf("unexpected children: %#v", workspace.Children)
	}
	if len(workspace.Files) != 4 {
		t.Fatalf("outer Jade should still see ordinary files inside nested Jades: %#v", workspace.Files)
	}

	child, err := LoadWorkspace(root, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if child.Parent != "." || child.Title != "Synthetic data" {
		t.Fatalf("unexpected child: %#v", child)
	}
	found, err := FindJadeRoot(filepath.Join(root, "synthetic", "generate.go"))
	want, realErr := filepath.EvalSymlinks(filepath.Join(root, "synthetic"))
	if err != nil || realErr != nil || found != want {
		t.Fatalf("nearest Jade root = %q, %v", found, err)
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

func TestInvalidMarkerIsNotWritten(t *testing.T) {
	root := t.TempDir()
	original := "# Valid\n"
	writeTestFile(t, filepath.Join(root, markerName), original)
	if err := WriteWorkspaceFile(root, ".", markerName, "# Invalid\nCommand: make\n"); err == nil {
		t.Fatal("expected invalid marker to fail")
	}
	contents, err := os.ReadFile(filepath.Join(root, markerName))
	if err != nil || string(contents) != original {
		t.Fatalf("marker changed: %q, %v", contents, err)
	}
}
