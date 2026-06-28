import XCTest
@testable import InterviewTimerCore

final class InterviewTemplateTests: XCTestCase {
    func testDecodeValidTemplateComputesTotalDuration() throws {
        let json = """
        {
          "templateName": "60min Interview",
          "warnings": {
            "stageLastSeconds": 60,
            "overallLastSeconds": 300
          },
          "stages": [
            { "id": "intro", "name": "开场与破冰", "durationMinutes": 5 },
            { "id": "resume", "name": "经历深挖", "durationMinutes": 15 }
          ]
        }
        """

        let template = try InterviewTemplate.decode(from: Data(json.utf8))

        XCTAssertEqual(template.templateName, "60min Interview")
        XCTAssertEqual(template.stages.count, 2)
        XCTAssertEqual(template.totalDuration, 1_200, accuracy: 0.001)
    }

    func testDecodeRejectsMissingStageDuration() {
        let json = """
        {
          "templateName": "Broken",
          "warnings": {
            "stageLastSeconds": 60,
            "overallLastSeconds": 300
          },
          "stages": [
            { "id": "intro", "name": "开场与破冰" }
          ]
        }
        """

        XCTAssertThrowsError(try InterviewTemplate.decode(from: Data(json.utf8)))
    }

    func testDecodeRejectsDuplicateStageIdentifiers() {
        let json = """
        {
          "templateName": "Broken",
          "warnings": {
            "stageLastSeconds": 60,
            "overallLastSeconds": 300
          },
          "stages": [
            { "id": "intro", "name": "开场与破冰", "durationMinutes": 5 },
            { "id": "intro", "name": "重复", "durationMinutes": 5 }
          ]
        }
        """

        XCTAssertThrowsError(try InterviewTemplate.decode(from: Data(json.utf8)))
    }
}
