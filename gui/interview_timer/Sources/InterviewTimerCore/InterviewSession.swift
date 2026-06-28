import Foundation

public enum SessionAlertLevel: String, Equatable {
    case normal
    case warning
    case overdue
}

public enum SessionAlertEvent: Equatable {
    case stageOverdue(stageName: String)
    case overallOverdue
}

public struct SessionSnapshot: Equatable {
    public let isRunning: Bool
    public let currentStageIndex: Int?
    public let stageCount: Int
    public let currentStageName: String?
    public let currentStageActualElapsed: TimeInterval
    public let currentStageRemaining: TimeInterval
    public let currentStagePlannedDuration: TimeInterval
    public let overallElapsed: TimeInterval
    public let overallRemaining: TimeInterval
    public let plannedTotalDuration: TimeInterval
    public let drift: TimeInterval
    public let stageAlertLevel: SessionAlertLevel
    public let overallAlertLevel: SessionAlertLevel
}

public final class InterviewSession {
    public let template: InterviewTemplate

    private var sessionStart: Date?
    private var activeStageIndex: Int?
    private var activeStageStartedAt: Date?
    private var accumulatedStageElapsed: [TimeInterval]
    private var stageOverdueNotifications = Set<Int>()
    private var hasSentOverallOverdueNotification = false

    public init(template: InterviewTemplate) {
        self.template = template
        self.accumulatedStageElapsed = Array(repeating: 0, count: template.stages.count)
    }

    public func start(at now: Date) {
        guard sessionStart == nil else {
            return
        }

        sessionStart = now
        activeStageIndex = 0
        activeStageStartedAt = now
    }

    public func reset() {
        sessionStart = nil
        activeStageIndex = nil
        activeStageStartedAt = nil
        accumulatedStageElapsed = Array(repeating: 0, count: template.stages.count)
        stageOverdueNotifications.removeAll()
        hasSentOverallOverdueNotification = false
    }

    public func advance(at now: Date) {
        guard sessionStart != nil else {
            start(at: now)
            return
        }

        guard let currentIndex = activeStageIndex else {
            return
        }

        closeActiveStage(at: now)
        activeStageIndex = min(currentIndex + 1, template.stages.count - 1)
        activeStageStartedAt = now
    }

    public func goBack(at now: Date) {
        guard sessionStart != nil else {
            return
        }

        guard let currentIndex = activeStageIndex else {
            return
        }

        closeActiveStage(at: now)
        activeStageIndex = max(currentIndex - 1, 0)
        activeStageStartedAt = now
    }

    public func snapshot(at now: Date) -> SessionSnapshot {
        guard
            let sessionStart,
            let currentIndex = activeStageIndex
        else {
            return SessionSnapshot(
                isRunning: false,
                currentStageIndex: nil,
                stageCount: template.stages.count,
                currentStageName: template.stages.first?.name,
                currentStageActualElapsed: 0,
                currentStageRemaining: template.stages.first?.duration ?? 0,
                currentStagePlannedDuration: template.stages.first?.duration ?? 0,
                overallElapsed: 0,
                overallRemaining: template.totalDuration,
                plannedTotalDuration: template.totalDuration,
                drift: 0,
                stageAlertLevel: .normal,
                overallAlertLevel: .normal
            )
        }

        let currentStage = template.stages[currentIndex]
        let currentStageElapsed = elapsedTime(for: currentIndex, at: now)
        let overallElapsed = max(0, now.timeIntervalSince(sessionStart))
        let plannedElapsed = plannedElapsedTime(currentStageIndex: currentIndex, currentStageElapsed: currentStageElapsed)
        let drift = overallElapsed - plannedElapsed
        let currentStageRemaining = currentStage.duration - currentStageElapsed
        let overallRemaining = template.totalDuration - overallElapsed

        return SessionSnapshot(
            isRunning: true,
            currentStageIndex: currentIndex,
            stageCount: template.stages.count,
            currentStageName: currentStage.name,
            currentStageActualElapsed: currentStageElapsed,
            currentStageRemaining: currentStageRemaining,
            currentStagePlannedDuration: currentStage.duration,
            overallElapsed: overallElapsed,
            overallRemaining: overallRemaining,
            plannedTotalDuration: template.totalDuration,
            drift: drift,
            stageAlertLevel: alertLevel(
                remaining: currentStageRemaining,
                warningThreshold: TimeInterval(template.warnings.stageLastSeconds)
            ),
            overallAlertLevel: alertLevel(
                remaining: overallRemaining,
                warningThreshold: TimeInterval(template.warnings.overallLastSeconds)
            )
        )
    }

    public func consumeAlertEvents(at now: Date) -> [SessionAlertEvent] {
        let snapshot = snapshot(at: now)
        guard snapshot.isRunning, let currentIndex = snapshot.currentStageIndex else {
            return []
        }

        var events: [SessionAlertEvent] = []

        if snapshot.stageAlertLevel == .overdue,
           !stageOverdueNotifications.contains(currentIndex),
           let currentStageName = snapshot.currentStageName {
            stageOverdueNotifications.insert(currentIndex)
            events.append(.stageOverdue(stageName: currentStageName))
        }

        if snapshot.overallAlertLevel == .overdue, !hasSentOverallOverdueNotification {
            hasSentOverallOverdueNotification = true
            events.append(.overallOverdue)
        }

        return events
    }

    private func closeActiveStage(at now: Date) {
        guard
            let currentIndex = activeStageIndex,
            let activeStageStartedAt
        else {
            return
        }

        let additionalElapsed = max(0, now.timeIntervalSince(activeStageStartedAt))
        accumulatedStageElapsed[currentIndex] += additionalElapsed
        self.activeStageStartedAt = nil
    }

    private func elapsedTime(for stageIndex: Int, at now: Date) -> TimeInterval {
        let persistedElapsed = accumulatedStageElapsed[stageIndex]

        guard stageIndex == activeStageIndex, let activeStageStartedAt else {
            return persistedElapsed
        }

        return persistedElapsed + max(0, now.timeIntervalSince(activeStageStartedAt))
    }

    private func plannedElapsedTime(currentStageIndex: Int, currentStageElapsed: TimeInterval) -> TimeInterval {
        let completedDuration = template.stages
            .prefix(currentStageIndex)
            .reduce(into: 0.0) { partialResult, stage in
                partialResult += stage.duration
            }

        let currentPlannedDuration = template.stages[currentStageIndex].duration
        return completedDuration + min(currentStageElapsed, currentPlannedDuration)
    }

    private func alertLevel(remaining: TimeInterval, warningThreshold: TimeInterval) -> SessionAlertLevel {
        if remaining < 0 {
            return .overdue
        }

        if remaining <= warningThreshold {
            return .warning
        }

        return .normal
    }
}
