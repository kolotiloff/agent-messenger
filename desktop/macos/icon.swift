import AppKit
let directory = URL(fileURLWithPath: CommandLine.arguments[1])
try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
// The same shapes as internal/localui/assets/icon.svg, with y flipped
// (AppKit counts from the bottom).
func circle(_ x: CGFloat, _ y: CGFloat, _ r: CGFloat) { NSBezierPath(ovalIn: NSRect(x: x-r, y: 512-y-r, width: 2*r, height: 2*r)).fill() }
let ink = NSColor(srgbRed: 0x1B/255.0, green: 0x1B/255.0, blue: 0x2F/255.0, alpha: 1)
for size in [16,32,128,256,512] {
 for scale in [1,2] {
  let n = size*scale
  let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: n, pixelsHigh: n, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
  NSGraphicsContext.saveGraphicsState();NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
  let t = AffineTransform(scale: CGFloat(n)/512);(t as NSAffineTransform).concat()
  NSColor(srgbRed: 0xFF/255.0, green: 0xC5/255.0, blue: 0x31/255.0, alpha: 1).setFill()
  NSBezierPath(roundedRect: NSRect(x:24,y:24,width:464,height:464), xRadius:104,yRadius:104).fill()
  ink.setFill()
  NSBezierPath(roundedRect:NSRect(x:80,y:150,width:352,height:242),xRadius:56,yRadius:56).fill()
  let tail=NSBezierPath();tail.move(to:NSPoint(x:158,y:150));tail.line(to:NSPoint(x:236,y:150));tail.line(to:NSPoint(x:158,y:86));tail.close();tail.fill()
  NSColor.white.setFill()
  circle(200,236,34);circle(312,236,34)
  ink.setFill()
  circle(210,244,14);circle(322,244,14)
  NSGraphicsContext.restoreGraphicsState()
  let suffix = scale==2 ? "@2x" : ""
  try bitmap.representation(using:.png,properties:[:])!.write(to:directory.appendingPathComponent("icon_\(size)x\(size)\(suffix).png"))
 }
}
