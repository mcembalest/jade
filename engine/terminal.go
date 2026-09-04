package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const systemTerminal = "/System/Applications/Utilities/Terminal.app"

type terminalApp struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type terminalState struct {
	Apps       []terminalApp `json:"apps"`
	Selected   string        `json:"selected"`
	Overridden bool          `json:"overridden"`
}

var terminalRoots = func() []string {
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, "Applications"), "/Applications"}
}

func terminalName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".app")
}

func installedTerminals() []terminalApp {
	apps := []terminalApp{{"Terminal", systemTerminal}}
	for _, name := range []string{"Ghostty", "iTerm", "WezTerm", "kitty", "Alacritty"} {
		for _, root := range terminalRoots() {
			path := filepath.Join(root, name+".app")
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				apps = append(apps, terminalApp{name, path})
				break
			}
		}
	}
	return apps
}

var terminalPreferencePath = func() (string, error) {
	directory, err := os.UserConfigDir()
	return filepath.Join(directory, "JaDE", "terminal"), err
}

func availableTerminals() terminalState {
	state := terminalState{Apps: installedTerminals(), Selected: systemTerminal}
	if len(state.Apps) > 1 {
		state.Selected = state.Apps[1].Path
	}
	if path, err := terminalPreferencePath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			for _, app := range state.Apps {
				if app.Path == string(data) {
					state.Selected = app.Path
				}
			}
		}
	}
	if override := strings.TrimSpace(os.Getenv("JADE_TERMINAL")); override != "" {
		state.Selected = override
		state.Overridden = true
		found := false
		for _, app := range state.Apps {
			if override == app.Path || override == app.Name || override == app.Name+".app" {
				state.Selected = app.Path
				found = true
				break
			}
		}
		if !found {
			state.Apps = append(state.Apps, terminalApp{terminalName(override), override})
		}
	}
	return state
}

func (a *app) terminals(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(response, http.StatusOK, availableTerminals())
}

func (a *app) terminalPreference(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !parseForm(response, request) {
		return
	}
	selected := request.FormValue("terminal")
	valid := false
	for _, app := range installedTerminals() {
		if app.Path == selected {
			valid = true
		}
	}
	if !valid {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "Choose an installed terminal app."})
		return
	}
	path, err := terminalPreferencePath()
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0700)
	}
	if err == nil {
		// Atomic replacement also keeps simultaneous engine instances from reading a partial preference.
		var file *os.File
		file, err = os.CreateTemp(filepath.Dir(path), ".terminal-*")
		if err == nil {
			defer os.Remove(file.Name())
			_, err = file.WriteString(selected)
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
			if err == nil {
				err = os.Rename(file.Name(), path)
			}
		}
	}
	if err != nil {
		writeJSON(response, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(response, http.StatusOK, availableTerminals())
}

func terminalArguments(app, directory string) []string {
	switch strings.ToLower(terminalName(app)) {
	case "ghostty", "alacritty":
		return []string{"-n", "-a", app, "--args", "--working-directory=" + directory}
	case "wezterm":
		return []string{"-n", "-a", app, "--args", "start", "--cwd", directory}
	case "kitty":
		return []string{"-n", "-a", app, "--args", "--directory", directory}
	default:
		// Terminal and iTerm accept a directory through Launch Services.
		return []string{"-a", app, directory}
	}
}

var launchTerminal = func(ctx context.Context, app, directory string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("opening terminal apps requires macOS")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/open", terminalArguments(app, directory)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func (a *app) terminal(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !parseForm(response, request) {
		return
	}
	cwd, err := workspaceDirectory(a.root, request.FormValue("jade"))
	selected := availableTerminals().Selected
	fallback := false
	if err == nil {
		err = launchTerminal(request.Context(), selected, cwd)
		if err != nil && selected != systemTerminal && request.Context().Err() == nil {
			selected = systemTerminal
			fallback = true
			err = launchTerminal(request.Context(), selected, cwd)
		}
	}
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	message := "Opened " + terminalName(selected) + "."
	if fallback {
		message = "Opened Terminal because the selected app was unavailable."
	}
	writeJSON(response, http.StatusOK, map[string]string{"message": message})
}
