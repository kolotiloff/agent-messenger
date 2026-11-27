//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// T-184: the icon in the notification area. Closing the window hides it
// there instead of closing it; minimising still minimises. A click on the
// icon opens the window again, its menu opens it or quits. Quitting closes
// only the window, as closing it did before: the background client and its
// tasks keep running. The window's own procedure (framelessProc) passes the
// messages here.

var (
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	shellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	loadImage           = user32.NewProc("LoadImageW")
	createPopupMenu     = user32.NewProc("CreatePopupMenu")
	appendMenu          = user32.NewProc("AppendMenuW")
	trackPopupMenu      = user32.NewProc("TrackPopupMenu")
	destroyMenu         = user32.NewProc("DestroyMenu")
	getCursorPos        = user32.NewProc("GetCursorPos")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")
	destroyWindow       = user32.NewProc("DestroyWindow")
	isIconic            = user32.NewProc("IsIconic")
	findWindow          = user32.NewProc("FindWindowW")
	registerWindowMsg   = user32.NewProc("RegisterWindowMessageW")
	trayShown           bool
	trayQuitting        bool
	trayData            notifyIconData
	trayTaskbarCreated  uintptr
)

const (
	wmNull          = 0x0000
	wmContextMenu   = 0x007B
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	// Not WM_APP itself: the window's message loop takes that one for its
	// dispatch queue.
	wmTray         = 0x8000 + 1
	nimAdd         = 0
	nimDelete      = 2
	nifMessage     = 0x1
	nifIcon        = 0x2
	nifTip         = 0x4
	imageIcon      = 1
	lrShared       = 0x8000
	smCxSmIcon     = 49
	smCySmIcon     = 50
	mfString       = 0x0
	mfSeparator    = 0x800
	tpmRightButton = 0x2
	tpmReturnCmd   = 0x100
	swHide         = 0
	swShow         = 5
	trayOpen       = 1
	trayQuit       = 2
)

// notifyIconData is NOTIFYICONDATAW; Go lays it out as C does.
type notifyIconData struct {
	Size            uint32
	Wnd             windows.HWND
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            windows.Handle
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUIDItem        windows.GUID
	BalloonIcon     windows.Handle
}

type point struct{ x, y int32 }

// windowClass and windowTitle name the window go-webview2 creates.
const (
	windowClass = "webview"
	windowTitle = "Kotel"
)

// showOpenWindow brings forward the window of a copy of the program that
// already runs, perhaps hidden in the notification area. It reports false
// when there is none.
func showOpenWindow() bool {
	class, _ := windows.UTF16PtrFromString(windowClass)
	// The previous window can remain in the tray while its client updates.
	for _, name := range []string{windowTitle, "Agent Messenger"} {
		title, _ := windows.UTF16PtrFromString(name)
		hwnd, _, _ := findWindow.Call(uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)))
		if hwnd != 0 {
			trayShowWindow(hwnd)
			return true
		}
	}
	return false
}

// addTray puts the program's icon in the notification area. It must run on
// the window's thread, after makeFrameless.
func addTray(hwnd windows.HWND) {
	if framelessOldProc == 0 {
		return // the messages would not reach trayProc
	}
	if trayTaskbarCreated == 0 {
		name, _ := windows.UTF16PtrFromString("TaskbarCreated")
		trayTaskbarCreated, _, _ = registerWindowMsg.Call(uintptr(unsafe.Pointer(name)))
	}
	var module windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &module)
	cx, _, _ := getSystemMetrics.Call(smCxSmIcon)
	cy, _, _ := getSystemMetrics.Call(smCySmIcon)
	// The icon from rsrc_windows_*.syso, as the window's (T-045).
	icon, _, _ := loadImage.Call(uintptr(module), 1, imageIcon, cx, cy, lrShared)
	trayData = notifyIconData{Wnd: hwnd, ID: 1, Flags: nifMessage | nifIcon | nifTip, CallbackMessage: wmTray, Icon: windows.Handle(icon)}
	trayData.Size = uint32(unsafe.Sizeof(trayData))
	tip, _ := windows.UTF16FromString(windowTitle)
	copy(trayData.Tip[:len(trayData.Tip)-1], tip)
	ok, _, _ := shellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&trayData)))
	trayShown = ok != 0
}

// removeTray takes the icon away when the window ends.
func removeTray() {
	if trayShown {
		_, _, _ = shellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&trayData)))
		trayShown = false
	}
}

// closeWindowNow destroys the window on its own thread before the program
// ends or opens again after an update, so the next copy does not find it.
func closeWindowNow(hwnd windows.HWND) {
	trayQuitting = true
	removeTray()
	_, _, _ = destroyWindow.Call(uintptr(hwnd))
}

// trayProc handles the notification area's messages and the close of the
// window. handled is false for every other message.
func trayProc(hwnd, msg, wp, lp uintptr) (result uintptr, handled bool) {
	switch {
	case msg == wmClose && trayShown && !trayQuitting:
		_, _, _ = showWindowProc.Call(hwnd, swHide)
		return 0, true
	case msg == wmTray:
		switch lp & 0xFFFF {
		case wmLButtonUp, wmLButtonDblClk:
			trayShowWindow(hwnd)
		case wmRButtonUp, wmContextMenu:
			trayMenu(hwnd)
		}
		return 0, true
	case trayTaskbarCreated != 0 && msg == trayTaskbarCreated && trayShown:
		// Explorer restarted and lost the icon.
		_, _, _ = shellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&trayData)))
	}
	return 0, false
}

func trayShowWindow(hwnd uintptr) {
	_, _, _ = showWindowProc.Call(hwnd, swShow)
	if iconic, _, _ := isIconic.Call(hwnd); iconic != 0 {
		_, _, _ = showWindowProc.Call(hwnd, swRestore)
	}
	_, _, _ = setForegroundWindow.Call(hwnd)
}

func trayMenu(hwnd uintptr) {
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	open, _ := windows.UTF16PtrFromString("Открыть окно")
	quit, _ := windows.UTF16PtrFromString("Выйти")
	_, _, _ = appendMenu.Call(menu, mfString, trayOpen, uintptr(unsafe.Pointer(open)))
	_, _, _ = appendMenu.Call(menu, mfSeparator, 0, 0)
	_, _, _ = appendMenu.Call(menu, mfString, trayQuit, uintptr(unsafe.Pointer(quit)))
	var p point
	_, _, _ = getCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	// The menu closes on a click elsewhere only when the window is in front.
	_, _, _ = setForegroundWindow.Call(hwnd)
	choice, _, _ := trackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(p.x), uintptr(p.y), 0, hwnd, 0)
	_, _, _ = postMessage.Call(hwnd, wmNull, 0, 0)
	switch choice {
	case trayOpen:
		trayShowWindow(hwnd)
	case trayQuit:
		// Destroying the window ends its loop (go-webview2 terminates on
		// WM_DESTROY); the client keeps running.
		trayQuitting = true
		removeTray()
		_, _, _ = destroyWindow.Call(hwnd)
	}
}
