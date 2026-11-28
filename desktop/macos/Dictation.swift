import AVFoundation
import Speech

// Used from the main queue. Audio goes directly from the input tap to Apple's
// on-device recognizer. Only the resulting text is sent to the composer.
final class ComposerDictation {
    var onEvent: ((String, String, String?) -> Void)?

    private var id: String?
    private var engine: AVAudioEngine?
    private var recognizer: SFSpeechRecognizer?
    private var request: SFSpeechAudioBufferRecognitionRequest?
    private var task: SFSpeechRecognitionTask?
    private var transcript = ""
    private var finishing = false

    func start(_ id: String, locale: Locale) {
        cancel()
        self.id = id
        guard let recognizer = SFSpeechRecognizer(locale: locale),
              recognizer.supportsOnDeviceRecognition, recognizer.isAvailable else {
            complete(id, event: "unavailable")
            return
        }
        AVCaptureDevice.requestAccess(for: .audio) { [weak self] granted in
            DispatchQueue.main.async {
                guard let self, self.id == id else { return }
                guard granted else { self.complete(id, event: "microphone-denied"); return }
                if SFSpeechRecognizer.authorizationStatus() == .authorized {
                    self.record(id, recognizer: recognizer)
                } else {
                    SFSpeechRecognizer.requestAuthorization { status in
                        DispatchQueue.main.async {
                            guard self.id == id else { return }
                            guard status == .authorized else { self.complete(id, event: "speech-denied"); return }
                            self.record(id, recognizer: recognizer)
                        }
                    }
                }
            }
        }
    }

    private func record(_ id: String, recognizer: SFSpeechRecognizer) {
        let engine = AVAudioEngine()
        let input = engine.inputNode
        let format = input.outputFormat(forBus: 0)
        guard format.channelCount > 0, format.sampleRate > 0 else {
            complete(id, event: "microphone-unavailable")
            return
        }
        let request = SFSpeechAudioBufferRecognitionRequest()
        request.requiresOnDeviceRecognition = true
        request.shouldReportPartialResults = true
        request.addsPunctuation = true
        self.request = request
        self.engine = engine
        self.recognizer = recognizer
        input.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in request.append(buffer) }
        engine.prepare()
        do { try engine.start() }
        catch { complete(id, event: "microphone-unavailable"); return }
        task = recognizer.recognitionTask(with: request) { [weak self] result, error in
            DispatchQueue.main.async {
                guard let self, self.id == id else { return }
                if let result { self.transcript = result.bestTranscription.formattedString }
                if result?.isFinal == true || error != nil {
                    self.finishTranscript(id, fallback: self.fallbackEvent(for: error))
                }
            }
        }
        onEvent?(id, "recording", nil)
        DispatchQueue.main.asyncAfter(deadline: .now() + 60) { [weak self] in self?.stop(id) }
    }

    private func fallbackEvent(for error: Error?) -> String {
        var underlying = error.map { $0 as NSError }
        while let current = underlying {
            // Apple documents this error for disabled Siri or Dictation.
            if current.domain == "kLSRErrorDomain", current.code == 201 {
                return "dictation-disabled"
            }
            underlying = current.userInfo[NSUnderlyingErrorKey] as? NSError
        }
        return error == nil || finishing ? "empty" : "failed"
    }

    func stop(_ id: String) {
        guard self.id == id, !finishing else { return }
        guard let request else { complete(id, event: "empty"); return }
        finishing = true
        releaseMicrophone()
        request.endAudio()
        onEvent?(id, "processing", nil)
        DispatchQueue.main.asyncAfter(deadline: .now() + 4) { [weak self] in
            guard let self, self.id == id else { return }
            self.finishTranscript(id, fallback: "empty")
        }
    }

    private func finishTranscript(_ id: String, fallback: String) {
        let text = transcript.trimmingCharacters(in: .whitespacesAndNewlines)
        complete(id, event: text.isEmpty ? fallback : "text", text: text.isEmpty ? nil : text)
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
        releaseMicrophone()
        if !finishing { request?.endAudio() }
        task?.cancel()
        request = nil
        task = nil
        recognizer = nil
        transcript = ""
        finishing = false
    }
}
