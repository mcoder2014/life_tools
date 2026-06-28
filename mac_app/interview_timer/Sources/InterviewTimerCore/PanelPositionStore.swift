import Foundation

public struct PanelPosition: Codable, Equatable {
    public let screenID: UInt32?
    public let originX: Double
    public let originY: Double

    public init(screenID: UInt32?, originX: Double, originY: Double) {
        self.screenID = screenID
        self.originX = originX
        self.originY = originY
    }
}

public final class PanelPositionStore {
    private let fileManager: FileManager
    public let baseDirectory: URL

    public init(fileManager: FileManager = .default, baseDirectory: URL? = nil) {
        self.fileManager = fileManager
        self.baseDirectory = baseDirectory
            ?? fileManager.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support/InterviewTimer", isDirectory: true)
    }

    public var panelPositionURL: URL {
        baseDirectory.appendingPathComponent("panel-position.json")
    }

    public func load() -> PanelPosition? {
        guard
            let data = try? Data(contentsOf: panelPositionURL),
            let placement = try? JSONDecoder().decode(PanelPosition.self, from: data)
        else {
            return nil
        }

        return placement
    }

    public func save(_ position: PanelPosition) throws {
        try fileManager.createDirectory(
            at: baseDirectory,
            withIntermediateDirectories: true,
            attributes: nil
        )

        let data = try JSONEncoder().encode(position)
        try data.write(to: panelPositionURL, options: .atomic)
    }
}
