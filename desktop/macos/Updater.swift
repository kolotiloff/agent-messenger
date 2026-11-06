import AppKit
import Sparkle
import Darwin

// Sparkle owns download verification, installation and relaunch. The feed is
// served by our authenticated loopback bridge from signed local metadata.
final class DesktopUpdater: NSObject, SPUUpdaterDelegate, SPUStandardUserDriverDelegate {
    weak var window: NSWindow?
    var endpoint: URL?
    var controller: SPUStandardUpdaterController!
    var availableVersion: String?
    var onStatus: ((String?) -> Void)?
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
        controller = SPUStandardUpdaterController(startingUpdater: false, updaterDelegate: self, userDriverDelegate: self)
    }
    func attach(menu: NSMenu) {
        checkItem = menu.addItem(withTitle: "Проверить обновления…", action: #selector(check), keyEquivalent: "")
        checkItem?.target = self
        automaticItem = menu.addItem(withTitle: "Проверять обновления автоматически", action: #selector(toggleAutomatic), keyEquivalent: "")
        automaticItem?.target = self
        let source = menu.addItem(withTitle: "Канал: GitHub Releases · подписанные пакеты", action: nil, keyEquivalent: "")
        source.isEnabled = false
        menu.addItem(.separator())
    }
    func connect(_ url: URL, window: NSWindow) {
        self.window = window
        endpoint = url
        controller.updater.httpHeaders = ["Authorization": "Bearer " + (url.fragment ?? "")]
        if !started {
            controller.startUpdater()
            started = true
            startWatchingLocalChannel()
            fetchTimer = Timer.scheduledTimer(withTimeInterval: 300, repeats: true) { [weak self] _ in
                self?.checkAutomaticallyIfDue()
            }
            fetchTimer?.tolerance = 15
            checkAutomaticallyIfDue()
            if controller.updater.automaticallyChecksForUpdates && !controller.updater.sessionInProgress {
                controller.updater.checkForUpdatesInBackground()
            }
        }
        automaticItem?.state = controller.updater.automaticallyChecksForUpdates ? .on : .off
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
                  self.controller.updater.automaticallyChecksForUpdates,
                  !self.controller.updater.sessionInProgress,
                  let data = self.manifestData(), data != self.lastManifest else { return }
            self.lastManifest = data
            // Local publication is an event, not model polling. The bridge verifies
            // the signed metadata; Sparkle verifies the archive before install.
            self.controller.updater.checkForUpdatesInBackground()
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
        // A gentle background reminder keeps the Sparkle session open. Bring
        // that verified update forward instead of blocking on sessionInProgress.
        if availableVersion != nil, controller.updater.canCheckForUpdates {
            busyRetry?.cancel(); waitingForDownload = false; busySince = nil
            controller.checkForUpdates(nil)
        } else {
            fetchRemote(manual: true)
        }
    }
    // Focus changes share the same throttle as the timer; waiting never calls a model.
    func checkAutomaticallyIfDue() {
        guard started, endpoint != nil, controller.updater.automaticallyChecksForUpdates,
              ProcessInfo.processInfo.systemUptime - lastRemoteCheck >= 300 else { return }
        fetchRemote(manual: false)
    }
    func fetchRemote(manual: Bool) {
        guard !fetching, endpoint != nil, !controller.updater.sessionInProgress,
              let executable = Bundle.main.url(forResource: "messenger", withExtension: nil) else { return }
        busyRetry?.cancel(); waitingForDownload = false
        lastRemoteCheck = ProcessInfo.processInfo.systemUptime
        fetching = true; fetchStatus = "Проверяем GitHub…"; onStatus?(availableVersion)
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
                        self.fetchStatus = "Загрузка ещё выполняется · повторить"
                        self.onStatus?(self.availableVersion)
                        return
                    }
                    self.waitingForDownload = true
                    self.fetchStatus = "Обновление загружается в фоне"
                    self.onStatus?(self.availableVersion)
                    let retry = DispatchWorkItem { [weak self] in
                        guard let self = self else { return }
                        self.waitingForDownload = false
                        self.onStatus?(self.availableVersion)
                        if (manual || self.controller.updater.automaticallyChecksForUpdates) && !self.controller.updater.sessionInProgress { self.fetchRemote(manual: manual) }
                    }
                    self.busyRetry = retry
                    DispatchQueue.main.asyncAfter(deadline: .now() + 5, execute: retry)
                    return
                }
                self.busySince = nil
                if child.terminationStatus != 0 {
                    self.fetchStatus = "Не удалось получить обновление · повторить"; self.onStatus?(self.availableVersion)
                    if manual { self.remoteMessage("Не удалось получить обновление", "Загрузка или проверка пакета не завершилась. Повторите позже. Сохранённый пакет и работающий клиент не изменены; неподписанные пакеты не принимаются.") }
                    return
                }
                if result?["state"] as? String == "no_release" {
                    self.fetchStatus = "На GitHub ещё нет сборки"; self.onStatus?(self.availableVersion)
                    if manual { self.remoteMessage("Пакет ещё не опубликован", "В GitHub Releases пока нет подписанной сборки для этого компьютера.") }
                    return
                }
                self.fetchStatus = ""; self.onStatus?(self.availableVersion)
                if result?["state"] as? String == "current", self.manifestData() == nil {
                    if manual { self.remoteMessage("Обновлений пока нет", "Установленная версия не старше опубликованной подписанной сборки.") }
                    return
                }
                guard self.endpoint != nil, !self.controller.updater.sessionInProgress else { return }
                if manual { self.controller.checkForUpdates(nil) }
                else if self.controller.updater.automaticallyChecksForUpdates { self.controller.updater.checkForUpdatesInBackground() }
            }
        }
        fetchProcess = process
        do { try process.run(); try? output.fileHandleForWriting.close(); try? errors.fileHandleForWriting.close() }
        catch { fetching = false; fetchProcess = nil; fetchStatus = "Проверка не запустилась"; onStatus?(availableVersion) }
    }
    func remoteMessage(_ title: String, _ text: String) {
        let alert = NSAlert(); alert.messageText = title; alert.informativeText = text
        if let window = window { alert.beginSheetModal(for: window) } else { alert.runModal() }
    }
    @objc func toggleAutomatic() {
        controller.updater.automaticallyChecksForUpdates.toggle()
        automaticItem?.state = controller.updater.automaticallyChecksForUpdates ? .on : .off
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
    var supportsGentleScheduledUpdateReminders: Bool { true }
    func standardUserDriverShouldHandleShowingScheduledUpdate(_ update: SUAppcastItem, andInImmediateFocus immediateFocus: Bool) -> Bool { false }
    func standardUserDriverWillHandleShowingUpdate(_ handleShowingUpdate: Bool, forUpdate update: SUAppcastItem, state: SPUUserUpdateState) {
        availableVersion = update.displayVersionString
        onStatus?(availableVersion)
    }
    func standardUserDriverWillFinishUpdateSession() {
        availableVersion = nil; onStatus?(nil)
    }
    func updater(_ updater: SPUUpdater, shouldDownloadReleaseNotesForUpdate updateItem: SUAppcastItem) -> Bool { false }
    func standardUserDriverShouldShowVersionHistory(for item: SUAppcastItem) -> Bool { false }
}
