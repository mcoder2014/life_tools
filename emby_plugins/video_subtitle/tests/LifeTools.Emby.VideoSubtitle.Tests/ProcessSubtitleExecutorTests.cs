using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class ProcessSubtitleExecutorTests
{
    [Fact]
    public async Task ExecutePassesArgumentsWithoutShellExpansion()
    {
        if (!OperatingSystem.IsLinux() && !OperatingSystem.IsMacOS())
        {
            return;
        }

        var outputPath = Path.Combine(Path.GetTempPath(), "life-tools-argv-" + Guid.NewGuid() + ".txt");
        var command = new SubtitleCommand(
            "/bin/sh",
            ["-c", "printf '%s\\n' \"$@\" > \"$0\"", outputPath, "semi;colon", "space value", "[brackets]"],
            "/media/video.mkv",
            "/media/video.zh-CN.srt");

        try
        {
            var result = await new ProcessSubtitleExecutor().ExecuteAsync(command, CancellationToken.None);

            Assert.Equal(0, result.ExitCode);
            Assert.Equal(new[] { "semi;colon", "space value", "[brackets]" }, File.ReadAllLines(outputPath));
        }
        finally
        {
            if (File.Exists(outputPath))
            {
                File.Delete(outputPath);
            }
        }
    }
}
