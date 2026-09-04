package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mcembalest/jade/engine"
)

func main() {
	root := flag.String("root", "", "workspace directory")
	address := flag.String("address", "127.0.0.1:7333", "HTTP listen address; port 0 picks a free port")
	flag.Parse()
	if *root == "" {
		log.Fatal("-root is required")
	}
	host, _, err := net.SplitHostPort(*address)
	if err != nil {
		log.Fatal(err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		log.Fatal("-address must bind a loopback IP")
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	handler, err := engine.NewHandler(*root, port)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{
		Handler:     handler,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	// The Swift app parses this line to learn the bound port.
	fmt.Printf("JaDE engine: http://%s\n", net.JoinHostPort(host, fmt.Sprint(port)))
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case err := <-served:
		log.Fatal(err)
	case <-ctx.Done():
		// BaseContext cancellation stops in-flight child commands
		// via exec.CommandContext; Shutdown then waits for handlers to return.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}
}
