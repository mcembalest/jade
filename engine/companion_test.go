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
func TestCompanionHistoryAndSchedule(t *testing.T) {
	fakeCompanion(t)
	now := time.Date(2026, 9, 5, 19, 59, 0, 0, time.Local)
	application := testApp(t)
	handler := http.HandlerFunc(application.guard(func(w http.ResponseWriter, r *http.Request) { application.companionAt(w, r, now) }))
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
	if s.ResearchNext != 0 {
		t.Fatal("new research should be due immediately")
	}
	if s.Next != now.Add(time.Minute).UnixMilli() {
		t.Fatal("not scheduled for 8pm", s.Next)
	}
	initialNext := s.Next
	if call("").Next != s.Next {
		t.Fatal("reload reset schedule")
	}
	if len(call(`{"action":"discover"}`).Messages) != 0 {
		t.Fatal("early discovery")
	}
	s = call(`{"action":"chat","message":"hello"}`)
	if s.Next != initialNext {
		t.Fatal("chat postponed the evening update")
	}
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
	s = call(`{"action":"research"}`)
	if len(s.Pending) != 1 || len(s.Messages) != 2 {
		t.Fatal("research did not stay pending", s)
	}
	now = now.Add(time.Minute)
	s = call(`{"action":"discover"}`)
	if s.DailyDate != "2026-09-05" || s.Next != now.AddDate(0, 0, 1).UnixMilli() {
		t.Fatal("daily limit not saved", s)
	}
	if len(s.Messages) != 3 || !s.Messages[2].Proactive {
		t.Fatal(s)
	}
	if len(call(`{"action":"discover"}`).Messages) != 3 {
		t.Fatal("duplicate discovery")
	}
	// Chat, reloads, stale deadlines and hide/restore cannot trigger a second update.
	call(`{"action":"enabled","enabled":false}`)
	call(`{"action":"enabled","enabled":true}`)
	now = now.Add(2 * time.Hour)
	if len(call(`{"action":"discover"}`).Messages) != 3 {
		t.Fatal("same-day update after restore")
	}
	if call("").DailyDate != "2026-09-05" {
		t.Fatal("reload lost daily limit")
	}
	now = now.AddDate(0, 0, 3)
	call(`{"action":"research"}`)
	if len(call(`{"action":"discover"}`).Messages) != 4 {
		t.Fatal("missed evenings did not resume")
	}
	if len(call(`{"action":"discover"}`).Messages) != 4 {
		t.Fatal("missed evenings accumulated")
	}
	t.Setenv("JADE_FAKE_MODE", "bad")
	req := httptest.NewRequest("POST", "http://127.0.0.1:7333/companion", strings.NewReader(`{"action":"chat","message":"fail"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 503 || len(call("").Messages) != 4 {
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
	answer, sources, err = runCompanion(ctx, nil, "Pending findings: []", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Research:", answer, sources)
	if answer != "" && len(sources) == 0 {
		t.Fatal("unsourced research")
	}
}

func TestCompanionEveningSchedule(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	parse := func(value string) time.Time {
		result, err := time.ParseInLocation("2006-01-02 15:04", value, zone)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	for _, tc := range []struct{ name, now, delivered, retry, want string }{
		{"before evening", "2026-09-05 19:59", "", "", "2026-09-05 20:00"},
		{"at eight", "2026-09-05 20:00", "", "", "2026-09-05 20:00"},
		{"late opening", "2026-09-05 22:00", "", "", "2026-09-05 20:00"},
		{"already delivered", "2026-09-05 21:00", "2026-09-05", "", "2026-09-06 20:00"},
		{"missed days", "2026-09-09 21:00", "2026-09-05", "", "2026-09-09 20:00"},
		{"research does not postpone delivery", "2026-09-05 20:30", "", "2026-09-05 21:00", "2026-09-05 20:00"},
		{"no morning catchup", "2026-09-06 00:20", "", "2026-09-06 00:30", "2026-09-06 20:00"},
		{"spring forward", "2026-03-07 21:00", "2026-03-07", "", "2026-03-08 20:00"},
		{"fall back", "2026-10-31 21:00", "2026-10-31", "", "2026-11-01 20:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := companionState{DailyDate: tc.delivered, Next: 1}
			if tc.retry != "" {
				s.ResearchNext = parse(tc.retry).UnixMilli()
			}
			s.schedule(parse(tc.now))
			if s.Next != parse(tc.want).UnixMilli() {
				t.Fatalf("got %v want %s", time.UnixMilli(s.Next).In(zone), tc.want)
			}
		})
	}
}

func TestCompanionResearchAndDelivery(t *testing.T) {
	fakeCompanion(t)
	application := testApp(t)
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.Local)
	call := func(action string) companionState {
		t.Helper()
		req := httptest.NewRequest("POST", "http://127.0.0.1:7333/companion", strings.NewReader(`{"action":"`+action+`"}`))
		if action == "" {
			req = httptest.NewRequest("GET", "http://127.0.0.1:7333/companion", nil)
		}
		rec := httptest.NewRecorder()
		application.companionAt(rec, req, now)
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		var s companionState
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	t.Setenv("JADE_FAKE_MODE", "bad")
	s := call("research")
	if s.ResearchError == "" || s.ResearchNext != now.Add(time.Hour).UnixMilli() {
		t.Fatal("failure not saved/throttled", s)
	}
	now = now.Add(59 * time.Minute)
	t.Setenv("JADE_FAKE_MODE", "normal")
	if len(call("research").Pending) != 0 {
		t.Fatal("early retry")
	}
	now = now.Add(time.Minute)
	t.Setenv("JADE_FAKE_MODE", "quiet")
	s = call("research")
	if s.ResearchNext != now.Add(time.Hour).UnixMilli() || s.ResearchError != "" || len(s.Pending) != 0 {
		t.Fatal("quiet retry", s)
	}
	now = now.Add(time.Hour)
	t.Setenv("JADE_FAKE_MODE", "normal")
	s = call("research")
	if len(s.Pending) != 1 || len(s.Messages) != 0 || s.DailyDate != "" {
		t.Fatal("daytime research sent a message", s)
	}
	if len(call("").Pending) != 1 {
		t.Fatal("pending research lost on reload")
	}
	if len(call("discover").Messages) != 0 {
		t.Fatal("delivered before 8pm")
	}
	now = now.Add(time.Hour)
	if len(call("research").Pending) != 1 {
		t.Fatal("duplicate source added")
	}
	now = time.Date(2026, 9, 5, 20, 0, 0, 0, time.Local)
	t.Setenv("JADE_FAKE_MODE", "bad") // Daily delivery must not invoke the model.
	s = call("discover")
	if len(s.Pending) != 0 || len(s.Messages) != 1 || len(s.Messages[0].Sources) != 1 || s.DailyDate != "2026-09-05" {
		t.Fatal("daily digest", s)
	}
	t.Setenv("JADE_FAKE_MODE", "normal")
	s = call("research")
	if len(s.Pending) != 1 || len(s.Messages) != 1 {
		t.Fatal("research did not continue after daily delivery", s)
	}
	if len(call("discover").Messages) != 1 {
		t.Fatal("extra daily bubble")
	}
	now = now.AddDate(0, 0, 1)
	s = call("discover")
	if len(s.Pending) != 0 || len(s.Messages) != 2 {
		t.Fatal("next evening", s)
	}
	// A full queue pauses spending without dropping findings.
	config, _ := os.UserConfigDir()
	path := filepath.Join(config, "JaDE", "companion", "chat.json")
	s.Pending = make([]companionMessage, 24)
	for i := range s.Pending {
		s.Pending[i] = companionMessage{Text: fmt.Sprint(i), FoundAt: now.UnixMilli()}
	}
	s.ResearchNext = 0
	raw, _ := json.Marshal(s)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	s = call("research")
	if len(s.Pending) != 24 || s.ResearchNext != 0 {
		t.Fatal("full queue changed", s)
	}
}
