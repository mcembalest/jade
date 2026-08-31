package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

type terminalMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func (a *app) terminal(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cwd, err := jadeDirectory(a.root, queryPath(request, "jade", "."))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	connection, err := websocket.Accept(response, request, nil)
	if err != nil {
		return
	}
	defer connection.CloseNow()
	connection.SetReadLimit(64 * 1024)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	command := exec.Command(shell, "-l")
	command.Dir = cwd
	command.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Cols: 100, Rows: 28})
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, err.Error())
		return
	}
	defer func() {
		_ = terminal.Close()
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	}()

	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	go func() {
		buffer := make([]byte, 32*1024)
		for {
			length, readErr := terminal.Read(buffer)
			if length > 0 {
				if writeErr := connection.Write(ctx, websocket.MessageBinary, buffer[:length]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					_ = connection.Close(websocket.StatusNormalClosure, "shell exited")
				}
				return
			}
		}
	}()

	for {
		kind, payload, readErr := connection.Read(ctx)
		if readErr != nil {
			return
		}
		switch kind {
		case websocket.MessageBinary:
			if _, err = terminal.Write(payload); err != nil {
				return
			}
		case websocket.MessageText:
			var message terminalMessage
			if json.Unmarshal(payload, &message) == nil && message.Type == "resize" && message.Cols >= 2 && message.Rows >= 2 && message.Cols <= 1000 && message.Rows <= 1000 {
				_ = pty.Setsize(terminal, &pty.Winsize{Cols: message.Cols, Rows: message.Rows})
			}
		}
	}
}
