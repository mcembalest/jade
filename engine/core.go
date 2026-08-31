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

var ignoredDirectories = map[string]bool{".pixi": true, "node_modules": true}

type Workspace struct {
	Title    string
	Path     string
	Markdown string
	Files    []string
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

func FindJadeRoot(start string) (string, error) {
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
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	for {
		if info, statErr := os.Stat(filepath.Join(absolute, markerName)); statErr == nil && info.Mode().IsRegular() {
			return absolute, nil
		}
		parent := filepath.Dir(absolute)
		if parent == absolute {
			return "", fmt.Errorf("no %s found at or above %s", markerName, start)
		}
		absolute = parent
	}
}

func jadeDirectory(runtimeRoot, jadePath string) (string, error) {
	root, err := filepath.EvalSymlinks(runtimeRoot)
	if err != nil {
		return "", err
	}
	if jadePath == "." {
		jadePath = ""
	}
	var candidate string
	if jadePath == "" {
		candidate = root
	} else if candidate, err = safeJoin(root, jadePath); err != nil {
		return "", err
	}
	actual, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	if !within(root, actual) {
		return "", errors.New("JaDE leaves the launched root")
	}
	info, err := os.Stat(filepath.Join(actual, markerName))
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a JaDE", jadePath)
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
	currentRoot, err := jadeDirectory(root, jadePath)
	if err != nil {
		return Workspace{}, err
	}
	marker, err := os.ReadFile(filepath.Join(currentRoot, markerName))
	if err != nil {
		return Workspace{}, err
	}
	title := jadeTitle(string(marker))
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
		Title:    title,
		Path:     relativeSlash(root, currentRoot),
		Markdown: string(marker),
		Files:    files,
	}, nil
}

func existingFile(runtimeRoot, jadePath, filePath string) (string, error) {
	currentRoot, err := jadeDirectory(runtimeRoot, jadePath)
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
	currentRoot, err := jadeDirectory(runtimeRoot, jadePath)
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
