import Foundation

public struct TemplateDescriptor: Equatable, Identifiable {
    public let url: URL
    public let displayName: String
    public let isLegacy: Bool

    public var id: String {
        url.path
    }

    public init(url: URL, displayName: String, isLegacy: Bool) {
        self.url = url
        self.displayName = displayName
        self.isLegacy = isLegacy
    }
}

public enum TemplateStoreError: Error, Equatable {
    case unableToCreateDirectory(String)
    case unreadableTemplateFile(String)
    case invalidTemplateFile(String)
    case templateFileNotFound(String)
}

extension TemplateStoreError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .unableToCreateDirectory(let path):
            return "Unable to create template directory at \(path)."
        case .unreadableTemplateFile(let path):
            return "Unable to read template file at \(path)."
        case .invalidTemplateFile(let path):
            return "Template file is invalid at \(path)."
        case .templateFileNotFound(let path):
            return "Template file does not exist at \(path)."
        }
    }
}

private struct TemplateSelection: Codable {
    let activeTemplatePath: String
}

public final class TemplateStore {
    public let fileManager: FileManager
    public let baseDirectory: URL

    public init(fileManager: FileManager = .default, baseDirectory: URL? = nil) {
        self.fileManager = fileManager
        self.baseDirectory = baseDirectory
            ?? fileManager.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support/InterviewTimer", isDirectory: true)
    }

    public var templateURL: URL {
        baseDirectory.appendingPathComponent("template.json")
    }

    public var legacyTemplateURL: URL {
        templateURL
    }

    public var templatesDirectoryURL: URL {
        baseDirectory.appendingPathComponent("templates", isDirectory: true)
    }

    public var selectionURL: URL {
        baseDirectory.appendingPathComponent("template-selection.json")
    }

    public func loadOrCreate() throws -> InterviewTemplate {
        let activeURL = try activeTemplateURL()

        let data: Data
        do {
            data = try Data(contentsOf: activeURL)
        } catch {
            throw TemplateStoreError.unreadableTemplateFile(activeURL.path)
        }

        do {
            return try InterviewTemplate.decode(from: data)
        } catch {
            throw TemplateStoreError.invalidTemplateFile(activeURL.path)
        }
    }

    public func availableTemplates() throws -> [TemplateDescriptor] {
        try ensureDirectory()

        var descriptors: [TemplateDescriptor] = []
        if fileManager.fileExists(atPath: legacyTemplateURL.path),
           let descriptor = try descriptorIfValid(for: legacyTemplateURL, isLegacy: true) {
            descriptors.append(descriptor)
        }

        let templateURLs = try fileManager.contentsOfDirectory(
            at: templatesDirectoryURL,
            includingPropertiesForKeys: nil
        )
            .filter { $0.pathExtension.lowercased() == "json" }
            .sorted { $0.lastPathComponent.localizedStandardCompare($1.lastPathComponent) == .orderedAscending }

        for url in templateURLs {
            if let descriptor = try descriptorIfValid(for: url, isLegacy: false) {
                descriptors.append(descriptor)
            }
        }

        if descriptors.isEmpty {
            _ = try ensureDefaultTemplate()
            return try availableTemplates()
        }

        return descriptors
    }

    public func activeTemplateURL() throws -> URL {
        try ensureDirectory()

        if let selectedURL = try readSelectedTemplateURL(),
           fileManager.fileExists(atPath: selectedURL.path) {
            return selectedURL
        }

        if fileManager.fileExists(atPath: legacyTemplateURL.path) {
            return legacyTemplateURL
        }

        let templates = try availableTemplates()
        if let firstTemplate = templates.first {
            return firstTemplate.url
        }

        return try ensureDefaultTemplate()
    }

    public func selectTemplate(at url: URL) throws {
        try ensureDirectory()

        guard fileManager.fileExists(atPath: url.path) else {
            throw TemplateStoreError.templateFileNotFound(url.path)
        }

        let selection = TemplateSelection(activeTemplatePath: url.path)
        let data = try JSONEncoder().encode(selection)
        try data.write(to: selectionURL, options: .atomic)
    }

    private func descriptorIfValid(for url: URL, isLegacy: Bool) throws -> TemplateDescriptor? {
        let data: Data
        do {
            data = try Data(contentsOf: url)
        } catch {
            throw TemplateStoreError.unreadableTemplateFile(url.path)
        }

        guard let template = try? InterviewTemplate.decode(from: data) else {
            return nil
        }

        return TemplateDescriptor(
            url: url,
            displayName: template.templateName,
            isLegacy: isLegacy
        )
    }

    private func readSelectedTemplateURL() throws -> URL? {
        guard fileManager.fileExists(atPath: selectionURL.path) else {
            return nil
        }

        let data: Data
        do {
            data = try Data(contentsOf: selectionURL)
        } catch {
            throw TemplateStoreError.unreadableTemplateFile(selectionURL.path)
        }

        guard let selection = try? JSONDecoder().decode(TemplateSelection.self, from: data) else {
            return nil
        }

        return URL(fileURLWithPath: selection.activeTemplatePath)
    }

    private func ensureDirectory() throws {
        do {
            try fileManager.createDirectory(
                at: baseDirectory,
                withIntermediateDirectories: true,
                attributes: nil
            )
            try fileManager.createDirectory(
                at: templatesDirectoryURL,
                withIntermediateDirectories: true,
                attributes: nil
            )
        } catch {
            throw TemplateStoreError.unableToCreateDirectory(baseDirectory.path)
        }
    }

    private func ensureDefaultTemplate() throws -> URL {
        let defaultURL = templatesDirectoryURL.appendingPathComponent("default.json")

        if !fileManager.fileExists(atPath: defaultURL.path) {
            let defaultTemplateData = try InterviewTemplate.defaultTemplate.encoded(prettyPrinted: true)
            try defaultTemplateData.write(to: defaultURL, options: .atomic)
        }

        return defaultURL
    }
}
