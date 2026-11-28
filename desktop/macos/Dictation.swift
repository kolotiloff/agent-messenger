import AVFoundation
import CryptoKit

private struct DictationModel {
    let id: String
    let name: String
    let size: Int64
    let sha256: String

    var fileName: String { "ggml-\(id.dropFirst("whisper-".count)).bin" }
    var downloadURL: URL {
        URL(string: "https://huggingface.co/ggerganov/whisper.cpp/resolve/5359861c739e955e79d9a303bcbc70fb988958b1/")!
            .appendingPathComponent(fileName)
    }
}

// The tap writes mono PCM to a bounded buffer. Encoding and inference happen after the tap is removed.
private final class DictationAudio {
    let sampleRate: Double
    private let lock = NSLock()
    private let maxFrames: Int
    private var samples: [Float] = []
    private var hasVoiceLevel = false
    private var lastAudibleFrame = 0
    private var peakPower: Float = 0
    private var sealed = false

    init(sampleRate: Double) {
        self.sampleRate = sampleRate
        maxFrames = Int(sampleRate * 60)
        samples.reserveCapacity(maxFrames)
    }

    func append(_ buffer: AVAudioPCMBuffer) {
        guard let channels = buffer.floatChannelData else { return }
        let stride = buffer.stride
        lock.lock()
        defer { lock.unlock() }
        guard !sealed else { return }
        let count = min(Int(buffer.frameLength), maxFrames - samples.count)
        var power: Float = 0
        for index in 0..<count {
            let value = channels[0][index * stride]
            samples.append(value)
            power += value * value
        }
        guard count > 0 else { return }
        let meanPower = power / Float(count)
        peakPower = max(peakPower, meanPower)
        if meanPower > 0.0000063 {
            hasVoiceLevel = true
            lastAudibleFrame = samples.count
        }
    }

    func takeLevel() -> Int {
        lock.lock()
        let power = peakPower
        peakPower = 0
        lock.unlock()
        guard power > 0.0000063 else { return 0 }
        let decibels = 10 * log10(power)
        return Int(min(100, max(0, (decibels + 52) * 2.5)))
    }

    func silenceDuration() -> Double {
        lock.lock()
        defer { lock.unlock() }
        return Double(samples.count - lastAudibleFrame) / sampleRate
    }

    func finish() -> [Float] {
        lock.lock()
        defer { lock.unlock() }
        sealed = true
        return hasVoiceLevel ? samples : []
    }
}

// The controller and URLSession delegate state are used on the main queue.
final class ComposerDictation: NSObject, URLSessionDownloadDelegate, @unchecked Sendable {
    var onEvent: ((String, String, String?) -> Void)?
    var onModels: (([String: Any]) -> Void)?

    private static let catalog = [
        DictationModel(id: "whisper-tiny", name: "Whisper Tiny", size: 77_691_713,
                       sha256: "be07e048e1e599ad46341c8d2a135645097a538221678b7acdd1b1919c6e1b21"),
        DictationModel(id: "whisper-base", name: "Whisper Base", size: 147_951_465,
                       sha256: "60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe"),
        DictationModel(id: "whisper-small", name: "Whisper Small", size: 487_601_967,
                       sha256: "1be3a9b2063867b937e64e2ec7483364a79917e157fa98c5d94b5c1fffea987b"),
    ]
    private static let enabledKey = "voiceDictationEnabled"
    private static let modelKey = "voiceDictationModel"
    private let worker = DispatchQueue(label: "kotel.dictation", qos: .userInitiated, attributes: .concurrent)
    private let files = FileManager.default

    private struct Download {
        let task: URLSessionDownloadTask
        let staging: URL
        var bytesDownloaded: Int64 = 0
    }

    private lazy var downloads = URLSession(configuration: .ephemeral, delegate: self, delegateQueue: .main)
    private var activeDownloads: [String: Download] = [:]
    private var downloadErrors: [String: String] = [:]
    private var removalErrors: [String: String] = [:]

    private var id: String?
    private var engine: AVAudioEngine?
    private var audio: DictationAudio?
    private var modelForRecording: URL?
    private var microphoneTest = false
    private var language = "auto"
    private var process: Process?
    private var processDirectory: URL?

    private var executable: URL? {
        guard let url = Bundle.main.executableURL?.deletingLastPathComponent().appendingPathComponent("whisper-cli"),
              files.isExecutableFile(atPath: url.path) else { return nil }
        return url
    }

    private var modelDirectory: URL? {
        files.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?
            .appendingPathComponent(Bundle.main.bundleIdentifier ?? "io.github.kolotiloff.agent-messenger", isDirectory: true)
            .appendingPathComponent("Voice Models", isDirectory: true)
    }

    private var enabled: Bool { UserDefaults.standard.bool(forKey: Self.enabledKey) }
    private var selected: String? { UserDefaults.standard.string(forKey: Self.modelKey) }

    private func installedURL(_ model: DictationModel) -> URL? {
        guard let url = modelDirectory?.appendingPathComponent(model.fileName),
              let values = try? url.resourceValues(forKeys: [.isRegularFileKey, .fileSizeKey]),
              values.isRegularFile == true, values.fileSize.map(Int64.init) == model.size else { return nil }
        return url
    }

    private var selectedURL: URL? {
        Self.catalog.first(where: { $0.id == selected }).flatMap(installedURL)
    }

    func reportModels() {
        let supported = executable != nil
        let models: [[String: Any]] = Self.catalog.map { model in
            var row: [String: Any] = ["id": model.id, "name": model.name, "size": model.size]
            if let job = activeDownloads[model.id] {
                row["state"] = "downloading"
                row["bytesDownloaded"] = job.bytesDownloaded
            } else if installedURL(model) != nil {
                row["state"] = "installed"
                row["removalError"] = removalErrors[model.id]
            } else if let error = downloadErrors[model.id] {
                row["state"] = "error"
                row["error"] = error
            } else {
                row["state"] = "available"
            }
            return row
        }
        onModels?([
            "enabled": enabled, "ready": supported && enabled && selectedURL != nil,
            "supported": supported, "selected": selected as Any? ?? NSNull(), "models": models,
        ])
    }

    func setEnabled(_ enabled: Bool) {
        UserDefaults.standard.set(enabled, forKey: Self.enabledKey)
        if !enabled { cancel() }
        reportModels()
    }

    func selectModel(_ id: String) {
        guard let model = Self.catalog.first(where: { $0.id == id }), installedURL(model) != nil else {
            reportModels()
            return
        }
        UserDefaults.standard.set(id, forKey: Self.modelKey)
        reportModels()
    }

    func removeModel(_ id: String) {
        removalErrors[id] = nil
        defer { reportModels() }
        guard let model = Self.catalog.first(where: { $0.id == id }), let url = installedURL(model) else { return }
        guard modelForRecording != url else {
            removalErrors[id] = "recording"
            return
        }
        do {
            try files.removeItem(at: url)
            if selected == id { UserDefaults.standard.removeObject(forKey: Self.modelKey) }
        } catch {
            removalErrors[id] = "storage"
        }
    }

    func downloadModel(_ id: String) {
        guard executable != nil, let model = Self.catalog.first(where: { $0.id == id }),
              activeDownloads[id] == nil, installedURL(model) == nil else { reportModels(); return }
        do {
            guard let directory = modelDirectory else { throw CocoaError(.fileNoSuchFile) }
            try files.createDirectory(at: directory, withIntermediateDirectories: true,
                                      attributes: [.posixPermissions: 0o700])
            let task = downloads.downloadTask(with: model.downloadURL)
            let staging = directory.appendingPathComponent(".\(UUID().uuidString).download")
            activeDownloads[id] = Download(task: task, staging: staging)
            downloadErrors[id] = nil
            task.resume()
        } catch {
            downloadErrors[id] = "storage-error"
        }
        reportModels()
    }

    func cancelDownload(_ id: String) {
        guard let job = activeDownloads.removeValue(forKey: id) else { reportModels(); return }
        job.task.cancel()
        try? files.removeItem(at: job.staging)
        reportModels()
    }

    private func downloadModel(for task: URLSessionTask) -> DictationModel? {
        Self.catalog.first(where: { activeDownloads[$0.id]?.task === task })
    }

    func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didWriteData bytesWritten: Int64,
                    totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64) {
        guard let model = downloadModel(for: downloadTask), var job = activeDownloads[model.id] else { return }
        guard totalBytesWritten <= model.size else {
            downloadTask.cancel()
            finishDownload(model, error: "checksum-failed")
            return
        }
        guard 100 * totalBytesWritten / model.size > 100 * job.bytesDownloaded / model.size else { return }
        job.bytesDownloaded = totalBytesWritten
        activeDownloads[model.id] = job
        reportModels()
    }

    func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {
        guard let model = downloadModel(for: downloadTask), let job = activeDownloads[model.id] else { return }
        guard let response = downloadTask.response as? HTTPURLResponse, (200..<300).contains(response.statusCode) else {
            finishDownload(model, error: "download-failed")
            return
        }
        do {
            try files.moveItem(at: location, to: job.staging)
        } catch {
            finishDownload(model, error: "storage-error")
            return
        }
        worker.async {
            let validationError: String?
            do {
                validationError = try Self.verify(job.staging, model: model) ? nil : "checksum-failed"
            } catch {
                validationError = "storage-error"
            }
            DispatchQueue.main.async {
                guard self.activeDownloads[model.id]?.task === downloadTask else { return }
                self.finishDownload(model, error: validationError)
            }
        }
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
        if error != nil, let model = downloadModel(for: task) {
            finishDownload(model, error: "download-failed")
        }
    }

    private static func verify(_ url: URL, model: DictationModel) throws -> Bool {
        guard try url.resourceValues(forKeys: [.fileSizeKey]).fileSize.map(Int64.init) == model.size else { return false }
        let file = try FileHandle(forReadingFrom: url)
        defer { try? file.close() }
        var hash = SHA256()
        while let chunk = try file.read(upToCount: 1_048_576), !chunk.isEmpty { hash.update(data: chunk) }
        return hash.finalize().map { String(format: "%02x", $0) }.joined() == model.sha256
    }

    private func finishDownload(_ model: DictationModel, error: String?) {
        guard let job = activeDownloads.removeValue(forKey: model.id) else { return }
        defer {
            try? files.removeItem(at: job.staging)
            reportModels()
        }
        guard error == nil else { downloadErrors[model.id] = error; return }
        do {
            let destination = job.staging.deletingLastPathComponent().appendingPathComponent(model.fileName)
            if files.fileExists(atPath: destination.path) {
                _ = try files.replaceItemAt(destination, withItemAt: job.staging)
            } else {
                try files.moveItem(at: job.staging, to: destination)
            }
            if selectedURL == nil { UserDefaults.standard.set(model.id, forKey: Self.modelKey) }
        } catch {
            downloadErrors[model.id] = "storage-error"
        }
    }

    func start(_ id: String, language: String, silence: Int) {
        cancel()
        self.id = id
        guard enabled, executable != nil, let model = selectedURL else {
            complete(id, event: "model-not-ready")
            return
        }
        modelForRecording = model
        requestRecording(id, language: language, silence: silence)
    }

    func testMicrophone(_ id: String, language: String) {
        cancel()
        self.id = id
        microphoneTest = true
        modelForRecording = selectedURL
        requestRecording(id, language: language, silence: 0)
    }

    private func requestRecording(_ id: String, language: String, silence: Int) {
        AVCaptureDevice.requestAccess(for: .audio) { [weak self] granted in
            DispatchQueue.main.async {
                guard let self, self.id == id else { return }
                guard granted else { self.complete(id, event: "microphone-denied"); return }
                self.record(id, language: language, silence: silence)
            }
        }
    }

    private func record(_ id: String, language: String, silence: Int) {
        let engine = AVAudioEngine()
        let input = engine.inputNode
        let format = input.outputFormat(forBus: 0)
        guard format.channelCount > 0, format.sampleRate.isFinite, (1...384_000).contains(format.sampleRate),
              format.commonFormat == .pcmFormatFloat32 else {
            complete(id, event: "microphone-unavailable")
            return
        }
        let audio = DictationAudio(sampleRate: format.sampleRate)
        self.audio = audio
        self.engine = engine
        input.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in audio.append(buffer) }
        engine.prepare()
        do { try engine.start() }
        catch { complete(id, event: "microphone-unavailable"); return }
        self.language = language
        let duration = microphoneTest ? 10 : 60
        onEvent?(id, "recording", microphoneTest ? String(duration) : nil)
        if !microphoneTest { watchLevel(id) }
        DispatchQueue.main.asyncAfter(deadline: .now() + .seconds(duration)) { [weak self] in self?.stop(id) }
        if silence > 0 { watchSilence(id, seconds: Double(silence)) }
    }

    private func watchLevel(_ id: String) {
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) { [weak self] in
            guard let self, self.id == id, let audio = self.audio else { return }
            self.onEvent?(id, "level", String(audio.takeLevel()))
            self.watchLevel(id)
        }
    }

    private func watchSilence(_ id: String, seconds: Double) {
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) { [weak self] in
            guard let self, self.id == id, let audio = self.audio else { return }
            if audio.silenceDuration() >= seconds { self.stop(id) }
            else { self.watchSilence(id, seconds: seconds) }
        }
    }

    func stop(_ id: String) {
        guard self.id == id, let audio else { return }
        let isTest = microphoneTest
        let model = modelForRecording
        let executable = self.executable
        guard isTest || (model != nil && executable != nil) else {
            complete(id, event: "model-not-ready")
            return
        }
        releaseMicrophone()
        self.audio = nil
        let samples = audio.finish()
        let language = self.language
        guard !samples.isEmpty else { complete(id, event: "empty"); return }
        onEvent?(id, "processing", nil)
        worker.async {
            let directory = FileManager.default.temporaryDirectory.appendingPathComponent("kotel-dictation-\(UUID().uuidString)", isDirectory: true)
            do {
                try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: false,
                                                        attributes: [.posixPermissions: 0o700])
                let wave = directory.appendingPathComponent("audio.wav")
                try Self.writeAudio(samples, sampleRate: audio.sampleRate, to: wave)
                let playback = isTest ? try Data(contentsOf: wave).base64EncodedString() : nil
                DispatchQueue.main.async {
                    guard self.id == id else { try? self.files.removeItem(at: directory); return }
                    if let playback { self.onEvent?(id, "microphone-audio", playback) }
                    if let executable, let model {
                        self.transcribe(id, executable: executable, model: model, language: language, directory: directory, wave: wave)
                    } else {
                        try? self.files.removeItem(at: directory)
                        self.complete(id, event: "microphone-ready")
                    }
                }
            } catch {
                try? FileManager.default.removeItem(at: directory)
                DispatchQueue.main.async {
                    if self.id == id { self.complete(id, event: "failed") }
                }
            }
        }
    }

    private static func writeAudio(_ samples: [Float], sampleRate: Double, to url: URL) throws {
        let targetRate = 16_000.0
        guard let inputFormat = AVAudioFormat(standardFormatWithSampleRate: sampleRate, channels: 1),
              let outputFormat = AVAudioFormat(standardFormatWithSampleRate: targetRate, channels: 1),
              let input = AVAudioPCMBuffer(pcmFormat: inputFormat, frameCapacity: AVAudioFrameCount(samples.count)),
              let channel = input.floatChannelData?[0] else { throw CocoaError(.fileWriteUnknown) }
        samples.withUnsafeBufferPointer { source in
            if let base = source.baseAddress { channel.update(from: base, count: source.count) }
        }
        input.frameLength = AVAudioFrameCount(samples.count)
        let output: AVAudioPCMBuffer
        if sampleRate == targetRate {
            output = input
        } else {
            let capacity = AVAudioFrameCount(ceil(Double(samples.count) * targetRate / sampleRate) + 1024)
            guard let converted = AVAudioPCMBuffer(pcmFormat: outputFormat, frameCapacity: capacity),
                  let converter = AVAudioConverter(from: inputFormat, to: outputFormat) else {
                throw CocoaError(.fileWriteUnknown)
            }
            var pending = true
            var conversionError: NSError?
            let status = converter.convert(to: converted, error: &conversionError) { _, state in
                guard pending else { state.pointee = .endOfStream; return nil }
                pending = false
                state.pointee = .haveData
                return input
            }
            guard status != .error, conversionError == nil, converted.frameLength > 0 else {
                throw conversionError ?? CocoaError(.fileWriteUnknown) as NSError
            }
            output = converted
        }
        let file = try AVAudioFile(forWriting: url, settings: [
            AVFormatIDKey: kAudioFormatLinearPCM, AVSampleRateKey: targetRate, AVNumberOfChannelsKey: 1,
            AVLinearPCMBitDepthKey: 16, AVLinearPCMIsFloatKey: false,
        ], commonFormat: .pcmFormatFloat32, interleaved: false)
        try file.write(from: output)
    }

    private func transcribe(_ id: String, executable: URL, model: URL, language: String, directory: URL, wave: URL) {
        let process = Process()
        let output = directory.appendingPathComponent("transcript")
        process.executableURL = executable
        process.arguments = [
            "-m", model.path, "-f", wave.path, "-l", language, "-ng", "-np", "-nt", "-sns",
            "-otxt", "-of", output.path,
        ]
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        process.terminationHandler = { [weak self] finished in
            let text = finished.terminationStatus == 0
                ? (try? String(contentsOf: output.appendingPathExtension("txt"), encoding: .utf8))?
                    .trimmingCharacters(in: .whitespacesAndNewlines)
                : nil
            try? FileManager.default.removeItem(at: directory)
            DispatchQueue.main.async {
                guard let self, self.id == id, self.process === finished else { return }
                self.process = nil
                self.processDirectory = nil
                guard let text else { self.complete(id, event: "failed"); return }
                self.complete(id, event: text.isEmpty ? "empty" : "text", text: text.isEmpty ? nil : text)
            }
        }
        do {
            try process.run()
            self.process = process
            processDirectory = directory
        } catch {
            try? files.removeItem(at: directory)
            complete(id, event: "failed")
            return
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 180) { [weak self, weak process] in
            guard let self, let process, self.id == id, self.process === process else { return }
            self.complete(id, event: "failed")
        }
    }

    private func releaseMicrophone() {
        guard let engine else { return }
        engine.stop()
        engine.inputNode.removeTap(onBus: 0)
        self.engine = nil
    }

    private func complete(_ id: String, event: String, text: String? = nil) {
        cancel()
        onEvent?(id, event, text)
    }

    func cancel(_ id: String? = nil) {
        guard id == nil || self.id == id else { return }
        self.id = nil
        modelForRecording = nil
        microphoneTest = false
        releaseMicrophone()
        audio = nil
        if process?.isRunning == true { process?.terminate() }
        process = nil
        if let processDirectory { try? files.removeItem(at: processDirectory) }
        processDirectory = nil
    }
}
