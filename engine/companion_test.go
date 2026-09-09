package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fake executable speaks the real stdio protocol, including server requests.
func TestCompanionCodexProcess(t *testing.T) {
	if os.Getenv("JADE_FAKE_CODEX") != "1" {
		return
	}
	scan := bufio.NewScanner(os.Stdin)
	send := func(v any) { _ = json.NewEncoder(os.Stdout).Encode(v) }
	for scan.Scan() {
		var p struct {
			ID     any            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal(scan.Bytes(), &p) != nil {
			os.Exit(2)
		}
		result := any(map[string]any{})
		switch p.Method {
		case "initialized":
			continue
		case "account/read":
			result = map[string]any{"account": map[string]string{"type": os.Getenv("JADE_FAKE_ACCOUNT")}}
		case "config/read":
			result = map[string]any{"config": map[string]any{"mcp_servers": map[string]any{"example": map[string]any{}}}}
		case "thread/start":
			cfg := p.Params["config"].(map[string]any)
			if cfg["features.shell_tool"] != false || cfg["features.apps"] != false || cfg["features.plugins"] != false || cfg["mcp_servers.example.enabled"] != false || cfg["web_search"] != "live" || p.Params["sandbox"] != "read-only" || p.Params["ephemeral"] != true {
				os.Exit(3)
			}
			result = map[string]any{"thread": map[string]string{"id": "sanjana"}}
		case "turn/start":
			if strings.Contains(fmt.Sprint(p.Params["input"]), "Quiet background research") && p.Params["effort"] != "low" {
				os.Exit(6)
			}
			send(map[string]any{"id": p.ID, "result": result})
			if os.Getenv("JADE_FAKE_MODE") == "wait" {
				time.Sleep(time.Minute)
				os.Exit(4)
			}
			send(map[string]any{"id": 900, "method": "item/commandExecution/requestApproval", "params": map[string]any{}})
			if !scan.Scan() || !strings.Contains(scan.Text(), "chat and web search only") {
				os.Exit(5)
			}
			answer := `{"message":"A new discovery.","sources":[{"title":"Source","url":"https://example.com/story"},{"title":"Bad","url":"javascript:alert(1)"}]}`
			if os.Getenv("JADE_FAKE_MODE") == "quiet" {
				answer = `{"message":"","sources":[]}`
			}
			if os.Getenv("JADE_FAKE_MODE") == "bad" {
				answer = `oops`
			}
			send(map[string]any{"method": "item/completed", "params": map[string]any{"item": map[string]string{"type": "agentMessage", "text": answer, "phase": "final_answer"}}})
			send(map[string]any{"method": "turn/completed", "params": map[string]any{"turn": map[string]string{"status": "completed"}}})
			continue
		}
		send(map[string]any{"id": p.ID, "result": result})
	}
	os.Exit(0)
}
func fakeCompanion(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(executable, "'", "'\\''") + "' -test.run=^TestCompanionCodexProcess$\n"
	if err = os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("JADE_FAKE_CODEX", "1")
	t.Setenv("JADE_FAKE_ACCOUNT", "chatgpt")
}
func TestCompanionProtocol(t *testing.T) {
	fakeCompanion(t)
	for _, mode := range []string{"normal", "quiet", "bad", "wait"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("JADE_FAKE_MODE", mode)
			timeout := 3 * time.Second
			if mode == "wait" {
				timeout = 300 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			text, sources, err := runCompanion(ctx, nil, "search", mode == "quiet")
			switch mode {
			case "normal":
				if err != nil || text != "A new discovery." || len(sources) != 1 {
					t.Fatalf("%q %v %v", text, sources, err)
				}
			case "quiet":
				if err != nil || text != "" {
					t.Fatalf("%q %v", text, err)
				}
			default:
				if err == nil {
					t.Fatal("expected error")
				}
			}
		})
	}
	t.Setenv("JADE_FAKE_ACCOUNT", "apikey")
	if _, _, err := runCompanion(context.Background(), nil, "hello", false); err == nil || !strings.Contains(err.Error(), "ChatGPT") {
		t.Fatal(err)
	}
}
func TestCompanionLive(t *testing.T) {
	if os.Getenv("JADE_LIVE_CHECK") != "1" {
		t.Skip()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	answer, sources, err := runCompanion(ctx, nil, "Please search the web for the official Dries Van Noten website and tell me one thing you found there, with a source link.", false)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(answer, sources)
	if len(sources) == 0 {
		t.Fatal("no sources")
	}
	answer, sources, err = runCompanion(ctx, nil, "Pending findings: []", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Research:", answer, sources)
	if answer != "" && len(sources) == 0 {
		t.Fatal("unsourced research")
	}
}

func TestCompanionCloudReader(t *testing.T) {
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	// Use the same isolated config mechanism as the other engine tests.
	_ = config
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	config, _ = os.UserConfigDir()
	dir := filepath.Join(config, "JaDE")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	reads, writes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/companion" || r.Header.Get("Authorization") != "Bearer test" {
			t.Error("wrong cloud route/auth")
		}
		if r.Method == "GET" {
			reads++
		} else {
			writes++
		}
		fmt.Fprint(w, `{"messages":[],"pending":[],"enabled":true,"paused":false,"researchNext":0,"next":0,"profile":"shared"}`)
	}))
	defer server.Close()
	raw, _ := json.Marshal(map[string]string{"endpoint": server.URL, "agentToken": "test"})
	_ = os.WriteFile(filepath.Join(dir, "remote.json"), raw, 0600)
	a := &app{}
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		a.companion(w, httptest.NewRequest("GET", "/companion", nil))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	for _, action := range []string{"research", "discover", "enabled"} {
		w := httptest.NewRecorder()
		a.companion(w, httptest.NewRequest("POST", "/companion", strings.NewReader(`{"action":"`+action+`"}`)))
		if w.Code != 400 {
			t.Fatal("legacy trigger accepted")
		}
	}
	if reads != 3 || writes != 0 {
		t.Fatal("reading triggered a write")
	}
	w := httptest.NewRecorder()
	a.companion(w, httptest.NewRequest("POST", "/companion", strings.NewReader(`{"action":"settings","paused":true}`)))
	if writes != 1 || w.Code != 200 {
		t.Fatal("explicit pause failed")
	}
	server.Close()
	w = httptest.NewRecorder()
	a.companion(w, httptest.NewRequest("GET", "/companion", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"offline":true`) {
		t.Fatal("offline cache missing")
	}
}
