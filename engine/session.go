package engine

// Editor sessions contain navigation preferences only. Document contents and
// recovery drafts continue to use their existing storage and save lifecycle.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type editorPosition struct {
	Head   int     `json:"head"`
	Scroll float64 `json:"scroll"`
}
type editorSession struct {
	File        string                    `json:"file"`
	Positions   map[string]editorPosition `json:"positions"`
	FilesOpen   bool                      `json:"filesOpen"`
	FilesPinned bool                      `json:"filesPinned"`
	Folders     []string                  `json:"folders"`
}

var editorSessionDirectory = func() (string, error) {
	directory, err := os.UserConfigDir()
	return filepath.Join(directory, "JaDE", "editor-sessions"), err
}

func (a *app) sessionPath(jade string) (string, error) {
	workspace, err := workspaceDirectory(a.root, jade)
	if err != nil {
		return "", err
	}
	directory, err := editorSessionDirectory()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(a.root + "\x00" + workspace))
	return filepath.Join(directory, hex.EncodeToString(digest[:])+".json"), nil
}

func (a *app) readSession(jade string) editorSession {
	var session editorSession
	path, err := a.sessionPath(jade)
	if err != nil {
		return session
	}
	file, err := os.Open(path)
	if err != nil {
		return session
	}
	defer file.Close()
	if json.NewDecoder(io.LimitReader(file, 65537)).Decode(&session) != nil || !validEditorSession(session) {
		return editorSession{}
	}
	return session
}

func validEditorSession(session editorSession) bool {
	if len(session.File) > 4096 || len(session.Positions) > 100 || len(session.Folders) > 500 {
		return false
	}
	for file, position := range session.Positions {
		if len(file) > 4096 || position.Head < 0 || position.Head > maximumTextBytes || position.Scroll < 0 || position.Scroll > 1e9 {
			return false
		}
	}
	for _, folder := range session.Folders {
		if len(folder) > 4096 {
			return false
		}
	}
	return true
}

func (a *app) sessionHTTP(response http.ResponseWriter, request *http.Request) {
	jade := queryPath(request, "jade", ".")
	if request.Method == http.MethodGet {
		writeJSON(response, http.StatusOK, a.readSession(jade))
		return
	}
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var session editorSession
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 65536))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&session); err != nil || !validEditorSession(session) {
		http.Error(response, "invalid editor session", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		http.Error(response, "invalid editor session", http.StatusBadRequest)
		return
	}
	path, err := a.sessionPath(jade)
	if err != nil {
		http.Error(response, "workspace unavailable", http.StatusBadRequest)
		return
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err == nil {
		var file *os.File
		file, err = os.CreateTemp(filepath.Dir(path), ".session-*")
		if err == nil {
			defer os.Remove(file.Name())
			err = json.NewEncoder(file).Encode(session)
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
		http.Error(response, "could not remember editor session", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (a *app) restoredPageData(jade, selected, view string, explicitFile bool) (pageData, error) {
	session := a.readSession(jade)
	restoring := !explicitFile && selected == "" && session.File != ""
	if restoring {
		selected = session.File
	}
	data, err := a.pageData(jade, selected, view, true)
	if err != nil && restoring {
		// A missing remembered file is optional; explicit URLs retain normal errors
		// and deleted files with drafts retain the existing recovery interface.
		data, err = a.pageData(jade, "", view, true)
		if err == nil {
			data.SessionNotice = "The previous file is unavailable. Opened this project’s default file."
		}
	}
	if err == nil {
		encoded, _ := json.Marshal(session)
		data.Session = template.JS(encoded)
	}
	return data, err
}
