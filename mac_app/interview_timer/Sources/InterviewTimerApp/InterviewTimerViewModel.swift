import Foundation
import SwiftUI
import InterviewTimerCore

@MainActor
final class InterviewTimerViewModel: ObservableObject {
    @Published private(set) var templateName = ""
    @Published private(set) var templatePath = ""
    @Published private(set) var configurationError: String?
    @Published private(set) var snapshot: SessionSnapshot?
    @Published private(set) var availableTemplates: [TemplateDescriptor] = []
    @Published private(set) var activeTemplateURL: URL?

    private let templateStore: TemplateStore
    private let notificationManager: NotificationManager
    private var session: InterviewSession?

    init(
        templateStore: TemplateStore = TemplateStore(),
        notificationManager: NotificationManager
    ) {
        self.templateStore = templateStore
        self.notificationManager = notificationManager

        templatePath = templateStore.templateURL.path
        reloadTemplate()
    }

    var primaryButtonTitle: String {
        snapshot?.isRunning == true ? "下一环节" : "开始"
    }

    var stageIndexText: String {
        guard let snapshot, let currentStageIndex = snapshot.currentStageIndex else {
            return "待开始"
        }

        return "\(currentStageIndex + 1)/\(snapshot.stageCount)"
    }

    var currentStageTitle: String {
        snapshot?.currentStageName ?? "未开始"
    }

    var stageTimeText: String {
        formatDuration(snapshot?.currentStageRemaining ?? 0)
    }

    var overallTimeText: String {
        formatDuration(snapshot?.overallRemaining ?? 0)
    }

    var driftText: String {
        guard let snapshot else {
            return "按计划"
        }

        if abs(snapshot.drift) < 1 {
            return "按计划"
        }

        let value = formatDuration(abs(snapshot.drift), allowsNegative: false)
        return snapshot.drift > 0 ? "落后 \(value)" : "提前 \(value)"
    }

    var stageAlertLevel: SessionAlertLevel {
        snapshot?.stageAlertLevel ?? .normal
    }

    var overallAlertLevel: SessionAlertLevel {
        snapshot?.overallAlertLevel ?? .normal
    }

    var resetDisabled: Bool {
        snapshot?.isRunning != true
    }

    var templatesDirectoryURL: URL {
        templateStore.templatesDirectoryURL
    }

    func handlePrimaryAction() {
        guard let session else {
            reloadTemplate()
            return
        }

        let now = Date()
        if snapshot?.isRunning == true {
            session.advance(at: now)
        } else {
            session.start(at: now)
        }

        publish(at: now)
    }

    func goBack() {
        guard let session else {
            return
        }

        let now = Date()
        session.goBack(at: now)
        publish(at: now)
    }

    func reset() {
        session?.reset()
        publish(at: Date())
    }

    func tick() {
        guard session != nil else {
            return
        }

        publish(at: Date())
    }

    func reloadTemplate() {
        do {
            availableTemplates = try templateStore.availableTemplates()
            let activeURL = try templateStore.activeTemplateURL()
            let template = try templateStore.loadOrCreate()
            templateName = template.templateName
            templatePath = activeURL.path
            activeTemplateURL = activeURL
            configurationError = nil

            let session = InterviewSession(template: template)
            self.session = session
            snapshot = session.snapshot(at: Date())
        } catch {
            availableTemplates = []
            activeTemplateURL = nil
            templateName = "Interview Timer"
            configurationError = error.localizedDescription
            session = nil
            snapshot = nil
        }
    }

    func selectTemplate(_ descriptor: TemplateDescriptor) {
        do {
            try templateStore.selectTemplate(at: descriptor.url)
            reloadTemplate()
        } catch {
            configurationError = error.localizedDescription
        }
    }

    private func publish(at now: Date) {
        guard let session else {
            return
        }

        snapshot = session.snapshot(at: now)
        for event in session.consumeAlertEvents(at: now) {
            notificationManager.send(event)
        }
    }

    private func formatDuration(_ duration: TimeInterval, allowsNegative: Bool = true) -> String {
        let wholeSeconds = Int(duration.rounded(.towardZero))
        let isNegative = wholeSeconds < 0
        let absoluteSeconds = abs(wholeSeconds)
        let minutes = absoluteSeconds / 60
        let seconds = absoluteSeconds % 60
        let sign = (isNegative && allowsNegative) ? "-" : ""
        return "\(sign)\(String(format: "%02d", minutes)):\(String(format: "%02d", seconds))"
    }
}
