package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gmtext "github.com/yuin/goldmark/text"
)

const publishTimeout = 2 * time.Minute

type repositoryState struct {
	Repository  string `json:"repository"`
	Root        string `json:"root"`
	Branch      string `json:"branch"`
	Changes     string `json:"changes"`
	PullRequest string `json:"pullRequest,omitempty"`
	Default     string `json:"defaultBranch,omitempty"`
	RemoteURL   string `json:"-"`
	RepoRoot    string `json:"-"`
	Worktree    bool   `json:"worktree"`
	CanPublish  bool   `json:"canPublish"`
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func gitOutput(ctx context.Context, directory string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}
	return strings.TrimSpace(string(output)), nil
}

func commandOutput(ctx context.Context, directory, name string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}
	return strings.TrimSpace(string(output)), nil
}

func githubRepositoryURL(remote string) string {
	remote = strings.TrimSpace(strings.TrimSuffix(remote, ".git"))
	if strings.HasPrefix(remote, "git@github.com:") {
		return "https://github.com/" + strings.TrimPrefix(remote, "git@github.com:")
	}
	if strings.HasPrefix(remote, "ssh://git@github.com/") {
		return "https://github.com/" + strings.TrimPrefix(remote, "ssh://git@github.com/")
	}
	parsed, err := url.Parse(remote)
	if err == nil && strings.EqualFold(parsed.Hostname(), "github.com") {
		return "https://github.com/" + strings.TrimPrefix(parsed.Path, "/")
	}
	return ""
}

func absoluteGitPath(repoRoot, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(repoRoot, path))
}

func (a *app) repositoryState(jadePath string) (repositoryState, error) {
	jadeRoot, err := jadeDirectory(a.root, jadePath)
	if err != nil {
		return repositoryState{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()
	repoRoot, err := gitOutput(ctx, jadeRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return repositoryState{}, errors.New("the active JaDE is not inside a Git repository")
	}
	repoRoot, err = filepath.EvalSymlinks(repoRoot)
	if err != nil || (!within(a.root, repoRoot) && !within(repoRoot, jadeRoot)) {
		return repositoryState{}, errors.New("the nearest Git repository does not contain the active JaDE")
	}
	branch, err := gitOutput(ctx, repoRoot, "branch", "--show-current")
	if err != nil || branch == "" {
		return repositoryState{}, errors.New("create a branch or worktree before publishing")
	}
	remote, err := gitOutput(ctx, repoRoot, "remote", "get-url", "origin")
	if err != nil || githubRepositoryURL(remote) == "" {
		return repositoryState{}, errors.New("the nearest repository needs a GitHub origin remote")
	}
	changes, _ := gitOutput(ctx, repoRoot, "status", "--short")
	defaultBranch, _ := gitOutput(ctx, repoRoot, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	defaultBranch = strings.TrimPrefix(defaultBranch, "origin/")
	gitDir, _ := gitOutput(ctx, repoRoot, "rev-parse", "--absolute-git-dir")
	commonDir, _ := gitOutput(ctx, repoRoot, "rev-parse", "--git-common-dir")
	worktree := gitDir != "" && commonDir != "" && absoluteGitPath(repoRoot, gitDir) != absoluteGitPath(repoRoot, commonDir)
	pullRequest := ""
	if _, lookErr := exec.LookPath("gh"); lookErr == nil {
		pullRequest, _ = commandOutput(ctx, repoRoot, "gh", "pr", "view", "--json", "url", "--jq", ".url")
	}
	rootLabel := filepath.Base(repoRoot)
	if within(a.root, repoRoot) {
		rootLabel = relativeSlash(a.root, repoRoot)
		if rootLabel == "." {
			rootLabel = filepath.Base(repoRoot)
		}
	} else {
		rootLabel += " (parent repository)"
	}
	isDefaultBranch := branch == defaultBranch
	if defaultBranch == "" {
		isDefaultBranch = branch == "main" || branch == "master"
	}
	state := repositoryState{
		Repository:  filepath.Base(repoRoot),
		Root:        rootLabel,
		Branch:      branch,
		Changes:     changes,
		PullRequest: pullRequest,
		Default:     defaultBranch,
		RemoteURL:   remote,
		RepoRoot:    repoRoot,
		Worktree:    worktree,
		CanPublish:  !isDefaultBranch,
	}
	return state, nil
}

func (a *app) publishStatus(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	state, err := a.repositoryState(queryPath(request, "jade", "."))
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !state.CanPublish {
		state.Changes = "The current branch is the repository’s default branch.\nCreate a branch or worktree before publishing.\n\n" + state.Changes
	}
	writeJSON(response, http.StatusOK, state)
}

func (a *app) publishGitHub(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	state, err := a.repositoryState(request.FormValue("jade"))
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !state.CanPublish {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "create a branch or worktree before publishing from the default branch"})
		return
	}
	message := strings.TrimSpace(request.FormValue("message"))
	ctx, cancel := context.WithTimeout(request.Context(), publishTimeout)
	defer cancel()
	if state.Changes != "" {
		if message == "" {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "commit message is required"})
			return
		}
		if _, err = gitOutput(ctx, state.RepoRoot, "add", "-A"); err == nil {
			_, err = gitOutput(ctx, state.RepoRoot, "commit", "-m", message)
		}
		if err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	if _, err = gitOutput(ctx, state.RepoRoot, "push", "-u", "origin", state.Branch); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	pullRequest := state.PullRequest
	if pullRequest == "" {
		if _, lookErr := exec.LookPath("gh"); lookErr == nil {
			pullRequest, err = commandOutput(ctx, state.RepoRoot, "gh", "pr", "create", "--fill")
			if err != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
		} else {
			base := state.Default
			if base == "" {
				base = "main"
			}
			pullRequest = fmt.Sprintf("%s/compare/%s...%s?expand=1", githubRepositoryURL(state.RemoteURL), url.PathEscape(base), url.PathEscape(state.Branch))
		}
	}
	message = "Pushed " + state.Branch + ". Opening GitHub for review."
	writeJSON(response, http.StatusOK, map[string]string{"message": message, "url": pullRequest})
}

func markdownDraft(markdown, fallbackTitle string) (title, body string) {
	title = strings.TrimSuffix(filepath.Base(fallbackTitle), filepath.Ext(fallbackTitle))
	lines := strings.Split(markdown, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "# ") && strings.TrimSpace(strings.TrimPrefix(line, "# ")) != "" {
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			lines = append(lines[:index], lines[index+1:]...)
			break
		}
	}
	return title, strings.TrimSpace(strings.Join(lines, "\n"))
}

func (a *app) publishSubstack(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	file := request.FormValue("file")
	if !strings.EqualFold(filepath.Ext(file), ".md") {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "select a Markdown file before publishing to Substack"})
		return
	}
	title, body := markdownDraft(request.FormValue("content"), file)
	source := []byte(body)
	document := a.markdown.Parser().Parse(gmtext.NewReader(source))
	var rendered bytes.Buffer
	if err := a.markdown.Renderer().Render(&rendered, source, document); err != nil {
		writeJSON(response, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"title": title, "text": body, "html": rendered.String()})
}
