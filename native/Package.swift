// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "JaDENative",
    platforms: [.macOS(.v13)],
    dependencies: [
        .package(
            url: "https://github.com/Lakr233/libghostty-spm.git",
            revision: "dacbb0b8bf8ca96a988afdcc6a7f359c05cd015b"
        ),
    ],
    targets: [
        .target(
            name: "CPTY",
            publicHeadersPath: "include",
            linkerSettings: [.linkedLibrary("util")]
        ),
        .executableTarget(
            name: "JaDENative",
            dependencies: [
                "CPTY",
                .product(name: "GhosttyTerminal", package: "libghostty-spm"),
            ]
        ),
    ],
)
