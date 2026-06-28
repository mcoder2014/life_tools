import XCTest
@testable import InterviewTimerCore

final class InterviewSessionTests: XCTestCase {
    func testStartBeginsAtFirstStage() throws {
        let session = InterviewSession(template: try makeTemplate())
        let start = Date(timeIntervalSince1970: 0)

        session.start(at: start)
        let snapshot = session.snapshot(at: start)

        XCTAssertTrue(snapshot.isRunning)
        XCTAssertEqual(snapshot.currentStageIndex, 0)
        XCTAssertEqual(snapshot.currentStageName, "开场")
        XCTAssertEqual(snapshot.currentStageRemaining, 300, accuracy: 0.001)
        XCTAssertEqual(snapshot.overallRemaining, 900, accuracy: 0.001)
    }

    func testGoingBackResumesPreviousStageElapsedWithoutRewindingOverallTime() throws {
        let session = InterviewSession(template: try makeTemplate())
        let start = Date(timeIntervalSince1970: 0)

        session.start(at: start)
        session.advance(at: Date(timeIntervalSince1970: 30))
        session.goBack(at: Date(timeIntervalSince1970: 45))
        let snapshot = session.snapshot(at: Date(timeIntervalSince1970: 45))

        XCTAssertEqual(snapshot.currentStageIndex, 0)
        XCTAssertEqual(snapshot.currentStageActualElapsed, 30, accuracy: 0.001)
        XCTAssertEqual(snapshot.overallElapsed, 45, accuracy: 0.001)
        XCTAssertEqual(snapshot.drift, 15, accuracy: 0.001)
    }

    func testEarlyAdvanceShowsAheadOfScheduleDrift() throws {
        let session = InterviewSession(template: try makeTemplate())
        let start = Date(timeIntervalSince1970: 0)

        session.start(at: start)
        session.advance(at: Date(timeIntervalSince1970: 120))
        let snapshot = session.snapshot(at: Date(timeIntervalSince1970: 120))

        XCTAssertEqual(snapshot.currentStageIndex, 1)
        XCTAssertEqual(snapshot.drift, -180, accuracy: 0.001)
    }

    func testStageAndOverallOvertimeBecomeOverdue() throws {
        let session = InterviewSession(template: try makeShortTemplate())
        let start = Date(timeIntervalSince1970: 0)

        session.start(at: start)

        let stageOverdue = session.snapshot(at: Date(timeIntervalSince1970: 75))
        XCTAssertEqual(stageOverdue.stageAlertLevel, .overdue)
        XCTAssertEqual(stageOverdue.currentStageRemaining, -15, accuracy: 0.001)

        let overallOverdue = session.snapshot(at: Date(timeIntervalSince1970: 130))
        XCTAssertEqual(overallOverdue.overallAlertLevel, .overdue)
        XCTAssertEqual(overallOverdue.overallRemaining, -10, accuracy: 0.001)
    }

    func testOverdueNotificationsAreEmittedOncePerThresholdCrossing() throws {
        let session = InterviewSession(template: try makeShortTemplate())
        let start = Date(timeIntervalSince1970: 0)

        session.start(at: start)

        XCTAssertEqual(
            session.consumeAlertEvents(at: Date(timeIntervalSince1970: 75)),
            [.stageOverdue(stageName: "开场")]
        )
        XCTAssertEqual(session.consumeAlertEvents(at: Date(timeIntervalSince1970: 80)), [])

        XCTAssertEqual(
            session.consumeAlertEvents(at: Date(timeIntervalSince1970: 130)),
            [.overallOverdue]
        )
        XCTAssertEqual(session.consumeAlertEvents(at: Date(timeIntervalSince1970: 131)), [])
    }

    private func makeTemplate() throws -> InterviewTemplate {
        let json = """
        {
          "templateName": "Sample Interview",
          "warnings": {
            "stageLastSeconds": 60,
            "overallLastSeconds": 300
          },
          "stages": [
            { "id": "intro", "name": "开场", "durationMinutes": 5 },
            { "id": "deep-dive", "name": "深挖", "durationMinutes": 10 }
          ]
        }
        """

        return try InterviewTemplate.decode(from: Data(json.utf8))
    }

    private func makeShortTemplate() throws -> InterviewTemplate {
        let json = """
        {
          "templateName": "Short Interview",
          "warnings": {
            "stageLastSeconds": 10,
            "overallLastSeconds": 30
          },
          "stages": [
            { "id": "intro", "name": "开场", "durationMinutes": 1 },
            { "id": "close", "name": "收尾", "durationMinutes": 1 }
          ]
        }
        """

        return try InterviewTemplate.decode(from: Data(json.utf8))
    }
}
