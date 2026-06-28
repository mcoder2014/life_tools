namespace LifeTools.Emby.VideoSubtitle;

public sealed class SubtitlePluginOptions
{
    public string ExecutablePath { get; set; } = "/usr/local/bin/video_subtitle";

    public List<string> ExtraArgs { get; set; } = new List<string>();

    public string ConfigPath { get; set; } = "/etc/life_tools/video_subtitle.json";

    public string DefaultSourceLanguage { get; set; } = "ja-JP";

    public int MaxLogTailBytes { get; set; } = 8192;
}

public sealed class SubtitleJobRequest
{
    public string? ItemId { get; set; }

    public string? VideoPath { get; set; }

    public string? SourceLanguage { get; set; }

    public bool ForceAsr { get; set; }

    public bool ForceSplit { get; set; }

    public bool ForceTranslate { get; set; }

    public bool ForceRequeue { get; set; }
}

public sealed class SubtitleCommand
{
    public SubtitleCommand(string executablePath, IReadOnlyList<string> arguments, string videoPath, string? expectedOutputPath)
    {
        ExecutablePath = executablePath;
        Arguments = arguments;
        VideoPath = videoPath;
        ExpectedOutputPath = expectedOutputPath;
    }

    public string ExecutablePath { get; }

    public IReadOnlyList<string> Arguments { get; }

    public string VideoPath { get; }

    public string? ExpectedOutputPath { get; }
}

public sealed class SubtitleExecutionResult
{
    public SubtitleExecutionResult(int exitCode, string? outputPath, string stdoutTail, string stderrTail)
    {
        ExitCode = exitCode;
        OutputPath = outputPath;
        StdoutTail = stdoutTail;
        StderrTail = stderrTail;
    }

    public int ExitCode { get; }

    public string? OutputPath { get; }

    public string StdoutTail { get; }

    public string StderrTail { get; }
}

public enum SubtitleJobStatus
{
    Queued,
    Running,
    Succeeded,
    Failed,
    CancelRequested,
    Canceled,
}

public sealed class SubtitleJob
{
    private SubtitleJob()
    {
    }

    public string JobId { get; private set; } = string.Empty;

    public string? BatchId { get; private set; }

    public string? ItemId { get; private set; }

    public string VideoPath { get; private set; } = string.Empty;

    public string? SourceLanguage { get; private set; }

    public bool ForceAsr { get; private set; }

    public bool ForceSplit { get; private set; }

    public bool ForceTranslate { get; private set; }

    public SubtitleJobStatus Status { get; private set; }

    public DateTimeOffset CreatedAt { get; private set; }

    public DateTimeOffset? StartedAt { get; private set; }

    public DateTimeOffset? FinishedAt { get; private set; }

    public string? OutputPath { get; private set; }

    public int? ExitCode { get; private set; }

    public string? StdoutTail { get; private set; }

    public string? StderrTail { get; private set; }

    public string? ErrorMessage { get; private set; }

    public static SubtitleJob Create(SubtitleJobRequest request, string? batchId, DateTimeOffset now)
    {
        if (string.IsNullOrWhiteSpace(request.VideoPath))
        {
            throw new ArgumentException("video path is required", nameof(request));
        }

        return new SubtitleJob
        {
            JobId = Guid.NewGuid().ToString("N"),
            BatchId = string.IsNullOrWhiteSpace(batchId) ? null : batchId,
            ItemId = string.IsNullOrWhiteSpace(request.ItemId) ? null : request.ItemId,
            VideoPath = request.VideoPath!,
            SourceLanguage = string.IsNullOrWhiteSpace(request.SourceLanguage) ? null : request.SourceLanguage,
            ForceAsr = request.ForceAsr,
            ForceSplit = request.ForceSplit,
            ForceTranslate = request.ForceTranslate,
            Status = SubtitleJobStatus.Queued,
            CreatedAt = now,
        };
    }

    public void MarkRunning(DateTimeOffset now)
    {
        Status = SubtitleJobStatus.Running;
        StartedAt = now;
    }

    public void MarkCancelRequested(DateTimeOffset now)
    {
        if (Status is SubtitleJobStatus.Succeeded or SubtitleJobStatus.Failed or SubtitleJobStatus.Canceled)
        {
            return;
        }

        Status = SubtitleJobStatus.CancelRequested;
    }

    public void MarkCanceled(DateTimeOffset now, string? message = null)
    {
        Status = SubtitleJobStatus.Canceled;
        FinishedAt = now;
        ErrorMessage = message;
    }

    public void MarkSucceeded(string? outputPath, int exitCode, string stdoutTail, string stderrTail, DateTimeOffset now)
    {
        Status = SubtitleJobStatus.Succeeded;
        OutputPath = outputPath;
        ExitCode = exitCode;
        StdoutTail = stdoutTail;
        StderrTail = stderrTail;
        FinishedAt = now;
    }

    public void MarkFailed(int? exitCode, string stdoutTail, string stderrTail, string message, DateTimeOffset now)
    {
        Status = SubtitleJobStatus.Failed;
        ExitCode = exitCode;
        StdoutTail = stdoutTail;
        StderrTail = stderrTail;
        ErrorMessage = message;
        FinishedAt = now;
    }

    internal static SubtitleJob Hydrate(
        string jobId,
        string? batchId,
        string? itemId,
        string videoPath,
        string? sourceLanguage,
        bool forceAsr,
        bool forceSplit,
        bool forceTranslate,
        SubtitleJobStatus status,
        DateTimeOffset createdAt,
        DateTimeOffset? startedAt,
        DateTimeOffset? finishedAt,
        string? outputPath,
        int? exitCode,
        string? stdoutTail,
        string? stderrTail,
        string? errorMessage)
    {
        return new SubtitleJob
        {
            JobId = jobId,
            BatchId = batchId,
            ItemId = itemId,
            VideoPath = videoPath,
            SourceLanguage = sourceLanguage,
            ForceAsr = forceAsr,
            ForceSplit = forceSplit,
            ForceTranslate = forceTranslate,
            Status = status,
            CreatedAt = createdAt,
            StartedAt = startedAt,
            FinishedAt = finishedAt,
            OutputPath = outputPath,
            ExitCode = exitCode,
            StdoutTail = stdoutTail,
            StderrTail = stderrTail,
            ErrorMessage = errorMessage,
        };
    }
}
