package engine

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type branchState struct {
	Current  string   `json:"current"`
	Branches []string `json:"branches"`
}

func (a *app) repositoryBranches(ctx context.Context, jadePath string) (branchState, string, error) {
	workspace, err := workspaceDirectory(a.root, jadePath)
	if err != nil {
		return branchState{}, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	repository, err := gitOutput(ctx, workspace, "rev-parse", "--show-toplevel")
	if err != nil {
		return branchState{}, "", errors.New("the active workspace is not inside a Git repository")
	}
	current, _ := gitOutput(ctx, repository, "branch", "--show-current")
	output, err := gitOutput(ctx, repository, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return branchState{}, "", err
	}
	branches := []string{}
	for _, branch := range strings.Split(output, "\n") {
		if branch = strings.TrimSpace(branch); branch != "" {
			branches = append(branches, branch)
		}
	}
	return branchState{Current: current, Branches: branches}, repository, nil
}

func (a *app) branches(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	state, _, err := a.repositoryBranches(request.Context(), queryPath(request, "jade", "."))
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(response, http.StatusOK, state)
}

func (a *app) switchBranch(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	state, repository, err := a.repositoryBranches(request.Context(), request.FormValue("jade"))
	branch := request.FormValue("branch")
	if err == nil {
		found := false
		for _, candidate := range state.Branches {
			if candidate == branch {
				found = true
				break
			}
		}
		if !found {
			err = errors.New("select an existing local branch")
		}
	}
	if err == nil && branch != state.Current {
		ctx, cancel := context.WithTimeout(request.Context(), publishTimeout)
		defer cancel()
		_, err = gitOutput(ctx, repository, "switch", branch)
	}
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"branch": branch})
}
