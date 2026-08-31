//go:build darwin

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"fyne.io/systray"
	"github.com/mcembalest/jade/engine"
)

var jadeAddress = "127.0.0.1:7333"

func run(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: jade [path]")
	}
	start := "."
	if len(args) == 1 {
		start = args[0]
	}

	root, err := engine.FindJadeRoot(start)
	if err != nil {
		return err
	}
	_, portText, err := net.SplitHostPort(jadeAddress)
	if err != nil {
		return fmt.Errorf("invalid JaDE address: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return fmt.Errorf("invalid JaDE port: %w", err)
	}
	jadeURL := "http://" + jadeAddress
	handler, err := engine.NewHandler(root, port)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", jadeAddress)
	if err != nil {
		return fmt.Errorf("start JaDE: %w", err)
	}
	server := &http.Server{Handler: handler}
	serveResult := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveResult <- err
		if err != nil {
			systray.Quit()
		}
	}()

	log.Printf("JaDE: %s", jadeURL)
	log.Printf("Root: %s", root)
	systray.Run(func() {
		systray.SetTitle("🐉")
		systray.SetTooltip("JaDE — " + filepath.Base(root))
		openItem := systray.AddMenuItem("Open JaDE", jadeURL)
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit JaDE", "Stop this JaDE")
		go func() {
			if err := openBrowser(jadeURL); err != nil {
				log.Printf("open browser: %v", err)
			}
			for {
				select {
				case <-openItem.ClickedCh:
					if err := openBrowser(jadeURL); err != nil {
						log.Printf("open browser: %v", err)
					}
				case <-quitItem.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, nil)

	shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("stop JaDE: %w", err)
	}
	if err := <-serveResult; err != nil {
		return fmt.Errorf("serve JaDE: %w", err)
	}
	return nil
}

func openBrowser(url string) error {
	return exec.Command("open", url).Run()
}
