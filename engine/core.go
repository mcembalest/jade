package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	markerName       = "jade.md"
	maximumTextBytes = 5_000_000
)

var ignoredDirectories = map[string]bool{
	".build":       true,
	".deps":        true,
	".pixi":        true,
	".tmp":         true,
	"node_modules": true,
}

type Workspace struct {
	Title     string
	Path      string
	Markdown  string
	Files     []string
	HasMarker bool
}

func jadeTitle(markdown string) string {
	inFence := false
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimLeft(strings.TrimSuffix(line, "\r"), " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if !inFence && strings.HasPrefix(line, "# ") {
			if title := strings.TrimSpace(strings.TrimPrefix(line, "# ")); title != "" {
				return title
			}
		}
	}
	return "Untitled JaDE"
}

func validateRelativePath(path string) error {
	if path == "" || strings.ContainsRune(path, 0) || filepath.IsAbs(filepath.FromSlash(path)) {
		return errors.New("path must be relative to its JaDE")
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("path cannot leave its JaDE")
	}
	return nil
}

func within(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func safeJoin(root, path string) (string, error) {
	if err := validateRelativePath(path); err != nil {
		return "", err
	}
	candidate := filepath.Join(root, filepath.FromSlash(path))
	if !within(root, candidate) {
		return "", errors.New("path leaves its JaDE")
	}
	return candidate, nil
}

// ResolveWorkspaceRoot resolves a folder or a file's containing folder.
// A workspace is any directory; jade.md enriches one but never gates opening it.
func ResolveWorkspaceRoot(start string) (string, error) {
	absolute, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	return filepath.EvalSymlinks(absolute)
}

func workspaceDirectory(runtimeRoot, workspacePath string) (string, error) {
	root, err := filepath.EvalSymlinks(runtimeRoot)
	if err != nil {
		return "", err
	}
	if workspacePath == "." {
		workspacePath = ""
	}
	var candidate string
	if workspacePath == "" {
		candidate = root
	} else if candidate, err = safeJoin(root, workspacePath); err != nil {
		return "", err
	}
	actual, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	if !within(root, actual) {
		return "", errors.New("workspace leaves the launched root")
	}
	info, err := os.Stat(actual)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is not a workspace directory", workspacePath)
	}
	if actual != root {
		marker, markerErr := os.Stat(filepath.Join(actual, markerName))
		if markerErr != nil || !marker.Mode().IsRegular() {
			return "", fmt.Errorf("%s is not an inner JaDE", workspacePath)
		}
	}
	return actual, nil
}

func relativeSlash(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	if rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

func editableText(path string, info fs.FileInfo) bool {
	if !info.Mode().IsRegular() || info.Size() > maximumTextBytes {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	prefix := make([]byte, 8192)
	length, err := io.ReadFull(file, prefix)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false
	}
	prefix = prefix[:length]
	if bytes.IndexByte(prefix, 0) >= 0 {
		return false
	}
	if length == cap(prefix) {
		for i := 0; i < 3 && len(prefix) > 0 && !utf8.Valid(prefix); i++ {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return utf8.Valid(prefix)
}

func scanWorkspace(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root && entry.Name() == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path != root && entry.IsDir() && ignoredDirectories[entry.Name()] {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel := relativeSlash(root, path)
		if editableText(path, info) {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

func LoadWorkspace(runtimeRoot, jadePath string) (Workspace, error) {
	root, err := filepath.EvalSymlinks(runtimeRoot)
	if err != nil {
		return Workspace{}, err
	}
	currentRoot, err := workspaceDirectory(root, jadePath)
	if err != nil {
		return Workspace{}, err
	}
	marker, markerErr := os.ReadFile(filepath.Join(currentRoot, markerName))
	hasMarker := markerErr == nil
	if markerErr != nil && !errors.Is(markerErr, os.ErrNotExist) {
		return Workspace{}, markerErr
	}
	title := filepath.Base(currentRoot)
	if hasMarker {
		title = jadeTitle(string(marker))
	}
	files, err := scanWorkspace(currentRoot)
	if err != nil {
		return Workspace{}, err
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i] == markerName {
			return true
		}
		if files[j] == markerName {
			return false
		}
		return files[i] < files[j]
	})
	return Workspace{
		Title:     title,
		Path:      relativeSlash(root, currentRoot),
		Markdown:  string(marker),
		Files:     files,
		HasMarker: hasMarker,
	}, nil
}

func existingFile(runtimeRoot, jadePath, filePath string) (string, error) {
	currentRoot, err := workspaceDirectory(runtimeRoot, jadePath)
	if err != nil {
		return "", err
	}
	candidate, err := safeJoin(currentRoot, filePath)
	if err != nil {
		return "", err
	}
	actual, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	if !within(currentRoot, actual) {
		return "", errors.New("file leaves its JaDE")
	}
	return actual, nil
}

func ReadWorkspaceFile(runtimeRoot, jadePath, filePath string) (string, error) {
	path, err := existingFile(runtimeRoot, jadePath, filePath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !editableText(path, info) {
		return "", errors.New("file is not editable text")
	}
	contents, err := os.ReadFile(path)
	return string(contents), err
}

func WriteWorkspaceFile(runtimeRoot, jadePath, filePath, contents string) error {
	if len([]byte(contents)) > maximumTextBytes {
		return errors.New("file is too large")
	}
	currentRoot, err := workspaceDirectory(runtimeRoot, jadePath)
	if err != nil {
		return err
	}
	candidate, err := safeJoin(currentRoot, filePath)
	if err != nil {
		return err
	}
	ancestor := filepath.Dir(candidate)
	for {
		if _, statErr := os.Stat(ancestor); statErr == nil {
			break
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return errors.New("cannot find a safe parent directory")
		}
		ancestor = parent
	}
	actualAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil || !within(currentRoot, actualAncestor) {
		return errors.New("file leaves its JaDE")
	}
	if info, lstatErr := os.Lstat(candidate); lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("refusing to write through a symlink")
	}
	if err := os.MkdirAll(filepath.Dir(candidate), 0o755); err != nil {
		return err
	}
	return os.WriteFile(candidate, []byte(contents), 0o644)
}
