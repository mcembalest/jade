package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mcembalest/jade/engine"
)

func main() {
	root := flag.String("root", "", "workspace directory")
	address := flag.String("address", "127.0.0.1:0", "HTTP loopback listen address; port 0 picks a free port")
	flag.Parse()
	if *root == "" {
		log.Fatal("-root is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := engine.Serve(ctx, *root, *address, func(url string) {
		// The native shell parses this readiness line.
		fmt.Println("JaDE engine: " + url)
	}); err != nil {
		log.Fatal(err)
	}
}
