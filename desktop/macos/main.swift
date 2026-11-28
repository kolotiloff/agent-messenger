import AppKit
import WebKit
import Darwin

// The page under a transparent title bar (T-082). The page reports the empty
// parts of its top (in page points, from the top left) and the controls in
// them; a press there moves the window, a double click does what the
// system's title bar does.
final class TitlebarWebView: WKWebView {
    var dragZones: [NSRect] = []
    var dragHoles: [NSRect] = []
    override func mouseDown(with event: NSEvent) {
        let local = convert(event.locationInWindow, from: nil)
        let point = NSPoint(x: local.x, y: isFlipped ? local.y : bounds.height - local.y)
        guard let window = window, dragZones.contains(where: { $0.contains(point) }), !dragHoles.contains(where: { $0.contains(point) }) else {
            super.mouseDown(with: event)
            return
        }
        if event.clickCount == 2 {
            switch UserDefaults.standard.string(forKey: "AppleActionOnDoubleClick") {
            case "Minimize": window.miniaturize(nil)
            case "None": break
            default: window.zoom(nil)
            }
            return
        }
        window.performDrag(with: event)
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate, WKNavigationDelegate, WKUIDelegate, WKScriptMessageHandler {
    var window: NSWindow!
    var web: WKWebView!
    var statusView: NSStackView!
    var statusLabel: NSTextField!
    var statusSpinner: NSProgressIndicator!
    var retryButton: NSButton!
    var statusNote: NSTextField!
    var child: Process?
    var input: Pipe?
    var origin: URL?
    var generation = 0
    var lockFD: Int32 = -1
    lazy var updates = DesktopUpdater()
    let dictation = ComposerDictation()
    var statusItem: NSStatusItem?
    private var reportWindows: [NSWindow] = []

    func applicationDidFinishLaunching(_ notification: Notification) {
        if let renamed = BundleName.migrate(Bundle.main.bundleURL) {
            let configuration = NSWorkspace.OpenConfiguration()
            configuration.createsNewApplicationInstance = true
            configuration.promptsUserIfNeeded = false
            NSWorkspace.shared.openApplication(at: renamed, configuration: configuration) { [weak self] application, _ in
                DispatchQueue.main.async {
                    if let application, application.processIdentifier != getpid(), !application.isTerminated {
                        NSApp.terminate(nil)
                    } else {
                        self?.startDesktop()
                    }
                }
            }
            return
        }
        startDesktop()
    }

    private func startDesktop() {
        // LaunchServices normally reuses the app. flock also covers open -n and
        // direct executable launches without affecting the client's own lock.
        let folder = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support/AgentMessenger")
        do { try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700]) }
        catch { fatalLaunch("Не удалось открыть каталог приложения."); return }
        lockFD = Darwin.open(folder.appendingPathComponent("desktop.lock").path, O_CREAT | O_RDWR | O_CLOEXEC | O_NOFOLLOW, 0o600)
        guard lockFD >= 0, flock(lockFD, LOCK_EX | LOCK_NB) == 0 else {
            if let other = NSRunningApplication.runningApplications(withBundleIdentifier: Bundle.main.bundleIdentifier ?? "").first(where: { $0.processIdentifier != getpid() }) {
                other.activate(options: [.activateAllWindows])
                NSApp.terminate(nil)
            } else { fatalLaunch("Другое окно Kotel уже запускается. Откройте его через Dock.") }
            return
        }
        // Read the installed resource directly after an in-place update.
        if let resource = Bundle.main.url(forResource: "KotelIcon", withExtension: "icns"),
           let icon = NSImage(contentsOf: resource) {
            NSApp.applicationIconImage = icon
        }
        applyTheme(UserDefaults.standard.string(forKey: "appearance") ?? "system")
        buildMenu()
        let config = WKWebViewConfiguration()
        config.websiteDataStore = .nonPersistent()
        config.userContentController.add(self, name: "updates")
        config.userContentController.add(self, name: "appearance")
        config.userContentController.add(self, name: "chatFont")
        config.userContentController.add(self, name: "windowPreference")
        config.userContentController.add(self, name: "folder")
        config.userContentController.add(self, name: "app")
        config.userContentController.add(self, name: "titlebar")
        config.userContentController.add(self, name: "composerDictation")
        config.userContentController.add(self, name: "reportFileSave")
        config.userContentController.add(self, name: "reportFileWindow")
        dictation.onEvent = { [weak self] id, event, text in
            let payload = ["id": id, "event": event, "text": text ?? ""]
            guard let data = try? JSONSerialization.data(withJSONObject: payload),
                  let json = String(data: data, encoding: .utf8) else { return }
            self?.web?.evaluateJavaScript("window.messengerDictationEvent?.(\(json))")
        }
        refreshPageScripts(config.userContentController)
        updates.onStatus = { [weak self] _ in self?.showUpdateStatus() }
        updates.onRun = { [weak self] state in self?.showUpdateRun(state) }
        web = TitlebarWebView(frame: .zero, configuration: config)
        web.navigationDelegate = self
        web.uiDelegate = self
        web.allowsBackForwardNavigationGestures = false
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1200, height: 850), styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView], backing: .buffered, defer: false)
        window.title = "Kotel"
        // T-082: no separate title bar; the page runs to the top edge and the
        // system's window buttons stay at the top left over it.
        window.titlebarAppearsTransparent = true
        window.titleVisibility = .hidden
        window.minSize = NSSize(width: 780, height: 600)
        window.isReleasedWhenClosed = false
        window.delegate = self
        window.setFrameAutosaveName("AgentMessengerMainWindow")
        window.center()
        let content = NSView()
        window.contentView = content
        web.translatesAutoresizingMaskIntoConstraints = false
        content.addSubview(web)
        NSLayoutConstraint.activate([web.leadingAnchor.constraint(equalTo: content.leadingAnchor), web.trailingAnchor.constraint(equalTo: content.trailingAnchor), web.topAnchor.constraint(equalTo: content.topAnchor), web.bottomAnchor.constraint(equalTo: content.bottomAnchor)])
        // While the window connects it shows only a spinner. The retry button
        // and the note appear only when connecting failed; choosing another
        // data directory stays in the File menu for the rare case it is needed.
        statusSpinner = NSProgressIndicator()
        statusSpinner.style = .spinning
        statusSpinner.controlSize = .regular
        statusLabel = NSTextField(wrappingLabelWithString: "")
        statusLabel.font = .systemFont(ofSize: 17)
        statusLabel.alignment = .center
        retryButton = NSButton(title: "Повторить подключение", target: self, action: #selector(connect))
        statusNote = NSTextField(wrappingLabelWithString: "Фоновый клиент, задачи и очередь сохраняются.")
        statusNote.alignment = .center
        statusNote.textColor = .secondaryLabelColor
        statusView = NSStackView(views: [statusSpinner, statusLabel, retryButton, statusNote])
        statusView.orientation = .vertical
        statusView.spacing = 18
        statusView.translatesAutoresizingMaskIntoConstraints = false
        content.addSubview(statusView)
        NSLayoutConstraint.activate([statusView.centerXAnchor.constraint(equalTo: content.centerXAnchor), statusView.centerYAnchor.constraint(equalTo: content.centerYAnchor), statusView.widthAnchor.constraint(equalToConstant: 560)])
        buildStatusItem()
        showWindow()
        connect()
    }
    // The menu bar button opens the window. Its view-level context menu is
    // available on secondary click, including Control-click (T-293).
    func buildStatusItem() {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        guard let button = item.button else { return }
        if let image = NSImage(systemSymbolName: "bubble.left.fill", accessibilityDescription: "Kotel") {
            image.isTemplate = true
            button.image = image
        } else {
            button.title = "AM"
        }
        button.toolTip = "Kotel"
        button.target = self
        button.action = #selector(showWindow)
        let menu = NSMenu()
        let open = menu.addItem(withTitle: "Открыть окно", action: #selector(showWindow), keyEquivalent: ""); open.target = self
        menu.addItem(.separator())
        menu.addItem(withTitle: "Закрыть приложение", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "")
        button.menu = menu
        statusItem = item
    }
    func windowWillClose(_ notification: Notification) {
        guard (notification.object as? NSWindow) === window else { return }
        dictation.cancel()
        web?.evaluateJavaScript("window.messengerCancelDictation?.()")
        NSApp.setActivationPolicy(.accessory)
    }

    func applicationDidBecomeActive(_ notification: Notification) {
        if window != nil { updates.checkAutomaticallyIfDue() }
    }

    func fatalLaunch(_ text: String) {
        let alert = NSAlert(); alert.messageText = "Kotel"; alert.informativeText = text
        alert.runModal(); NSApp.terminate(nil)
    }
    func buildMenu() {
        let bar = NSMenu()
        let appItem = NSMenuItem(); bar.addItem(appItem)
        let appMenu = NSMenu(); appItem.submenu = appMenu
        appMenu.addItem(withTitle: "О Kotel", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: "")
        appMenu.addItem(.separator())
        updates.attach(menu: appMenu)
        appMenu.addItem(withTitle: "Скрыть Kotel", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        appMenu.addItem(withTitle: "Закрыть приложение", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        let file = NSMenuItem(); file.title = "Файл"; bar.addItem(file)
        let fileMenu = NSMenu(title: "Файл"); file.submenu = fileMenu
        let show = fileMenu.addItem(withTitle: "Открыть окно", action: #selector(showWindow), keyEquivalent: "0"); show.target = self
        let choose = fileMenu.addItem(withTitle: "Выбрать каталог данных…", action: #selector(chooseData), keyEquivalent: ""); choose.target = self
        let auto = fileMenu.addItem(withTitle: "Использовать сохранённые данные автоматически", action: #selector(automaticData), keyEquivalent: ""); auto.target = self
        fileMenu.addItem(withTitle: "Закрыть окно", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        let edit = NSMenuItem(); edit.title = "Правка"; bar.addItem(edit)
        let editMenu = NSMenu(title: "Правка"); edit.submenu = editMenu
        for (title, selector, key) in [("Отменить", "undo:", "z"), ("Вырезать", "cut:", "x"), ("Копировать", "copy:", "c"), ("Вставить", "paste:", "v"), ("Выделить всё", "selectAll:", "a")] {
            editMenu.addItem(withTitle: title, action: Selector(selector), keyEquivalent: key)
        }
        let view = NSMenuItem(); view.title = "Вид"; bar.addItem(view)
        let viewMenu = NSMenu(title: "Вид"); view.submenu = viewMenu
        let refresh = viewMenu.addItem(withTitle: "Обновить задачи и статусы", action: #selector(refreshMessages), keyEquivalent: "r"); refresh.target = self
        NSApp.mainMenu = bar
    }
    @objc func refreshMessages() {
        guard origin != nil, !web.isHidden else { return }
        web.evaluateJavaScript("window.messengerRefresh && window.messengerRefresh()", completionHandler: nil)
    }
    @objc func showWindow() {
        if NSApp.activationPolicy() != .regular { NSApp.setActivationPolicy(.regular) }
        if window?.isMiniaturized == true { window?.deminiaturize(nil) }
        window?.makeKeyAndOrderFront(nil)
        positionWindowButtons()
        NSApp.activate(ignoringOtherApps: true)
    }
    // Keep the native controls on the web toolbar's 44-point row.
    private func positionWindowButtons() {
        guard let window, let content = window.contentView, !window.styleMask.contains(.fullScreen) else { return }
        let controls: [NSWindow.ButtonType] = [.closeButton, .miniaturizeButton, .zoomButton]
        for (index, type) in controls.enumerated() {
            guard let button = window.standardWindowButton(type), let bar = button.superview else { continue }
            let center = NSPoint(x: 24 + CGFloat(index) * 23, y: content.isFlipped ? 22 : content.bounds.maxY - 22)
            let point = bar.convert(center, from: content)
            button.setFrameOrigin(NSPoint(x: point.x - button.frame.width / 2, y: point.y - button.frame.height / 2))
        }
    }
    func windowDidResize(_ notification: Notification) { positionWindowButtons() }
    func windowDidBecomeKey(_ notification: Notification) { positionWindowButtons() }
    func windowDidExitFullScreen(_ notification: Notification) { positionWindowButtons() }

    @objc func chooseData() {
        let panel = NSOpenPanel(); panel.canChooseDirectories = true; panel.canChooseFiles = false
        panel.allowsMultipleSelection = false; panel.canCreateDirectories = false
        panel.prompt = "Подключить"; panel.message = "Выберите существующий каталог данных Kotel. Файлы и защитные резервы не переносятся и не удаляются."
        if panel.runModal() == .OK, let url = panel.url {
            UserDefaults.standard.set(url.path, forKey: "dataRoot"); connect()
        }
    }
    @objc func automaticData() { UserDefaults.standard.removeObject(forKey: "dataRoot"); connect() }
    func stopUI() {
        dictation.cancel()
        generation += 1
        // EOF revokes this app-owned HTTP server; never send shutdown to client.
        try? input?.fileHandleForWriting.close(); input = nil
        child = nil
        origin = nil
        if lockFD >= 0 { updates.endpoint = nil }
        web?.stopLoading()
    }
    // A failure: the reason, a retry button and a short note.
    func message(_ text: String) {
        statusSpinner.stopAnimation(nil); statusSpinner.isHidden = true
        statusLabel.stringValue = text; statusLabel.isHidden = false
        retryButton.isHidden = false; statusNote.isHidden = false
        statusView.isHidden = false; web.isHidden = true
    }
    // Normal start or restart: only a spinner, no buttons or texts.
    func showConnecting() {
        statusLabel.isHidden = true; retryButton.isHidden = true; statusNote.isHidden = true
        statusSpinner.isHidden = false; statusSpinner.startAnimation(nil)
        statusView.isHidden = false; web.isHidden = true
    }
    @objc func connect() {
        stopUI()
        let version = generation
        refreshPageScripts(web.configuration.userContentController)
        showConnecting()
        guard let executable = Bundle.main.url(forResource: "kotel", withExtension: nil) else { message("В приложении отсутствует клиент. Заново распакуйте проверенную сборку."); return }
        let process = Process(); process.executableURL = executable
        var args: [String] = []
        if let root = UserDefaults.standard.string(forKey: "dataRoot"), !root.isEmpty { args += ["--root", root] }
        args.append("desktop"); process.arguments = args
        let output = Pipe(), errors = Pipe(), lifetime = Pipe()
        process.standardOutput = output; process.standardError = errors; process.standardInput = lifetime
        // The app child never inherits a terminal, shell command, or model flag.
        process.terminationHandler = { [weak self] p in
            let data = errors.fileHandleForReading.readDataToEndOfFile()
            let detail = String(data: data.prefix(8192), encoding: .utf8) ?? ""
            DispatchQueue.main.async {
                guard let self = self, self.generation == version else { return }
                self.message("Соединение с окном завершено. Фоновый клиент и письма сохраняются.\n\n" + (detail.isEmpty ? "Повторите подключение." : detail))
            }
        }
        child = process; input = lifetime
        do { try process.run() } catch { stopUI(); message("Не удалось запустить встроенный клиент: \(error.localizedDescription)"); return }
        // Close the parent's duplicates; only the child holds these pipe ends.
        try? output.fileHandleForWriting.close(); try? errors.fileHandleForWriting.close(); try? lifetime.fileHandleForReading.close()
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            var line = Data()
            while line.count < 2048 {
                let byte = output.fileHandleForReading.readData(ofLength: 1)
                if byte.isEmpty || byte == Data([10]) { break }
                line.append(byte)
            }
            let text = String(data: line, encoding: .utf8) ?? ""
            DispatchQueue.main.async {
                guard let self = self, self.generation == version, process.isRunning else { return }
                guard let url = URL(string: text), self.validEndpoint(url) else { self.stopUI(); self.message("Клиент не подтвердил безопасный локальный адрес окна."); return }
                self.origin = url
                self.updates.connect(url, window: self.window)
                self.web.load(URLRequest(url: url))
            }
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 30) { [weak self] in
            guard let self = self, self.generation == version, self.origin == nil else { return }
            self.stopUI(); self.message("Клиент не ответил вовремя. Защитные резервы сохранены. Повторите подключение.")
        }
    }
    func validEndpoint(_ url: URL) -> Bool {
        guard url.scheme == "http", url.host == "127.0.0.1", let port = url.port, port > 0 && port < 65536,
              url.user == nil, url.password == nil, url.path == "/", url.query == nil,
              let token = url.fragment, token.count == 64 else { return false }
        return token.allSatisfy { "0123456789abcdef".contains($0) }
    }
    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction, decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let url = navigationAction.request.url, let allowed = origin,
              url.scheme == allowed.scheme, url.host == allowed.host, url.port == allowed.port,
              url.user == nil, url.password == nil, url.path == "/", url.query == nil,
              navigationAction.targetFrame?.isMainFrame == true else { decisionHandler(.cancel); return }
        decisionHandler(.allow)
    }
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { statusSpinner.stopAnimation(nil); statusView.isHidden = true; web.isHidden = false; showUpdateStatus() }
    func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) { dictation.cancel() }
    func showUpdateStatus() {
        let status: [String: Any] = ["version": updates.shownVersion ?? "", "installedVersion": Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "", "channel": "github", "detail": updates.fetchStatus, "checking": updates.fetching || updates.waitingForDownload]
        guard let data = try? JSONSerialization.data(withJSONObject: status), let json = String(data: data, encoding: .utf8) else { return }
        web?.evaluateJavaScript("window.messengerUpdateStatus && window.messengerUpdateStatus(" + json + ")", completionHandler: nil)
    }
    // The update dialog of the page follows Sparkle's steps (T-199).
    func showUpdateRun(_ state: [String: Any]) {
        guard let data = try? JSONSerialization.data(withJSONObject: state), let json = String(data: data, encoding: .utf8) else { return }
        web?.evaluateJavaScript("window.messengerUpdateRun && window.messengerUpdateRun(" + json + ")", completionHandler: nil)
    }
    // The project folder is chosen in a standard panel; only its path goes to the page.
    func pickProjectFolder() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true; panel.canChooseFiles = false; panel.allowsMultipleSelection = false
        panel.canCreateDirectories = true; panel.prompt = "Выбрать"; panel.message = "Папка проекта на этом компьютере"
        guard let window = window else { return }
        panel.beginSheetModal(for: window) { [weak self] response in
            guard response == .OK, let url = panel.url,
                  let data = try? JSONSerialization.data(withJSONObject: [url.path]), let json = String(data: data, encoding: .utf8) else { return }
            self?.web?.evaluateJavaScript("window.messengerFolderPicked && window.messengerFolderPicked(" + json + "[0])", completionHandler: nil)
        }
    }
    private static let chatFonts = ["system", "system-readable", "inter", "source-sans"]

    private var preferencesKey: String {
        "windowPreferences." + (UserDefaults.standard.string(forKey: "dataRoot") ?? "default")
    }
    private static let windowPreferenceKeys = ["kotel-task-panes", "kotel-active-pane", "kotel-pane-layout", "messenger-notice-dismissal"]
    func refreshPageScripts(_ controller: WKUserContentController) {
        let defaults = UserDefaults.standard
        let savedTheme = defaults.string(forKey: "appearance") ?? "system"
        let theme = ["system", "light", "dark"].contains(savedTheme) ? savedTheme : "system"
        let savedFont = defaults.string(forKey: "chatFont") ?? "inter"
        let font = Self.chatFonts.contains(savedFont) ? savedFont : "inter"
        controller.removeAllUserScripts()
        let source = "window.messengerInitialTheme = '\(theme)'; window.messengerInitialChatFont = '\(font)'; window.amMacTitlebar = true"
        let preferences = defaults.dictionary(forKey: preferencesKey) as? [String: String] ?? [:]
        let data = try? JSONSerialization.data(withJSONObject: preferences)
        let json = data.flatMap { String(data: $0, encoding: .utf8) } ?? "{}"
        controller.addUserScript(WKUserScript(source: source + "; window.messengerPreferences = " + json, injectionTime: .atDocumentStart, forMainFrameOnly: true))
    }
    func rects(_ value: Any?) -> [NSRect] {
        guard let list = value as? [[Double]] else { return [] }
        return list.prefix(64).compactMap { r in r.count == 4 && r.allSatisfy({ $0.isFinite }) ? NSRect(x: r[0], y: r[1], width: r[2], height: r[3]) : nil }
    }
    func applyTheme(_ theme: String) {
        NSApp.appearance = theme == "dark" ? NSAppearance(named: .darkAqua) : theme == "light" ? NSAppearance(named: .aqua) : nil
    }
    private func saveReportFile(_ body: [String: String]) {
        guard let id = body["id"].flatMap(UUID.init(uuidString:)) else { return }
        func finish(_ result: String) {
            web?.evaluateJavaScript("window.taskFiles?.saved('\(id.uuidString.lowercased())', '\(result)')")
        }
        guard let name = body["name"], !name.isEmpty, name != ".", name != "..",
              name.utf8.count <= 255, name.rangeOfCharacter(from: CharacterSet(charactersIn: "/\\:").union(.controlCharacters)) == nil,
              let encoded = body["data"], encoded.utf8.count <= 1_400_000,
              let data = Data(base64Encoded: encoded), data.count <= 1_048_576,
              let window = window, window.isKeyWindow, window.attachedSheet == nil else { finish("failed"); return }
        let panel = NSSavePanel()
        panel.nameFieldStringValue = name
        panel.canCreateDirectories = true
        panel.beginSheetModal(for: window) { response in
            guard response == .OK, let url = panel.url else { finish("cancelled"); return }
            do { try data.write(to: url, options: .atomic); finish("saved") }
            catch { finish("failed") }
        }
    }
    private func openReportWindow(_ body: [String: Any]) {
        guard let name = body["name"] as? String, !name.isEmpty, name.utf8.count <= 1024,
              let text = body["text"] as? String, text.utf8.count <= 1_048_576,
              window.isKeyWindow else { return }
        let preview = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 860, height: 680),
                               styleMask: [.titled, .closable, .miniaturizable, .resizable],
                               backing: .buffered, defer: false)
        preview.title = name
        preview.isReleasedWhenClosed = false
        preview.minSize = NSSize(width: 340, height: 240)
        let scroll = NSTextView.scrollableTextView()
        guard let content = scroll.documentView as? NSTextView else { return }
        content.isEditable = false
        content.textContainerInset = NSSize(width: 16, height: 16)
        content.font = .monospacedSystemFont(ofSize: NSFont.systemFontSize, weight: .regular)
        content.string = text
        preview.contentView = scroll
        reportWindows.removeAll { !$0.isVisible && !$0.isMiniaturized }
        reportWindows.append(preview)
        preview.center()
        preview.makeKeyAndOrderFront(nil)
        if let line = body["line"] as? Int, line > 0 {
            let source = text as NSString
            var start = 0
            for _ in 1..<line {
                if start >= source.length { return }
                start = NSMaxRange(source.lineRange(for: NSRange(location: start, length: 0)))
            }
            if start < source.length {
                let range = source.lineRange(for: NSRange(location: start, length: 0))
                content.setSelectedRange(range)
                content.scrollRangeToVisible(range)
            }
        }
    }
    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.frameInfo.isMainFrame,
              let page = message.frameInfo.request.url, let origin = origin,
              page.scheme == origin.scheme, page.host == origin.host, page.port == origin.port, page.path == "/" else { return }
        if message.name == "updates", message.body as? String == "check" { updates.check() }
        if message.name == "updates", message.body as? String == "install" { updates.install() }
        if message.name == "folder", message.body as? String == "pick" { pickProjectFolder() }
        if message.name == "reportFileWindow", let body = message.body as? [String: Any] { openReportWindow(body) }
        if message.name == "reportFileSave", let body = message.body as? [String: String] { saveReportFile(body) }
        if message.name == "composerDictation", let body = message.body as? [String: String],
           let id = body["id"], UUID(uuidString: id) != nil {
            switch body["action"] {
            case "start":
                guard window.isKeyWindow else { dictation.onEvent?(id, "failed", nil); return }
                let language = body["language"] == "en" ? "en-US" : "ru-RU"
                dictation.start(id, locale: Locale(identifier: language))
            case "stop": dictation.stop(id)
            case "cancel": dictation.cancel(id)
            default: break
            }
        }
        // Quits only the app; the background client and its reservations stay.
        if message.name == "app", message.body as? String == "quit" { NSApp.terminate(nil) }
        if message.name == "appearance", let theme = message.body as? String, ["system", "light", "dark"].contains(theme) {
            UserDefaults.standard.set(theme, forKey: "appearance"); applyTheme(theme)
            // A reconnect creates a new document in the same WebView.
            refreshPageScripts(userContentController)
        }
        if message.name == "windowPreference", let body = message.body as? [String: String],
           let key = body["key"], Self.windowPreferenceKeys.contains(key),
           let value = body["value"], value.utf8.count <= 262144 {
            var preferences = UserDefaults.standard.dictionary(forKey: preferencesKey) as? [String: String] ?? [:]
            preferences[key] = value
            UserDefaults.standard.set(preferences, forKey: preferencesKey)
            refreshPageScripts(userContentController)
        }
        if message.name == "chatFont", let font = message.body as? String, Self.chatFonts.contains(font) {
            UserDefaults.standard.set(font, forKey: "chatFont")
            refreshPageScripts(userContentController)
        }
        // The empty parts of the page's top that move the window (T-082).
        if message.name == "titlebar", let body = message.body as? [String: Any], let view = web as? TitlebarWebView {
            view.dragZones = rects(body["zones"])
            view.dragHoles = rects(body["holes"])
        }
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) { message("Не удалось открыть окно: \(error.localizedDescription)") }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) { message("Ошибка окна: \(error.localizedDescription)") }
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        dictation.cancel()
        message("Окно было остановлено системой. Повторите подключение; сохранённые исходящие останутся в клиенте.")
    }
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration, for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? { nil }
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool { showWindow(); return true }
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }
    func applicationWillTerminate(_ notification: Notification) { stopUI(); if lockFD >= 0 { Darwin.close(lockFD) } }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.setActivationPolicy(.regular)
app.delegate = delegate
app.run()
