package engine

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gmtext "github.com/yuin/goldmark/text"
)

type fileNode struct {
	Name      string
	Path      string
	URL       string
	JadePath  string
	Directory bool
	Jade      bool
	Children  []*fileNode
}

type pageData struct {
	Workspace Workspace
	Selected  string
	Contents  string
	Revision  string
	CRLF      bool
	Files     []*fileNode
	IsJade    bool
	View      string
	ViewURL   string
}

type app struct {
	root     string
	markdown goldmark.Markdown
	page     *template.Template
	hosts    map[string]bool
}

//go:embed web/dist/editor.bundle.js
var appScript string

//go:embed web/dist/THIRD_PARTY_NOTICES.txt
var thirdPartyNotices string

//go:embed web/page.html
var pageTemplate string

//go:embed web/style.css
var appStyle string

//go:embed web/preview.css
var previewStyle string

func newApp(root string, port int) (*app, error) {
	page, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return nil, err
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return nil, err
	}
	hosts := map[string]bool{}
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		hosts[net.JoinHostPort(host, strconv.Itoa(port))] = true
	}
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Typographer))
	return &app{root: realRoot, markdown: markdown, page: page, hosts: hosts}, nil
}

func (a *app) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if !a.hosts[request.Host] {
			http.Error(response, "unexpected Host header", http.StatusForbidden)
			return
		}
		if origin := request.Header.Get("Origin"); origin != "" && origin != "http://"+request.Host && origin != "https://"+request.Host {
			http.Error(response, "cross-origin request rejected", http.StatusForbidden)
			return
		}
		// A link clicked in an opaque preview navigates the top-level document
		// as cross-site. Only the read-only shell accepts that navigation;
		// API requests and embedded documents still require the same origin.
		shellNavigation := request.Method == http.MethodGet && request.URL.Path == "/" &&
			request.Header.Get("Sec-Fetch-Mode") == "navigate" && request.Header.Get("Sec-Fetch-Dest") == "document"
		if site := request.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" && !shellNavigation {
			http.Error(response, "cross-site request rejected", http.StatusForbidden)
			return
		}
		next(response, request)
	}
}

func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.guard(a.home))
	mux.HandleFunc("/file", a.guard(a.file))
	mux.HandleFunc("/save", a.guard(a.save))
	mux.HandleFunc("/drafts", a.guard(a.drafts))
	mux.HandleFunc("/new", a.guard(a.create))
	mux.HandleFunc("/front", a.guard(a.front))
	mux.HandleFunc("/view", a.guard(a.view))
	mux.HandleFunc("/terminals", a.guard(a.terminals))
	mux.HandleFunc("/terminal/preference", a.guard(a.terminalPreference))
	mux.HandleFunc("/terminal", a.guard(a.terminal))
	mux.HandleFunc("/app.js", a.guard(a.script))
	mux.HandleFunc("/style.css", a.guard(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/css; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(response, appStyle)
	}))
	mux.HandleFunc("/licenses.txt", a.guard(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(response, thirdPartyNotices)
	}))
	return mux
}

func (a *app) script(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(response, appScript)
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func queryPath(request *http.Request, name, fallback string) string {
	if value := request.URL.Query().Get(name); value != "" {
		return value
	}
	return fallback
}

func buildFileTree(workspace Workspace) []*fileNode {
	root := &fileNode{}
	directories := map[string]*fileNode{"": root}
	for _, file := range workspace.Files {
		parts := strings.Split(file, "/")
		parentPath := ""
		for _, part := range parts[:len(parts)-1] {
			path := strings.TrimPrefix(parentPath+"/"+part, "/")
			if directories[path] == nil {
				node := &fileNode{Name: part, Directory: true}
				directories[parentPath].Children = append(directories[parentPath].Children, node)
				directories[path] = node
			}
			parentPath = path
		}
		jadePath, isJade := workspace.Path, parts[len(parts)-1] == markerName
		if isJade && parentPath != "" {
			jadePath = filepath.ToSlash(filepath.Clean(filepath.Join(workspace.Path, filepath.FromSlash(parentPath))))
			if workspace.Path == "." {
				jadePath = parentPath
			}
			directories[parentPath].Jade = true
		}
		url := "/?jade=" + template.URLQueryEscaper(jadePath) + "&file=" + template.URLQueryEscaper(parts[len(parts)-1])
		if !isJade || parentPath == "" {
			url = "/?jade=" + template.URLQueryEscaper(workspace.Path) + "&file=" + template.URLQueryEscaper(file)
		}
		directories[parentPath].Children = append(directories[parentPath].Children, &fileNode{Name: parts[len(parts)-1], Path: file, URL: url, JadePath: jadePath, Jade: isJade})
	}
	var order func([]*fileNode)
	order = func(nodes []*fileNode) {
		sort.SliceStable(nodes, func(i, j int) bool {
			if nodes[i].Directory != nodes[j].Directory {
				return nodes[i].Directory
			}
			return nodes[i].Name < nodes[j].Name
		})
		for _, node := range nodes {
			order(node.Children)
		}
	}
	order(root.Children)
	return root.Children
}

func externalDestination(destination string) bool {
	return destination == "" || strings.Contains(destination, "://") || strings.HasPrefix(destination, "#") || strings.HasPrefix(destination, "/") || strings.HasPrefix(destination, "mailto:") || strings.HasPrefix(destination, "data:")
}

func (a *app) rewriteDestination(jadePath string, destination []byte, isImage bool) []byte {
	dest := string(destination)
	if externalDestination(dest) {
		return destination
	}
	clean := strings.TrimSuffix(dest, "/")
	resolved, err := existingFile(a.root, jadePath, clean)
	if err != nil {
		return destination
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return destination
	}
	query := template.URLQueryEscaper
	if info.IsDir() {
		if marker, markerErr := os.Stat(filepath.Join(resolved, markerName)); markerErr == nil && marker.Mode().IsRegular() {
			return []byte("/?jade=" + query(relativeSlash(a.root, resolved)))
		}
		return destination
	}
	rel := filepath.ToSlash(filepath.Clean(clean))
	if isImage {
		// Sandboxed previews have an opaque origin. Embed bounded local images so
		// browsers can display them without weakening our cross-site request guard.
		kind := mime.TypeByExtension(filepath.Ext(resolved))
		if !strings.HasPrefix(kind, "image/") || !info.Mode().IsRegular() || info.Size() > maximumTextBytes {
			return destination
		}
		file, err := os.Open(resolved)
		if err != nil {
			return destination
		}
		defer file.Close()
		contents, err := io.ReadAll(io.LimitReader(file, maximumTextBytes+1))
		if err != nil || len(contents) > maximumTextBytes {
			return destination
		}
		return []byte("data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(contents))
	}
	return []byte("/?jade=" + query(jadePath) + "&file=" + query(markerName) + "&view=" + query(rel))
}

func (a *app) rewriteDestinations(jadePath string, document ast.Node) {
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Link:
			if !bytes.HasPrefix(typed.Destination, []byte("#")) {
				typed.SetAttributeString("target", []byte("_top"))
			}
			typed.Destination = a.rewriteDestination(jadePath, typed.Destination, false)
		case *ast.AutoLink:
			typed.SetAttributeString("target", []byte("_top"))
		case *ast.Image:
			typed.Destination = a.rewriteDestination(jadePath, typed.Destination, true)
		}
		return ast.WalkContinue, nil
	})
}

func (a *app) pageData(jadePath, selected, view string, includeTree bool) (pageData, error) {
	workspace, err := loadWorkspace(a.root, jadePath, includeTree)
	if err != nil {
		return pageData{}, err
	}
	if selected == "" {
		if workspace.HasMarker {
			selected = markerName
		} else if len(workspace.Files) > 0 {
			selected = workspace.Files[0]
		}
	}
	contents := ""
	if selected != "" {
		contents, err = ReadWorkspaceFile(a.root, workspace.Path, selected)
		if err != nil {
			// A restored browser URL must still offer recovery for a deleted file.
			recovered := false
			if includeTree && errors.Is(err, os.ErrNotExist) {
				if directory, draftErr := a.draftDirectory(workspace.Path, selected); draftErr == nil {
					if entries, draftErr := os.ReadDir(directory); draftErr == nil {
						for _, entry := range entries {
							if filepath.Ext(entry.Name()) == ".json" {
								recovered = true
								break
							}
						}
					}
				}
			}
			if !recovered {
				return pageData{}, err
			}
		}
	}
	data := pageData{Workspace: workspace, Selected: selected, Contents: contents, Revision: fileRevision(contents), CRLF: strings.Contains(contents, "\r\n"), Files: buildFileTree(workspace), IsJade: selected == markerName}
	if !data.IsJade {
		return data, nil
	}
	if view != "" {
		if _, viewErr := existingFile(a.root, workspace.Path, view); viewErr != nil {
			view = ""
		}
	}
	data.View = view
	if view == "" {
		data.ViewURL = "/front?jade=" + template.URLQueryEscaper(workspace.Path)
	} else {
		data.ViewURL = "/view?jade=" + template.URLQueryEscaper(workspace.Path) + "&file=" + template.URLQueryEscaper(view)
	}
	return data, nil
}

func (a *app) home(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}

	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := a.pageData(queryPath(request, "jade", "."), request.URL.Query().Get("file"), request.URL.Query().Get("view"), true)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-src 'self'; img-src 'self' data:; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	if err := a.page.Execute(response, data); err != nil {
		log.Printf("render: %v", err)
	}
}

func (a *app) file(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := a.pageData(queryPath(request, "jade", "."), request.URL.Query().Get("file"), request.URL.Query().Get("view"), false)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"selected": data.Selected, "contents": data.Contents, "revision": data.Revision, "isJade": data.IsJade, "view": data.View, "viewURL": data.ViewURL, "title": data.Workspace.Title})
}

func parseForm(response http.ResponseWriter, request *http.Request) bool {
	request.Body = http.MaxBytesReader(response, request.Body, maximumTextBytes*3+64_000)
	if err := request.ParseMultipartForm(maximumTextBytes + 64_000); err != nil {
		if err = request.ParseForm(); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return false
		}
	}
	return true
}

func (a *app) save(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !parseForm(response, request) {
		return
	}
	revision := request.FormValue("revision")
	if revision == "" {
		http.Error(response, "reload the file before saving", http.StatusPreconditionRequired)
		return
	}
	jadePath, filePath, contents := request.FormValue("jade"), request.FormValue("file"), request.FormValue("content")
	err := updateWorkspaceFile(a.root, jadePath, filePath, contents, revision)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, errFileChanged) || errors.Is(err, os.ErrNotExist) {
			code = http.StatusConflict
		}
		message := err.Error()
		if errors.Is(err, os.ErrPermission) {
			message = "Cannot save here. Check the file and folder permissions, then try saving again."
		}
		http.Error(response, message, code)
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"revision": fileRevision(contents)})
}

func (a *app) create(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	jadePath, filePath := request.FormValue("jade"), strings.TrimSpace(request.FormValue("path"))
	currentRoot, err := workspaceDirectory(a.root, jadePath)
	if err == nil {
		var candidate string
		candidate, err = safeJoin(currentRoot, filePath)
		if err == nil {
			if _, statErr := os.Stat(candidate); statErr == nil {
				err = errors.New("file already exists")
			} else if !errors.Is(statErr, os.ErrNotExist) {
				err = statErr
			}
		}
	}
	if err == nil {
		contents := ""
		if filepath.Base(filePath) == markerName {
			contents = "# Untitled JaDE\n"
		}
		err = CreateWorkspaceFile(a.root, jadePath, filePath, contents)
	}
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	location := "/?jade=" + template.URLQueryEscaper(jadePath) + "&file=" + template.URLQueryEscaper(filePath)
	_, _ = io.WriteString(response, location)
}

func (a *app) renderMarkdown(response http.ResponseWriter, jadePath string, markdown []byte) {
	document := a.markdown.Parser().Parse(gmtext.NewReader(markdown))
	a.rewriteDestinations(jadePath, document)
	var rendered bytes.Buffer
	if err := a.markdown.Renderer().Render(&rendered, markdown, document); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Security-Policy", "sandbox allow-top-navigation-by-user-activation; default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'")
	_, _ = io.WriteString(response, `<!doctype html><meta charset="utf-8"><style>`+previewStyle+`</style>`)
	_, _ = response.Write(rendered.Bytes())
}

func (a *app) front(response http.ResponseWriter, request *http.Request) {
	workspace, err := LoadWorkspace(a.root, queryPath(request, "jade", "."))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	a.renderMarkdown(response, workspace.Path, []byte(workspace.Markdown))
}

func (a *app) view(response http.ResponseWriter, request *http.Request) {
	jadePath, filePath := queryPath(request, "jade", "."), request.URL.Query().Get("file")
	if filePath == "" {
		http.Error(response, "file is required", http.StatusBadRequest)
		return
	}
	path, err := existingFile(a.root, jadePath, filePath)
	if err != nil {
		http.Error(response, err.Error(), http.StatusNotFound)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	if strings.EqualFold(filepath.Ext(path), ".md") {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			http.Error(response, readErr.Error(), http.StatusInternalServerError)
			return
		}
		a.renderMarkdown(response, jadePath, contents)
		return
	}
	response.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'")
	if kind := mime.TypeByExtension(filepath.Ext(path)); kind != "" {
		response.Header().Set("Content-Type", kind)
	}
	http.ServeFile(response, request, path)
}

// NewHandler returns the HTTP interface for the JaDE rooted at root.
func NewHandler(root string, port int) (http.Handler, error) {
	application, err := newApp(root, port)
	if err != nil {
		return nil, err
	}
	return application.handler(), nil
}
