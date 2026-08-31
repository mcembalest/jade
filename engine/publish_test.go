package engine

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func runGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func initializeRepository(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "JaDE Test")
	runGit(t, root, "config", "user.email", "jade@example.test")
	runGit(t, root, "config", "commit.gpgsign", "false")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "Initial")
}

func TestRepositoryStateAllowsParentRepository(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "engine")
	writeTestFile(t, filepath.Join(inner, markerName), "# Engine\n")
	initializeRepository(t, root)
	runGit(t, root, "remote", "add", "origin", "git@github.com:example/parent.git")

	application, err := newApp(inner, 7333)
	if err != nil {
		t.Fatal(err)
	}
	state, err := application.repositoryState(".")
	if err != nil {
		t.Fatal(err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.RepoRoot != realRoot || state.Root != filepath.Base(realRoot)+" (parent repository)" {
		t.Fatalf("parent repository state = %#v", state)
	}
}

func TestRepositoryStateRecognizesNestedRepoWorktree(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, markerName), "# Outer\n")
	source := filepath.Join(root, "source")
	writeTestFile(t, filepath.Join(source, "seed.txt"), "seed\n")
	initializeRepository(t, source)
	runGit(t, source, "remote", "add", "origin", "git@github.com:example/nested.git")
	worktree := filepath.Join(root, "parallel")
	runGit(t, source, "worktree", "add", "-b", "parallel-work", worktree)
	writeTestFile(t, filepath.Join(worktree, markerName), "# Parallel\n")

	application, err := newApp(root, 7333)
	if err != nil {
		t.Fatal(err)
	}
	state, err := application.repositoryState("parallel")
	if err != nil {
		t.Fatal(err)
	}
	realWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Worktree || state.Branch != "parallel-work" || state.RepoRoot != realWorktree {
		t.Fatalf("worktree state = %#v", state)
	}
}

func TestGitHubPublishCommitsPushesAndReturnsReviewURL(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, markerName), "# Publish\n")
	writeTestFile(t, filepath.Join(root, "note.md"), "before\n")
	initializeRepository(t, root)

	bare := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, filepath.Dir(bare), "init", "--bare", bare)
	runGit(t, root, "remote", "add", "origin", "https://github.com/example/jade-test.git")
	runGit(t, root, "remote", "set-url", "--push", "origin", bare)
	runGit(t, root, "push", "-u", "origin", "main")
	runGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	runGit(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	runGit(t, root, "switch", "-c", "publish-flow")
	writeTestFile(t, filepath.Join(root, "note.md"), "after\n")

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err = os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	application, err := newApp(root, 7333)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"jade": {"."}, "message": {"Publish note"}}
	response := postForm(application.handler(), "/publish/github", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	var result map[string]string
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("publish = %d: %#v", response.Code, result)
	}
	wantURL := "https://github.com/example/jade-test/compare/main...publish-flow?expand=1"
	if result["url"] != wantURL {
		t.Fatalf("review URL = %q, want %q", result["url"], wantURL)
	}
	published := runGit(t, bare, "show", "refs/heads/publish-flow:note.md")
	if published != "after" {
		t.Fatalf("published note = %q", published)
	}
}

func TestSubstackDraftSeparatesTitleAndBody(t *testing.T) {
	application := testApp(t)
	form := url.Values{
		"file":    {"essay.md"},
		"content": {"# Exact title\n\nA **clear** body.\n"},
	}
	response := postForm(application.handler(), "/publish/substack", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	var draft map[string]string
	if err := json.NewDecoder(response.Body).Decode(&draft); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || draft["title"] != "Exact title" || draft["text"] != "A **clear** body." || !strings.Contains(draft["html"], "<strong>clear</strong>") || strings.Contains(draft["html"], "<h1>") {
		t.Fatalf("draft = %d %#v", response.Code, draft)
	}
}

func TestArxivPublishPackagesPaperSources(t *testing.T) {
	application := testApp(t)
	writeTestFile(t, filepath.Join(application.root, "paper", "main.tex"), "\\documentclass{article}\n")
	writeTestFile(t, filepath.Join(application.root, "paper", "references.bib"), "@book{x}\n")
	writeTestFile(t, filepath.Join(application.root, "paper", "main.log"), "generated\n")
	writeTestFile(t, filepath.Join(application.root, "paper", "build", "main.pdf"), "generated\n")

	form := url.Values{"jade": {"."}, "file": {"paper/main.tex"}}
	response := postForm(application.handler(), "/publish/arxiv", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusOK {
		t.Fatalf("arXiv package = %d: %s", response.Code, response.Body.String())
	}
	if disposition := response.Header().Get("Content-Disposition"); disposition != `attachment; filename="main-arxiv.zip"` {
		t.Fatalf("content disposition = %q", disposition)
	}
	archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "main.tex,references.bib" {
		t.Fatalf("arXiv package files = %#v", names)
	}
}
