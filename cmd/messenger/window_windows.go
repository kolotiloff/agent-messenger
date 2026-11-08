//go:build windows

package main

import (
	"context"
	"io"
	"path/filepath"
	"runtime"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

// showWindow shows the page in the program's own window on Windows (D-054):
// the WebView2 engine Windows ships with, not a browser tab. The page's key
// goes straight to the engine and never appears in a command line. When the
// engine is missing, the page opens in the default browser as before.
func showWindow(ctx context.Context, cancel context.CancelFunc, home string, page windowPage, out io.Writer) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  filepath.Join(home, "window"),
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title: "Agent Messenger", Width: 1280, Height: 820, Center: true,
		},
	})
	if w == nil {
		showInBrowser(ctx, page, out)
		return
	}
	defer w.Destroy()
	hideOwnConsole()
	w.Navigate(page.address)
	// An update or Ctrl+C closes the window too.
	stop := context.AfterFunc(ctx, w.Terminate)
	defer stop()
	w.Run()
}

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	freeConsole           = kernel32.NewProc("FreeConsole")
)

// hideOwnConsole closes the console Windows opened for a double click. A
// console shared with another program (a terminal the owner typed in) is
// kept.
func hideOwnConsole() {
	var list [2]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&list[0])), 2)
	if n == 1 {
		_, _, _ = freeConsole.Call()
	}
}
