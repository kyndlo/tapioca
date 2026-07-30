// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "TapiocaImageRuntime",
    platforms: [.macOS(.v26)],
    dependencies: [
        .package(
            url: "https://github.com/xocialize/qwen-image-edit-swift",
            exact: "0.7.2")
    ],
    targets: [
        .executableTarget(
            name: "tapioca-image-runtime",
            dependencies: [
                .product(
                    name: "MLXQwenImageFlash",
                    package: "qwen-image-edit-swift")
            ])
    ]
)
