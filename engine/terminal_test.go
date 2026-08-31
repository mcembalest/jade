package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestTerminalRunsInteractiveShellInActiveJade(t *testing.T) {
	application := testApp(t)
	server := httptest.NewServer(http.HandlerFunc(application.terminal))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/terminal?jade=."
	connection, _, err := websocket.Dial(ctx, address, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()

	if err = connection.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":91,"rows":27}`)); err != nil {
		t.Fatal(err)
	}
	if err = connection.Write(ctx, websocket.MessageBinary, []byte("printf '__JADE_TERMINAL__\\n'; pwd\n")); err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	for !strings.Contains(output.String(), "__JADE_TERMINAL__") || !strings.Contains(output.String(), application.root) {
		kind, payload, readErr := connection.Read(ctx)
		if readErr != nil {
			t.Fatalf("terminal read: %v; output=%q", readErr, output.String())
		}
		if kind == websocket.MessageBinary {
			output.Write(payload)
		}
	}
	if err = connection.Write(ctx, websocket.MessageBinary, []byte("exit\n")); err != nil {
		t.Fatal(err)
	}
}
