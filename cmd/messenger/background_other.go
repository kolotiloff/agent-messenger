//go:build !darwin && !linux

package main

import "github.com/kolotiloff/agent-messenger/internal/platform"

func startBackground(string, []string) (int, string, error) {
	return 0, "", platform.ErrUnsupported
}

func runBackgroundChild(string, []string) error { return platform.ErrUnsupported }
