using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class BatchQueueTests
{
    [Fact]
    public async Task EnqueueBatchCreatesOneBatchIdAndAJobForEachVideo()
    {
        var executor = new ImmediateSubtitleExecutor();
        await using var store = await SqliteJobStore.OpenAsync(":memory:");
        var queue = new SubtitleJobQueue(store, executor, new SubtitlePluginOptions());

        var batch = await queue.EnqueueBatchAsync(new SubtitleBatchRequest
        {
            Items =
            [
                new SubtitleJobRequest { ItemId = "item-1", VideoPath = "/media/one.mkv", ForceAsr = true },
                new SubtitleJobRequest { ItemId = "item-2", VideoPath = "/media/two.mkv", ForceTranslate = true },
            ],
        });

        Assert.False(string.IsNullOrWhiteSpace(batch.BatchId));
        Assert.Equal(2, batch.Jobs.Count);
        Assert.All(batch.Jobs, job => Assert.Equal(batch.BatchId, job.BatchId));
        Assert.Contains(batch.Jobs, job => job.ItemId == "item-1" && job.VideoPath == "/media/one.mkv");
        Assert.Contains(batch.Jobs, job => job.ItemId == "item-2" && job.VideoPath == "/media/two.mkv");
    }

    private sealed class ImmediateSubtitleExecutor : ISubtitleExecutor
    {
        public Task<SubtitleExecutionResult> ExecuteAsync(SubtitleCommand command, CancellationToken cancellationToken)
        {
            return Task.FromResult(new SubtitleExecutionResult(0, command.ExpectedOutputPath, "ok", string.Empty));
        }
    }
}
