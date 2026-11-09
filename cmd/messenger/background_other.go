//go:build !darwin && !linux && !windows

package main

import (
	"os"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func startBackground(string, []string) (int, string, error) {
	return 0, "", platform.ErrUnsupported
}

func startBackgroundAt(string, []string, string) (int, string, error) {
	return 0, "", platform.ErrUnsupported
}

func runBackgroundChild(string, []string) error { return platform.ErrUnsupported }

func startDetached(string, []string, *os.File) error { return platform.ErrUnsupported }
