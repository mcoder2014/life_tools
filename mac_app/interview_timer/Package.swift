// swift-tools-version: 5.8

import PackageDescription

let package = Package(
    name: "InterviewTimer",
    platforms: [
        .macOS(.v13),
    ],
    products: [
        .library(name: "InterviewTimerCore", targets: ["InterviewTimerCore"]),
        .executable(name: "InterviewTimerApp", targets: ["InterviewTimerApp"]),
    ],
    targets: [
        .target(
            name: "InterviewTimerCore"
        ),
        .executableTarget(
            name: "InterviewTimerApp",
            dependencies: ["InterviewTimerCore"],
            linkerSettings: [
                .linkedFramework("AppKit"),
                .linkedFramework("SwiftUI"),
                .linkedFramework("UserNotifications"),
            ]
        ),
        .testTarget(
            name: "InterviewTimerCoreTests",
            dependencies: ["InterviewTimerCore"]
        ),
    ]
)
