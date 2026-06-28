using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class FileJobStoreTests
{
    [Fact]
    public async Task FileStorePersistsJobSummaryAcrossInstances()
    {
        var filePath = Path.Combine(Path.GetTempPath(), "life-tools-emby-subtitle-" + Guid.NewGuid() + ".jobs");
        try
        {
            var job = SubtitleJob.Create(
                new SubtitleJobRequest
                {
                    VideoPath = "/media/video.mkv",
                    SourceLanguage = "ja-JP",
                    ForceAsr = true,
                    ForceSplit = true,
                    ForceTranslate = false,
                },
                batchId: "batch-1",
                now: DateTimeOffset.Parse("2026-06-26T08:00:00Z"));
            job.MarkRunning(DateTimeOffset.Parse("2026-06-26T08:01:00Z"));
            job.MarkSucceeded("/media/video.zh-CN.srt", exitCode: 0, stdoutTail: "ok", stderrTail: "", DateTimeOffset.Parse("2026-06-26T08:02:00Z"));

            var store = await FileJobStore.OpenAsync(filePath);
            await store.UpsertJobAsync(job);

            var reopened = await FileJobStore.OpenAsync(filePath);
            var loaded = await reopened.GetJobAsync(job.JobId);

            Assert.NotNull(loaded);
            Assert.Equal(job.JobId, loaded.JobId);
            Assert.Equal("batch-1", loaded.BatchId);
            Assert.Equal("ja-JP", loaded.SourceLanguage);
            Assert.True(loaded.ForceAsr);
            Assert.True(loaded.ForceSplit);
            Assert.False(loaded.ForceTranslate);
            Assert.Equal(SubtitleJobStatus.Succeeded, loaded.Status);
            Assert.Equal("/media/video.zh-CN.srt", loaded.OutputPath);
            Assert.Equal("ok", loaded.StdoutTail);
        }
        finally
        {
            if (File.Exists(filePath))
            {
                File.Delete(filePath);
            }
        }
    }
}
