import AppKit
import WebKit
import Darwin

final class AppDelegate: NSObject, NSApplicationDelegate, WKNavigationDelegate, WKUIDelegate, WKScriptMessageHandler {
    var window: NSWindow!
    var web: WKWebView!
    var statusView: NSStackView!
    var statusLabel: NSTextField!
    var child: Process?
    var input: Pipe?
    var origin: URL?
    var generation = 0
    var lockFD: Int32 = -1
    let updates = DesktopUpdater()

    func applicationDidFinishLaunching(_ notification: Notification) {
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
            } else { fatalLaunch("Другое окно Messenger уже запускается. Откройте его через Dock.") }
            return
        }
        applyTheme(UserDefaults.standard.string(forKey: "appearance") ?? "system")
        buildMenu()
        let config = WKWebViewConfiguration()
        config.websiteDataStore = .nonPersistent()
        config.userContentController.add(self, name: "updates")
        config.userContentController.add(self, name: "appearance")
        let savedTheme = UserDefaults.standard.string(forKey: "appearance") ?? "system"
        let safeTheme = ["system", "light", "dark"].contains(savedTheme) ? savedTheme : "system"
        config.userContentController.addUserScript(WKUserScript(source: "window.messengerInitialTheme = '\(safeTheme)'", injectionTime: .atDocumentStart, forMainFrameOnly: true))
        updates.onStatus = { [weak self] _ in self?.showUpdateStatus() }
        web = WKWebView(frame: .zero, configuration: config)
        web.navigationDelegate = self
        web.uiDelegate = self
        web.allowsBackForwardNavigationGestures = false
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1200, height: 850), styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.title = "Agent Messenger"
        window.minSize = NSSize(width: 780, height: 600)
        window.isReleasedWhenClosed = false
        window.setFrameAutosaveName("AgentMessengerMainWindow")
        window.center()
        let content = NSView()
        window.contentView = content
        web.translatesAutoresizingMaskIntoConstraints = false
        content.addSubview(web)
        NSLayoutConstraint.activate([web.leadingAnchor.constraint(equalTo: content.leadingAnchor), web.trailingAnchor.constraint(equalTo: content.trailingAnchor), web.topAnchor.constraint(equalTo: content.topAnchor), web.bottomAnchor.constraint(equalTo: content.bottomAnchor)])
        statusLabel = NSTextField(wrappingLabelWithString: "Подключаемся к Messenger…")
        statusLabel.font = .systemFont(ofSize: 17)
        statusLabel.alignment = .center
        let retry = NSButton(title: "Повторить подключение", target: self, action: #selector(connect))
        let choose = NSButton(title: "Выбрать каталог данных…", target: self, action: #selector(chooseData))
        let note = NSTextField(wrappingLabelWithString: "Открытие окна не запускает модели. Закрытие окна сохраняет фоновый клиент и очередь.")
        note.alignment = .center
        note.textColor = .secondaryLabelColor
        statusView = NSStackView(views: [statusLabel, retry, choose, note])
        statusView.orientation = .vertical
        statusView.spacing = 18
        statusView.translatesAutoresizingMaskIntoConstraints = false
        content.addSubview(statusView)
        NSLayoutConstraint.activate([statusView.centerXAnchor.constraint(equalTo: content.centerXAnchor), statusView.centerYAnchor.constraint(equalTo: content.centerYAnchor), statusView.widthAnchor.constraint(equalToConstant: 560)])
        showWindow()
        connect()
    }

    func fatalLaunch(_ text: String) {
        let alert = NSAlert(); alert.messageText = "Agent Messenger"; alert.informativeText = text
        alert.runModal(); NSApp.terminate(nil)
    }
    func buildMenu() {
        let bar = NSMenu()
        let appItem = NSMenuItem(); bar.addItem(appItem)
        let appMenu = NSMenu(); appItem.submenu = appMenu
        appMenu.addItem(withTitle: "Об Agent Messenger", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: "")
        appMenu.addItem(.separator())
        updates.attach(menu: appMenu)
        appMenu.addItem(withTitle: "Скрыть Agent Messenger", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
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
        NSApp.mainMenu = bar
    }
    @objc func showWindow() { window?.makeKeyAndOrderFront(nil); NSApp.activate(ignoringOtherApps: true) }
    @objc func chooseData() {
        let panel = NSOpenPanel(); panel.canChooseDirectories = true; panel.canChooseFiles = false
        panel.allowsMultipleSelection = false; panel.canCreateDirectories = false
        panel.prompt = "Подключить"; panel.message = "Выберите существующий каталог данных Messenger. Файлы и защитные резервы не переносятся и не удаляются."
        if panel.runModal() == .OK, let url = panel.url {
            UserDefaults.standard.set(url.path, forKey: "dataRoot"); connect()
        }
    }
    @objc func automaticData() { UserDefaults.standard.removeObject(forKey: "dataRoot"); connect() }
    func stopUI() {
        generation += 1
        // EOF revokes this app-owned HTTP server; never send shutdown to client.
        try? input?.fileHandleForWriting.close(); input = nil
        child = nil
        origin = nil
        updates.endpoint = nil
        web?.stopLoading()
    }
    func message(_ text: String) { statusLabel.stringValue = text; statusView.isHidden = false; web.isHidden = true }
    @objc func connect() {
        stopUI()
        let version = generation
        message("Подключаемся к Messenger…")
        guard let executable = Bundle.main.url(forResource: "messenger", withExtension: nil) else { message("В приложении отсутствует клиент. Заново распакуйте проверенную сборку."); return }
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
                self.message("Соединение с окном завершено. Фоновый клиент и письма сохраняются.\n\n" + (detail.isEmpty ? "Повторите подключение или выберите прежний каталог данных." : detail))
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
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { statusView.isHidden = true; web.isHidden = false; showUpdateStatus() }
    func showUpdateStatus() {
        let status: [String: Any] = ["version": updates.availableVersion ?? "", "installedVersion": Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "", "channel": "github", "detail": updates.fetchStatus, "checking": updates.fetching]
        guard let data = try? JSONSerialization.data(withJSONObject: status), let json = String(data: data, encoding: .utf8) else { return }
        web?.evaluateJavaScript("window.messengerUpdateStatus && window.messengerUpdateStatus(" + json + ")", completionHandler: nil)
    }
    func applyTheme(_ theme: String) {
        NSApp.appearance = theme == "dark" ? NSAppearance(named: .darkAqua) : theme == "light" ? NSAppearance(named: .aqua) : nil
    }
    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.frameInfo.isMainFrame,
              let page = message.frameInfo.request.url, let origin = origin,
              page.scheme == origin.scheme, page.host == origin.host, page.port == origin.port, page.path == "/" else { return }
        if message.name == "updates", message.body as? String == "check" { updates.check() }
        if message.name == "appearance", let theme = message.body as? String, ["system", "light", "dark"].contains(theme) {
            UserDefaults.standard.set(theme, forKey: "appearance"); applyTheme(theme)
            // A reconnect creates a new document in the same WebView.
            web.configuration.userContentController.removeAllUserScripts()
            web.configuration.userContentController.addUserScript(WKUserScript(source: "window.messengerInitialTheme = '\(theme)'", injectionTime: .atDocumentStart, forMainFrameOnly: true))
        }
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) { message("Не удалось открыть окно: \(error.localizedDescription)") }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) { message("Ошибка окна: \(error.localizedDescription)") }
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) { message("Окно было остановлено системой. Повторите подключение; сохранённые исходящие останутся в клиенте.") }
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
