namespace LifeTools.Emby.VideoSubtitle;

public sealed class SubtitleBatchRequest
{
    public List<SubtitleJobRequest> Items { get; set; } = new List<SubtitleJobRequest>();

    public bool ForceRequeue { get; set; }
}

public sealed class SubtitleBatchResult
{
    public SubtitleBatchResult(string batchId, IReadOnlyList<SubtitleJob> jobs)
    {
        BatchId = batchId;
        Jobs = jobs;
    }

    public string BatchId { get; }

    public IReadOnlyList<SubtitleJob> Jobs { get; }
}
