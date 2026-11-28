import AppKit

let source = URL(fileURLWithPath: CommandLine.arguments[1])
let directory = URL(fileURLWithPath: CommandLine.arguments[2])
let master = NSImage(data: try Data(contentsOf: source))!
// The artwork has built-in transparency. Fill about 87% of the Dock tile.
let dockScale: CGFloat = 1.08

// Follow the existing alpha edge so the reflection scales with the artwork.
func applyEdgeSheen(to bitmap: NSBitmapImageRep) {
    guard let data = bitmap.bitmapData else { return }
    let width = bitmap.pixelsWide
    let height = bitmap.pixelsHigh
    let band = max(1, Double(width) * 0.014)
    for y in 0..<height {
        let position = (Double(y) + 0.5) / Double(height)
        let vertical = max(0, 1 - pow((position - 0.43) / 0.38, 2))
        guard vertical > 0 else { continue }
        let row = data.advanced(by: y * bitmap.bytesPerRow)
        for direction in [1, -1] {
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
                let amount = 0.12 * vertical * pow(1 - Double(offset) / band, 2)
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
