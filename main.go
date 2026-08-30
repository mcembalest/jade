package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gmtext "github.com/yuin/goldmark/text"
)

const commandTimeout = 10 * time.Minute

type runnable struct {
	Command string
	Label   string
}

type pageData struct {
	Workspace     Workspace
	Selected      string
	Contents      string
	Runnables     []runnable
	View          string
	ViewURL       string
	FrontURL      string
	CommandOutput string
	CommandFailed bool
	Problem       string
}

type app struct {
	root     string
	realRoot string
	markdown goldmark.Markdown
	page     *template.Template
	hosts    map[string]bool
}

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{.Workspace.Title}} · Jade</title>
  <style>
    * { box-sizing: border-box; }
    body { margin: 0; color: #17201b; background: #f7f8f6; font: 15px/1.45 system-ui, sans-serif; }
    header, nav, .controls, .new-file { display: flex; gap: .65rem; align-items: center; flex-wrap: wrap; }
    header { padding: .8rem 1rem; border-bottom: 1px solid #cbd2cd; background: white; }
    header strong { color: #176b43; }
    a, button { color: #145c3b; }
    button, select, input { font: inherit; padding: .4rem .6rem; }
    button { cursor: pointer; }
    nav { margin-left: auto; }
    main { padding: 1rem; }
    .problem, .output { margin: 0 0 1rem; padding: .7rem; white-space: pre-wrap; border: 1px solid #cbd2cd; background: white; }
    .problem { border-color: #ae4d4d; color: #7a2222; }
    .failed { border-color: #ae4d4d; }
    details { margin-bottom: 1rem; border: 1px solid #cbd2cd; background: white; }
    summary { cursor: pointer; padding: .65rem; font-weight: 600; }
    .front { width: 100%; height: 12rem; border: 0; border-top: 1px solid #e0e4e1; }
    .controls { margin-bottom: .75rem; }
    .controls form { display: inline-flex; gap: .4rem; }
    .controls input[name=command] { min-width: 18rem; font-family: ui-monospace, monospace; }
    .new-file { margin-left: auto; }
    .workbench { display: grid; grid-template-columns: minmax(20rem, 1fr) minmax(20rem, 1fr); gap: 1rem; min-height: 62vh; }
    .pane { display: flex; min-width: 0; flex-direction: column; border: 1px solid #cbd2cd; background: white; }
    .pane h2 { margin: 0; padding: .55rem .7rem; border-bottom: 1px solid #e0e4e1; font-size: .9rem; }
    textarea { width: 100%; min-height: 55vh; flex: 1; resize: vertical; border: 0; padding: .8rem; outline: none; tab-size: 2; font: 14px/1.5 ui-monospace, monospace; }
    .save { padding: .6rem; border-top: 1px solid #e0e4e1; }
    .viewer { width: 100%; min-height: 58vh; flex: 1; border: 0; }
    .empty { margin: auto; color: #68716b; }
    @media (max-width: 850px) { .workbench { grid-template-columns: 1fr; } nav { margin-left: 0; width: 100%; } }
  </style>
</head>
<body>
  <header>
    <strong>Jade</strong>
    {{if .Workspace.Parent}}<a href="/?jade={{urlquery .Workspace.Parent}}">↑ parent</a>{{end}}
    <span>{{.Workspace.Path}} / {{.Workspace.Title}}</span>
    <nav aria-label="Nested Jades">
      {{range .Workspace.Children}}<a href="/?jade={{urlquery .Path}}">{{.Title}}</a>{{end}}
    </nav>
  </header>
  <main>
    {{if .Problem}}<pre class="problem">{{.Problem}}</pre>{{end}}
    <details open>
      <summary>{{.Workspace.Title}}</summary>
      <iframe class="front" sandbox="allow-top-navigation-by-user-activation" src="{{.FrontURL}}" title="Jade front page"></iframe>
    </details>

    <div class="controls">
      <form method="get" action="/">
        <input type="hidden" name="jade" value="{{.Workspace.Path}}">
        <input type="hidden" name="view" value="{{.View}}">
        <label>File
          <select name="file">
            {{range .Workspace.Files}}<option value="{{.}}" {{if eq . $.Selected}}selected{{end}}>{{.}}</option>{{end}}
          </select>
        </label>
        <button type="submit">Open</button>
      </form>
      {{range .Runnables}}
      <form method="post" action="/run">
        <input type="hidden" name="jade" value="{{$.Workspace.Path}}">
        <input type="hidden" name="file" value="{{$.Selected}}">
        <input type="hidden" name="view" value="{{$.View}}">
        <input type="hidden" name="command" value="{{.Command}}">
        <button type="submit" title="{{.Command}}">Run: {{.Label}}</button>
      </form>
      {{end}}
      <form method="post" action="/run">
        <input type="hidden" name="jade" value="{{.Workspace.Path}}">
        <input type="hidden" name="file" value="{{.Selected}}">
        <input type="hidden" name="view" value="{{.View}}">
        <input name="command" placeholder="sh command in this Jade" aria-label="Command">
        <button type="submit">Run</button>
      </form>
      <form class="new-file" method="post" action="/new">
        <input type="hidden" name="jade" value="{{.Workspace.Path}}">
        <input name="path" required placeholder="new/file.md" aria-label="New file path">
        <button type="submit">Create</button>
      </form>
    </div>

    {{if .CommandOutput}}<pre class="output {{if .CommandFailed}}failed{{end}}">{{.CommandOutput}}</pre>{{end}}

    <div class="workbench">
      <form class="pane" method="post" action="/save">
        <h2>{{.Selected}}</h2>
        <input type="hidden" name="jade" value="{{.Workspace.Path}}">
        <input type="hidden" name="file" value="{{.Selected}}">
        <input type="hidden" name="view" value="{{.View}}">
        <textarea name="content" spellcheck="false">{{.Contents}}</textarea>
        <div class="save"><button type="submit">Save</button></div>
      </form>
      <section class="pane">
        <h2>{{if .View}}{{.View}}{{else}}View{{end}}</h2>
        {{if .ViewURL}}<iframe class="viewer" sandbox="allow-top-navigation-by-user-activation" src="{{.ViewURL}}" title="Viewed file"></iframe>{{else}}<p class="empty">Link a file in jade.md to open it here.</p>{{end}}
      </section>
    </div>
  </main>
  <script src="/app.js" defer></script>
</body>
</html>`

const appScript = `document.addEventListener("keydown", (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
    const form = document.querySelector("form.pane");
    if (form) {
      event.preventDefault();
      form.requestSubmit();
    }
  }
});
const editor = document.querySelector("textarea[name=content]");
if (editor) {
  editor.addEventListener("keydown", (event) => {
    if (event.key === "Tab" && !event.shiftKey) {
      event.preventDefault();
      editor.setRangeText("\t", editor.selectionStart, editor.selectionEnd, "end");
    }
  });
  const key = "jade-cursor:" + location.search;
  const saved = sessionStorage.getItem(key);
  if (saved !== null) {
    const position = Math.min(Number(saved), editor.value.length);
    editor.setSelectionRange(position, position);
  }
  editor.form.addEventListener("submit", () => {
    sessionStorage.setItem(key, String(editor.selectionStart));
  });
}
`

func newApp(root string, port int) (*app, error) {
	page, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	hosts := map[string]bool{}
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		hosts[net.JoinHostPort(host, strconv.Itoa(port))] = true
	}
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Typographer))
	return &app{root: root, realRoot: realRoot, markdown: markdown, page: page, hosts: hosts}, nil
}

func (a *app) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if !a.hosts[request.Host] {
			http.Error(response, "unexpected Host header", http.StatusForbidden)
			return
		}
		if request.Method == http.MethodPost {
			if origin := request.Header.Get("Origin"); origin != "" && origin != "http://"+request.Host {
				http.Error(response, "cross-origin request rejected", http.StatusForbidden)
				return
			}
			if site := request.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
				http.Error(response, "cross-site request rejected", http.StatusForbidden)
				return
			}
		}
		next(response, request)
	}
}

func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.guard(a.home))
	mux.HandleFunc("/save", a.guard(a.save))
	mux.HandleFunc("/new", a.guard(a.create))
	mux.HandleFunc("/run", a.guard(a.run))
	mux.HandleFunc("/front", a.guard(a.front))
	mux.HandleFunc("/view", a.guard(a.view))
	mux.HandleFunc("/app.js", a.guard(a.script))
	return mux
}

func (a *app) script(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = io.WriteString(response, appScript)
}

func queryPath(request *http.Request, name, fallback string) string {
	if value := request.URL.Query().Get(name); value != "" {
		return value
	}
	return fallback
}

func externalDestination(destination string) bool {
	return destination == "" ||
		strings.Contains(destination, "://") ||
		strings.HasPrefix(destination, "#") ||
		strings.HasPrefix(destination, "/") ||
		strings.HasPrefix(destination, "mailto:") ||
		strings.HasPrefix(destination, "data:")
}

// rewriteDestination maps a relative Markdown destination onto the app:
// a nested Jade directory becomes a Jade link, an existing file opens in the
// viewer pane, and images load through /view. Everything else is untouched.
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
			return []byte("/?jade=" + query(relativeSlash(a.realRoot, resolved)))
		}
		return destination
	}
	rel := filepath.ToSlash(filepath.Clean(clean))
	if isImage {
		return []byte("/view?jade=" + query(jadePath) + "&file=" + query(rel))
	}
	return []byte("/?jade=" + query(jadePath) + "&view=" + query(rel))
}

func (a *app) rewriteDestinations(jadePath string, document ast.Node) {
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Link:
			typed.Destination = a.rewriteDestination(jadePath, typed.Destination, false)
		case *ast.Image:
			typed.Destination = a.rewriteDestination(jadePath, typed.Destination, true)
		}
		return ast.WalkContinue, nil
	})
}

// defaultView returns the first link in jade.md that resolves to a regular
// file inside the Jade — a convention, not a contract.
func (a *app) defaultView(workspace Workspace) string {
	source := []byte(workspace.Markdown)
	document := a.markdown.Parser().Parse(gmtext.NewReader(source))
	view := ""
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := node.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		dest := string(link.Destination)
		if externalDestination(dest) {
			return ast.WalkContinue, nil
		}
		resolved, err := existingFile(a.root, workspace.Path, strings.TrimSuffix(dest, "/"))
		if err != nil {
			return ast.WalkContinue, nil
		}
		if info, statErr := os.Stat(resolved); statErr == nil && info.Mode().IsRegular() {
			view = filepath.ToSlash(filepath.Clean(strings.TrimSuffix(dest, "/")))
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return view
}

func commandLabel(command string) string {
	label := command
	if index := strings.IndexByte(label, '\n'); index >= 0 {
		label = label[:index] + " …"
	}
	if runes := []rune(label); len(runes) > 80 {
		label = string(runes[:77]) + "…"
	}
	return label
}

func (a *app) pageData(jadePath, selected, view string) (pageData, error) {
	workspace, err := LoadWorkspace(a.root, jadePath)
	if err != nil {
		return pageData{}, err
	}
	if selected == "" {
		selected = markerName
	}
	contents, err := ReadWorkspaceFile(a.root, workspace.Path, selected)
	if err != nil {
		return pageData{}, err
	}
	runnables := make([]runnable, 0, len(workspace.Commands))
	for _, command := range workspace.Commands {
		runnables = append(runnables, runnable{Command: command, Label: commandLabel(command)})
	}
	if view != "" {
		if _, viewErr := existingFile(a.root, workspace.Path, view); viewErr != nil {
			view = ""
		}
	}
	if view == "" {
		view = a.defaultView(workspace)
	}
	data := pageData{
		Workspace: workspace,
		Selected:  selected,
		Contents:  contents,
		Runnables: runnables,
		View:      view,
		FrontURL:  "/front?jade=" + template.URLQueryEscaper(workspace.Path),
	}
	if view != "" {
		data.ViewURL = "/view?jade=" + template.URLQueryEscaper(workspace.Path) + "&file=" + template.URLQueryEscaper(view)
	}
	return data, nil
}

func (a *app) renderPage(response http.ResponseWriter, jadePath, selected, view string, update func(*pageData)) {
	data, err := a.pageData(jadePath, selected, view)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	if update != nil {
		update(&data)
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; frame-src 'self'; form-action 'self'; base-uri 'none'")
	if err := a.page.Execute(response, data); err != nil {
		log.Printf("render: %v", err)
	}
}

func (a *app) home(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.renderPage(response, queryPath(request, "jade", "."), queryPath(request, "file", markerName), request.URL.Query().Get("view"), nil)
}

func parseForm(response http.ResponseWriter, request *http.Request) bool {
	request.Body = http.MaxBytesReader(response, request.Body, maximumTextBytes+64_000)
	if err := request.ParseForm(); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func redirectToFile(response http.ResponseWriter, request *http.Request, jadePath, filePath, view string) {
	location := "/?jade=" + template.URLQueryEscaper(jadePath) + "&file=" + template.URLQueryEscaper(filePath)
	if view != "" {
		location += "&view=" + template.URLQueryEscaper(view)
	}
	http.Redirect(response, request, location, http.StatusSeeOther)
}

func (a *app) save(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	jadePath, filePath, view := request.FormValue("jade"), request.FormValue("file"), request.FormValue("view")
	if err := WriteWorkspaceFile(a.root, jadePath, filePath, request.FormValue("content")); err != nil {
		a.renderPage(response, jadePath, filePath, view, func(data *pageData) { data.Problem = err.Error() })
		return
	}
	redirectToFile(response, request, jadePath, filePath, view)
}

func (a *app) create(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	jadePath, filePath := request.FormValue("jade"), strings.TrimSpace(request.FormValue("path"))
	currentRoot, err := jadeDirectory(a.root, jadePath)
	if err == nil {
		var candidate string
		candidate, err = safeJoin(currentRoot, filePath)
		if err == nil {
			_, statErr := os.Stat(candidate)
			if statErr == nil {
				err = errors.New("file already exists")
			} else if !errors.Is(statErr, os.ErrNotExist) {
				err = statErr
			}
		}
	}
	if err == nil {
		contents := ""
		if filepath.Base(filePath) == markerName {
			contents = "# Untitled Jade\n"
		}
		err = WriteWorkspaceFile(a.root, jadePath, filePath, contents)
	}
	if err != nil {
		a.renderPage(response, jadePath, markerName, "", func(data *pageData) { data.Problem = err.Error() })
		return
	}
	redirectToFile(response, request, jadePath, filePath, "")
}

func (a *app) run(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	jadePath, selected, view := request.FormValue("jade"), request.FormValue("file"), request.FormValue("view")
	command := strings.TrimSpace(request.FormValue("command"))
	cwd, err := jadeDirectory(a.root, jadePath)
	if err == nil && command == "" {
		err = errors.New("command is empty")
	}
	output := []byte(nil)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		defer cancel()
		process := exec.CommandContext(ctx, "/bin/sh", "-c", command)
		process.Dir = cwd
		process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		process.Cancel = func() error {
			return syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
		}
		process.WaitDelay = 5 * time.Second
		output, err = process.CombinedOutput()
		if ctx.Err() != nil {
			err = fmt.Errorf("command timed out after %s", commandTimeout)
		}
	}
	a.renderPage(response, jadePath, selected, view, func(data *pageData) {
		data.CommandOutput = string(output)
		if data.CommandOutput == "" && err == nil {
			data.CommandOutput = "Command completed."
		}
		if err != nil {
			data.CommandFailed = true
			data.CommandOutput += "\n" + err.Error()
		}
	})
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
	_, _ = io.WriteString(response, `<!doctype html><meta charset="utf-8"><base target="_parent"><style>body{margin:1rem;color:#17201b;font:15px/1.5 system-ui,sans-serif}pre,code{font-family:ui-monospace,monospace}a{color:#145c3b}img{max-width:100%}table{border-collapse:collapse;margin:.8rem 0}th,td{border:1px solid #cbd2cd;padding:.3rem .6rem}</style>`)
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
	jadePath := queryPath(request, "jade", ".")
	filePath := request.URL.Query().Get("file")
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

func main() {
	start := "."
	port := 7333
	for _, argument := range os.Args[1:] {
		if strings.HasPrefix(argument, "--port=") {
			value, err := strconv.Atoi(strings.TrimPrefix(argument, "--port="))
			if err != nil || value < 1 || value > 65535 {
				log.Fatal("--port must be between 1 and 65535")
			}
			port = value
		} else {
			start = argument
		}
	}
	root, err := FindJadeRoot(start)
	if err != nil {
		log.Fatal(err)
	}
	application, err := newApp(root, port)
	if err != nil {
		log.Fatal(err)
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	fmt.Printf("Jade: http://%s\nRoot: %s\n", address, root)
	log.Fatal(http.ListenAndServe(address, application.handler()))
}
