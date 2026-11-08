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
	"golang.org/x/sys/windows/registry"
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
			// The icon from rsrc_windows_*.syso (scripts/gen-icon, T-045).
			IconId: 1,
		},
	})
	if w == nil {
		showInBrowser(ctx, page, out)
		return
	}
	defer w.Destroy()
	hideOwnConsole()
	// The page draws no frame of its own here, and colours the title bar
	// like itself (D-054).
	w.Init(`window.amDesktop = "windows";`)
	hwnd := windows.HWND(uintptr(w.Window()))
	_ = w.Bind("amTitleBar", func(dark bool) { w.Dispatch(func() { colorTitleBar(hwnd, dark) }) })
	colorTitleBar(hwnd, systemDark())
	w.Navigate(page.address)
	// An update or Ctrl+C closes the window too. Terminate posts WM_QUIT to
	// the calling thread's queue, so it must run on the window's thread:
	// called from another goroutine it was lost and the window stayed open
	// after «Обновиться» (T-045).
	stop := context.AfterFunc(ctx, func() { w.Dispatch(w.Terminate) })
	defer stop()
	w.Run()
}

var (
	dwmapi                = windows.NewLazySystemDLL("dwmapi.dll")
	dwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
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

// Window attributes of the title bar. Windows 10 knows only dark mode;
// Windows 11 also takes the exact colours.
const (
	dwmUseDarkMode  = 20
	dwmCaptionColor = 35
	dwmTextColor    = 36
)

// colorTitleBar gives the title bar the page's background and text colours
// (--am-bg and --am-text of the window's theme).
func colorTitleBar(hwnd windows.HWND, dark bool) {
	set := func(attr uint32, value uint32) {
		_, _, _ = dwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(attr), uintptr(unsafe.Pointer(&value)), 4)
	}
	// COLORREF is 0x00BBGGRR.
	caption, text, mode := uint32(0x00FFFFFF), uint32(0x002B2420), uint32(0)
	if dark {
		caption, text, mode = 0x00191919, 0x00EFEAE8, 1
	}
	set(dwmUseDarkMode, mode)
	set(dwmCaptionColor, caption)
	set(dwmTextColor, text)
}

// systemDark reports whether Windows apps use the dark theme.
func systemDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}
