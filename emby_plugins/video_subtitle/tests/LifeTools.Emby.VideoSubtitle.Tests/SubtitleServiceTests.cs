using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class SubtitleServiceTests
{
    [Fact]
    public async Task ServiceSubmitReturnsJobResponseShapeForApiAdapters()
    {
        await using var store = await SqliteJobStore.OpenAsync(":memory:");
        var queue = new SubtitleJobQueue(store, new ImmediateSubtitleExecutor(), new SubtitlePluginOptions());
        var service = new SubtitleService(queue);

        var response = await service.SubmitAsync(new SubmitSubtitleJobRequest
        {
            ItemId = "item-1",
            VideoPath = "/media/video.mkv",
            SourceLanguage = "ja-JP",
            ForceAsr = true,
        });

        Assert.False(string.IsNullOrWhiteSpace(response.JobId));
        Assert.Equal("item-1", response.ItemId);
        Assert.Equal("/media/video.mkv", response.VideoPath);
        Assert.Equal(SubtitleJobStatus.Queued, response.Status);
    }

    private sealed class ImmediateSubtitleExecutor : ISubtitleExecutor
    {
        public Task<SubtitleExecutionResult> ExecuteAsync(SubtitleCommand command, CancellationToken cancellationToken)
        {
            return Task.FromResult(new SubtitleExecutionResult(0, command.ExpectedOutputPath, "ok", string.Empty));
        }
    }
}
