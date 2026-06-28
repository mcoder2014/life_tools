using MediaBrowser.Model.Plugins;

namespace LifeTools.Emby.VideoSubtitle.Emby;

public sealed class PluginConfiguration : BasePluginConfiguration
{
    public string ExecutablePath { get; set; } = "/usr/local/bin/video_subtitle";

    public string ExtraArgs { get; set; } = string.Empty;

    public string ConfigPath { get; set; } = "/etc/life_tools/video_subtitle.json";

    public string DefaultSourceLanguage { get; set; } = "ja-JP";

    public int MaxLogTailBytes { get; set; } = 8192;
}
