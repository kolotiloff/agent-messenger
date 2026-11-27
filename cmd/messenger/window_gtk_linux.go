//go:build linux

package main

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
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
	valueToString   func(value uintptr) uintptr
	free            func(p uintptr)
	// The window without the system title bar (T-082).
	setTitlebar    func(window, widget uintptr)
	boxNew         func(orientation, spacing int32) uintptr
	styleContext   func(widget uintptr) uintptr
	addClass       func(context uintptr, name string)
	cssNew         func() uintptr
	cssLoad        func(provider uintptr, data string, length int, err uintptr) bool
	screenDefault  func() uintptr
	addProvider    func(screen, provider uintptr, priority uint32)
	beginMoveDrag  func(window uintptr, button, x, y int32, time uint32)
	iconify        func(window uintptr)
	maximize       func(window uintptr)
	unmaximize     func(window uintptr)
	isMaximized    func(window uintptr) bool
	closeWindow    func(window uintptr)
	displayDefault func() uintptr
	defaultSeat    func(display uintptr) uintptr
	seatPointer    func(seat uintptr) uintptr
	devicePosition func(device, screen uintptr, x, y *int32)
	// The tray icon (T-184). GtkStatusIcon shows only where the desktop
	// has a tray; without one closing the window closes it, as before.
	hide           func(widget uintptr)
	present        func(window uintptr)
	statusIconNew  func(name string) uintptr
	statusTooltip  func(icon uintptr, text string)
	statusEmbedded func(icon uintptr) bool
	statusVisible  func(icon uintptr, visible bool)
	menuNew        func() uintptr
	menuItemNew    func(label string) uintptr
	separatorNew   func() uintptr
	menuAppend     func(shell, child uintptr)
	menuPopup      func(menu, event uintptr)
	tray           bool
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
	bind(&g.valueToString, webkit, "jsc_value_to_string")
	bind(&g.free, gtk, "g_free")
	bind(&g.setTitlebar, gtk, "gtk_window_set_titlebar")
	bind(&g.boxNew, gtk, "gtk_box_new")
	bind(&g.styleContext, gtk, "gtk_widget_get_style_context")
	bind(&g.addClass, gtk, "gtk_style_context_add_class")
	bind(&g.cssNew, gtk, "gtk_css_provider_new")
	bind(&g.cssLoad, gtk, "gtk_css_provider_load_from_data")
	bind(&g.screenDefault, gtk, "gdk_screen_get_default")
	bind(&g.addProvider, gtk, "gtk_style_context_add_provider_for_screen")
	bind(&g.beginMoveDrag, gtk, "gtk_window_begin_move_drag")
	bind(&g.iconify, gtk, "gtk_window_iconify")
	bind(&g.maximize, gtk, "gtk_window_maximize")
	bind(&g.unmaximize, gtk, "gtk_window_unmaximize")
	bind(&g.isMaximized, gtk, "gtk_window_is_maximized")
	bind(&g.closeWindow, gtk, "gtk_window_close")
	bind(&g.displayDefault, gtk, "gdk_display_get_default")
	bind(&g.defaultSeat, gtk, "gdk_display_get_default_seat")
	bind(&g.seatPointer, gtk, "gdk_seat_get_pointer")
	bind(&g.devicePosition, gtk, "gdk_device_get_position")
	bind(&g.hide, gtk, "gtk_widget_hide")
	bind(&g.present, gtk, "gtk_window_present")
	// The tray is optional: without these the window works as before.
	g.tray = true
	optional := func(fn any, name string) {
		if _, err := purego.Dlsym(gtk, name); err != nil {
			g.tray = false
			return
		}
		purego.RegisterLibFunc(fn, gtk, name)
	}
	optional(&g.statusIconNew, "gtk_status_icon_new_from_icon_name")
	optional(&g.statusTooltip, "gtk_status_icon_set_tooltip_text")
	optional(&g.statusEmbedded, "gtk_status_icon_is_embedded")
	optional(&g.statusVisible, "gtk_status_icon_set_visible")
	optional(&g.menuNew, "gtk_menu_new")
	optional(&g.menuItemNew, "gtk_menu_item_new_with_label")
	optional(&g.separatorNew, "gtk_separator_menu_item_new")
	optional(&g.menuAppend, "gtk_menu_shell_append")
	optional(&g.menuPopup, "gtk_menu_popup_at_pointer")
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

// The page asks for a dark or light theme the same way as on Windows
// (window.amTitleBar); here it becomes GTK's dark-theme preference. As on
// Windows, the page draws the window buttons and drags the window by the
// empty part of its top (window.amWindow, T-082).
const gtkPageScript = `window.amDesktop = "linux"; window.amFrameless = true;
window.amTitleBar = function (dark) { window.webkit.messageHandlers.amtheme.postMessage(!!dark); };
window.amWindow = function (action) { window.webkit.messageHandlers.amwindow.postMessage(String(action)); };`

// gtkNoTitleCSS makes the empty title bar widget take no room; GTK keeps its
// own window border, so the window still resizes from its edges.
const gtkNoTitleCSS = `.am-no-title { min-height: 0; padding: 0; margin: 0; border: 0; box-shadow: none; background: none; }`

// GTK_STYLE_PROVIDER_PRIORITY_APPLICATION
const gtkStyleApplication = 600

var (
	gtkOnce  sync.Once
	gtkState struct {
		api       *gtkAPI
		window    uintptr
		closing   bool
		destroyed uintptr
		quit      uintptr
		theme     uintptr
		action    uintptr
		// T-184: the tray icon, its menu and the window's close.
		icon     uintptr
		menu     uintptr
		quitting bool
		deleted  uintptr
		show     uintptr
		activate uintptr
		popup    uintptr
		menuOpen uintptr
		menuQuit uintptr
	}
)

// showGTKWindow shows the page in a WebKitGTK window and returns when the
// window closes or ctx ends. It reports false, having shown nothing, when
// the libraries or a display are missing.
func showGTKWindow(ctx context.Context, home string, page windowPage) bool {
	g, ok := loadGTK()
	if !ok || !g.initCheck(0, 0) {
		return false
	}
	// One window: a copy already open, perhaps hidden in the tray (T-184),
	// is asked to come forward instead.
	socket := filepath.Join(home, "window.sock")
	if conn, err := net.DialTimeout("unix", socket, time.Second); err == nil {
		// T-209: a window of an older installed version gives way.
		pid, stale := staleWindowProcess(conn)
		if !stale {
			_, _ = conn.Write([]byte("show\n"))
			_ = conn.Close()
			return true
		}
		_ = conn.Close()
		if !closeStaleWindow(pid, socket) {
			if conn, err := net.DialTimeout("unix", socket, time.Second); err == nil {
				_, _ = conn.Write([]byte("show\n"))
				_ = conn.Close()
			}
			return true
		}
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
		gtkState.action = purego.NewCallback(func(manager, result, data uintptr) uintptr {
			g := gtkState.api
			raw := g.valueToString(g.resultValue(result))
			if raw == 0 {
				return 0
			}
			action := cString(raw, 16)
			g.free(raw)
			gtkWindowAction(g, gtkState.window, action)
			return 0
		})
		// Closing hides the window while the tray shows its icon.
		gtkState.deleted = purego.NewCallback(func(widget, event, data uintptr) uintptr {
			g := gtkState.api
			if gtkState.icon != 0 && !gtkState.quitting && !gtkState.closing && g.statusEmbedded(gtkState.icon) {
				g.hide(widget)
				return 1
			}
			return 0
		})
		gtkState.show = purego.NewCallback(func(data uintptr) uintptr {
			if gtkState.window != 0 {
				gtkState.api.present(gtkState.window)
			}
			return 0
		})
		gtkState.activate = purego.NewCallback(func(icon, data uintptr) uintptr {
			gtkState.api.present(gtkState.window)
			return 0
		})
		gtkState.popup = purego.NewCallback(func(icon, button, time, data uintptr) uintptr {
			gtkState.api.menuPopup(gtkState.menu, 0)
			return 0
		})
		gtkState.menuOpen = purego.NewCallback(func(item, data uintptr) uintptr {
			gtkState.api.present(gtkState.window)
			return 0
		})
		gtkState.menuQuit = purego.NewCallback(func(item, data uintptr) uintptr {
			// Only the window closes; the client keeps running.
			gtkState.quitting = true
			gtkState.api.mainQuit()
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
	g.setIconName("kotel")
	win := g.windowNew(gtkWindowToplevel)
	gtkState.window = win
	g.setTitle(win, "Kotel")
	g.setDefaultSize(win, 1280, 820)
	g.setPosition(win, gtkWinPosCenter)
	removeGTKTitleBar(g, win)
	view := g.webViewNew()
	manager := g.contentManager(view)
	g.signalConnect(manager, "script-message-received::amtheme", gtkState.theme, 0, 0, 0)
	g.registerHandler(manager, "amtheme")
	g.signalConnect(manager, "script-message-received::amwindow", gtkState.action, 0, 0, 0)
	g.registerHandler(manager, "amwindow")
	g.addScript(manager, g.userScriptNew(gtkPageScript, webkitTopFrame, webkitDocumentStart, 0, 0))
	g.containerAdd(win, view)
	g.signalConnect(win, "destroy", gtkState.destroyed, 0, 0, 0)
	g.signalConnect(win, "delete-event", gtkState.deleted, 0, 0, 0)
	gtkState.quitting = false
	if g.tray {
		addGTKTray(g)
	}
	if listener, err := listenForShow(socket, g); err == nil {
		defer listener.Close()
	}
	// The page's key goes straight to the engine, never into a command line.
	g.loadURI(view, page.address)
	g.showAll(win)
	// An update or Ctrl+C closes the window too; g_idle_add is the one call
	// that is safe from another thread.
	stop := context.AfterFunc(ctx, func() { g.idleAdd(gtkState.quit, 0) })
	defer stop()
	g.main()
	if gtkState.icon != 0 {
		g.statusVisible(gtkState.icon, false)
	}
	if ctx.Err() != nil || gtkState.quitting {
		gtkState.closing = true
		g.destroy(win)
	}
	return true
}

// addGTKTray puts the program's icon in the tray with «Открыть окно» and
// «Выйти». The icon is made once per process.
func addGTKTray(g *gtkAPI) {
	if gtkState.icon != 0 {
		g.statusVisible(gtkState.icon, true)
		return
	}
	icon := g.statusIconNew("agent-messenger")
	if icon == 0 {
		return
	}
	g.statusTooltip(icon, "Kotel")
	g.signalConnect(icon, "activate", gtkState.activate, 0, 0, 0)
	g.signalConnect(icon, "popup-menu", gtkState.popup, 0, 0, 0)
	menu := g.menuNew()
	open := g.menuItemNew("Открыть окно")
	quit := g.menuItemNew("Выйти")
	g.signalConnect(open, "activate", gtkState.menuOpen, 0, 0, 0)
	g.signalConnect(quit, "activate", gtkState.menuQuit, 0, 0, 0)
	g.menuAppend(menu, open)
	g.menuAppend(menu, g.separatorNew())
	g.menuAppend(menu, quit)
	g.showAll(menu)
	gtkState.icon, gtkState.menu = icon, menu
}

// listenForShow answers a second copy of the program: it brings this
// window forward. A socket left by a copy that ended is replaced.
func listenForShow(socket string, g *gtkAPI) (net.Listener, error) {
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			line, _ := bufio.NewReader(conn).ReadString('\n')
			_ = conn.Close()
			if line == "show\n" {
				// g_idle_add is the one call that is safe from another thread.
				g.idleAdd(gtkState.show, 0)
			}
		}
	}()
	return listener, nil
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

// removeGTKTitleBar gives the window an empty title bar, so the page runs to
// the top edge (T-082). It must run before the window is shown.
func removeGTKTitleBar(g *gtkAPI, win uintptr) {
	if screen := g.screenDefault(); screen != 0 {
		css := g.cssNew()
		if g.cssLoad(css, gtkNoTitleCSS, len(gtkNoTitleCSS), 0) {
			g.addProvider(screen, css, gtkStyleApplication)
		}
	}
	box := g.boxNew(0, 0)
	g.addClass(g.styleContext(box), "am-no-title")
	g.setTitlebar(win, box)
}

// gtkWindowAction is what the page's controls ask for: minimise, maximise or
// restore, close, or drag the window by an empty part of its top.
func gtkWindowAction(g *gtkAPI, win uintptr, action string) {
	if win == 0 {
		return
	}
	switch action {
	case "minimize":
		g.iconify(win)
	case "maximize":
		if g.isMaximized(win) {
			g.unmaximize(win)
		} else {
			g.maximize(win)
		}
	case "close":
		g.closeWindow(win)
	case "drag":
		var x, y int32
		if display := g.displayDefault(); display != 0 {
			if seat := g.defaultSeat(display); seat != 0 {
				if pointer := g.seatPointer(seat); pointer != 0 {
					g.devicePosition(pointer, 0, &x, &y)
				}
			}
		}
		// Button 1 and the current time: the window manager moves the
		// window while the button the page saw pressed stays down.
		g.beginMoveDrag(win, 1, x, y, 0)
	}
}

// cString copies a C string of at most max bytes.
func cString(p uintptr, max int) string {
	// p points at memory GLib owns, not at a Go object.
	base := *(*unsafe.Pointer)(unsafe.Pointer(&p))
	var b []byte
	for i := 0; i < max; i++ {
		c := *(*byte)(unsafe.Add(base, i))
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}
