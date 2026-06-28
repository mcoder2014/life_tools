using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class JobStoreTests
{
    [Fact]
    public async Task SqliteStorePersistsJobSummaryAcrossInstances()
    {
        var dbPath = Path.Combine(Path.GetTempPath(), "life-tools-emby-subtitle-" + Guid.NewGuid() + ".db");
        try
        {
            var job = SubtitleJob.Create(
                new SubtitleJobRequest { VideoPath = "/media/video.mkv" },
                batchId: "batch-1",
                now: DateTimeOffset.Parse("2026-06-26T08:00:00Z"));
            job.MarkRunning(DateTimeOffset.Parse("2026-06-26T08:01:00Z"));
            job.MarkSucceeded("/media/video.zh-CN.srt", exitCode: 0, stdoutTail: "ok", stderrTail: "", DateTimeOffset.Parse("2026-06-26T08:02:00Z"));

            await using (var store = await SqliteJobStore.OpenAsync(dbPath))
            {
                await store.UpsertJobAsync(job);
            }

            await using var reopened = await SqliteJobStore.OpenAsync(dbPath);
            var loaded = await reopened.GetJobAsync(job.JobId);

            Assert.NotNull(loaded);
            Assert.Equal(job.JobId, loaded.JobId);
            Assert.Equal("batch-1", loaded.BatchId);
            Assert.Equal(SubtitleJobStatus.Succeeded, loaded.Status);
            Assert.Equal("/media/video.zh-CN.srt", loaded.OutputPath);
            Assert.Equal("ok", loaded.StdoutTail);
        }
        finally
        {
            if (File.Exists(dbPath))
            {
                File.Delete(dbPath);
            }
        }
    }
}
