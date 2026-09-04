package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) > 2 {
		log.Fatal("usage: jade [path]")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	application := filepath.Join(home, "Applications", "JaDE.app")
	if _, err = os.Stat(application); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatal("JaDE.app is not installed; run uv run native/install.py from the JaDE repository")
		}
		log.Fatal(err)
	}
	arguments := []string{"-a", application}
	if len(os.Args) == 2 {
		path, pathErr := filepath.Abs(os.Args[1])
		if pathErr != nil {
			log.Fatal(pathErr)
		}
		arguments = append(arguments, path)
	}
	if err = exec.Command("/usr/bin/open", arguments...).Run(); err != nil {
		log.Fatal(err)
	}
}
