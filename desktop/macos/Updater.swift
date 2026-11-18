import AppKit
import Sparkle
import Darwin

// Sparkle owns download verification, installation and relaunch. The feed is
// served by our authenticated loopback bridge from signed local metadata.
// Sparkle's own windows are not shown (owner, 2026-11-18): this class is its
// user driver, and the page shows the same dialog as on Windows and Linux,
// with the download, the install and the restart (T-199).
final class DesktopUpdater: NSObject, SPUUpdaterDelegate, SPUUserDriver {
    weak var window: NSWindow?
    var endpoint: URL?
    var updater: SPUUpdater!
    var availableVersion: String?
    // A newer release found on GitHub but not downloaded (T-213): the icon
    // shows it, and the download starts when the owner presses it.
    var remoteVersion: String?
    var shownVersion: String? { availableVersion ?? remoteVersion }
    var onStatus: ((String?) -> Void)?
    // The page's dialog: what Sparkle does now, sent to the page.
    var onRun: (([String: Any]) -> Void)?
    // The owner pressed the update icon; until the run ends every step goes
    // to the dialog instead of an alert.
    var running = false
    var received: UInt64 = 0
    var expected: UInt64 = 0
    // A found update waits here until the owner presses the icon: the
    // session stays open, like Sparkle's gentle reminders.
    var pendingChoice: ((SPUUserUpdateChoice) -> Void)?
    var started = false
    var fetching = false
    var fetchStatus = ""
    var fetchTimer: Timer?
    var lastRemoteCheck = -Double.infinity
    var fetchProcess: Process?
    var waitingForDownload = false
    var busySince: TimeInterval?
    var busyRetry: DispatchWorkItem?
    var watcher: DispatchSourceFileSystemObject?
    var watchWork: DispatchWorkItem?
    var lastManifest: Data?
    let channelDirectory = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support/AgentMessenger/updates")
    var checkItem: NSMenuItem?
    var automaticItem: NSMenuItem?

    override init() {
        super.init()
        updater = SPUUpdater(hostBundle: Bundle.main, applicationBundle: Bundle.main, userDriver: self, delegate: self)
    }
    func attach(menu: NSMenu) {
        checkItem = menu.addItem(withTitle: "Проверить обновления…", action: #selector(check), keyEquivalent: "")
        checkItem?.target = self
        automaticItem = menu.addItem(withTitle: "Проверять обновления автоматически", action: #selector(toggleAutomatic), keyEquivalent: "")
        automaticItem?.target = self
        let source = menu.addItem(withTitle: "Канал: GitHub Releases, подписанные пакеты", action: nil, keyEquivalent: "")
        source.isEnabled = false
        menu.addItem(.separator())
    }
    func connect(_ url: URL, window: NSWindow) {
        self.window = window
        endpoint = url
        updater.httpHeaders = ["Authorization": "Bearer " + (url.fragment ?? "")]
        if !started {
            do { try updater.start() } catch { fetchStatus = "Обновления не запустились"; onStatus?(shownVersion); return }
            started = true
            startWatchingLocalChannel()
            fetchTimer = Timer.scheduledTimer(withTimeInterval: 1800, repeats: true) { [weak self] _ in
                self?.checkAutomaticallyIfDue()
            }
            fetchTimer?.tolerance = 15
            checkAutomaticallyIfDue()
            if updater.automaticallyChecksForUpdates && !updater.sessionInProgress {
                updater.checkForUpdatesInBackground()
            }
        }
        automaticItem?.state = updater.automaticallyChecksForUpdates ? .on : .off
    }
    func manifestData() -> Data? {
        let name = FileManager.default.fileExists(atPath: channelDirectory.appendingPathComponent("current.json").path) ? "current.json" : "manifest.json"
        guard let file = try? FileHandle(forReadingFrom: channelDirectory.appendingPathComponent(name)) else { return nil }
        defer { try? file.close() }
        guard let data = try? file.read(upToCount: 46001), data.count <= 46000 else { return nil }
        return data
    }
    func startWatchingLocalChannel() {
        try? FileManager.default.createDirectory(at: channelDirectory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        lastManifest = manifestData()
        let fd = Darwin.open(channelDirectory.path, O_EVTONLY | O_CLOEXEC | O_NOFOLLOW)
        guard fd >= 0 else { return }
        let source = DispatchSource.makeFileSystemObjectSource(fileDescriptor: fd, eventMask: [.write], queue: .main)
        source.setEventHandler { [weak self] in self?.scheduleLocalCheck() }
        source.setCancelHandler { Darwin.close(fd) }
        watcher = source; source.resume()
    }
    func scheduleLocalCheck() {
        watchWork?.cancel()
        let work = DispatchWorkItem { [weak self] in
            guard let self = self, self.started, self.endpoint != nil,
                  self.updater.automaticallyChecksForUpdates,
                  !self.updater.sessionInProgress,
                  let data = self.manifestData(), data != self.lastManifest else { return }
            self.lastManifest = data
            // Local publication is an event, not model polling. The bridge verifies
            // the signed metadata; Sparkle verifies the archive before install.
            self.updater.checkForUpdatesInBackground()
        }
        watchWork = work; DispatchQueue.main.asyncAfter(deadline: .now() + 2, execute: work)
    }
    func updater(_ updater: SPUUpdater, didFinishUpdateCycleFor updateCheck: SPUUpdateCheck, error: Error?) { scheduleLocalCheck() }
    @objc func check() {
        busySince = nil
        guard endpoint != nil, started else {
            let alert = NSAlert(); alert.messageText = "Обновления пока недоступны"
            alert.informativeText = "Сначала подключите окно к локальному клиенту. Ни очередь, ни сессии не изменены."
            alert.runModal(); return
        }
        // A found update keeps the Sparkle session open. Bring that verified
        // update forward instead of blocking on sessionInProgress.
        if let reply = pendingChoice {
            pendingChoice = nil
            if running { startRun(); reply(.install) } else { offer(reply) }
        } else if availableVersion != nil, updater.canCheckForUpdates {
            busyRetry?.cancel(); waitingForDownload = false; busySince = nil
            updater.checkForUpdates()
        } else {
            findRemote(manual: true)
        }
    }
    // The page's update icon: download, install and restart, shown in the
    // page's dialog. Nothing is retried on its own.
    func install() {
        running = true
        sendRun(["phase": "downloading"])
        busySince = nil
        guard endpoint != nil, started else { failRun("Обновления пока недоступны. Программа осталась прежней."); return }
        if let reply = pendingChoice {
            pendingChoice = nil; startRun(); reply(.install)
        } else if availableVersion != nil, updater.canCheckForUpdates {
            updater.checkForUpdates()
        } else {
            // Only now the package comes from GitHub, with its progress in
            // the page's dialog (T-213).
            fetchRemote(manual: true)
        }
    }
    func startRun() {
        received = 0; expected = 0
        sendRun(["phase": "downloading"])
    }
    func sendRun(_ state: [String: Any]) {
        var state = state
        if let version = shownVersion { state["version"] = version }
        onRun?(state)
    }
    func failRun(_ text: String) {
        running = false
        sendRun(["phase": "error", "error": text])
    }
    // The menu's «Проверить обновления…» found one: ask before installing.
    func offer(_ reply: @escaping (SPUUserUpdateChoice) -> Void) {
        let alert = NSAlert()
        alert.messageText = "Вышла версия " + (shownVersion ?? "")
        alert.informativeText = "Программа загрузит обновление, установит его и перезапустится. Задачи и история сохранятся."
        alert.addButton(withTitle: "Обновить")
        alert.addButton(withTitle: "Позже")
        let answer = { [weak self] (response: NSApplication.ModalResponse) in
            guard let self = self else { return }
            if response == .alertFirstButtonReturn { self.running = true; self.startRun(); reply(.install) }
            else { self.pendingChoice = reply }
        }
        if let window = window, window.isVisible { alert.beginSheetModal(for: window, completionHandler: answer) } else { answer(alert.runModal()) }
    }
    // Focus changes share the same throttle as the timer; waiting never calls a model.
    func checkAutomaticallyIfDue() {
        guard started, endpoint != nil, updater.automaticallyChecksForUpdates,
              ProcessInfo.processInfo.systemUptime - lastRemoteCheck >= 1800 - 60 else { return }
        findRemote(manual: false)
    }
    // findRemote asks GitHub whether a newer signed release exists and
    // downloads nothing (T-213). A found release shows the icon; the menu's
    // check offers it at once.
    func findRemote(manual: Bool) {
        if manual && fetching {
            remoteMessage("Проверка обновлений ещё идёт", "Дождитесь её окончания. Программа не изменена.")
            return
        }
        guard !fetching, endpoint != nil,
              let executable = Bundle.main.url(forResource: "messenger", withExtension: nil) else { return }
        lastRemoteCheck = ProcessInfo.processInfo.systemUptime
        fetching = true; fetchStatus = "Проверяем GitHub…"; onStatus?(shownVersion)
        let process = Process(); process.executableURL = executable; process.arguments = ["update-find"]
        process.environment = ["HOME": FileManager.default.homeDirectoryForCurrentUser.path, "PATH": "/usr/bin:/bin"]
        let output = Pipe(), errors = Pipe()
        process.standardInput = FileHandle.nullDevice; process.standardOutput = output; process.standardError = errors
        process.terminationHandler = { [weak self] child in
            let data = output.fileHandleForReading.readDataToEndOfFile()
            _ = errors.fileHandleForReading.readDataToEndOfFile()
            let result = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
            DispatchQueue.main.async {
                guard let self = self else { return }
                self.fetching = false; self.fetchProcess = nil; self.fetchStatus = ""
                let state = child.terminationStatus == 0 ? (result?["state"] as? String ?? "") : ""
                switch state {
                case "found":
                    self.remoteVersion = result?["version"] as? String
                    self.onStatus?(self.shownVersion)
                    if manual, self.availableVersion == nil { self.offerDownload() }
                case "current":
                    self.remoteVersion = nil; self.onStatus?(self.shownVersion)
                    // A package downloaded earlier is installed from the local channel.
                    if self.manifestData() != nil, !self.updater.sessionInProgress {
                        if manual { self.updater.checkForUpdates() } else if self.updater.automaticallyChecksForUpdates { self.updater.checkForUpdatesInBackground() }
                    } else if manual {
                        self.remoteMessage("Обновлений пока нет", "Установленная версия не старше опубликованной подписанной сборки.")
                    }
                case "rate_limited":
                    var when = ""
                    if let at = result?["retry_at"] as? String, let date = ISO8601DateFormatter().date(from: at) {
                        let f = DateFormatter(); f.dateFormat = "HH:mm"; when = " после " + f.string(from: date)
                    }
                    self.fetchStatus = "GitHub ограничил проверки, повторить" + when; self.onStatus?(self.shownVersion)
                    if manual { self.remoteMessage("GitHub временно ограничил проверки", "С этого адреса было слишком много проверок обновлений. Повторите" + (when.isEmpty ? " позже" : when) + ". Программа не изменена.") }
                case "no_release":
                    if manual { self.remoteMessage("Пакет ещё не опубликован", "В GitHub Releases пока нет подписанной сборки для этого компьютера.") }
                default:
                    self.fetchStatus = "Не удалось проверить обновления, повторить"; self.onStatus?(self.shownVersion)
                    if manual { self.remoteMessage("Не удалось проверить обновления", "GitHub не ответил или ответ не прошёл проверку подписи. Повторите позже. Программа не изменена.") }
                }
            }
        }
        fetchProcess = process
        do { try process.run(); try? output.fileHandleForWriting.close(); try? errors.fileHandleForWriting.close() }
        catch { fetching = false; fetchProcess = nil; fetchStatus = "Проверка не запустилась"; onStatus?(shownVersion) }
    }
    // The menu's check found a release that is not downloaded yet.
    func offerDownload() {
        let alert = NSAlert()
        alert.messageText = "Вышла версия " + (shownVersion ?? "")
        alert.informativeText = "Программа загрузит обновление с GitHub, установит его и перезапустится. Задачи и история сохранятся."
        alert.addButton(withTitle: "Обновить")
        alert.addButton(withTitle: "Позже")
        let answer = { [weak self] (response: NSApplication.ModalResponse) in
            guard let self = self, response == .alertFirstButtonReturn else { return }
            self.install()
        }
        if let window = window, window.isVisible { alert.beginSheetModal(for: window, completionHandler: answer) } else { answer(alert.runModal()) }
    }
    func fetchRemote(manual: Bool) {
        // A manual check always ends with a message (T-203): it never just
        // returns because a check or a Sparkle session is still open.
        if manual && fetching {
            remoteMessage("Обновление уже загружается", "Загрузка с GitHub ещё идёт. На медленной связи она может занять несколько минут; когда пакет будет проверен, появится значок обновления.")
            return
        }
        if manual && updater.sessionInProgress {
            remoteMessage("Проверка обновлений ещё идёт", "Предыдущая проверка не закончилась. Повторите через минуту. Программа не изменена.")
            return
        }
        guard !fetching, endpoint != nil, !updater.sessionInProgress,
              let executable = Bundle.main.url(forResource: "messenger", withExtension: nil) else { return }
        busyRetry?.cancel(); waitingForDownload = false
        lastRemoteCheck = ProcessInfo.processInfo.systemUptime
        fetching = true; fetchStatus = "Проверяем GitHub…"; onStatus?(shownVersion)
        let process = Process(); process.executableURL = executable; process.arguments = ["update-fetch"]
        // No shell, GitHub token, provider settings or permission flags.
        process.environment = ["HOME": FileManager.default.homeDirectoryForCurrentUser.path, "PATH": "/usr/bin:/bin"]
        let output = Pipe(), errors = Pipe()
        process.standardInput = FileHandle.nullDevice; process.standardOutput = output; process.standardError = errors
        process.terminationHandler = { [weak self] child in
            let data = output.fileHandleForReading.readDataToEndOfFile()
            _ = errors.fileHandleForReading.readDataToEndOfFile()
            let result = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
            DispatchQueue.main.async {
                guard let self = self else { return }
                self.fetching = false; self.fetchProcess = nil
                if child.terminationStatus == 0, result?["state"] as? String == "busy" {
                    // An updater from an earlier window may still own the download.
                    // Wait for it; never kill it or mistake the lock for a network error.
                    let now = ProcessInfo.processInfo.systemUptime
                    if self.busySince == nil { self.busySince = now }
                    guard now - self.busySince! < 300 else {
                        self.fetchStatus = "Загрузка ещё выполняется, повторить"
                        self.onStatus?(self.shownVersion)
                        if manual { self.remoteMessage("Обновление уже загружается", "Другая загрузка обновления ещё идёт. Повторите проверку позже. Программа не изменена.") }
                        return
                    }
                    self.waitingForDownload = true
                    self.fetchStatus = "Обновление загружается в фоне"
                    self.onStatus?(self.shownVersion)
                    let retry = DispatchWorkItem { [weak self] in
                        guard let self = self else { return }
                        self.waitingForDownload = false
                        self.onStatus?(self.shownVersion)
                        if (manual || self.updater.automaticallyChecksForUpdates) && !self.updater.sessionInProgress { self.fetchRemote(manual: manual) }
                    }
                    self.busyRetry = retry
                    DispatchQueue.main.asyncAfter(deadline: .now() + 5, execute: retry)
                    return
                }
                self.busySince = nil
                if child.terminationStatus == 0, result?["state"] as? String == "rate_limited" {
                    // GitHub's hourly limit per address, shared by the owner's computers.
                    var when = ""
                    if let at = result?["retry_at"] as? String, let date = ISO8601DateFormatter().date(from: at) {
                        let f = DateFormatter(); f.dateFormat = "HH:mm"; when = " после " + f.string(from: date)
                    }
                    self.fetchStatus = "GitHub ограничил проверки, повторить" + when; self.onStatus?(self.shownVersion)
                    if manual { self.remoteMessage("GitHub временно ограничил проверки", "С этого адреса сегодня было слишком много проверок обновлений. Повторите" + (when.isEmpty ? " позже" : when) + ". Программа и сохранённый пакет не изменены.") }
                    return
                }
                if child.terminationStatus != 0 {
                    self.fetchStatus = "Не удалось получить обновление, повторить"; self.onStatus?(self.shownVersion)
                    if manual { self.remoteMessage("Не удалось получить обновление", "Загрузка или проверка пакета не завершилась. Повторите позже. Сохранённый пакет и работающий клиент не изменены; неподписанные пакеты не принимаются.") }
                    return
                }
                if result?["state"] as? String == "no_release" {
                    self.fetchStatus = "На GitHub ещё нет сборки"; self.onStatus?(self.shownVersion)
                    if manual { self.remoteMessage("Пакет ещё не опубликован", "В GitHub Releases пока нет подписанной сборки для этого компьютера.") }
                    return
                }
                self.fetchStatus = ""; self.onStatus?(self.shownVersion)
                if result?["state"] as? String == "current", self.manifestData() == nil {
                    if manual { self.remoteMessage("Обновлений пока нет", "Установленная версия не старше опубликованной подписанной сборки.") }
                    return
                }
                guard self.endpoint != nil, !self.updater.sessionInProgress else { return }
                if manual { self.updater.checkForUpdates() }
                else if self.updater.automaticallyChecksForUpdates { self.updater.checkForUpdatesInBackground() }
            }
        }
        fetchProcess = process
        do { try process.run(); try? output.fileHandleForWriting.close(); try? errors.fileHandleForWriting.close() }
        catch { fetching = false; fetchProcess = nil; fetchStatus = "Проверка не запустилась"; onStatus?(shownVersion) }
    }
    func remoteMessage(_ title: String, _ text: String) {
        if running { failRun(title + ". " + text); return }
        let alert = NSAlert(); alert.messageText = title; alert.informativeText = text
        if let window = window { alert.beginSheetModal(for: window) } else { alert.runModal() }
    }
    @objc func toggleAutomatic() {
        updater.automaticallyChecksForUpdates.toggle()
        automaticItem?.state = updater.automaticallyChecksForUpdates ? .on : .off
    }
    func feedURLString(for updater: SPUUpdater) -> String? {
        guard let endpoint = endpoint else { return nil }
        return "http://127.0.0.1:\(endpoint.port!)/api/updates/feed.xml"
    }
    func updater(_ updater: SPUUpdater, mayPerform updateCheck: SPUUpdateCheck) throws {
        guard endpoint != nil else { throw NSError(domain: "AgentMessenger.Update", code: 1, userInfo: [NSLocalizedDescriptionKey: "Локальный канал обновлений не подключён."]) }
    }
    func updater(_ updater: SPUUpdater, shouldProceedWithUpdate item: SUAppcastItem, updateCheck: SPUUpdateCheck) throws {
        guard let url = item.fileURL, let endpoint = endpoint,
              url.scheme == "http", url.host == "127.0.0.1", url.port == endpoint.port,
              url.user == nil, url.password == nil, url.query == nil, url.fragment == nil,
              url.path.range(of: "^/api/updates/[0-9a-f]{64}\\.zip$", options: .regularExpression) != nil else {
            throw NSError(domain: "AgentMessenger.Update", code: 2, userInfo: [NSLocalizedDescriptionKey: "Пакет не принадлежит проверенному локальному каналу."])
        }
    }
    func updater(_ updater: SPUUpdater, shouldDownloadReleaseNotesForUpdate updateItem: SUAppcastItem) -> Bool { false }

    // SPUUserDriver: what Sparkle would show, sent to the page instead.
    func show(_ request: SPUUpdatePermissionRequest, reply: @escaping (SUUpdatePermissionResponse) -> Void) {
        // The menu's «Проверять обновления автоматически» changes it later.
        reply(SUUpdatePermissionResponse(automaticUpdateChecks: true, sendSystemProfile: false))
    }
    func showUserInitiatedUpdateCheck(cancellation: @escaping () -> Void) {}
    func showUpdateFound(with appcastItem: SUAppcastItem, state: SPUUserUpdateState, reply: @escaping (SPUUserUpdateChoice) -> Void) {
        availableVersion = appcastItem.displayVersionString
        remoteVersion = nil
        onStatus?(shownVersion)
        if running { startRun(); reply(.install) }
        else if state.userInitiated { offer(reply) }
        else { pendingChoice = reply }
    }
    func showUpdateReleaseNotes(with downloadData: SPUDownloadData) {}
    func showUpdateReleaseNotesFailedToDownloadWithError(_ error: Error) {}
    func showUpdateNotFoundWithError(_ error: Error, acknowledgement: @escaping () -> Void) {
        acknowledgement()
        remoteMessage("Обновлений пока нет", "Установленная версия не старше опубликованной подписанной сборки.")
    }
    func showUpdaterError(_ error: Error, acknowledgement: @escaping () -> Void) {
        acknowledgement()
        remoteMessage("Обновление не установлено", error.localizedDescription + " Программа осталась прежней.")
    }
    func showDownloadInitiated(cancellation: @escaping () -> Void) { received = 0; expected = 0 }
    func showDownloadDidReceiveExpectedContentLength(_ expectedContentLength: UInt64) { expected = expectedContentLength }
    func showDownloadDidReceiveData(ofLength length: UInt64) {
        received += length
        if running { sendRun(["phase": "downloading", "done": received, "total": expected]) }
    }
    func showDownloadDidStartExtractingUpdate() { if running { sendRun(["phase": "installing"]) } }
    func showExtractionReceivedProgress(_ progress: Double) {}
    func showReady(toInstallAndRelaunch reply: @escaping (SPUUserUpdateChoice) -> Void) {
        if running { sendRun(["phase": "restarting"]); reply(.install) } else { pendingChoice = reply }
    }
    func showInstallingUpdate(withApplicationTerminated applicationTerminated: Bool, retryTerminatingApplication: @escaping () -> Void) {
        if running { sendRun(["phase": "restarting"]) }
    }
    func showUpdateInstalledAndRelaunched(_ relaunched: Bool, acknowledgement: @escaping () -> Void) { acknowledgement() }
    func showUpdateInFocus() { window?.makeKeyAndOrderFront(nil); NSApp.activate(ignoringOtherApps: true) }
    func dismissUpdateInstallation() {
        // The session ended. A run that did not restart the program failed;
        // the old program stays.
        if running { failRun("Обновление не установлено. Программа осталась прежней.") }
        pendingChoice = nil
        availableVersion = nil; onStatus?(shownVersion)
    }
}
