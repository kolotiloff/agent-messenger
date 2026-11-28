import Foundation

@main
enum CheckMacBundleName {
    static func main() throws {
        let installed = URL(fileURLWithPath: CommandLine.arguments[1], isDirectory: true)
        let parent = installed.deletingLastPathComponent()
        let renamed = parent.appendingPathComponent(BundleName.current, isDirectory: true)
        let files = FileManager.default

        // An existing app at the new name is never replaced.
        try files.createDirectory(at: renamed, withIntermediateDirectories: false)
        precondition(BundleName.migrate(installed) == nil)
        precondition(Bundle(url: installed)?.bundleIdentifier != nil)
        try files.removeItem(at: renamed)

        let bookmark = try installed.bookmarkData()
        let fileID = try installed.resourceValues(forKeys: [.fileIdentifierKey]).fileIdentifier
        precondition(fileID != nil)
        precondition(BundleName.migrate(installed)?.path == renamed.path)
        let compatibilityPath = try files.destinationOfSymbolicLink(atPath: installed.path)
        precondition(compatibilityPath == "./Kotel.app")
        precondition(BundleName.migrate(installed) == nil)
        precondition(BundleName.migrate(renamed) == nil)
        let renamedID = try renamed.resourceValues(forKeys: [.fileIdentifierKey]).fileIdentifier
        precondition(renamedID == fileID)

        var stale = false
        let resolved = try URL(resolvingBookmarkData: bookmark, options: [.withoutUI, .withoutMounting], bookmarkDataIsStale: &stale)
        precondition(resolved.resolvingSymlinksInPath().path == renamed.path)

        // Recover an interruption between staging the symlink and swapping it.
        try files.removeItem(at: installed)
        try files.moveItem(at: renamed, to: installed)
        try files.createSymbolicLink(atPath: renamed.path, withDestinationPath: "./Kotel.app")
        precondition(BundleName.migrate(installed)?.path == renamed.path)
        print("Bundle file identity, saved bookmark, legacy path and name collision verified")
    }
}
