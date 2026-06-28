namespace LifeTools.Emby.VideoSubtitle;

public sealed class SubmitSubtitleJobRequest
{
    public string? ItemId { get; set; }

    public string? VideoPath { get; set; }

    public string? SourceLanguage { get; set; }

    public bool ForceAsr { get; set; }

    public bool ForceSplit { get; set; }

    public bool ForceTranslate { get; set; }

    public bool ForceRequeue { get; set; }
}

public sealed class SubmitSubtitleBatchRequest
{
    public List<SubmitSubtitleJobRequest> Items { get; set; } = new List<SubmitSubtitleJobRequest>();

    public bool ForceRequeue { get; set; }
}

public sealed class SubtitleJobResponse
{
    public SubtitleJobResponse(string jobId, string? batchId, string? itemId, string videoPath, SubtitleJobStatus status, string? outputPath, int? exitCode, string? errorMessage, DateTimeOffset createdAt, DateTimeOffset? startedAt, DateTimeOffset? finishedAt)
    {
        JobId = jobId;
        BatchId = batchId;
        ItemId = itemId;
        VideoPath = videoPath;
        Status = status;
        OutputPath = outputPath;
        ExitCode = exitCode;
        ErrorMessage = errorMessage;
        CreatedAt = createdAt;
        StartedAt = startedAt;
        FinishedAt = finishedAt;
    }

    public string JobId { get; }
    public string? BatchId { get; }
    public string? ItemId { get; }
    public string VideoPath { get; }
    public SubtitleJobStatus Status { get; }
    public string? OutputPath { get; }
    public int? ExitCode { get; }
    public string? ErrorMessage { get; }
    public DateTimeOffset CreatedAt { get; }
    public DateTimeOffset? StartedAt { get; }
    public DateTimeOffset? FinishedAt { get; }
}

public sealed class SubtitleBatchResponse
{
    public SubtitleBatchResponse(string batchId, IReadOnlyList<SubtitleJobResponse> jobs)
    {
        BatchId = batchId;
        Jobs = jobs;
    }

    public string BatchId { get; }

    public IReadOnlyList<SubtitleJobResponse> Jobs { get; }
}
