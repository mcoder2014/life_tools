using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class SubtitleJobQueueRaceTests
{
    [Fact]
    public async Task SecondJobRunsAfterWorkerHasAlreadyDrainedQueue()
    {
        var executor = new CountingSubtitleExecutor();
        await using var store = await SqliteJobStore.OpenAsync(":memory:");
        var queue = new SubtitleJobQueue(store, executor, new SubtitlePluginOptions());

        var first = await queue.EnqueueAsync(new SubtitleJobRequest { VideoPath = "/media/one.mkv" });
        await WaitForTerminalStatusAsync(queue, first.JobId);

        var second = await queue.EnqueueAsync(new SubtitleJobRequest { VideoPath = "/media/two.mkv" });
        var completed = await WaitForTerminalStatusAsync(queue, second.JobId);

        Assert.Equal(SubtitleJobStatus.Succeeded, completed.Status);
        Assert.Equal(2, executor.Count);
    }

    private static async Task<SubtitleJob> WaitForTerminalStatusAsync(SubtitleJobQueue queue, string jobId)
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(2));
        while (!timeout.IsCancellationRequested)
        {
            var job = await queue.GetJobAsync(jobId);
            if (job?.Status is SubtitleJobStatus.Succeeded or SubtitleJobStatus.Failed or SubtitleJobStatus.Canceled)
            {
                return job;
            }

            await Task.Delay(10, timeout.Token);
        }

        throw new TimeoutException("job did not reach a terminal status");
    }

    private sealed class CountingSubtitleExecutor : ISubtitleExecutor
    {
        public int Count { get; private set; }

        public Task<SubtitleExecutionResult> ExecuteAsync(SubtitleCommand command, CancellationToken cancellationToken)
        {
            Count++;
            return Task.FromResult(new SubtitleExecutionResult(0, command.ExpectedOutputPath, "ok", string.Empty));
        }
    }
}
