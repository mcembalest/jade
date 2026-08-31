import Foundation

extension Bundle {
    static let module: Bundle = {
        let candidates = [
            Bundle.main.resourceURL?.appendingPathComponent("GhosttyTerminal.bundle"),
            Bundle.main.bundleURL.appendingPathComponent("Contents/Resources/GhosttyTerminal.bundle"),
        ].compactMap { $0 }
        for candidate in candidates {
            if let bundle = Bundle(url: candidate) {
                return bundle
            }
        }
        fatalError("GhosttyTerminal.bundle is missing")
    }()
}
