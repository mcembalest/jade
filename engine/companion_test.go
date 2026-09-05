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

	"github.com/gofrs/flock"
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
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
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
func TestCompanionHistoryAndSchedule(t *testing.T) {
	fakeCompanion(t)
	handler := testApp(t).handler()
	call := func(body string) companionState {
		t.Helper()
		method := "POST"
		if body == "" {
			method = "GET"
		}
		req := httptest.NewRequest(method, "http://127.0.0.1:7333/companion", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		var s companionState
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := call("")
	if delay := time.Until(time.UnixMilli(s.Next)); delay < 20*time.Minute-time.Second || delay > 60*time.Minute {
		t.Fatal(delay)
	}
	if call("").Next != s.Next {
		t.Fatal("reload reset schedule")
	}
	if len(call(`{"action":"discover"}`).Messages) != 0 {
		t.Fatal("early discovery")
	}
	s = call(`{"action":"chat","message":"hello"}`)
	if len(s.Messages) != 2 || s.Messages[0].Role != "user" || s.Messages[1].Text != "A new discovery." {
		t.Fatal(s)
	}
	s = call(fmt.Sprintf(`{"action":"seen","seen":%q}`, s.Messages[1].ID))
	if s.Seen != s.Messages[1].ID {
		t.Fatal("seen not saved")
	}
	s = call(`{"action":"enabled","enabled":false}`)
	if s.Enabled || call("").Enabled {
		t.Fatal("hide not saved")
	}
	if len(call(`{"action":"discover"}`).Messages) != 2 {
		t.Fatal("hidden companion ran")
	}
	call(`{"action":"enabled","enabled":true}`)
	config, _ := os.UserConfigDir()
	path := filepath.Join(config, "JaDE", "companion", "chat.json")
	raw, _ := os.ReadFile(path)
	_ = json.Unmarshal(raw, &s)
	s.Next = 1
	raw, _ = json.Marshal(s)
	_ = os.WriteFile(path, raw, 0600)
	s = call(`{"action":"discover"}`)
	if len(s.Messages) != 3 || !s.Messages[2].Proactive {
		t.Fatal(s)
	}
	if len(call(`{"action":"discover"}`).Messages) != 3 {
		t.Fatal("duplicate discovery")
	}
	t.Setenv("JADE_FAKE_MODE", "bad")
	req := httptest.NewRequest("POST", "http://127.0.0.1:7333/companion", strings.NewReader(`{"action":"chat","message":"fail"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 503 || len(call("").Messages) != 3 {
		t.Fatal("failed reply damaged history")
	}
	for _, body := range []string{`{"action":"chat","message":""}`, `{"action":"unknown"}`, `not json`} {
		req := httptest.NewRequest("POST", "http://127.0.0.1:7333/companion", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Fatal(rec.Code)
		}
	}
	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7333/companion", strings.NewReader(`{"action":"chat","message":"hello"}`))
	req.Header.Set("Origin", "https://example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatal(rec.Code)
	}
}

func TestCompanionConcurrentWindows(t *testing.T) {
	fakeCompanion(t)
	t.Setenv("JADE_FAKE_MODE", "wait")
	handler := testApp(t).handler()
	call := func(body string) *httptest.ResponseRecorder {
		method := "POST"
		if body == "" {
			method = "GET"
		}
		req := httptest.NewRequest(method, "http://127.0.0.1:7333/companion", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	call("")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("POST", "http://127.0.0.1:7333/companion", strings.NewReader(`{"action":"chat","message":"wait"}`)).WithContext(ctx)
	done := make(chan struct{})
	go func() { defer close(done); handler.ServeHTTP(httptest.NewRecorder(), req) }()
	config, _ := os.UserConfigDir()
	running := flock.New(filepath.Join(config, "JaDE", "companion", ".running"))
	defer running.Close()
	deadline := time.Now().Add(3 * time.Second)
	for {
		acquired, err := running.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		if !acquired {
			break
		}
		_ = running.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("request never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := call(`{"action":"chat","message":"duplicate"}`).Code; got != 409 {
		t.Fatal(got)
	}
	if got := call("").Code; got != 200 {
		t.Fatal("history blocked", got)
	}
	if got := call(`{"action":"enabled","enabled":false}`).Code; got != 200 {
		t.Fatal("hide blocked", got)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not cancel")
	}
	var state companionState
	_ = json.Unmarshal(call("").Body.Bytes(), &state)
	if state.Enabled || len(state.Messages) != 0 {
		t.Fatal("cancelled request changed state")
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
}
