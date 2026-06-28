import Foundation
import UserNotifications
import InterviewTimerCore

final class NotificationManager {
    private let center: UNUserNotificationCenter
    private var didRequestAuthorization = false
    private var notificationsEnabled = false

    init(center: UNUserNotificationCenter = .current()) {
        self.center = center
    }

    func requestAuthorizationIfNeeded() {
        guard !didRequestAuthorization else {
            return
        }

        didRequestAuthorization = true
        center.requestAuthorization(options: [.alert]) { [weak self] granted, _ in
            self?.notificationsEnabled = granted
        }
    }

    func send(_ event: SessionAlertEvent) {
        guard notificationsEnabled else {
            return
        }

        let content = UNMutableNotificationContent()
        content.sound = nil

        switch event {
        case .stageOverdue(let stageName):
            content.title = "当前环节已超时"
            content.body = "\(stageName) 已进入超时状态"
        case .overallOverdue:
            content.title = "整体面试已超时"
            content.body = "请尽快收口，避免后续整体延误"
        }

        let request = UNNotificationRequest(
            identifier: UUID().uuidString,
            content: content,
            trigger: nil
        )
        center.add(request)
    }
}
