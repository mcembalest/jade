package engine

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"
)

var errFileChanged = errors.New("the file changed on disk; your edits have been kept")
var fileWriteMu sync.Mutex

func fileRevision(contents string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(contents))) }

func updateWorkspaceFile(root, jade, file, contents, revision string) error {
	fileWriteMu.Lock()
	defer fileWriteMu.Unlock()
	if len(contents) > maximumTextBytes || !utf8.ValidString(contents) {
		return errors.New("file must be UTF-8 text within the size limit")
	}
	path, err := existingFile(root, jade, file)
	if err != nil {
		return err
	}
	// Keep the same symlink policy as file creation.
	candidate, err := workspaceFilePath(root, jade, file)
	if err != nil {
		return err
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("refusing to replace a symlink or non-regular file")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(before) == contents {
		return nil
	} // Safe retry after a lost HTTP response.
	if fileRevision(string(before)) != revision {
		return errFileChanged
	}
	return replaceFile(path, contents, info.Mode().Perm(), func() error {
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if fileRevision(string(current)) != revision {
			return errFileChanged
		}
		return nil
	})
}

// Prepare a complete file alongside the original, then replace it atomically.
// Recheck before replacement to detect edits made while the temporary file was written.
func replaceFile(path, contents string, mode os.FileMode, check func() error) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".jade-save-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err = file.Chmod(mode); err != nil {
		return err
	}
	if _, err = file.WriteString(contents); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if check != nil {
		if err = check(); err != nil {
			return err
		}
	}
	return os.Rename(file.Name(), path)
}
