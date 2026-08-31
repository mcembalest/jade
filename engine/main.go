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
  <style>
    :root { --ink:#121815; --muted:#68736d; --line:#dfe5e1; --canvas:#fbfcfb; --panel:#f3f6f4; --paper:#fff; --jade:#0b6b42; --jade-soft:#e2f0e8; }
    * { box-sizing:border-box; }
    html, body { height:100%; }
    body { margin:0; overflow:hidden; color:var(--ink); background:var(--canvas); font:13px/1.45 -apple-system,BlinkMacSystemFont,"SF Pro Text",system-ui,sans-serif; }
    button, input, select, textarea { font:inherit; }
    button { min-height:30px; color:var(--ink); background:var(--paper); border:1px solid var(--line); border-radius:7px; padding:.34rem .68rem; cursor:pointer; }
    button:hover { border-color:#aeb9b2; background:#f8faf9; }
    button:focus-visible, a:focus-visible, textarea:focus-visible, input:focus-visible, select:focus-visible { outline:2px solid #38a172; outline-offset:2px; }
    button.primary { color:white; border-color:var(--jade); background:var(--jade); }
    button.primary:hover { background:#075b37; }
    button:disabled { cursor:not-allowed; opacity:.45; }
    header { height:48px; display:flex; align-items:center; padding:0 12px 0 14px; border-bottom:1px solid var(--line); background:var(--paper); }
    .identity { min-width:0; display:flex; align-items:center; gap:8px; }
    .brand { color:var(--jade); font-size:12px; font-weight:800; letter-spacing:.12em; }
    .project { min-width:0; overflow:hidden; font-weight:650; text-overflow:ellipsis; white-space:nowrap; }
    .path { min-width:0; overflow:hidden; color:var(--muted); font:11px/1.3 ui-monospace,SFMono-Regular,Menlo,monospace; text-overflow:ellipsis; white-space:nowrap; }
    .branch-control { display:flex; align-items:center; gap:5px; margin-left:4px; padding-left:10px; border-left:1px solid var(--line); color:var(--muted); font-size:10px; }
    .branch-control[hidden] { display:none; }
    .branch-control select { max-width:180px; height:28px; border:0; border-radius:6px; padding:0 24px 0 6px; color:#26322b; background:#f2f5f3; font:11px/1.2 ui-monospace,SFMono-Regular,Menlo,monospace; }
    .header-actions { display:flex; gap:6px; margin-left:auto; padding-left:12px; }
    #shell { height:calc(100% - 48px); display:grid; grid-template-columns:236px minmax(0,1fr); }
    aside { min-width:0; overflow:auto; border-right:1px solid var(--line); background:var(--panel); }
    .explorer-head { position:sticky; top:0; z-index:1; height:38px; display:flex; align-items:center; padding:0 7px 0 14px; border-bottom:1px solid var(--line); background:var(--panel); color:var(--muted); font-size:10px; font-weight:750; letter-spacing:.1em; text-transform:uppercase; }
    .explorer-head button { margin-left:auto; padding:0; width:25px; min-height:25px; border:0; background:transparent; font-size:17px; line-height:1; }
    .tree, .tree ul { margin:0; padding:0; list-style:none; }
    .tree { padding:6px 0 12px; }
    .tree ul { padding-left:11px; }
    .tree details > summary { display:flex; align-items:center; gap:5px; padding:4px 9px; cursor:pointer; color:#37423c; list-style:none; }
    .tree details > summary::before { content:'›'; width:8px; color:#89948d; transition:transform .1s linear; }
    .tree details[open] > summary::before { transform:rotate(90deg); }
    .tree a { position:relative; display:block; overflow:hidden; padding:4px 9px 4px 25px; color:var(--ink); text-decoration:none; text-overflow:ellipsis; white-space:nowrap; }
    .tree a:hover { background:#e9eeeb; }
    .tree a.active { color:#064d2f; background:var(--jade-soft); font-weight:650; }
    .tree a.active::after { position:absolute; inset:4px auto 4px 0; width:3px; border-radius:0 2px 2px 0; background:var(--jade); content:''; }
    .tree a.jade-file::before { content:'◆'; margin-right:6px; color:var(--jade); font-size:8px; }
    .jade-mark { margin-left:auto; color:var(--jade); font-size:8px; font-weight:800; letter-spacing:.06em; text-transform:uppercase; }
    #workbench { min-width:0; min-height:0; }
    #document { width:100%; height:100%; min-width:0; min-height:0; display:grid; grid-template-columns:minmax(0,1fr); }
    #document.jade-open { grid-template-columns:minmax(320px,1fr) minmax(360px,1fr); }
    #editor-form, #resolved { min-width:0; min-height:0; display:flex; flex-direction:column; }
    #resolved { border-left:1px solid var(--line); }
    .filebar { height:38px; flex:0 0 auto; display:flex; align-items:center; gap:8px; padding:0 14px; border-bottom:1px solid var(--line); background:var(--paper); font:11px/1.3 ui-monospace,SFMono-Regular,Menlo,monospace; }
    #save-status { margin-left:auto; color:var(--muted); font:11px/1.3 -apple-system,BlinkMacSystemFont,system-ui,sans-serif; }
    textarea { width:100%; min-height:0; flex:1; resize:none; border:0; padding:20px clamp(20px,4vw,56px); color:#151c18; background:var(--paper); tab-size:2; outline:0; font:13px/1.62 ui-monospace,SFMono-Regular,Menlo,monospace; }
    #resolved[hidden] { display:none; }
    #view-frame { width:100%; min-height:0; flex:1; border:0; background:white; }
    dialog { width:min(540px,calc(100vw - 28px)); padding:0; border:1px solid #aeb8b1; border-radius:12px; background:var(--paper); box-shadow:0 20px 70px #1020182b; }
    dialog::backdrop { background:#17201b4a; backdrop-filter:blur(2px); }
    .modal-head, .modal-actions { display:flex; align-items:center; gap:8px; padding:12px 14px; }
    .modal-head { border-bottom:1px solid var(--line); }
    .modal-head strong { font-size:14px; }
    .modal-head button { margin-left:auto; border:0; background:transparent; font-size:17px; }
    .modal-body { display:grid; gap:12px; padding:16px; }
    .modal-body label { display:grid; gap:5px; color:var(--muted); font-size:11px; font-weight:600; }
    .modal-body label[hidden] { display:none; }
    .modal-body input, .modal-body select { width:100%; padding:8px 9px; border:1px solid var(--line); border-radius:7px; color:var(--ink); background:white; }
    #publish-summary { min-height:76px; max-height:220px; overflow:auto; margin:0; padding:10px; border:1px solid var(--line); border-radius:7px; background:#f7f9f8; white-space:pre-wrap; font:11px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace; }
    #publish-note { min-height:18px; margin:0; color:var(--muted); font-size:11px; }
    .modal-actions { justify-content:flex-end; border-top:1px solid var(--line); }
    @media (max-width:850px) {
      #shell { grid-template-columns:178px minmax(0,1fr); }
      .path { display:none; }
      #document.jade-open { grid-template-columns:1fr; grid-template-rows:minmax(240px,1fr) minmax(240px,1fr); overflow:auto; }
      #document.jade-open #resolved { border-top:1px solid var(--line); border-left:0; }
    }
    @media (prefers-reduced-motion:reduce) { *, *::before, *::after { scroll-behavior:auto!important; transition:none!important; } }
  </style>
</head>
<body data-jade="{{.Workspace.Path}}" data-file="{{.Selected}}">
  <header>
    <div class="identity"><span class="brand">JADE</span><span class="project">{{.Workspace.Title}}</span><span class="path">{{.Workspace.Path}}</span><label class="branch-control" hidden><span>branch</span><select id="branch-select" aria-label="Git branch"></select></label></div>
    <div class="header-actions">
      <button id="terminal-toggle" type="button">Open terminal</button>
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
    </main>
  </div>
  <dialog id="publish-dialog">
    <div class="modal-head"><strong>Publish</strong><button id="publish-close" type="button" aria-label="Close">×</button></div>
    <div class="modal-body">
      <label>Destination<select id="publish-destination"><option value="github">GitHub</option><option value="arxiv">arXiv</option><option value="substack">Substack</option></select></label>
      <pre id="publish-summary">Checking the nearest repository…</pre>
      <label id="commit-field">Commit message<input id="commit-message" value="Update from JaDE"></label>
      <p id="publish-note" role="status"></p>
    </div>
    <div class="modal-actions"><button id="publish-cancel" type="button">Cancel</button><button id="publish-confirm" class="primary" type="button">Commit, push & open PR</button></div>
  </dialog>
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
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "j") { event.preventDefault(); openTerminal(); }
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

  const terminalToggle = document.querySelector("#terminal-toggle");
  async function openTerminal() {
    if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.jade) {
      window.webkit.messageHandlers.jade.postMessage({type:"terminal", jade:body.dataset.jade});
      status.textContent = "Terminal opened";
      return;
    }
    terminalToggle.disabled = true;
    terminalToggle.textContent = "Opening…";
    const data = new FormData();
    data.set("jade", body.dataset.jade);
    const response = await fetch("/terminal", {method:"POST", body:data});
    const result = await response.json();
    terminalToggle.disabled = false;
    terminalToggle.textContent = "Open terminal";
    status.textContent = response.ok ? "Terminal opened" : (result.error || "Could not open Ghostty");
  }
  terminalToggle.addEventListener("click", openTerminal);

  const branchControl = document.querySelector(".branch-control");
  const branchSelect = document.querySelector("#branch-select");
  let currentBranch = "";
  async function loadBranches() {
    const response = await fetch("/git/branches?jade=" + encodeURIComponent(body.dataset.jade));
    if (!response.ok) return;
    const state = await response.json();
    currentBranch = state.current;
    branchSelect.replaceChildren(...state.branches.map(branch => {
      const option = document.createElement("option");
      option.value = branch; option.textContent = branch; option.selected = branch === state.current;
      return option;
    }));
    branchControl.hidden = state.branches.length === 0;
  }
  branchSelect.addEventListener("change", async () => {
    const branch = branchSelect.value;
    if (branch === currentBranch) return;
    if (!(await save())) { branchSelect.value = currentBranch; return; }
    branchSelect.disabled = true; status.textContent = "Switching branch…";
    const data = new FormData(); data.set("jade", body.dataset.jade); data.set("branch", branch);
    const response = await fetch("/git/switch", {method:"POST", body:data});
    const result = await response.json();
    if (!response.ok) { status.textContent = result.error || "Could not switch branch"; branchSelect.value = currentBranch; branchSelect.disabled = false; return; }
    location.href = "/";
  });
  loadBranches();

  const dialog = document.querySelector("#publish-dialog");
  const destination = document.querySelector("#publish-destination");
  const summary = document.querySelector("#publish-summary");
  const note = document.querySelector("#publish-note");
  const commitField = document.querySelector("#commit-field");
  const confirm = document.querySelector("#publish-confirm");
  let publishStatus;

  async function loadPublish() {
    const file = body.dataset.file.toLowerCase();
    if (destination.value === "substack") {
      commitField.hidden = true;
      summary.textContent = file.endsWith(".md") ? "The active Markdown will be copied as rich text, then Substack’s editor will open." : "Select a Markdown file before publishing to Substack.";
      note.textContent = "JaDE never receives your Substack credentials.";
      confirm.textContent = "Copy & open Substack";
      confirm.disabled = !file.endsWith(".md");
      return;
    }
    if (destination.value === "arxiv") {
      const ready = /\.(tex|pdf|zip)$/.test(file);
      commitField.hidden = true;
      summary.textContent = ready ? (file.endsWith(".tex") ? "JaDE will package the active TeX file and its neighboring source files, download the ZIP, then open arXiv’s submission workflow." : "JaDE will download the active paper, then open arXiv’s submission workflow.") : "Select a TeX, PDF, or ZIP paper before publishing to arXiv.";
      note.textContent = "Submission stays interactive, as arXiv recommends for individual authors. JaDE never receives your arXiv credentials.";
      confirm.textContent = "Download & open arXiv";
      confirm.disabled = !ready;
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
    confirm.disabled = true;
    note.textContent = destination.value === "github" ? "Publishing…" : destination.value === "arxiv" ? "Packaging paper…" : "Preparing draft…";
    if (destination.value === "github") {
      const data = new FormData(); data.set("jade", body.dataset.jade); data.set("message", document.querySelector("#commit-message").value);
      const response = await fetch("/publish/github", {method:"POST", body:data}); const result = await response.json();
      if (!response.ok) { note.textContent = result.error; confirm.disabled = false; return; }
      note.textContent = result.message; if (result.url) window.open(result.url, "_blank", "noopener"); dialog.close(); return;
    }
    if (destination.value === "arxiv") {
      const opened = window.open("about:blank", "_blank");
      const data = new FormData(); data.set("jade", body.dataset.jade); data.set("file", body.dataset.file);
      const response = await fetch("/publish/arxiv", {method:"POST", body:data});
      if (!response.ok) { note.textContent = await response.text(); opened && opened.close(); confirm.disabled = false; return; }
      const blob = await response.blob();
      const match = (response.headers.get("Content-Disposition") || "").match(/filename="([^"]+)"/);
      const link = document.createElement("a"); link.href = URL.createObjectURL(blob); link.download = match ? match[1] : "paper-arxiv.zip"; link.click();
      setTimeout(() => URL.revokeObjectURL(link.href), 1000);
      if (opened) opened.location = "https://arxiv.org/submit"; else window.open("https://arxiv.org/submit", "_blank", "noopener");
      dialog.close(); return;
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
	mux.HandleFunc("/git/branches", a.guard(a.branches))
	mux.HandleFunc("/git/switch", a.guard(a.switchBranch))
	mux.HandleFunc("/publish/arxiv", a.guard(a.publishArxiv))
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
			return pageData{}, err
		}
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
	data, err := a.pageData(queryPath(request, "jade", "."), request.URL.Query().Get("file"), request.URL.Query().Get("view"))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-src 'self'; img-src 'self' data:; form-action 'self'; base-uri 'none'")
	if err := a.page.Execute(response, data); err != nil {
		log.Printf("render: %v", err)
	}
}

func (a *app) file(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := a.pageData(queryPath(request, "jade", "."), request.URL.Query().Get("file"), request.URL.Query().Get("view"))
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
