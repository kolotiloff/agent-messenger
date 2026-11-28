import Foundation
import Darwin

enum BundleName {
    static let legacy = "Agent Messenger.app"
    static let current = "Kotel.app"
    private static let compatibilityTarget = "./Kotel.app"
    private static let identifier = "io.github.kolotiloff.agent-messenger"

    // Sparkle replaces the installed bundle at its existing path. Keep the
    // legacy path usable for managed commands and bookmarks, including later
    // Sparkle updates that replace the directory at the new path.
    static func migrate(_ installedBundle: URL) -> URL? {
        let bundle = installedBundle.standardizedFileURL.resolvingSymlinksInPath()
        guard bundle.lastPathComponent == legacy,
              Bundle(url: bundle)?.bundleIdentifier == identifier else { return nil }

        let parent = bundle.deletingLastPathComponent()
        let destination = parent.appendingPathComponent(current, isDirectory: true)
        let directory = open(parent.path, O_RDONLY | O_DIRECTORY | O_CLOEXEC | O_NOFOLLOW)
        guard directory >= 0 else { return nil }
        defer { close(directory) }

        let source = openat(directory, legacy, O_RDONLY | O_DIRECTORY | O_CLOEXEC | O_NOFOLLOW)
        guard source >= 0 else { return nil }
        defer { close(source) }
        // Keep two concurrent launches from moving the same installation.
        guard flock(source, LOCK_EX | LOCK_NB) == 0 else { return nil }
        var opened = stat(), named = stat()
        guard fstat(source, &opened) == 0,
              fstatat(directory, legacy, &named, AT_SYMLINK_NOFOLLOW) == 0,
              named.st_dev == opened.st_dev, named.st_ino == opened.st_ino else { return nil }

        // Stage a self-link at the new name, then exchange names atomically.
        // The old path resolves to the original bundle before and after the
        // exchange. A process interrupted before the exchange can resume it.
        let staged = symlinkat(compatibilityTarget, directory, current) == 0
        guard staged || (errno == EEXIST && isStagingLink(directory)) else { return nil }
        guard renameatx_np(directory, legacy, directory, current, UInt32(RENAME_SWAP)) == 0 else {
            if isStagingLink(directory) { _ = unlinkat(directory, current, 0) }
            return nil
        }
        return destination
    }

    private static func isStagingLink(_ directory: Int32) -> Bool {
        var info = stat()
        guard fstatat(directory, current, &info, AT_SYMLINK_NOFOLLOW) == 0,
              info.st_uid == geteuid() else { return false }
        var bytes = [UInt8](repeating: 0, count: 64)
        let length = bytes.withUnsafeMutableBytes { buffer in
            readlinkat(directory, current, buffer.baseAddress!.assumingMemoryBound(to: CChar.self), buffer.count)
        }
        return length > 0 && String(decoding: bytes.prefix(length), as: UTF8.self) == compatibilityTarget
    }
}
