//go:build linux

package main

import (
	"context"
	"testing"
)

// Without WebKitGTK the window reports false at once and shows nothing, so
// the Chromium and browser fallbacks run (T-069).
func TestGTKWindowWithoutWebKit(t *testing.T) {
	saved := webkitLibraries
	webkitLibraries = []string{"libwebkit-missing-for-test.so.0"}
	defer func() { webkitLibraries = saved }()
	if showGTKWindow(context.Background(), windowPage{address: "http://127.0.0.1:1/"}) {
		t.Fatal("a window without WebKitGTK")
	}
}
