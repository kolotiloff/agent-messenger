//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// The window without the system title bar (T-062): the page runs to the top
// edge and draws minimise, maximise and close itself. The left, right and
// bottom borders stay the system's own, so the window still resizes, snaps
// and keeps its shadow; only the caption is taken away.

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	setWindowLongPtr     = user32.NewProc("SetWindowLongPtrW")
	callWindowProc       = user32.NewProc("CallWindowProcW")
	setWindowPos         = user32.NewProc("SetWindowPos")
	getSystemMetrics     = user32.NewProc("GetSystemMetrics")
	isZoomed             = user32.NewProc("IsZoomed")
	showWindowProc       = user32.NewProc("ShowWindow")
	releaseCapture       = user32.NewProc("ReleaseCapture")
	postMessage          = user32.NewProc("PostMessageW")
	framelessOldProc     uintptr
	framelessProcPointer = windows.NewCallback(framelessProc)
)

const (
	gwlpWndProc       = ^uintptr(3) // -4
	wmNCCalcSize      = 0x0083
	wmNCLButtonDown   = 0x00A1
	wmSysCommand      = 0x0112
	wmClose           = 0x0010
	htCaption         = 2
	scMinimize        = 0xF020
	scMaximize        = 0xF030
	scRestore         = 0xF120
	smCyFrame         = 33
	smCxPaddedBorder  = 92
	swpNoSize         = 0x0001
	swpNoMove         = 0x0002
	swpNoZOrder       = 0x0004
	swpFrameChanged   = 0x0020
	swpNoActivate     = 0x0010
	swShowMaximized   = 3
	swRestore         = 9
	nccalcsizeParamsW = 1
)

type rect struct{ left, top, right, bottom int32 }

// framelessProc keeps the system frame on three sides and gives the caption
// to the page. A maximised window would hang its top over the screen edge by
// the frame's width, so then the page starts that much lower.
func framelessProc(hwnd, msg, wp, lp uintptr) uintptr {
	if msg == wmNCCalcSize && wp == nccalcsizeParamsW {
		// lp points at memory Windows owns, not at a Go object.
		r := *(**rect)(unsafe.Pointer(&lp))
		top := r.top
		ret, _, _ := callWindowProc.Call(framelessOldProc, hwnd, msg, wp, lp)
		r.top = top
		if zoomed, _, _ := isZoomed.Call(hwnd); zoomed != 0 {
			frame, _, _ := getSystemMetrics.Call(smCyFrame)
			padding, _, _ := getSystemMetrics.Call(smCxPaddedBorder)
			r.top += int32(frame + padding)
		}
		return ret
	}
	ret, _, _ := callWindowProc.Call(framelessOldProc, hwnd, msg, wp, lp)
	return ret
}

// makeFrameless replaces the caption with the page's own controls. It must
// run on the window's thread, once.
func makeFrameless(hwnd windows.HWND) {
	old, _, _ := setWindowLongPtr.Call(uintptr(hwnd), gwlpWndProc, framelessProcPointer)
	if old == 0 {
		return
	}
	framelessOldProc = old
	_, _, _ = setWindowPos.Call(uintptr(hwnd), 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)
}

// windowAction is what the page's controls ask for: minimise, maximise or
// restore, close, or drag the window by an empty part of its top.
func windowAction(hwnd windows.HWND, action string) {
	h := uintptr(hwnd)
	switch action {
	case "minimize":
		_, _, _ = postMessage.Call(h, wmSysCommand, scMinimize, 0)
	case "maximize":
		if zoomed, _, _ := isZoomed.Call(h); zoomed != 0 {
			_, _, _ = showWindowProc.Call(h, swRestore)
		} else {
			_, _, _ = showWindowProc.Call(h, swShowMaximized)
		}
	case "close":
		_, _, _ = postMessage.Call(h, wmClose, 0, 0)
	case "drag":
		_, _, _ = releaseCapture.Call()
		_, _, _ = postMessage.Call(h, wmNCLButtonDown, htCaption, 0)
	}
}
