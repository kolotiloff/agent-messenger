//go:build linux

package main

import (
	"context"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The program's own window on Linux (T-069): the system's WebKitGTK, loaded
// when the window opens, so the program stays one file that starts without
// these libraries. When they are missing, or there is no display, the
// window falls back to a Chromium-type browser in app mode and then to the
// default browser (D-054).

// GTK must keep running on the thread that started it. The main goroutine
// stays on the program's main thread, where the window is shown.
func init() { runtime.LockOSThread() }

var webkitLibraries = []string{"libwebkit2gtk-4.1.so.0", "libwebkit2gtk-4.0.so.37"}

type gtkAPI struct {
	initCheck       func(argc, argv uintptr) bool
	setPrgname      func(name string)
	setProgramClass func(name string)
	windowNew       func(kind int32) uintptr
	setTitle        func(window uintptr, title string)
	setDefaultSize  func(window uintptr, width, height int32)
	setPosition     func(window uintptr, position int32)
	setIconName     func(name string)
	containerAdd    func(container, widget uintptr)
	showAll         func(widget uintptr)
	destroy         func(widget uintptr)
	main            func()
	mainQuit        func()
	idleAdd         func(fn, data uintptr) uint32
	signalConnect   func(instance uintptr, signal string, handler, data, notify uintptr, flags int32) uint64
	settingsDefault func() uintptr
	valueInit       func(value unsafe.Pointer, kind uintptr) unsafe.Pointer
	valueSetBool    func(value unsafe.Pointer, on bool)
	objectSet       func(object uintptr, name string, value unsafe.Pointer)
	webViewNew      func() uintptr
	loadURI         func(view uintptr, uri string)
	contentManager  func(view uintptr) uintptr
	registerHandler func(manager uintptr, name string) bool
	userScriptNew   func(source string, frames, time int32, allow, block uintptr) uintptr
	addScript       func(manager, script uintptr)
	resultValue     func(result uintptr) uintptr
	valueToBool     func(value uintptr) bool
}

// loadGTK opens GTK 3 and WebKitGTK. It reports false when either is
// missing; nothing is shown then.
func loadGTK() (*gtkAPI, bool) {
	const flags = purego.RTLD_NOW | purego.RTLD_GLOBAL
	gtk, err := purego.Dlopen("libgtk-3.so.0", flags)
	if err != nil {
		return nil, false
	}
	var webkit uintptr
	for _, name := range webkitLibraries {
		if webkit, err = purego.Dlopen(name, flags); err == nil {
			break
		}
	}
	if err != nil {
		return nil, false
	}
	g := &gtkAPI{}
	ok := true
	bind := func(fn any, lib uintptr, name string) {
		if _, err := purego.Dlsym(lib, name); err != nil {
			ok = false
			return
		}
		purego.RegisterLibFunc(fn, lib, name)
	}
	bind(&g.initCheck, gtk, "gtk_init_check")
	bind(&g.setPrgname, gtk, "g_set_prgname")
	bind(&g.setProgramClass, gtk, "gdk_set_program_class")
	bind(&g.windowNew, gtk, "gtk_window_new")
	bind(&g.setTitle, gtk, "gtk_window_set_title")
	bind(&g.setDefaultSize, gtk, "gtk_window_set_default_size")
	bind(&g.setPosition, gtk, "gtk_window_set_position")
	bind(&g.setIconName, gtk, "gtk_window_set_default_icon_name")
	bind(&g.containerAdd, gtk, "gtk_container_add")
	bind(&g.showAll, gtk, "gtk_widget_show_all")
	bind(&g.destroy, gtk, "gtk_widget_destroy")
	bind(&g.main, gtk, "gtk_main")
	bind(&g.mainQuit, gtk, "gtk_main_quit")
	bind(&g.idleAdd, gtk, "g_idle_add")
	bind(&g.signalConnect, gtk, "g_signal_connect_data")
	bind(&g.settingsDefault, gtk, "gtk_settings_get_default")
	bind(&g.valueInit, gtk, "g_value_init")
	bind(&g.valueSetBool, gtk, "g_value_set_boolean")
	bind(&g.objectSet, gtk, "g_object_set_property")
	bind(&g.webViewNew, webkit, "webkit_web_view_new")
	bind(&g.loadURI, webkit, "webkit_web_view_load_uri")
	bind(&g.contentManager, webkit, "webkit_web_view_get_user_content_manager")
	bind(&g.registerHandler, webkit, "webkit_user_content_manager_register_script_message_handler")
	bind(&g.userScriptNew, webkit, "webkit_user_script_new")
	bind(&g.addScript, webkit, "webkit_user_content_manager_add_script")
	bind(&g.resultValue, webkit, "webkit_javascript_result_get_js_value")
	bind(&g.valueToBool, webkit, "jsc_value_to_boolean")
	return g, ok
}

const (
	gtkWindowToplevel = 0
	gtkWinPosCenter   = 1
	gTypeBoolean      = 5 << 2
	// WEBKIT_USER_CONTENT_INJECT_TOP_FRAME, WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_START
	webkitTopFrame      = 1
	webkitDocumentStart = 0
)

// The page asks for a dark or light title bar the same way as on Windows
// (window.amTitleBar); here it becomes GTK's dark-theme preference.
const gtkPageScript = `window.amDesktop = "linux";
window.amTitleBar = function (dark) { window.webkit.messageHandlers.amtheme.postMessage(!!dark); };`

var (
	gtkOnce  sync.Once
	gtkState struct {
		api       *gtkAPI
		window    uintptr
		closing   bool
		destroyed uintptr
		quit      uintptr
		theme     uintptr
	}
)

// showGTKWindow shows the page in a WebKitGTK window and returns when the
// window closes or ctx ends. It reports false, having shown nothing, when
// the libraries or a display are missing.
func showGTKWindow(ctx context.Context, page windowPage) bool {
	g, ok := loadGTK()
	if !ok || !g.initCheck(0, 0) {
		return false
	}
	// Callbacks cannot be freed, and the window opens once per process.
	gtkOnce.Do(func() {
		gtkState.destroyed = purego.NewCallback(func(widget, data uintptr) uintptr {
			if !gtkState.closing {
				gtkState.api.mainQuit()
			}
			return 0
		})
		gtkState.quit = purego.NewCallback(func(data uintptr) uintptr {
			gtkState.api.mainQuit()
			return 0
		})
		gtkState.theme = purego.NewCallback(func(manager, result, data uintptr) uintptr {
			setGTKDark(gtkState.api, gtkState.api.valueToBool(gtkState.api.resultValue(result)))
			return 0
		})
	})
	gtkState.api = g
	gtkState.closing = false
	// The desktop entry (installMenuEntry) is agent-messenger.desktop with
	// StartupWMClass=AgentMessenger: the program name matches it on Wayland,
	// the class on X11, so the dock shows the entry's icon and name.
	g.setPrgname("agent-messenger")
	g.setProgramClass("AgentMessenger")
	g.setIconName("agent-messenger")
	win := g.windowNew(gtkWindowToplevel)
	gtkState.window = win
	g.setTitle(win, "Agent Messenger")
	g.setDefaultSize(win, 1280, 820)
	g.setPosition(win, gtkWinPosCenter)
	view := g.webViewNew()
	manager := g.contentManager(view)
	g.signalConnect(manager, "script-message-received::amtheme", gtkState.theme, 0, 0, 0)
	g.registerHandler(manager, "amtheme")
	g.addScript(manager, g.userScriptNew(gtkPageScript, webkitTopFrame, webkitDocumentStart, 0, 0))
	g.containerAdd(win, view)
	g.signalConnect(win, "destroy", gtkState.destroyed, 0, 0, 0)
	// The page's key goes straight to the engine, never into a command line.
	g.loadURI(view, page.address)
	g.showAll(win)
	// An update or Ctrl+C closes the window too; g_idle_add is the one call
	// that is safe from another thread.
	stop := context.AfterFunc(ctx, func() { g.idleAdd(gtkState.quit, 0) })
	defer stop()
	g.main()
	if ctx.Err() != nil {
		gtkState.closing = true
		g.destroy(win)
	}
	return true
}

// setGTKDark sets GTK's dark-theme preference, which colours the title bar.
func setGTKDark(g *gtkAPI, dark bool) {
	settings := g.settingsDefault()
	if settings == 0 {
		return
	}
	// A GValue: its type and two words of data.
	var value [3]uint64
	p := unsafe.Pointer(&value[0])
	g.valueInit(p, gTypeBoolean)
	g.valueSetBool(p, dark)
	g.objectSet(settings, "gtk-application-prefer-dark-theme", p)
}
