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
	title, commands := ParseJade("# A paper\n\nProse first.\n\n```sh\ntypst compile paper.typ paper.pdf\n```\n\n```python\nprint('not runnable')\n```\n\n```bash\nmake fetch\nmake all\n```\n")
	if title != "A paper" {
		t.Fatalf("title = %q", title)
	}
	if len(commands) != 2 || commands[0] != "typst compile paper.typ paper.pdf" || commands[1] != "make fetch\nmake all" {
		t.Fatalf("commands = %#v", commands)
	}

	title, commands = ParseJade("plain marker, no heading, no fences\n")
	if title != "Untitled Jade" || len(commands) != 0 {
		t.Fatalf("plain markdown should be a valid marker: %q %#v", title, commands)
	}

	_, commands = ParseJade("````markdown\n```sh\nnested example, not runnable\n```\n````\n")
	if len(commands) != 0 {
		t.Fatalf("documented sh fences should not become commands: %#v", commands)
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
	if len(workspace.Children) != 1 || workspace.Children[0].Path != "synthetic" {
		t.Fatalf("unexpected children: %#v", workspace.Children)
	}
	if len(workspace.Files) != 4 {
		t.Fatalf("outer Jade should still see ordinary files inside nested Jades: %#v", workspace.Files)
	}
	if len(workspace.Commands) != 0 {
		t.Fatalf("outer Jade should not inherit nested commands: %#v", workspace.Commands)
	}

	child, err := LoadWorkspace(root, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if child.Parent != "." || child.Title != "Synthetic data" {
		t.Fatalf("unexpected child: %#v", child)
	}
	if len(child.Commands) != 1 || child.Commands[0] != "printf 'x\\n1\\n' > data.csv" {
		t.Fatalf("unexpected child commands: %#v", child.Commands)
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
