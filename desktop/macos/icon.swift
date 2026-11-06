import AppKit
let directory = URL(fileURLWithPath: CommandLine.arguments[1])
try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
for size in [16,32,128,256,512] {
 for scale in [1,2] {
  let n = size*scale
  let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: n, pixelsHigh: n, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
  NSGraphicsContext.saveGraphicsState();NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
  let t = AffineTransform(scale: CGFloat(n)/512);(t as NSAffineTransform).concat()
  NSColor(calibratedRed: 0.12, green: 0.14, blue: 0.19, alpha: 1).setFill()
  NSBezierPath(roundedRect: NSRect(x:24,y:24,width:464,height:464), xRadius:104,yRadius:104).fill()
  NSColor(calibratedRed:0.52,green:0.66,blue:0.94,alpha:1).setFill()
  NSBezierPath(roundedRect:NSRect(x:94,y:177,width:260,height:207),xRadius:44,yRadius:44).fill()
  let tail=NSBezierPath();tail.move(to:NSPoint(x:132,y:190));tail.line(to:NSPoint(x:132,y:125));tail.line(to:NSPoint(x:208,y:190));tail.close();tail.fill()
  NSColor(calibratedWhite:0.95,alpha:1).setFill()
  NSBezierPath(roundedRect:NSRect(x:235,y:100,width:182,height:143),xRadius:32,yRadius:32).fill()
  NSGraphicsContext.restoreGraphicsState()
  let suffix = scale==2 ? "@2x" : ""
  try bitmap.representation(using:.png,properties:[:])!.write(to:directory.appendingPathComponent("icon_\(size)x\(size)\(suffix).png"))
 }
}
