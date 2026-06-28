namespace LifeTools.Emby.VideoSubtitle;

public sealed class SubtitleJobQueue
{
    private readonly ISubtitleJobStore _store;
    private readonly ISubtitleExecutor _executor;
    private readonly SubtitlePluginOptions _options;
    private readonly ISubtitleJobCompletionSink? _completionSink;
    private readonly SemaphoreSlim _lock = new(1, 1);
    private readonly Queue<string> _pendingJobIds = new();
    private readonly Dictionary<string, RunningJob> _runningJobs = new(StringComparer.Ordinal);
    private Task? _workerTask;

    public SubtitleJobQueue(ISubtitleJobStore store, ISubtitleExecutor executor, SubtitlePluginOptions options, ISubtitleJobCompletionSink? completionSink = null)
    {
        _store = store;
        _executor = executor;
        _options = options;
        _completionSink = completionSink;
    }

    public async Task<SubtitleJob> EnqueueAsync(SubtitleJobRequest request)
    {
        if (request == null)
        {
            throw new ArgumentNullException(nameof(request));
        }

        await _lock.WaitAsync();
        try
        {
            var job = await EnqueueLockedAsync(request, batchId: null, forceRequeue: request.ForceRequeue);
            EnsureWorkerStarted();
            return job;
        }
        finally
        {
            _lock.Release();
        }
    }

    public async Task<SubtitleBatchResult> EnqueueBatchAsync(SubtitleBatchRequest request)
    {
        if (request == null)
        {
            throw new ArgumentNullException(nameof(request));
        }
        if (request.Items.Count == 0)
        {
            throw new ArgumentException("batch must contain at least one item", nameof(request));
        }

        var batchId = Guid.NewGuid().ToString("N");
        var jobs = new List<SubtitleJob>(request.Items.Count);

        await _lock.WaitAsync();
        try
        {
            foreach (var item in request.Items)
            {
                jobs.Add(await EnqueueLockedAsync(item, batchId, request.ForceRequeue || item.ForceRequeue));
            }

            EnsureWorkerStarted();
        }
        finally
        {
            _lock.Release();
        }

        return new SubtitleBatchResult(batchId, jobs);
    }

    private async Task<SubtitleJob> EnqueueLockedAsync(SubtitleJobRequest request, string? batchId, bool forceRequeue)
    {
        if (!forceRequeue && !string.IsNullOrWhiteSpace(request.VideoPath))
        {
            var existing = await _store.FindActiveJobByVideoPathAsync(request.VideoPath!);
            if (existing is not null)
            {
                return existing;
            }
        }

        var job = SubtitleJob.Create(request, batchId, DateTimeOffset.UtcNow);
        await _store.UpsertJobAsync(job);
        _pendingJobIds.Enqueue(job.JobId);
        return job;
    }

    public Task<SubtitleJob?> GetJobAsync(string jobId)
    {
        return _store.GetJobAsync(jobId);
    }

    public Task<IReadOnlyList<SubtitleJob>> ListJobsAsync(int limit)
    {
        return _store.ListJobsAsync(limit);
    }

    public async Task<bool> CancelJobAsync(string jobId)
    {
        if (string.IsNullOrWhiteSpace(jobId))
        {
            return false;
        }

        RunningJob? running = null;
        await _lock.WaitAsync();
        try
        {
            var job = await _store.GetJobAsync(jobId);
            if (job is null)
            {
                return false;
            }

            if (job.Status == SubtitleJobStatus.Queued)
            {
                job.MarkCanceled(DateTimeOffset.UtcNow, "Canceled before execution.");
                await _store.UpsertJobAsync(job);
                return true;
            }

            if (job.Status != SubtitleJobStatus.Running)
            {
                return false;
            }

            job.MarkCancelRequested(DateTimeOffset.UtcNow);
            await _store.UpsertJobAsync(job);
            if (_runningJobs.TryGetValue(jobId, out running))
            {
                running.Cancellation.Cancel();
            }
        }
        finally
        {
            _lock.Release();
        }

        if (running is not null)
        {
            await Task.WhenAny(running.Finished.Task, Task.Delay(TimeSpan.FromMilliseconds(500)));
        }

        return true;
    }

    private void EnsureWorkerStarted()
    {
        if (_workerTask is null)
        {
            _workerTask = Task.Run(ProcessJobsAsync);
        }
    }

    private async Task ProcessJobsAsync()
    {
        while (true)
        {
            var jobId = await TryDequeueAsync();
            if (jobId is null)
            {
                return;
            }

            await RunJobAsync(jobId);
        }
    }

    private async Task<string?> TryDequeueAsync()
    {
        await _lock.WaitAsync();
        try
        {
            if (_pendingJobIds.Count == 0)
            {
                _workerTask = null;
                return null;
            }

            return _pendingJobIds.Dequeue();
        }
        finally
        {
            _lock.Release();
        }
    }

    private async Task RunJobAsync(string jobId)
    {
        var job = await _store.GetJobAsync(jobId);
        if (job is null || job.Status == SubtitleJobStatus.Canceled)
        {
            return;
        }

        var running = new RunningJob();
        await _lock.WaitAsync();
        try
        {
            job.MarkRunning(DateTimeOffset.UtcNow);
            await _store.UpsertJobAsync(job);
            _runningJobs[job.JobId] = running;
        }
        finally
        {
            _lock.Release();
        }

        try
        {
            var command = SubtitleCommandBuilder.Build(_options, job);
            var result = await _executor.ExecuteAsync(command, running.Cancellation.Token);
            if (result.ExitCode == 0)
            {
                job.MarkSucceeded(result.OutputPath, result.ExitCode, result.StdoutTail, result.StderrTail, DateTimeOffset.UtcNow);
            }
            else
            {
                job.MarkFailed(result.ExitCode, result.StdoutTail, result.StderrTail, "video_subtitle exited with code " + result.ExitCode, DateTimeOffset.UtcNow);
            }
        }
        catch (OperationCanceledException)
        {
            job.MarkCanceled(DateTimeOffset.UtcNow, "Execution canceled.");
        }
        catch (Exception ex)
        {
            job.MarkFailed(null, string.Empty, string.Empty, ex.Message, DateTimeOffset.UtcNow);
        }
        finally
        {
            await _lock.WaitAsync();
            try
            {
                _runningJobs.Remove(job.JobId);
                await _store.UpsertJobAsync(job);
                running.Finished.TrySetResult(true);
            }
            finally
            {
                _lock.Release();
                running.Cancellation.Dispose();
            }

            await NotifyCompletedAsync(job);
        }
    }

    private async Task NotifyCompletedAsync(SubtitleJob job)
    {
        if (_completionSink == null)
        {
            return;
        }

        try
        {
            await _completionSink.OnJobCompletedAsync(job);
        }
        catch
        {
        }
    }

    private sealed class RunningJob
    {
        public CancellationTokenSource Cancellation { get; } = new();

        public TaskCompletionSource<bool> Finished { get; } = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
    }
}
