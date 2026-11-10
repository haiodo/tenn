// swift-tools-version: 5.9
import PackageDescription

// The format core only needs Foundation. The Tenniarb application layer (element model, layout, UI
// highlighting) is not part of this package.
//
// The manifest sits at the root of the repository and the sources in swift/Sources: SwiftPM looks for
// Package.swift at the root of a package fetched by URL, and a manifest in a subdirectory is not found.
let package = Package(
    name: "Tenn",
    platforms: [.macOS(.v13)],
    products: [
        .library(name: "Tenn", targets: ["Tenn"])
    ],
    targets: [
        .target(name: "Tenn", path: "swift/Sources/Tenn")
    ]
)
