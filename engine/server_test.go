package engine

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestServeChoosesPortAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, t.TempDir(), "127.0.0.1:0", func(url string) { ready <- url }) }()
	var url string
	select {
	case url = <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("no readiness")
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestServeRejectsPublicListener(t *testing.T) {
	if err := Serve(context.Background(), t.TempDir(), "0.0.0.0:0", nil); err == nil {
		t.Fatal("allowed a public listener")
	}
}
