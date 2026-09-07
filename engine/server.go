package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Serve binds a loopback listener before reporting readiness. A zero port lets
// the OS choose a free port. The jade command uses this for browser and headless runs.
func Serve(ctx context.Context, root, address string, ready func(string)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	registry := newProjectRegistry(ctx)
	err := serveProject(ctx, root, address, func(url string) {
		actual, err := ResolveWorkspaceRoot(root)
		if err == nil {
			registry.mu.Lock()
			registry.urls[actual] = url
			registry.mu.Unlock()
		}
		if ready != nil {
			ready(url)
		}
	}, registry, true)
	cancel()
	// Opening holds this lock while registering a child; after cancellation no
	// new child can be added, so waiting also drains in-flight child HTTP work.
	registry.mu.Lock()
	registry.mu.Unlock()
	registry.children.Wait()
	return err
}

// Child desktop projects reuse HTTP behavior without enabling Notes sync.
func serveProject(ctx context.Context, root, address string, ready func(string), registry *projectRegistry, enableSync bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return errors.New("address must bind a loopback IP")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()
	application, err := newApp(root, listener.Addr().(*net.TCPAddr).Port)
	if err != nil {
		return err
	}
	application.projects = registry
	if enableSync {
		application.syncer, err = openWorkspaceSync(application.root)
		if err != nil {
			return err
		}
		if application.syncer != nil {
			go application.syncer.run(ctx)
		}
	}
	handler := application.handler()
	server := &http.Server{Handler: handler, BaseContext: func(net.Listener) context.Context { return ctx }, ReadHeaderTimeout: 5 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	url := "http://" + net.JoinHostPort(host, fmt.Sprint(listener.Addr().(*net.TCPAddr).Port))
	if ready != nil {
		ready(url)
	}
	select {
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}
