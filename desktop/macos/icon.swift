import AppKit

let source = URL(fileURLWithPath: CommandLine.arguments[1])
let directory = URL(fileURLWithPath: CommandLine.arguments[2])
let master = NSImage(data: try Data(contentsOf: source))!
// The artwork includes transparent margins. Match the footprint of adjacent Dock tiles.
let dockScale: CGFloat = 1.025

// Follow the existing alpha edge so the reflection scales with the artwork.
func applyEdgeSheen(to bitmap: NSBitmapImageRep) {
    guard let data = bitmap.bitmapData else { return }
    let width = bitmap.pixelsWide
    let height = bitmap.pixelsHigh
    let band = max(1, Double(width) * 0.045)
    for y in 0..<height {
        let position = (Double(y) + 0.5) / Double(height)
        let row = data.advanced(by: y * bitmap.bytesPerRow)
        for direction in [1, -1] {
            // One reflection rolls over the upper shoulder, the other fades lower on the side.
            let center = direction > 0 ? 0.32 : 0.58
            let vertical = max(0, 1 - pow((position - center) / 0.35, 2))
            guard vertical > 0 else { continue }
            var edge = direction > 0 ? 0 : width - 1
            while edge >= 0, edge < width, row[edge * 4 + 3] < 200 {
                edge += direction
            }
            guard edge >= 0, edge < width else { continue }
            for offset in 0..<Int(ceil(band)) {
                let x = edge + direction * offset
                guard x >= 0, x < width else { break }
                let pixel = row.advanced(by: x * 4)
                let alpha = Double(pixel[3])
                let amount = 0.24 * vertical * pow(1 - Double(offset) / band, 3)
                for channel in 0..<3 {
                    let value = Double(pixel[channel])
                    pixel[channel] = UInt8((value + (max(value, alpha * 0.70) - value) * amount).rounded())
                }
            }
        }
    }
}

try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
for size in [16, 32, 128, 256, 512] {
    for scale in [1, 2] {
        let pixels = size * scale
        let rect = NSRect(x: 0, y: 0, width: pixels, height: pixels)
        let inset = rect.width * (1 - dockScale) / 2
        let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: pixels, pixelsHigh: pixels,
            bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
            colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
        NSGraphicsContext.current?.imageInterpolation = .high
        master.draw(in: rect.insetBy(dx: inset, dy: inset), from: .zero, operation: .copy, fraction: 1)
        NSGraphicsContext.restoreGraphicsState()
        applyEdgeSheen(to: bitmap)
        let suffix = scale == 2 ? "@2x" : ""
        let name = "icon_\(size)x\(size)\(suffix).png"
        try bitmap.representation(using: .png, properties: [:])!.write(to: directory.appendingPathComponent(name))
    }
}
