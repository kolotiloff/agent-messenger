//go:build !linux && !windows

package main

import "context"

// showGTKWindow exists on Linux only (T-069).
func showGTKWindow(context.Context, windowPage) bool { return false }
