package engine

import (
	"bytes"
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

// Native application commands create a new session in the running app. Paths
// travel as argv data, never as AppleScript source or keystrokes in an old shell.
func terminalScript(app string) string {
	switch strings.ToLower(terminalName(app)) {
	case "ghostty":
		return `on run argv
 tell application id "com.mitchellh.ghostty"
  set cfg to new surface configuration
  set initial working directory of cfg to item 1 of argv
  set win to new window with configuration cfg
  activate window win
  return id of win
 end tell
end run`
	case "terminal":
		return `on run argv
 tell application id "com.apple.Terminal"
  set session to do script ("cd -- " & quoted form of (item 1 of argv))
  activate
  return tty of session
 end tell
end run`
	default:
		return ""
	}
}

func terminalArguments(app, directory string) []string {
	switch strings.ToLower(terminalName(app)) {
	case "alacritty":
		return []string{"-n", "-a", app, "--args", "--working-directory=" + directory}
	case "wezterm":
		return []string{"-a", app, "--args", "start", "--cwd", directory}
	case "kitty":
		return []string{"-a", app, "--args", "--directory", directory}
	default:
		return []string{"-a", app, directory}
	}
}

var launchTerminal = func(ctx context.Context, app, directory string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("opening terminal apps requires macOS")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if script := terminalScript(app); script != "" {
		cmd = exec.CommandContext(ctx, "/usr/bin/osascript", "-", directory)
		cmd.Stdin = strings.NewReader(script)
	} else {
		cmd = exec.CommandContext(ctx, "/usr/bin/open", terminalArguments(app, directory)...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if bytes.Contains(output, []byte("-1743")) || bytes.Contains(output, []byte("not authorized")) {
			return errors.New("macOS denied terminal automation. Allow JaDE’s host app to control " + terminalName(app) + " in System Settings → Privacy & Security → Automation, then retry.")
		}
		if ctx.Err() != nil {
			return errors.New("Terminal launch was not confirmed. Check for a macOS permission prompt before retrying.")
		}
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	if terminalScript(app) != "" && len(bytes.TrimSpace(output)) == 0 {
		return errors.New("The terminal did not confirm a new session")
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
	if err == nil {
		err = launchTerminal(request.Context(), selected, cwd)
	}
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	message := "Opened " + terminalName(selected) + "."
	writeJSON(response, http.StatusOK, map[string]string{"message": message})
}
