import AppKit
import SwiftUI
import InterviewTimerCore

struct InterviewTimerView: View {
    @ObservedObject var viewModel: InterviewTimerViewModel

    var body: some View {
        Group {
            if let configurationError = viewModel.configurationError {
                errorView(message: configurationError)
            } else {
                contentView
            }
        }
        .padding(14)
        .frame(width: 320, height: 248)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(Color(red: 0.97, green: 0.97, blue: 0.96))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke(Color.black.opacity(0.08), lineWidth: 1)
        )
        .clipShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
        .contextMenu {
            Button("上一环节") {
                viewModel.goBack()
            }

            Button("下一环节") {
                viewModel.handlePrimaryAction()
            }

            Button("重置本次面试") {
                viewModel.reset()
            }
            .disabled(viewModel.resetDisabled)

            Divider()

            Button("退出") {
                NSApplication.shared.terminate(nil)
            }
        }
    }

    private var contentView: some View {
        VStack(alignment: .leading, spacing: 8) {
            header

            Text(viewModel.currentStageTitle)
                .font(.system(size: 19, weight: .semibold))
                .foregroundStyle(Color.primary)
                .lineLimit(2)
                .minimumScaleFactor(0.82)
                .frame(maxWidth: .infinity, alignment: .leading)

            HStack(spacing: 8) {
                metricCard(
                    title: "当前环节",
                    value: viewModel.stageTimeText,
                    level: viewModel.stageAlertLevel
                )

                metricCard(
                    title: "整体剩余",
                    value: viewModel.overallTimeText,
                    level: viewModel.overallAlertLevel
                )
            }

            HStack(alignment: .center, spacing: 8) {
                Text("进度")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(Color.secondary)

                Spacer()

                Text(viewModel.driftText)
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(driftColor)
                    .lineLimit(1)
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 9)
            .background(
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .fill(Color.white.opacity(0.82))
            )
            .overlay(
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .stroke(Color.black.opacity(0.05), lineWidth: 1)
            )

            Button(action: viewModel.handlePrimaryAction) {
                HStack(spacing: 8) {
                    Text(viewModel.primaryButtonTitle)
                    Image(systemName: "arrow.right")
                        .font(.system(size: 12, weight: .bold))
                }
                .font(.system(size: 14, weight: .semibold))
                .frame(maxWidth: .infinity)
            }
            .buttonStyle(PrimaryActionButtonStyle())
        }
    }

    private var header: some View {
        HStack(alignment: .center, spacing: 8) {
            Text(viewModel.templateName)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(Color.secondary)
                .lineLimit(1)

            Spacer()

            Text(viewModel.stageIndexText)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(Color(red: 0.22, green: 0.22, blue: 0.22))
                .padding(.horizontal, 10)
                .padding(.vertical, 5)
                .background(
                    Capsule(style: .continuous)
                        .fill(Color.white.opacity(0.92))
                )
                .overlay(
                    Capsule(style: .continuous)
                        .stroke(Color.black.opacity(0.07), lineWidth: 1)
                )
        }
    }

    private func errorView(message: String) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("模板加载失败")
                .font(.system(size: 18, weight: .semibold))
                .foregroundStyle(Color.primary)

            Text(message)
                .font(.system(size: 12))
                .foregroundStyle(Color.secondary)
                .fixedSize(horizontal: false, vertical: true)

            Text(viewModel.templatePath)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(Color.secondary)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)

            Spacer(minLength: 0)

            Button("重新加载模板") {
                viewModel.reloadTemplate()
            }
            .buttonStyle(PrimaryActionButtonStyle())
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }

    private func metricCard(
        title: String,
        value: String,
        level: SessionAlertLevel
    ) -> some View {
        let tone = metricTone(for: level)

        return VStack(alignment: .leading, spacing: 6) {
            Text(title)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(Color.secondary)
            Text(value)
                .font(.system(size: 25, weight: .bold, design: .rounded))
                .monospacedDigit()
                .foregroundStyle(tone.primary)
                .lineLimit(1)
                .minimumScaleFactor(0.85)
        }
        .padding(.horizontal, 11)
        .padding(.vertical, 10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .fill(Color.white.opacity(0.88))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .stroke(tone.stroke, lineWidth: 1)
        )
    }

    private func metricTone(for level: SessionAlertLevel) -> (primary: Color, stroke: Color) {
        switch level {
        case .normal:
            return (
                primary: Color(red: 0.18, green: 0.20, blue: 0.24),
                stroke: Color.black.opacity(0.06)
            )
        case .warning:
            return (
                primary: Color(red: 0.63, green: 0.40, blue: 0.07),
                stroke: Color(red: 0.86, green: 0.72, blue: 0.44).opacity(0.65)
            )
        case .overdue:
            return (
                primary: Color(red: 0.68, green: 0.18, blue: 0.19),
                stroke: Color(red: 0.86, green: 0.48, blue: 0.46).opacity(0.62)
            )
        }
    }

    private var driftColor: Color {
        if viewModel.driftText.hasPrefix("提前") {
            return Color(red: 0.18, green: 0.49, blue: 0.25)
        }

        if viewModel.driftText.hasPrefix("落后") {
            return Color(red: 0.68, green: 0.18, blue: 0.19)
        }

        return Color.primary
    }
}

private struct PrimaryActionButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(Color.white)
            .padding(.horizontal, 14)
            .padding(.vertical, 11)
            .background(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .fill(configuration.isPressed ? Color.black.opacity(0.75) : Color.black.opacity(0.88))
            )
            .scaleEffect(configuration.isPressed ? 0.99 : 1)
    }
}
