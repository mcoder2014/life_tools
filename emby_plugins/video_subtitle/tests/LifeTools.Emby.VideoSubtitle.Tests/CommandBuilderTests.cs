using LifeTools.Emby.VideoSubtitle;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class CommandBuilderTests
{
    [Fact]
    public void BuildCreatesArgvWithoutShellExpansion()
    {
        var options = new SubtitlePluginOptions
        {
            ExecutablePath = "/usr/bin/python3",
            ExtraArgs = ["/opt/life tools/video_subtitle.py"],
            ConfigPath = "/etc/life_tools/video_subtitle.json",
            DefaultSourceLanguage = "ja-JP",
        };
        var request = new SubtitleJobRequest
        {
            VideoPath = "/media/anime/Episode 01 [1080p].mkv",
            ForceAsr = true,
            ForceTranslate = true,
            ForceSplit = false,
        };

        var command = SubtitleCommandBuilder.Build(options, request);

        Assert.Equal("/usr/bin/python3", command.ExecutablePath);
        Assert.Equal(
            [
                "/opt/life tools/video_subtitle.py",
                "--input",
                "/media/anime/Episode 01 [1080p].mkv",
                "--config",
                "/etc/life_tools/video_subtitle.json",
                "--source-language",
                "ja-JP",
                "--yes",
                "--force-asr",
                "--force-translate",
            ],
            command.Arguments);
    }
}
