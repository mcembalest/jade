import Foundation
import CoreGraphics
import ImageIO
import UniformTypeIdentifiers

let size = 1024
let context = CGContext(data:nil,width:size,height:size,bitsPerComponent:8,bytesPerRow:0,space:CGColorSpaceCreateDeviceRGB(),bitmapInfo:CGImageAlphaInfo.noneSkipLast.rawValue)!
context.setFillColor(CGColor(red:0.96,green:0.98,blue:0.95,alpha:1));context.fill(CGRect(x:0,y:0,width:size,height:size))
context.setFillColor(CGColor(red:0.07,green:0.38,blue:0.26,alpha:1))
context.move(to:CGPoint(x:512,y:165));context.addLine(to:CGPoint(x:824,y:512));context.addLine(to:CGPoint(x:512,y:859));context.addLine(to:CGPoint(x:200,y:512));context.closePath();context.fillPath()
context.setFillColor(CGColor(red:0.33,green:0.65,blue:0.45,alpha:1))
context.move(to:CGPoint(x:512,y:859));context.addLine(to:CGPoint(x:512,y:512));context.addLine(to:CGPoint(x:200,y:512));context.closePath();context.fillPath()
let url=URL(fileURLWithPath:CommandLine.arguments[1]);try FileManager.default.createDirectory(at:url.deletingLastPathComponent(),withIntermediateDirectories:true)
let destination=CGImageDestinationCreateWithURL(url as CFURL,UTType.png.identifier as CFString,1,nil)!
CGImageDestinationAddImage(destination,context.makeImage()!,nil);CGImageDestinationFinalize(destination)
