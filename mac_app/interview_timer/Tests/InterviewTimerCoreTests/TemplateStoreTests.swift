import XCTest
@testable import InterviewTimerCore

final class TemplateStoreTests: XCTestCase {
    func testLoadOrCreateCreatesDefaultTemplateWhenMissing() throws {
        let directory = try makeTemporaryDirectory()
        let store = TemplateStore(baseDirectory: directory)

        let template = try store.loadOrCreate()
        let activeTemplateURL = try store.activeTemplateURL()

        XCTAssertEqual(template.templateName, "60min Interview")
        XCTAssertTrue(FileManager.default.fileExists(atPath: activeTemplateURL.path))
    }

    func testLoadOrCreateThrowsForInvalidTemplateFile() throws {
        let directory = try makeTemporaryDirectory()
        let store = TemplateStore(baseDirectory: directory)

        try FileManager.default.createDirectory(
            at: directory,
            withIntermediateDirectories: true,
            attributes: nil
        )
        try Data("{not-json}".utf8).write(to: store.templateURL)

        XCTAssertThrowsError(try store.loadOrCreate())
    }

    func testLoadOrCreateUsesLegacyTemplateAsDefaultWhenPresent() throws {
        let directory = try makeTemporaryDirectory()
        let store = TemplateStore(baseDirectory: directory)

        try FileManager.default.createDirectory(
            at: directory,
            withIntermediateDirectories: true,
            attributes: nil
        )
        try makeTemplateData(name: "Legacy Interview").write(to: store.legacyTemplateURL)

        let template = try store.loadOrCreate()

        XCTAssertEqual(template.templateName, "Legacy Interview")
        XCTAssertEqual(try store.activeTemplateURL(), store.legacyTemplateURL)
    }

    func testAvailableTemplatesIncludesLegacyAndDirectoryTemplates() throws {
        let directory = try makeTemporaryDirectory()
        let store = TemplateStore(baseDirectory: directory)

        try FileManager.default.createDirectory(
            at: store.templatesDirectoryURL,
            withIntermediateDirectories: true,
            attributes: nil
        )
        try makeTemplateData(name: "Legacy Interview").write(to: store.legacyTemplateURL)
        try makeTemplateData(name: "Backend Panel").write(
            to: store.templatesDirectoryURL.appendingPathComponent("backend.json")
        )
        try makeTemplateData(name: "Frontend Panel").write(
            to: store.templatesDirectoryURL.appendingPathComponent("frontend.json")
        )

        let templates = try store.availableTemplates()

        XCTAssertEqual(templates.count, 3)
        XCTAssertTrue(templates.contains(where: { $0.displayName == "Legacy Interview" }))
        XCTAssertTrue(templates.contains(where: { $0.displayName == "Backend Panel" }))
        XCTAssertTrue(templates.contains(where: { $0.displayName == "Frontend Panel" }))
    }

    func testSelectTemplatePersistsAndLoadsSelectedTemplate() throws {
        let directory = try makeTemporaryDirectory()
        let store = TemplateStore(baseDirectory: directory)

        try FileManager.default.createDirectory(
            at: store.templatesDirectoryURL,
            withIntermediateDirectories: true,
            attributes: nil
        )
        let firstURL = store.templatesDirectoryURL.appendingPathComponent("first.json")
        let secondURL = store.templatesDirectoryURL.appendingPathComponent("second.json")
        try makeTemplateData(name: "First Panel").write(to: firstURL)
        try makeTemplateData(name: "Second Panel").write(to: secondURL)

        try store.selectTemplate(at: secondURL)

        XCTAssertEqual(try store.activeTemplateURL(), secondURL)
        XCTAssertEqual(try store.loadOrCreate().templateName, "Second Panel")
    }

    private func makeTemporaryDirectory() throws -> URL {
        let directory = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(
            at: directory,
            withIntermediateDirectories: true,
            attributes: nil
        )
        return directory
    }

    private func makeTemplateData(name: String) throws -> Data {
        let json = """
        {
          "templateName": "\(name)",
          "warnings": {
            "stageLastSeconds": 60,
            "overallLastSeconds": 300
          },
          "stages": [
            { "id": "intro", "name": "开场与破冰", "durationMinutes": 5 },
            { "id": "deep-dive", "name": "经历深挖", "durationMinutes": 15 }
          ]
        }
        """
        return Data(json.utf8)
    }
}
