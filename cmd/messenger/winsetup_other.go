//go:build !windows

package main

import (
	"errors"
	"io"
)

func selfInstall(io.Writer) bool { return false }

func runUninstall(string, io.Writer) error {
	return errors.New("uninstall is for Windows; elsewhere remove the program file")
}

func runWinInstall(io.Writer) error { return errors.New("windows only") }
