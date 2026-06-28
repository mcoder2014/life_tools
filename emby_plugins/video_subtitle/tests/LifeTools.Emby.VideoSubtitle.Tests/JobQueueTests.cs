using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class JobQueueTests
{
    [Fact]
    public async Task EnqueueReturnsExistingQueuedJobForSameVideoPath()
    {
        var executor = new RecordingSubtitleExecutor(TimeSpan.FromMilliseconds(10));
        await using var store = await SqliteJobStore.OpenAsync(":memory:");
        var queue = new SubtitleJobQueue(store, executor, new SubtitlePluginOptions());
        var request = new SubtitleJobRequest { VideoPath = "/media/video.mkv" };

        var first = await queue.EnqueueAsync(request);
        var second = await queue.EnqueueAsync(request);

        Assert.Equal(first.JobId, second.JobId);
        Assert.Single(await queue.ListJobsAsync(10));
    }

    [Fact]
    public async Task CancelRunningJobRequestsExecutorCancellation()
    {
        var executor = new RecordingSubtitleExecutor(TimeSpan.FromMinutes(1));
        await using var store = await SqliteJobStore.OpenAsync(":memory:");
        var queue = new SubtitleJobQueue(store, executor, new SubtitlePluginOptions());

        var job = await queue.EnqueueAsync(new SubtitleJobRequest { VideoPath = "/media/video.mkv" });
        await executor.WaitUntilStartedAsync(TimeSpan.FromSeconds(2));

        var canceled = await queue.CancelJobAsync(job.JobId);

        Assert.True(canceled);
        Assert.True(executor.CancellationRequested);
        var loaded = await queue.GetJobAsync(job.JobId);
        Assert.True(loaded!.Status is SubtitleJobStatus.CancelRequested or SubtitleJobStatus.Canceled);
    }

    [Fact]
    public async Task CompletedJobNotifiesCompletionSinkAfterSucceededStateIsStored()
    {
        var executor = new RecordingSubtitleExecutor(TimeSpan.FromMilliseconds(1));
        var sink = new RecordingCompletionSink();
        await using var store = await SqliteJobStore.OpenAsync(":memory:");
        var queue = new SubtitleJobQueue(store, executor, new SubtitlePluginOptions(), sink);

        var job = await queue.EnqueueAsync(new SubtitleJobRequest { VideoPath = "/media/video.mkv" });
        var completed = await sink.WaitForJobAsync(TimeSpan.FromSeconds(2));
        var loaded = await store.GetJobAsync(job.JobId);

        Assert.Equal(job.JobId, completed.JobId);
        Assert.Equal(SubtitleJobStatus.Succeeded, completed.Status);
        Assert.Equal(SubtitleJobStatus.Succeeded, loaded!.Status);
        Assert.Equal(1, sink.Count);
    }

    private sealed class RecordingSubtitleExecutor(TimeSpan delay) : ISubtitleExecutor
    {
        private readonly TaskCompletionSource _started = new(TaskCreationOptions.RunContinuationsAsynchronously);

        public bool CancellationRequested { get; private set; }

        public async Task<SubtitleExecutionResult> ExecuteAsync(SubtitleCommand command, CancellationToken cancellationToken)
        {
            _started.TrySetResult();
            try
            {
                await Task.Delay(delay, cancellationToken);
                return new SubtitleExecutionResult(0, command.ExpectedOutputPath, "ok", "");
            }
            catch (OperationCanceledException)
            {
                CancellationRequested = true;
                throw;
            }
        }

        public Task WaitUntilStartedAsync(TimeSpan timeout)
        {
            return _started.Task.WaitAsync(timeout);
        }
    }

    private sealed class RecordingCompletionSink : ISubtitleJobCompletionSink
    {
        private readonly TaskCompletionSource<SubtitleJob> _completed = new(TaskCreationOptions.RunContinuationsAsynchronously);

        public int Count { get; private set; }

        public Task OnJobCompletedAsync(SubtitleJob job)
        {
            Count++;
            _completed.TrySetResult(job);
            return Task.CompletedTask;
        }

        public Task<SubtitleJob> WaitForJobAsync(TimeSpan timeout)
        {
            return _completed.Task.WaitAsync(timeout);
        }
    }
}
