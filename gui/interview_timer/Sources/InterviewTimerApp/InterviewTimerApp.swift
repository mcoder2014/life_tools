import AppKit
import SwiftUI
import InterviewTimerCore

@main
struct InterviewTimerApplication {
    static func main() {
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        app.setActivationPolicy(.regular)
        app.run()
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate {
    private let panelPositionStore = PanelPositionStore()
    private let notificationManager = NotificationManager()
    private var viewModel: InterviewTimerViewModel!
    private var panel: NSPanel?
    private var tickTimer: Timer?
    private let panelSize = NSSize(width: 320, height: 248)

    func applicationDidFinishLaunching(_ notification: Notification) {
        viewModel = InterviewTimerViewModel(
            notificationManager: notificationManager
        )

        let hasSavedPanelPosition = panelPositionStore.load() != nil
        let frame = initialPanelFrame()
        let floatingPanel = FloatingPanel(contentRect: frame) {
            InterviewTimerView(viewModel: viewModel)
        }
        floatingPanel.delegate = self
        floatingPanel.makeKeyAndOrderFront(nil)
        panel = floatingPanel
        NSApplication.shared.activate(ignoringOtherApps: true)

        if !hasSavedPanelPosition {
            DispatchQueue.main.async { [weak self, weak floatingPanel] in
                guard
                    let self,
                    let floatingPanel,
                    let targetScreen = self.preferredLaunchScreen()
                else {
                    return
                }

                floatingPanel.setFrame(self.defaultFrame(on: targetScreen), display: true)
                floatingPanel.makeKeyAndOrderFront(nil)
            }
        }

        rebuildMainMenu()

        notificationManager.requestAuthorizationIfNeeded()

        let timer = Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in
            Task { @MainActor [weak self] in
                self?.viewModel.tick()
            }
        }
        RunLoop.main.add(timer, forMode: .common)
        tickTimer = timer
    }

    func applicationWillTerminate(_ notification: Notification) {
        tickTimer?.invalidate()
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        panel?.makeKeyAndOrderFront(nil)
        NSApplication.shared.activate(ignoringOtherApps: true)
        return true
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        false
    }

    func windowDidMove(_ notification: Notification) {
        guard
            let panel,
            let screen = panel.screen
        else {
            return
        }

        let placement = PanelPosition(
            screenID: screen.displayID,
            originX: panel.frame.origin.x,
            originY: panel.frame.origin.y
        )
        try? panelPositionStore.save(placement)
    }

    private func rebuildMainMenu() {
        let mainMenu = NSMenu()

        let appMenuItem = NSMenuItem()
        mainMenu.addItem(appMenuItem)
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "关于 InterviewTimer", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: "")
        appMenu.addItem(.separator())
        let hideAppItem = NSMenuItem(title: "隐藏 InterviewTimer", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        hideAppItem.target = NSApplication.shared
        appMenu.addItem(hideAppItem)
        appMenu.addItem(.separator())
        let quitItem = NSMenuItem(title: "退出 InterviewTimer", action: #selector(quitApplication), keyEquivalent: "q")
        quitItem.target = self
        appMenu.addItem(quitItem)
        appMenuItem.submenu = appMenu

        let templateMenuItem = NSMenuItem(title: "模板", action: nil, keyEquivalent: "")
        let templateMenu = NSMenu(title: "模板")

        let openCurrentTemplateItem = NSMenuItem(title: "打开当前模板文件", action: #selector(openCurrentTemplateFile), keyEquivalent: "e")
        openCurrentTemplateItem.target = self
        openCurrentTemplateItem.isEnabled = viewModel.activeTemplateURL != nil
        templateMenu.addItem(openCurrentTemplateItem)

        let openTemplatesDirectoryItem = NSMenuItem(title: "打开模板目录", action: #selector(openTemplatesDirectory), keyEquivalent: "d")
        openTemplatesDirectoryItem.target = self
        templateMenu.addItem(openTemplatesDirectoryItem)

        let reloadItem = NSMenuItem(title: "重新加载模板", action: #selector(reloadTemplates), keyEquivalent: "r")
        reloadItem.target = self
        templateMenu.addItem(reloadItem)
        templateMenu.addItem(.separator())

        let switchTemplateItem = NSMenuItem(title: "切换模板", action: nil, keyEquivalent: "")
        let switchTemplateMenu = NSMenu(title: "切换模板")
        for descriptor in viewModel.availableTemplates {
            let item = NSMenuItem(title: descriptor.displayName, action: #selector(selectTemplateFromMenu(_:)), keyEquivalent: "")
            item.target = self
            item.representedObject = descriptor.url.path
            item.state = descriptor.url == viewModel.activeTemplateURL ? .on : .off
            switchTemplateMenu.addItem(item)
        }
        if switchTemplateMenu.items.isEmpty {
            let emptyItem = NSMenuItem(title: "暂无模板", action: nil, keyEquivalent: "")
            emptyItem.isEnabled = false
            switchTemplateMenu.addItem(emptyItem)
        }
        templateMenu.setSubmenu(switchTemplateMenu, for: switchTemplateItem)
        templateMenu.addItem(switchTemplateItem)

        mainMenu.addItem(templateMenuItem)
        mainMenu.setSubmenu(templateMenu, for: templateMenuItem)

        let windowMenuItem = NSMenuItem(title: "窗口", action: nil, keyEquivalent: "")
        let windowMenu = NSMenu(title: "窗口")
        let togglePanelItem = NSMenuItem(title: panel?.isVisible == true ? "隐藏悬浮窗" : "显示悬浮窗", action: #selector(togglePanelVisibility), keyEquivalent: "1")
        togglePanelItem.target = self
        windowMenu.addItem(togglePanelItem)
        let resetItem = NSMenuItem(title: "重置本次面试", action: #selector(resetSession), keyEquivalent: "0")
        resetItem.target = self
        resetItem.isEnabled = !viewModel.resetDisabled
        windowMenu.addItem(resetItem)
        mainMenu.addItem(windowMenuItem)
        mainMenu.setSubmenu(windowMenu, for: windowMenuItem)
        NSApplication.shared.windowsMenu = windowMenu

        NSApplication.shared.mainMenu = mainMenu
    }

    @objc
    private func togglePanelVisibility() {
        guard let panel else {
            return
        }

        if panel.isVisible {
            panel.orderOut(nil)
        } else {
            panel.makeKeyAndOrderFront(nil)
            NSApplication.shared.activate(ignoringOtherApps: true)
        }

        rebuildMainMenu()
    }

    @objc
    private func openCurrentTemplateFile() {
        guard let url = viewModel.activeTemplateURL else {
            return
        }

        NSWorkspace.shared.open(url)
    }

    @objc
    private func openTemplatesDirectory() {
        let directoryURL = viewModel.templatesDirectoryURL
        try? FileManager.default.createDirectory(
            at: directoryURL,
            withIntermediateDirectories: true,
            attributes: nil
        )
        NSWorkspace.shared.open(directoryURL)
    }

    @objc
    private func reloadTemplates() {
        viewModel.reloadTemplate()
        rebuildMainMenu()
    }

    @objc
    private func selectTemplateFromMenu(_ sender: NSMenuItem) {
        guard
            let path = sender.representedObject as? String
        else {
            return
        }

        let selectedURL = URL(fileURLWithPath: path)
        if let descriptor = viewModel.availableTemplates.first(where: { $0.url == selectedURL }) {
            viewModel.selectTemplate(descriptor)
            rebuildMainMenu()
        }
    }

    @objc
    private func resetSession() {
        viewModel.reset()
        rebuildMainMenu()
    }

    @objc
    private func quitApplication() {
        NSApplication.shared.terminate(nil)
    }

    private func initialPanelFrame() -> NSRect {
        guard let targetScreen = preferredLaunchScreen() else {
            return NSRect(origin: .zero, size: panelSize)
        }

        guard let savedPosition = panelPositionStore.load() else {
            return defaultFrame(on: targetScreen)
        }

        guard
            let savedScreenID = savedPosition.screenID,
            let savedScreen = NSScreen.screens.first(where: { $0.displayID == savedScreenID })
        else {
            return defaultFrame(on: targetScreen)
        }

        let savedOrigin = NSPoint(x: savedPosition.originX, y: savedPosition.originY)
        return NSRect(
            origin: clampedOrigin(savedOrigin, on: savedScreen),
            size: panelSize
        )
    }

    private func preferredLaunchScreen() -> NSScreen? {
        let mouseLocation = NSEvent.mouseLocation

        if let pointerScreen = NSScreen.screens.first(where: { $0.frame.contains(mouseLocation) }) {
            return pointerScreen
        }

        return NSScreen.main ?? NSScreen.screens.first
    }

    private func defaultFrame(on screen: NSScreen) -> NSRect {
        let visibleFrame = screen.visibleFrame
        let origin = NSPoint(
            x: visibleFrame.maxX - panelSize.width - 20,
            y: visibleFrame.maxY - panelSize.height - 20
        )
        return NSRect(origin: origin, size: panelSize)
    }

    private func clampedOrigin(_ origin: NSPoint, on screen: NSScreen) -> NSPoint {
        let visibleFrame = screen.visibleFrame

        let minX = visibleFrame.minX
        let maxX = visibleFrame.maxX - panelSize.width
        let minY = visibleFrame.minY
        let maxY = visibleFrame.maxY - panelSize.height

        return NSPoint(
            x: min(max(origin.x, minX), maxX),
            y: min(max(origin.y, minY), maxY)
        )
    }
}

private extension NSScreen {
    var displayID: UInt32? {
        let key = NSDeviceDescriptionKey("NSScreenNumber")
        return (deviceDescription[key] as? NSNumber)?.uint32Value
    }
}
