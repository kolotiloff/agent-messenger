//go:build !linux && !windows

package main

import "context"

// showGTKWindow exists on Linux only (T-069).
func showGTKWindow(context.Context, string, windowPage) bool { return false }
