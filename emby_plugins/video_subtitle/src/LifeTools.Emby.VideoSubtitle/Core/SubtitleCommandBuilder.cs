namespace LifeTools.Emby.VideoSubtitle;

public static class SubtitleCommandBuilder
{
    public static SubtitleCommand Build(SubtitlePluginOptions options, SubtitleJobRequest request)
    {
        if (string.IsNullOrWhiteSpace(options.ExecutablePath))
        {
            throw new ArgumentException("executable path is required", nameof(options));
        }
        if (string.IsNullOrWhiteSpace(request.VideoPath))
        {
            throw new ArgumentException("video path is required", nameof(request));
        }

        var videoPath = request.VideoPath!;

        var args = new List<string>();
        args.AddRange(options.ExtraArgs.Where(x => !string.IsNullOrWhiteSpace(x)));
        args.Add("--input");
        args.Add(videoPath);

        if (!string.IsNullOrWhiteSpace(options.ConfigPath))
        {
            args.Add("--config");
            args.Add(options.ConfigPath);
        }

        var language = string.IsNullOrWhiteSpace(request.SourceLanguage)
            ? options.DefaultSourceLanguage
            : request.SourceLanguage;
        if (!string.IsNullOrWhiteSpace(language))
        {
            args.Add("--source-language");
            args.Add(language!);
        }

        args.Add("--yes");
        AddFlag(args, request.ForceAsr, "--force-asr");
        AddFlag(args, request.ForceSplit, "--force-split");
        AddFlag(args, request.ForceTranslate, "--force-translate");

        return new SubtitleCommand(
            options.ExecutablePath,
            args,
            videoPath,
            GuessDefaultOutputPath(videoPath));
    }

    public static SubtitleCommand Build(SubtitlePluginOptions options, SubtitleJob job)
    {
        return Build(options, new SubtitleJobRequest
        {
            ItemId = job.ItemId,
            VideoPath = job.VideoPath,
            SourceLanguage = job.SourceLanguage,
            ForceAsr = job.ForceAsr,
            ForceSplit = job.ForceSplit,
            ForceTranslate = job.ForceTranslate,
        });
    }

    private static void AddFlag(List<string> args, bool enabled, string flag)
    {
        if (enabled)
        {
            args.Add(flag);
        }
    }

    private static string GuessDefaultOutputPath(string videoPath)
    {
        var directory = Path.GetDirectoryName(videoPath) ?? string.Empty;
        var fileName = Path.GetFileNameWithoutExtension(videoPath);
        return Path.Combine(directory, fileName + ".zh-CN.srt");
    }
}
