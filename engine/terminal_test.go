package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func isolateTerminals(t *testing.T) string {
	t.Helper()
	t.Setenv("JADE_TERMINAL", "")
	root := t.TempDir()
	previousRoots, previousPreference, previousLaunch := terminalRoots, terminalPreferencePath, launchTerminal
	terminalRoots = func() []string { return []string{root} }
	terminalPreferencePath = func() (string, error) { return filepath.Join(root, "config", "terminal"), nil }
	t.Cleanup(func() {
		terminalRoots, terminalPreferencePath, launchTerminal = previousRoots, previousPreference, previousLaunch
	})
	return root
}

func TestTerminalDiscoveryAndPreference(t *testing.T) {
	root := isolateTerminals(t)
	if state := availableTerminals(); len(state.Apps) != 1 || state.Selected != systemTerminal {
		t.Fatalf("fallback = %+v", state)
	}
	ghostty := filepath.Join(root, "Ghostty.app")
	if err := os.Mkdir(ghostty, 0700); err != nil {
		t.Fatal(err)
	}
	state := availableTerminals()
	if len(state.Apps) != 2 || state.Selected != ghostty {
		t.Fatalf("discovery = %+v", state)
	}
	application := testApp(t)
	response := postForm(application.handler(), "/terminal/preference", "127.0.0.1:7333", "http://127.0.0.1:7333", url.Values{"terminal": {systemTerminal}})
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	// A new app instance reads the saved preference, independent of its workspace or port.
	request := httptest.NewRequest(http.MethodGet, "/terminals", nil)
	request.Host = "127.0.0.1:7333"
	recorder := httptest.NewRecorder()
	testApp(t).handler().ServeHTTP(recorder, request)
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.Selected != systemTerminal {
		t.Fatalf("saved choice = %+v", state)
	}
	t.Setenv("JADE_TERMINAL", "Ghostty")
	if state = availableTerminals(); state.Selected != ghostty || !state.Overridden {
		t.Fatalf("override = %+v", state)
	}
	t.Setenv("JADE_TERMINAL", "/custom/My Terminal.app")
	if state = availableTerminals(); state.Selected != "/custom/My Terminal.app" || len(state.Apps) != 3 {
		t.Fatalf("path override = %+v", state)
	}
	response = postForm(application.handler(), "/terminal/preference", "127.0.0.1:7333", "http://127.0.0.1:7333", url.Values{"terminal": {"sh -c bad"}})
	if response.Code != http.StatusBadRequest {
		t.Fatal("accepted an unlisted preference")
	}
}

func TestTerminalLaunchAndFallback(t *testing.T) {
	isolateTerminals(t)
	application := testApp(t)
	for _, tc := range []struct {
		name, jade, override string
		fail                 bool
		status               int
		apps                 []string
	}{
		{"root", ".", "", false, 200, []string{systemTerminal}},
		{"nested", "inner", "Ghostty", false, 200, []string{"Ghostty"}},
		{"missing override", "inner", "/missing/Ghostty.app", true, 200, []string{"/missing/Ghostty.app", systemTerminal}},
		{"traversal", "../outside", "", false, 400, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JADE_TERMINAL", tc.override)
			var apps []string
			launchTerminal = func(_ context.Context, app, directory string) error {
				apps = append(apps, app)
				expected := application.root
				if tc.jade == "inner" {
					expected = filepath.Join(expected, "inner")
				}
				if directory != expected {
					t.Fatalf("directory = %q, want %q", directory, expected)
				}
				if tc.fail && app != systemTerminal {
					return errors.New("unavailable")
				}
				return nil
			}
			response := postForm(application.handler(), "/terminal", "127.0.0.1:7333", "http://127.0.0.1:7333", url.Values{"jade": {tc.jade}})
			if response.Code != tc.status || !reflect.DeepEqual(apps, tc.apps) {
				t.Fatalf("response=%d %s, apps=%v", response.Code, response.Body.String(), apps)
			}
		})
	}
}

func TestTerminalCancellationDoesNotLaunchFallback(t *testing.T) {
	isolateTerminals(t)
	t.Setenv("JADE_TERMINAL", "Ghostty")
	calls := 0
	launchTerminal = func(ctx context.Context, _, _ string) error { calls++; return ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/terminal?jade=.", nil).WithContext(ctx)
	request.Host = "127.0.0.1:7333"
	response := httptest.NewRecorder()
	testApp(t).handler().ServeHTTP(response, request)
	if calls != 1 || response.Code != 400 {
		t.Fatalf("calls=%d status=%d", calls, response.Code)
	}
}

func TestTerminalArgumentsKeepPathsLiteral(t *testing.T) {
	directory := "/tmp/space ' quote ; $(touch nope)"
	for _, app := range []string{systemTerminal, "/Applications/Ghostty.app"} {
		args := terminalArguments(app, directory)
		expected := directory
		if app != systemTerminal {
			expected = "--working-directory=" + directory
		}
		if args[len(args)-1] != expected {
			t.Fatalf("arguments = %q", args)
		}
	}
}
