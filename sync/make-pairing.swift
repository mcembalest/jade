// Produce a private QR code locally. No QR service receives the pairing key.
import Foundation
import CoreImage
import ImageIO
import UniformTypeIdentifiers

struct Config: Decodable { var endpoint: String; var token: String }
let config = try JSONDecoder().decode(Config.self, from: Data(contentsOf:URL(fileURLWithPath:CommandLine.arguments[1])))
var components=URLComponents();components.scheme="jade";components.host="pair"
components.queryItems=[URLQueryItem(name:"endpoint",value:config.endpoint),URLQueryItem(name:"token",value:config.token)]
let filter=CIFilter(name:"CIQRCodeGenerator")!;filter.setValue(components.url!.absoluteString.data(using:.utf8)!,forKey:"inputMessage");filter.setValue("M",forKey:"inputCorrectionLevel")
let output=filter.outputImage!.transformed(by:CGAffineTransform(scaleX:10,y:10))
let extent=output.extent.insetBy(dx:-40,dy:-40)
let white=CIImage(color:CIColor(red:1,green:1,blue:1)).cropped(to:extent)
let image=CIContext().createCGImage(output.composited(over:white),from:extent)!
let destination=CGImageDestinationCreateWithURL(URL(fileURLWithPath:CommandLine.arguments[2]) as CFURL,UTType.png.identifier as CFString,1,nil)!
CGImageDestinationAddImage(destination,image,nil);guard CGImageDestinationFinalize(destination) else {fatalError("Could not write pairing QR")}
try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:CommandLine.arguments[2])
print("Private pairing QR saved locally.")
