package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/mcembalest/jade/engine"
)

func main() {
	noOpen := flag.Bool("no-open", false, "serve without opening a browser")
	address := flag.String("address", "127.0.0.1:0", "HTTP loopback listen address")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: jade [options] [folder or file]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	path := "."
	if flag.NArg() == 1 {
		path = flag.Arg(0)
	}
	root, err := engine.ResolveWorkspaceRoot(path)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = engine.Serve(ctx, root, *address, func(url string) {
		url = launchURL(url, path)
		fmt.Println("JaDE: " + url)
		fmt.Println("Press Ctrl+C to stop JaDE after saving your work.")
		if !*noOpen {
			go func() {
				if err := openBrowser(ctx, url); err != nil {
					fmt.Fprintln(os.Stderr, "Could not open a browser; open the URL above:", err)
				}
			}()
		}
	})
	if err != nil {
		log.Fatal(err)
	}
}

func openBrowser(ctx context.Context, url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "/usr/bin/open", url).Run()
	case "windows":
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url).Run()
	default:
		return exec.CommandContext(ctx, "xdg-open", url).Run()
	}
}

// An explicit file argument overrides any remembered selection for its folder.
func launchURL(base, path string) string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return base
	}
	target, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := target.Query()
	query.Set("file", filepath.Base(path))
	target.RawQuery = query.Encode()
	return target.String()
}
