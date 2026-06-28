using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class ProcessSubtitleExecutorCancellationTests
{
    [Fact]
    public async Task ExecuteKillsProcessWhenCancellationIsRequested()
    {
        if (!OperatingSystem.IsLinux() && !OperatingSystem.IsMacOS())
        {
            return;
        }

        using var cancellation = new CancellationTokenSource(TimeSpan.FromMilliseconds(100));
        var command = new SubtitleCommand(
            "/bin/sh",
            ["-c", "sleep 10"],
            "/media/video.mkv",
            "/media/video.zh-CN.srt");

        await Assert.ThrowsAsync<TaskCanceledException>(() =>
            new ProcessSubtitleExecutor().ExecuteAsync(command, cancellation.Token));
    }
}
