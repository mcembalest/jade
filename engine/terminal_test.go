package engine

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestTerminalOpensGhosttyInActiveWorkspace(t *testing.T) {
	application := testApp(t)
	original := openGhostty
	defer func() { openGhostty = original }()
	opened := ""
	openGhostty = func(_ context.Context, directory string) error {
		opened = directory
		return nil
	}

	form := url.Values{"jade": {"."}}
	response := postForm(application.handler(), "/terminal", "127.0.0.1:7333", "http://127.0.0.1:7333", form)
	if response.Code != http.StatusOK {
		t.Fatalf("terminal = %d: %s", response.Code, response.Body.String())
	}
	if opened != application.root {
		t.Fatalf("Ghostty directory = %q, want %q", opened, application.root)
	}
}
