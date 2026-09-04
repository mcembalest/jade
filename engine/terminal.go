package engine

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"runtime"
)

const ghosttyScript = `on run argv
set targetDirectory to item 1 of argv
tell application "Ghostty"
	set cfg to new surface configuration
	set initial working directory of cfg to targetDirectory
	activate
	new window with configuration cfg
end tell
end run`

var openGhostty = func(ctx context.Context, directory string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("the native Ghostty terminal requires macOS")
	}
	if _, err := exec.LookPath("/usr/bin/osascript"); err != nil {
		return errors.New("AppleScript is unavailable")
	}
	command := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", ghosttyScript, directory)
	if output, err := command.CombinedOutput(); err != nil {
		message := string(output)
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}
	return nil
}

func (a *app) terminal(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	cwd, err := workspaceDirectory(a.root, request.FormValue("jade"))
	if err == nil {
		err = openGhostty(request.Context(), cwd)
	}
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"message": "Opened a native Ghostty terminal."})
}
