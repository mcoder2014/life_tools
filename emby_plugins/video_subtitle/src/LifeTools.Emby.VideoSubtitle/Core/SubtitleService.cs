namespace LifeTools.Emby.VideoSubtitle;

public sealed class SubtitleService
{
    private readonly SubtitleJobQueue _queue;

    public SubtitleService(SubtitleJobQueue queue)
    {
        _queue = queue;
    }

    public async Task<SubtitleJobResponse> SubmitAsync(SubmitSubtitleJobRequest request)
    {
        if (request == null)
        {
            throw new ArgumentNullException(nameof(request));
        }
        var job = await _queue.EnqueueAsync(ToJobRequest(request));
        return ToResponse(job);
    }

    public async Task<SubtitleBatchResponse> SubmitBatchAsync(SubmitSubtitleBatchRequest request)
    {
        if (request == null)
        {
            throw new ArgumentNullException(nameof(request));
        }
        var batch = await _queue.EnqueueBatchAsync(new SubtitleBatchRequest
        {
            ForceRequeue = request.ForceRequeue,
            Items = request.Items.Select(ToJobRequest).ToList(),
        });

        return new SubtitleBatchResponse(batch.BatchId, batch.Jobs.Select(ToResponse).ToList());
    }

    public async Task<SubtitleJobResponse?> GetAsync(string jobId)
    {
        var job = await _queue.GetJobAsync(jobId);
        return job is null ? null : ToResponse(job);
    }

    public async Task<IReadOnlyList<SubtitleJobResponse>> ListAsync(int limit)
    {
        var jobs = await _queue.ListJobsAsync(limit);
        return jobs.Select(ToResponse).ToList();
    }

    public Task<bool> CancelAsync(string jobId)
    {
        return _queue.CancelJobAsync(jobId);
    }

    private static SubtitleJobRequest ToJobRequest(SubmitSubtitleJobRequest request)
    {
        return new SubtitleJobRequest
        {
            ItemId = request.ItemId,
            VideoPath = request.VideoPath,
            SourceLanguage = request.SourceLanguage,
            ForceAsr = request.ForceAsr,
            ForceSplit = request.ForceSplit,
            ForceTranslate = request.ForceTranslate,
            ForceRequeue = request.ForceRequeue,
        };
    }

    private static SubtitleJobResponse ToResponse(SubtitleJob job)
    {
        return new SubtitleJobResponse(
            job.JobId,
            job.BatchId,
            job.ItemId,
            job.VideoPath,
            job.Status,
            job.OutputPath,
            job.ExitCode,
            job.ErrorMessage,
            job.CreatedAt,
            job.StartedAt,
            job.FinishedAt);
    }
}
