package engine

import (
	"bytes"
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

const pageTemplate = `{{define "tree"}}{{range .}}{{if .Directory}}<li><details open><summary>{{.Name}}{{if .Jade}}<span class="jade-mark">JaDE</span>{{end}}</summary><ul>{{template "tree" .Children}}</ul></details></li>{{else}}<li><a href="{{.URL}}" data-file="{{.Path}}" data-jade="{{.JadePath}}" class="file-link {{if .Jade}}jade-file{{end}}">{{.Name}}</a></li>{{end}}{{end}}{{end}}<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{.Workspace.Title}} · JaDE</title>
  <link rel="icon" href="data:image/svg+xml,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%20viewBox=%220%200%20100%20100%22%3E%3Ctext%20y=%22.9em%22%20font-size=%2290%22%3E🐉%3C/text%3E%3C/svg%3E">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.min.css">
  <style>
    :root { --ink:#17201b; --muted:#66716a; --line:#d7ddd9; --paper:#fff; --side:#f0f3f1; --jade:#176b43; --terminal:#111713; }
    * { box-sizing:border-box; }
    html, body { height:100%; }
    body { margin:0; overflow:hidden; color:var(--ink); background:var(--paper); font:14px/1.45 system-ui,sans-serif; }
    button, input, select, textarea { font:inherit; }
    button { color:var(--ink); background:var(--paper); border:1px solid var(--line); padding:.38rem .62rem; cursor:pointer; }
    button:hover { border-color:#9eaaa2; }
    button:focus-visible, a:focus-visible, textarea:focus-visible, input:focus-visible, select:focus-visible { outline:2px solid var(--jade); outline-offset:-2px; }
    button.primary { color:white; border-color:var(--jade); background:var(--jade); }
    button:disabled { cursor:not-allowed; opacity:.5; }
    header { height:44px; display:flex; align-items:center; gap:.7rem; padding:0 .8rem; border-bottom:1px solid var(--line); background:var(--paper); }
    .brand { color:var(--jade); font-weight:750; letter-spacing:-.02em; }
    .project { min-width:0; overflow:hidden; color:var(--muted); text-overflow:ellipsis; white-space:nowrap; }
    .header-actions { display:flex; gap:.45rem; margin-left:auto; }
    #shell { height:calc(100% - 44px); display:grid; grid-template-columns:224px minmax(0,1fr); }
    aside { min-width:0; overflow:auto; border-right:1px solid var(--line); background:var(--side); }
    .explorer-head { position:sticky; top:0; z-index:1; height:36px; display:flex; align-items:center; padding:0 .55rem .0rem .75rem; border-bottom:1px solid var(--line); background:var(--side); color:var(--muted); font-size:11px; font-weight:700; letter-spacing:.08em; text-transform:uppercase; }
    .explorer-head button { margin-left:auto; padding:0; width:24px; height:24px; border:0; background:transparent; font-size:18px; line-height:1; }
    .tree, .tree ul { margin:0; padding:0; list-style:none; }
    .tree { padding:.4rem 0 .8rem; }
    .tree ul { padding-left:.8rem; }
    .tree details > summary { display:flex; align-items:center; gap:.35rem; padding:.24rem .6rem; cursor:pointer; color:#36423b; list-style:none; }
    .tree details > summary::before { content:'›'; width:.7rem; color:#849087; transition:transform .1s linear; }
    .tree details[open] > summary::before { transform:rotate(90deg); }
    .tree a { display:block; overflow:hidden; padding:.25rem .6rem .25rem 1.3rem; color:var(--ink); text-decoration:none; text-overflow:ellipsis; white-space:nowrap; }
    .tree a:hover { background:#e2e8e4; }
    .tree a.active { color:#0d5835; background:#dbe9e0; font-weight:650; }
    .tree a.jade-file::before { content:'◆'; margin-right:.42rem; color:var(--jade); font-size:.62rem; }
    .jade-mark { margin-left:auto; color:var(--jade); font-size:9px; font-weight:750; letter-spacing:.05em; text-transform:uppercase; }
    #workbench { min-width:0; min-height:0; display:grid; grid-template-rows:minmax(0,1fr) auto; }
    #document { min-width:0; min-height:0; display:grid; grid-template-columns:minmax(0,1fr); }
    #document.jade-open { grid-template-columns:minmax(280px,1fr) minmax(320px,1fr); }
    #editor-form, #resolved { min-width:0; min-height:0; display:flex; flex-direction:column; }
    #resolved { border-left:1px solid var(--line); }
    .filebar { height:36px; flex:0 0 auto; display:flex; align-items:center; gap:.65rem; padding:0 .75rem; border-bottom:1px solid var(--line); background:#fafbfa; font:12px ui-monospace,SFMono-Regular,Menlo,monospace; }
    #save-status { margin-left:auto; color:var(--muted); font:11px system-ui,sans-serif; }
    textarea { width:100%; min-height:0; flex:1; resize:none; border:0; padding:1rem 1.1rem; color:#18221c; background:var(--paper); tab-size:2; outline:0; font:13px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace; }
    #resolved[hidden], #terminal-panel[hidden] { display:none; }
    #view-frame { width:100%; min-height:0; flex:1; border:0; background:white; }
    #terminal-panel { height:36vh; min-height:180px; border-top:1px solid #2b352e; background:var(--terminal); }
    #terminal { height:100%; padding:.45rem; }
    dialog { width:min(560px,calc(100vw - 2rem)); padding:0; border:1px solid #aeb8b1; background:var(--paper); box-shadow:0 18px 60px #0d1b1333; }
    dialog::backdrop { background:#17201b55; }
    .modal-head, .modal-actions { display:flex; align-items:center; gap:.6rem; padding:.75rem .9rem; }
    .modal-head { border-bottom:1px solid var(--line); }
    .modal-head strong { font-size:15px; }
    .modal-head button { margin-left:auto; border:0; font-size:18px; }
    .modal-body { display:grid; gap:.8rem; padding:1rem; }
    .modal-body label { display:grid; gap:.3rem; color:var(--muted); font-size:12px; }
    .modal-body input, .modal-body select { width:100%; padding:.48rem .55rem; border:1px solid var(--line); color:var(--ink); background:white; }
    #publish-summary { min-height:5rem; max-height:15rem; overflow:auto; margin:0; padding:.7rem; border:1px solid var(--line); background:#f7f9f7; white-space:pre-wrap; font:12px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace; }
    #publish-note { min-height:1.3rem; margin:0; color:var(--muted); font-size:12px; }
    .modal-actions { justify-content:flex-end; border-top:1px solid var(--line); }
    @media (max-width:850px) {
      #shell { grid-template-columns:170px minmax(0,1fr); }
      #document.jade-open { grid-template-columns:1fr; grid-template-rows:minmax(240px,1fr) minmax(240px,1fr); overflow:auto; }
      #document.jade-open #resolved { border-top:1px solid var(--line); border-left:0; }
    }
  </style>
</head>
<body data-jade="{{.Workspace.Path}}" data-file="{{.Selected}}">
  <header>
    <span class="brand">JaDE</span>
    <span class="project">{{.Workspace.Path}} / {{.Workspace.Title}}</span>
    <div class="header-actions">
      <button id="terminal-toggle" type="button" aria-expanded="false">Terminal</button>
      <button id="publish-open" class="primary" type="button">Publish</button>
    </div>
  </header>
  <div id="shell">
    <aside aria-label="Files">
      <div class="explorer-head">Files <button id="new-file" type="button" title="New file" aria-label="New file">+</button></div>
      <ul class="tree">{{template "tree" .Files}}</ul>
    </aside>
    <main id="workbench">
      <div id="document" class="{{if .IsJade}}jade-open{{end}}">
        <form id="editor-form" method="post" action="/save">
          <div class="filebar"><span id="file-name">{{.Selected}}</span><span id="save-status" role="status">Saved</span></div>
          <input type="hidden" name="jade" value="{{.Workspace.Path}}">
          <input type="hidden" name="file" value="{{.Selected}}">
          <textarea name="content" spellcheck="false" aria-label="Editor">{{.Contents}}</textarea>
        </form>
        <section id="resolved" {{if not .IsJade}}hidden{{end}}>
          <div class="filebar"><span id="view-name">{{if .View}}{{.View}}{{else}}{{.Workspace.Title}}{{end}}</span></div>
          <iframe id="view-frame" sandbox="allow-top-navigation-by-user-activation" src="{{.ViewURL}}" title="Resolved JaDE view"></iframe>
        </section>
      </div>
      <section id="terminal-panel" hidden aria-label="Terminal"><div id="terminal"></div></section>
    </main>
  </div>
  <dialog id="publish-dialog">
    <div class="modal-head"><strong>Publish</strong><button id="publish-close" type="button" aria-label="Close">×</button></div>
    <div class="modal-body">
      <label>Destination<select id="publish-destination"><option value="github">GitHub</option><option value="substack">Substack</option></select></label>
      <pre id="publish-summary">Checking the nearest repository…</pre>
      <label id="commit-field">Commit message<input id="commit-message" value="Update from JaDE"></label>
      <p id="publish-note"></p>
    </div>
    <div class="modal-actions"><button id="publish-cancel" type="button">Cancel</button><button id="publish-confirm" class="primary" type="button">Commit, push & open PR</button></div>
  </dialog>
  <script src="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.min.js"></script>
  <script src="/app.js" defer></script>
</body>
</html>`

const appScript = `(() => {
  const body = document.body;
  const form = document.querySelector("#editor-form");
  const editor = form.querySelector("textarea");
  const fileInput = form.querySelector("input[name=file]");
  const status = document.querySelector("#save-status");
  const documentPane = document.querySelector("#document");
  const resolved = document.querySelector("#resolved");
  const viewFrame = document.querySelector("#view-frame");
  const viewName = document.querySelector("#view-name");
  let dirty = false;

  function setDirty(value) {
    dirty = value;
    status.textContent = value ? "Edited" : "Saved";
  }

  function cursorKey(file) {
    return "jade-cursor:" + body.dataset.jade + ":" + file;
  }

  function rememberCursor() {
    sessionStorage.setItem(cursorKey(fileInput.value), String(editor.selectionStart));
  }

  function restoreCursor() {
    const saved = sessionStorage.getItem(cursorKey(fileInput.value));
    const position = Math.min(saved === null ? 0 : Number(saved), editor.value.length);
    editor.setSelectionRange(position, position);
    if (position === 0) editor.scrollTop = 0;
  }

  async function save() {
    if (!dirty) return true;
    status.textContent = "Saving…";
    const response = await fetch("/save", {method:"POST", body:new FormData(form)});
    if (!response.ok) { status.textContent = await response.text(); return false; }
    const data = await response.json();
    rememberCursor();
    setDirty(false);
    if (data.viewURL && fileInput.value === "jade.md") {
      viewFrame.src = data.viewURL;
      viewName.textContent = data.view || body.dataset.jade;
    }
    return true;
  }

  async function openFile(link) {
    if (link.dataset.jade !== body.dataset.jade) { location.href = link.href; return; }
    if (!(await save())) return;
    rememberCursor();
    const url = new URL("/file", location.origin);
    url.searchParams.set("jade", body.dataset.jade);
    url.searchParams.set("file", link.dataset.file);
    const response = await fetch(url);
    if (!response.ok) { status.textContent = await response.text(); return; }
    const data = await response.json();
    editor.value = data.contents;
    fileInput.value = data.selected;
    body.dataset.file = data.selected;
    restoreCursor();
    document.querySelector("#file-name").textContent = data.selected;
    document.querySelectorAll(".file-link.active").forEach(node => node.classList.remove("active"));
    link.classList.add("active");
    documentPane.classList.toggle("jade-open", data.isJade);
    resolved.hidden = !data.isJade;
    if (data.isJade) { viewFrame.src = data.viewURL; viewName.textContent = data.view || data.title; }
    history.pushState({}, "", link.href);
    setDirty(false);
    editor.focus();
  }

  document.querySelectorAll(".file-link").forEach(link => {
    if (link.dataset.file === body.dataset.file && link.dataset.jade === body.dataset.jade) link.classList.add("active");
    link.addEventListener("click", event => { event.preventDefault(); openFile(link); });
  });
  restoreCursor();
  editor.addEventListener("input", () => setDirty(true));
  editor.addEventListener("keydown", event => {
    if (event.key === "Tab" && !event.shiftKey) {
      event.preventDefault();
      editor.setRangeText("\t", editor.selectionStart, editor.selectionEnd, "end");
      setDirty(true);
    }
  });
  form.addEventListener("submit", event => { event.preventDefault(); save(); });
  addEventListener("keydown", event => {
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") { event.preventDefault(); save(); }
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "j") { event.preventDefault(); toggleTerminal(); }
  });
  addEventListener("beforeunload", event => { rememberCursor(); if (dirty) { event.preventDefault(); event.returnValue = ""; } });
  addEventListener("popstate", () => location.reload());

  document.querySelector("#new-file").addEventListener("click", async () => {
    const path = prompt("New file path");
    if (!path || !(await save())) return;
    const data = new FormData(); data.set("jade", body.dataset.jade); data.set("path", path);
    const response = await fetch("/new", {method:"POST", body:data});
    if (!response.ok) { status.textContent = await response.text(); return; }
    location.href = await response.text();
  });

  let terminal, socket, fit;
  const terminalPanel = document.querySelector("#terminal-panel");
  const terminalToggle = document.querySelector("#terminal-toggle");
  function fitTerminal() {
    if (!fit || terminalPanel.hidden) return;
    fit.fit();
    if (socket && socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({type:"resize", cols:terminal.cols, rows:terminal.rows}));
  }
  function startTerminal() {
    if (terminal) return;
    if (!window.Terminal || !window.FitAddon) { document.querySelector("#terminal").textContent = "Terminal assets unavailable."; return; }
    terminal = new Terminal({cursorBlink:true, convertEol:false, fontFamily:"ui-monospace, SFMono-Regular, Menlo, monospace", fontSize:13, theme:{background:"#111713", foreground:"#dbe5dd", cursor:"#6fd49c"}});
    fit = new FitAddon.FitAddon(); terminal.loadAddon(fit); terminal.open(document.querySelector("#terminal"));
    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    socket = new WebSocket(protocol + "//" + location.host + "/terminal?jade=" + encodeURIComponent(body.dataset.jade));
    socket.binaryType = "arraybuffer";
    socket.addEventListener("open", fitTerminal);
    socket.addEventListener("message", event => terminal.write(new Uint8Array(event.data)));
    socket.addEventListener("close", () => terminal.write("\r\n[terminal closed]\r\n"));
    terminal.onData(data => { if (socket.readyState === WebSocket.OPEN) socket.send(new TextEncoder().encode(data)); });
    new ResizeObserver(fitTerminal).observe(terminalPanel);
  }
  function toggleTerminal() {
    terminalPanel.hidden = !terminalPanel.hidden;
    terminalToggle.setAttribute("aria-expanded", String(!terminalPanel.hidden));
    if (!terminalPanel.hidden) { startTerminal(); requestAnimationFrame(() => { fitTerminal(); terminal && terminal.focus(); }); }
  }
  terminalToggle.addEventListener("click", toggleTerminal);

  const dialog = document.querySelector("#publish-dialog");
  const destination = document.querySelector("#publish-destination");
  const summary = document.querySelector("#publish-summary");
  const note = document.querySelector("#publish-note");
  const commitField = document.querySelector("#commit-field");
  const confirm = document.querySelector("#publish-confirm");
  let publishStatus;

  async function loadPublish() {
    if (destination.value === "substack") {
      commitField.hidden = true;
      summary.textContent = body.dataset.file.endsWith(".md") ? "The active Markdown will be copied as rich text, then Substack’s editor will open." : "Select a Markdown file before publishing to Substack.";
      note.textContent = "JaDE never receives your Substack credentials.";
      confirm.textContent = "Copy & open Substack";
      confirm.disabled = !body.dataset.file.endsWith(".md");
      return;
    }
    commitField.hidden = false; confirm.textContent = "Commit, push & open PR"; confirm.disabled = true;
    summary.textContent = "Checking the nearest repository…"; note.textContent = "";
    const response = await fetch("/publish/status?jade=" + encodeURIComponent(body.dataset.jade));
    publishStatus = await response.json();
    if (!response.ok) { summary.textContent = publishStatus.error || "Repository unavailable."; return; }
    summary.textContent = publishStatus.repository + " · " + publishStatus.branch + (publishStatus.worktree ? " · worktree" : "") + "\n" + publishStatus.root + "\n\n" + (publishStatus.changes || "No uncommitted changes.");
    note.textContent = !publishStatus.canPublish ? "Create a branch or worktree before publishing." : publishStatus.pullRequest ? "Additional commits will update the existing PR." : "A PR will be created, then opened in GitHub for review.";
    confirm.disabled = !publishStatus.canPublish;
  }

  document.querySelector("#publish-open").addEventListener("click", async () => { if (!(await save())) return; dialog.showModal(); loadPublish(); });
  document.querySelector("#publish-close").addEventListener("click", () => dialog.close());
  document.querySelector("#publish-cancel").addEventListener("click", () => dialog.close());
  destination.addEventListener("change", loadPublish);
  confirm.addEventListener("click", async () => {
    confirm.disabled = true; note.textContent = destination.value === "github" ? "Publishing…" : "Preparing draft…";
    if (destination.value === "github") {
      const data = new FormData(); data.set("jade", body.dataset.jade); data.set("message", document.querySelector("#commit-message").value);
      const response = await fetch("/publish/github", {method:"POST", body:data}); const result = await response.json();
      if (!response.ok) { note.textContent = result.error; confirm.disabled = false; return; }
      note.textContent = result.message; if (result.url) window.open(result.url, "_blank", "noopener"); dialog.close(); return;
    }
    const opened = window.open("https://substack.com/home/post/publish", "_blank");
    const data = new FormData(); data.set("file", body.dataset.file); data.set("content", editor.value);
    const response = await fetch("/publish/substack", {method:"POST", body:data}); const result = await response.json();
    if (!response.ok) { note.textContent = result.error; opened && opened.close(); confirm.disabled = false; return; }
    try {
      await navigator.clipboard.write([new ClipboardItem({"text/html":new Blob([result.html],{type:"text/html"}), "text/plain":new Blob([result.text],{type:"text/plain"})})]);
      note.textContent = "Draft copied. Paste it into Substack; use “" + result.title + "” as the title.";
    } catch (_) { await navigator.clipboard.writeText(result.text); note.textContent = "Markdown copied. Paste it into Substack."; }
    dialog.close();
  });
})();`

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
		if site := request.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
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
	mux.HandleFunc("/new", a.guard(a.create))
	mux.HandleFunc("/front", a.guard(a.front))
	mux.HandleFunc("/view", a.guard(a.view))
	mux.HandleFunc("/terminal", a.guard(a.terminal))
	mux.HandleFunc("/publish/status", a.guard(a.publishStatus))
	mux.HandleFunc("/publish/github", a.guard(a.publishGitHub))
	mux.HandleFunc("/publish/substack", a.guard(a.publishSubstack))
	mux.HandleFunc("/app.js", a.guard(a.script))
	return mux
}

func (a *app) script(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(response, appScript)
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
		return []byte("/view?jade=" + query(jadePath) + "&file=" + query(rel))
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
			typed.Destination = a.rewriteDestination(jadePath, typed.Destination, false)
		case *ast.Image:
			typed.Destination = a.rewriteDestination(jadePath, typed.Destination, true)
		}
		return ast.WalkContinue, nil
	})
}

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
	data := pageData{Workspace: workspace, Selected: selected, Contents: contents, Files: buildFileTree(workspace), IsJade: selected == markerName}
	if !data.IsJade {
		return data, nil
	}
	if view != "" {
		if _, viewErr := existingFile(a.root, workspace.Path, view); viewErr != nil {
			view = ""
		}
	}
	if view == "" {
		view = a.defaultView(workspace)
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
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := a.pageData(queryPath(request, "jade", "."), queryPath(request, "file", markerName), request.URL.Query().Get("view"))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; connect-src 'self' ws: wss:; frame-src 'self'; img-src 'self' data:; form-action 'self'; base-uri 'none'")
	if err := a.page.Execute(response, data); err != nil {
		log.Printf("render: %v", err)
	}
}

func (a *app) file(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := a.pageData(queryPath(request, "jade", "."), queryPath(request, "file", markerName), request.URL.Query().Get("view"))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"selected": data.Selected, "contents": data.Contents, "isJade": data.IsJade, "view": data.View, "viewURL": data.ViewURL, "title": data.Workspace.Title})
}

func parseForm(response http.ResponseWriter, request *http.Request) bool {
	request.Body = http.MaxBytesReader(response, request.Body, maximumTextBytes+64_000)
	if err := request.ParseMultipartForm(maximumTextBytes + 64_000); err != nil {
		if err = request.ParseForm(); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return false
		}
	}
	return true
}

func (a *app) save(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !parseForm(response, request) {
		return
	}
	jadePath, filePath := request.FormValue("jade"), request.FormValue("file")
	if err := WriteWorkspaceFile(a.root, jadePath, filePath, request.FormValue("content")); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	data, err := a.pageData(jadePath, filePath, "")
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"view": data.View, "viewURL": data.ViewURL})
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
		err = WriteWorkspaceFile(a.root, jadePath, filePath, contents)
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
	_, _ = io.WriteString(response, `<!doctype html><meta charset="utf-8"><base target="_parent"><style>body{max-width:52rem;margin:2rem auto;padding:0 1.5rem;color:#17201b;font:15px/1.55 system-ui,sans-serif}pre,code{font-family:ui-monospace,monospace}pre{overflow:auto;padding:.8rem;background:#f0f3f1}a{color:#145c3b}img{max-width:100%}table{border-collapse:collapse;margin:.8rem 0}th,td{border:1px solid #cbd2cd;padding:.3rem .6rem}</style>`)
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
