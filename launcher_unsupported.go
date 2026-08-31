//go:build !darwin

package main

import "errors"

func run(_ []string) error {
	return errors.New("JaDE menu-bar launcher requires macOS")
}
