package engine

// The desktop adapter speaks the same small HTTP protocol as the iPhone. The
// editor still reads ordinary files; sync metadata never becomes their format.
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gofrs/flock"
)

type syncConfig struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
}
type remoteFile struct {
	Path     string            `json:"path"`
	Content  string            `json:"content"`
	Revision string            `json:"revision"`
	Acks     map[string]string `json:"acks"`
}
type syncMutation struct {
	Path         string `json:"path"`
	Content      string `json:"content"`
	BaseRevision string `json:"baseRevision"`
	MutationID   string `json:"mutationId"`
	DeviceID     string `json:"deviceId"`
}
type syncRecord struct {
	BaseContent  string            `json:"baseContent"`
	BaseRevision string            `json:"baseRevision"`
	Pending      *syncMutation     `json:"pending,omitempty"`
	Conflict     *remoteFile       `json:"conflict,omitempty"`
	Acks         map[string]string `json:"acks,omitempty"`
}
type syncView struct {
	Enabled  bool              `json:"enabled"`
	Message  string            `json:"message"`
	Files    map[string]string `json:"files"`
	LastSync string            `json:"lastSync"`
}
type workspaceSync struct {
	mu      sync.Mutex
	root    string
	config  syncConfig
	records map[string]*syncRecord
	status  syncView
	client  *http.Client
	lock    *flock.Flock
	wake    chan struct{}
}

var syncSegment = regexp.MustCompile(`^[a-zA-Z0-9 _().-]+$`)

func validSyncPath(p string) bool {
	if len(p) > 240 || p == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	if ext != ".md" && ext != ".txt" {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if !syncSegment.MatchString(part) || strings.HasPrefix(part, ".") || strings.TrimSpace(part) != part {
			return false
		}
	}
	return true
}
func mutationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func openWorkspaceSync(root string) (*workspaceSync, error) {
	raw, err := os.ReadFile(filepath.Join(root, ".jade-sync", "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := &workspaceSync{root: root, records: map[string]*syncRecord{}, client: &http.Client{Timeout: 20 * time.Second}, wake: make(chan struct{}, 1)}
	if err = json.Unmarshal(raw, &s.config); err != nil {
		return nil, err
	}
	u, err := url.Parse(s.config.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(s.config.Token) < 32 {
		return nil, errors.New("invalid sync endpoint or pairing key")
	}
	s.config.Endpoint = strings.TrimRight(s.config.Endpoint, "/")
	s.lock = flock.New(filepath.Join(root, ".jade-sync", "process.lock"))
	ok, err := s.lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("this workspace already has a sync process; use its existing JaDE window")
	}
	raw, err = os.ReadFile(filepath.Join(root, ".jade-sync", "state.json"))
	if err == nil {
		err = json.Unmarshal(raw, &s.records)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.lock.Close()
		return nil, fmt.Errorf("sync state cannot be read; refusing to replace it: %w", err)
	}
	if s.records == nil {
		s.lock.Close()
		return nil, errors.New("invalid empty sync state")
	}
	s.status = syncView{Enabled: true, Message: "Saved on Mac · waiting to sync", Files: map[string]string{}}
	return s, nil
}
func (s *workspaceSync) persist() error {
	raw, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return err
	}
	return replaceFile(filepath.Join(s.root, ".jade-sync", "state.json"), string(raw), 0600, nil)
}
func (s *workspaceSync) request(ctx context.Context, method, path string, body any, result any) (int, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, s.config.Endpoint+path, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+s.config.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return 0, errors.New("server unreachable; local changes are pending")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32*1024*1024))
	if err != nil {
		return res.StatusCode, err
	}
	if res.StatusCode == 409 {
		if result != nil {
			_ = json.Unmarshal(raw, result)
		}
		return 409, nil
	}
	if res.StatusCode != 200 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return res.StatusCode, fmt.Errorf("sync failed (%d): %s", res.StatusCode, e.Error)
	}
	if result != nil {
		err = json.Unmarshal(raw, result)
	}
	return res.StatusCode, err
}
func (s *workspaceSync) local(path string) (string, error) {
	if !validSyncPath(path) {
		return "", errors.New("unsupported sync path")
	}
	// Refuse every symlink component, including directories.
	p := s.root
	for _, part := range strings.Split(path, "/") {
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlinks are not synced")
		}
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 512*1024 {
		return "", errors.New("sync supports text files up to 512 KB")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", errors.New("file is not UTF-8")
	}
	return string(raw), nil
}
func (s *workspaceSync) writeLocal(path, content string, expected *string) error {
	fileWriteMu.Lock()
	defer fileWriteMu.Unlock()
	if !validSyncPath(path) {
		return errors.New("unsafe sync path")
	}
	// Create each directory independently and never follow a symlink.
	p := s.root
	parts := strings.Split(path, "/")
	for _, part := range parts[:len(parts)-1] {
		p = filepath.Join(p, part)
		if err := os.Mkdir(p, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe sync directory")
		}
	}
	check := func() error {
		current, err := s.local(path)
		if expected == nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return errors.New("file appeared during sync; retry")
		}
		if err != nil {
			return err
		}
		if current != *expected {
			return errFileChanged
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	return replaceFile(filepath.Join(s.root, filepath.FromSlash(path)), content, 0600, check)
}
func (s *workspaceSync) run(ctx context.Context) {
	defer s.lock.Close()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		s.cycle(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		}
	}
}
func (s *workspaceSync) cycle(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Message = "Syncing…"
	if err := s.exchange(ctx); err != nil {
		s.status.Message = err.Error()
		return
	}
	s.status.LastSync = time.Now().UTC().Format(time.RFC3339)
	s.status.Message = "Connected · local files saved"
}
func (s *workspaceSync) exchange(ctx context.Context) error {
	paths := map[string]bool{}
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == s.root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, _ := filepath.Rel(s.root, p)
		rel = filepath.ToSlash(rel)
		if !d.IsDir() && validSyncPath(rel) {
			paths[rel] = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	var snapshot struct {
		Files []remoteFile `json:"files"`
	}
	if _, err = s.request(ctx, "GET", "/v1/files", nil, &snapshot); err != nil {
		return err
	}
	remote := map[string]remoteFile{}
	for _, f := range snapshot.Files {
		if !validSyncPath(f.Path) {
			return errors.New("server returned unsupported path")
		}
		remote[f.Path] = f
		paths[f.Path] = true
	}
	s.status.Files = map[string]string{}
	for path := range paths {
		content, readErr := s.local(path)
		missing := errors.Is(readErr, os.ErrNotExist)
		if readErr != nil && !missing {
			s.status.Files[path] = readErr.Error()
			continue
		}
		r := s.records[path]
		if r == nil {
			r = &syncRecord{}
			s.records[path] = r
		}
		f, exists := remote[path]
		if missing && r.BaseRevision != "" {
			s.status.Files[path] = "Deleted on Mac · deletion sync not enabled (remote copy retained)"
			continue
		}
		// A previously persisted upload is retried before fetching a new base. This
		// recovers a lost response even if another device has edited since acceptance.
		if r.Pending != nil {
			var result struct {
				AcceptedRevision string      `json:"acceptedRevision"`
				File             *remoteFile `json:"file"`
			}
			code, e := s.request(ctx, "POST", "/v1/files", r.Pending, &result)
			if e != nil {
				return e
			}
			if code == 409 {
				r.Pending = nil
				if result.File != nil {
					r.Conflict = result.File
					f = *result.File
					exists = true
				}
			} else {
				if result.AcceptedRevision != r.Pending.MutationID {
					return errors.New("server did not acknowledge this edit; pending upload retained")
				}
				r.BaseContent = r.Pending.Content
				r.BaseRevision = result.AcceptedRevision
				r.Pending = nil
				if result.File != nil {
					f = *result.File
					exists = true
				}
			}
			if e = s.persist(); e != nil {
				return e
			}
		}
		if exists {
			r.Acks = f.Acks
			if missing {
				if err = s.writeLocal(path, f.Content, nil); err != nil {
					return err
				}
				content = f.Content
				missing = false
				r.BaseContent = f.Content
				r.BaseRevision = f.Revision
			}
			if content == f.Content {
				r.BaseContent = f.Content
				r.BaseRevision = f.Revision
				r.Conflict = nil
			} else if f.Revision != r.BaseRevision {
				if content == r.BaseContent && r.BaseRevision != "" {
					if err = s.writeLocal(path, f.Content, &content); err != nil {
						return err
					}
					content = f.Content
					r.BaseContent = f.Content
					r.BaseRevision = f.Revision
					r.Conflict = nil
				} else {
					r.Conflict = &f
				}
			}
		}
		if r.Conflict != nil {
			s.status.Files[path] = "Conflict · both versions kept"
			if err = s.persist(); err != nil {
				return err
			}
			continue
		}
		if content != r.BaseContent || r.BaseRevision == "" {
			r.Pending = &syncMutation{Path: path, Content: content, BaseRevision: r.BaseRevision, MutationID: mutationID(), DeviceID: "mac"}
			if err = s.persist(); err != nil {
				return err
			}
			var result struct {
				AcceptedRevision string      `json:"acceptedRevision"`
				File             *remoteFile `json:"file"`
			}
			code, e := s.request(ctx, "POST", "/v1/files", r.Pending, &result)
			if e != nil {
				return e
			}
			if code == 409 {
				r.Pending = nil
				r.Conflict = result.File
				s.status.Files[path] = "Conflict · both versions kept"
				if err = s.persist(); err != nil {
					return err
				}
				continue
			}
			if result.AcceptedRevision != r.Pending.MutationID {
				return errors.New("server did not acknowledge this edit; pending upload retained")
			}
			r.BaseContent = content
			r.BaseRevision = result.AcceptedRevision
			r.Pending = nil
			if result.File != nil {
				r.Acks = result.File.Acks
			}
		}
		if err = s.persist(); err != nil {
			return err
		}
		// Only acknowledge a revision after both the actual file and sync state exist.
		var receipt struct {
			File *remoteFile `json:"file"`
		}
		if r.Acks["mac"] != r.BaseRevision {
			if _, err = s.request(ctx, "POST", "/v1/ack", map[string]string{"path": path, "deviceId": "mac", "revision": r.BaseRevision}, &receipt); err != nil {
				return err
			}
			if receipt.File != nil {
				r.Acks = receipt.File.Acks
			}
		}
		s.status.Files[path] = "Uploaded · iPhone pending"
		if r.Acks["iphone"] == r.BaseRevision {
			s.status.Files[path] = "Synced with iPhone"
		}
	}
	return s.persist()
}
func (s *workspaceSync) view() syncView {
	if !s.mu.TryLock() {
		return syncView{Enabled: true, Message: "Syncing · local saves remain on this Mac", Files: map[string]string{}}
	}
	defer s.mu.Unlock()
	v := s.status
	v.Files = map[string]string{}
	for p, status := range s.status.Files {
		v.Files[p] = status
	}
	for p, r := range s.records {
		content, err := s.local(p)
		if err == nil && (content != r.BaseContent || r.Pending != nil) {
			v.Files[p] = "Saved on Mac · pending sync"
		}
		if r.Conflict != nil {
			v.Files[p] = "Conflict · both versions kept"
		}
	}
	return v
}
func (s *workspaceSync) keepBoth(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.records[path]
	if r == nil || r.Conflict == nil {
		return errors.New("no conflict for this file")
	}
	content, err := s.local(path)
	if err != nil {
		return err
	}
	ext := filepath.Ext(path)
	copyPath := strings.TrimSuffix(path, ext) + " (Mac conflict " + mutationID()[:8] + ")" + ext
	if err = s.writeLocal(copyPath, content, nil); err != nil {
		return err
	}
	f := r.Conflict
	if err = s.writeLocal(path, f.Content, &content); err != nil {
		return err
	}
	r.BaseContent = f.Content
	r.BaseRevision = f.Revision
	r.Conflict = nil
	r.Pending = nil
	return s.persist()
}
func (a *app) syncHTTP(w http.ResponseWriter, r *http.Request) {
	if a.syncer == nil {
		writeJSON(w, 200, syncView{})
		return
	}
	if r.Method == "GET" {
		writeJSON(w, 200, a.syncer.view())
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !parseForm(w, r) {
		return
	}
	if path := r.FormValue("keepBoth"); path != "" {
		if err := a.syncer.keepBoth(path); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
	}
	select {
	case a.syncer.wake <- struct{}{}:
	default:
	}
	writeJSON(w, 200, map[string]bool{"queued": true})
}
