import Foundation

public enum InterviewTemplateError: Error, Equatable {
    case emptyTemplateName
    case noStages
    case duplicateStageID(String)
    case emptyStageName(String)
    case nonPositiveDuration(String)
    case negativeStageWarning
    case negativeOverallWarning
}

extension InterviewTemplateError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .emptyTemplateName:
            return "Template name cannot be empty."
        case .noStages:
            return "Template must include at least one stage."
        case .duplicateStageID(let stageID):
            return "Stage IDs must be unique. Duplicate id: \(stageID)"
        case .emptyStageName(let stageID):
            return "Stage name cannot be empty for stage id: \(stageID)"
        case .nonPositiveDuration(let stageID):
            return "Stage duration must be positive for stage id: \(stageID)"
        case .negativeStageWarning:
            return "Stage warning threshold cannot be negative."
        case .negativeOverallWarning:
            return "Overall warning threshold cannot be negative."
        }
    }
}

public struct InterviewTemplate: Codable, Equatable {
    public struct WarningThresholds: Codable, Equatable {
        public let stageLastSeconds: Int
        public let overallLastSeconds: Int

        public init(stageLastSeconds: Int, overallLastSeconds: Int) {
            self.stageLastSeconds = stageLastSeconds
            self.overallLastSeconds = overallLastSeconds
        }
    }

    public struct Stage: Codable, Equatable, Identifiable {
        public let id: String
        public let name: String
        public let durationMinutes: Int

        public init(id: String, name: String, durationMinutes: Int) {
            self.id = id
            self.name = name
            self.durationMinutes = durationMinutes
        }

        public var duration: TimeInterval {
            TimeInterval(durationMinutes * 60)
        }
    }

    public let templateName: String
    public let warnings: WarningThresholds
    public let stages: [Stage]

    public init(templateName: String, warnings: WarningThresholds, stages: [Stage]) {
        self.templateName = templateName
        self.warnings = warnings
        self.stages = stages
    }

    public var totalDuration: TimeInterval {
        stages.reduce(into: 0.0) { partialResult, stage in
            partialResult += stage.duration
        }
    }

    public func validate() throws {
        if templateName.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            throw InterviewTemplateError.emptyTemplateName
        }

        if warnings.stageLastSeconds < 0 {
            throw InterviewTemplateError.negativeStageWarning
        }

        if warnings.overallLastSeconds < 0 {
            throw InterviewTemplateError.negativeOverallWarning
        }

        if stages.isEmpty {
            throw InterviewTemplateError.noStages
        }

        var seenStageIDs = Set<String>()
        for stage in stages {
            if !seenStageIDs.insert(stage.id).inserted {
                throw InterviewTemplateError.duplicateStageID(stage.id)
            }

            if stage.name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                throw InterviewTemplateError.emptyStageName(stage.id)
            }

            if stage.durationMinutes <= 0 {
                throw InterviewTemplateError.nonPositiveDuration(stage.id)
            }
        }
    }

    public func encoded(prettyPrinted: Bool = true) throws -> Data {
        let encoder = JSONEncoder()
        if prettyPrinted {
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        }
        return try encoder.encode(self)
    }

    public static func decode(from data: Data) throws -> InterviewTemplate {
        let template = try JSONDecoder().decode(InterviewTemplate.self, from: data)
        try template.validate()
        return template
    }

    public static let defaultTemplate = InterviewTemplate(
        templateName: "60min Interview",
        warnings: WarningThresholds(
            stageLastSeconds: 60,
            overallLastSeconds: 300
        ),
        stages: [
            Stage(id: "intro", name: "开场与破冰", durationMinutes: 5),
            Stage(id: "resume", name: "经历深挖", durationMinutes: 15),
            Stage(id: "coding", name: "技术问题", durationMinutes: 20),
            Stage(id: "qa", name: "候选人提问", durationMinutes: 10),
            Stage(id: "close", name: "结束收口", durationMinutes: 10),
        ]
    )
}
