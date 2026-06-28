using System;
using System.Collections.Generic;
using System.IO;
using MediaBrowser.Common.Configuration;
using MediaBrowser.Common.Plugins;
using MediaBrowser.Model.Logging;
using MediaBrowser.Model.Plugins;
using MediaBrowser.Model.Serialization;

namespace LifeTools.Emby.VideoSubtitle.Emby;

public sealed class Plugin : BasePlugin<PluginConfiguration>, IHasWebPages
{
    public const string PluginName = "Life Tools Video Subtitle";
    public const string ConfigurationPageName = "LifeToolsVideoSubtitleV11";
    public const string ControllerPageName = "LifeToolsVideoSubtitleJsV11";
    public const string LegacyConfigurationPageName = "LifeToolsVideoSubtitle";
    public const string LegacyControllerPageName = "LifeToolsVideoSubtitleJs";
    public const string LegacyV2ConfigurationPageName = "LifeToolsVideoSubtitleV2";
    public const string LegacyV2ControllerPageName = "LifeToolsVideoSubtitleJsV2";
    public const string LegacyV3ConfigurationPageName = "LifeToolsVideoSubtitleV3";
    public const string LegacyV3ControllerPageName = "LifeToolsVideoSubtitleJsV3";
    public const string LegacyV4ConfigurationPageName = "LifeToolsVideoSubtitleV4";
    public const string LegacyV4ControllerPageName = "LifeToolsVideoSubtitleJsV4";
    public const string LegacyV5ConfigurationPageName = "LifeToolsVideoSubtitleV5";
    public const string LegacyV5ControllerPageName = "LifeToolsVideoSubtitleJsV5";
    public const string LegacyV6ConfigurationPageName = "LifeToolsVideoSubtitleV6";
    public const string LegacyV6ControllerPageName = "LifeToolsVideoSubtitleJsV6";
    public const string LegacyV7ConfigurationPageName = "LifeToolsVideoSubtitleV7";
    public const string LegacyV7ControllerPageName = "LifeToolsVideoSubtitleJsV7";
    public const string LegacyV8ConfigurationPageName = "LifeToolsVideoSubtitleV8";
    public const string LegacyV8ControllerPageName = "LifeToolsVideoSubtitleJsV8";
    public const string LegacyV9ConfigurationPageName = "LifeToolsVideoSubtitleV9";
    public const string LegacyV9ControllerPageName = "LifeToolsVideoSubtitleJsV9";
    public const string LegacyV10ConfigurationPageName = "LifeToolsVideoSubtitleV10";
    public const string LegacyV10ControllerPageName = "LifeToolsVideoSubtitleJsV10";

    public static readonly Guid PluginId = new Guid("C62D8714-7F3C-49F0-B4BB-A1B2D9C77A55");

    private readonly ILogger _logger;
    private readonly string _jobStorePath;
    private readonly object _serviceLock = new object();
    private SubtitleService? _subtitleService;
    private FileJobStore? _store;
    private ISubtitleJobCompletionSink? _completionSink;

    public Plugin(IApplicationPaths applicationPaths, IXmlSerializer xmlSerializer, ILogManager logManager)
        : base(applicationPaths, xmlSerializer)
    {
        _logger = logManager.GetLogger(Name);
        _jobStorePath = Path.Combine(applicationPaths.DataPath, "life_tools_video_subtitle", "jobs.tsv");
        _logger.Info("{0} loaded. Job store path: {1}", Name, _jobStorePath);
    }

    public override string Description => "Generate Chinese SRT subtitles for Emby videos by calling life_tools video_subtitle.";

    public override Guid Id => PluginId;

    public override string Name => PluginName;

    public IEnumerable<PluginPageInfo> GetPages()
    {
        return new[]
        {
            new PluginPageInfo
            {
                Name = ConfigurationPageName,
                DisplayName = "字幕生成",
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
                EnableInMainMenu = true,
                IsMainConfigPage = true,
                MenuIcon = "closed_caption",
                MenuSection = "advanced",
            },
            new PluginPageInfo
            {
                Name = ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV2ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV2ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV3ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV3ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV4ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV4ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV5ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV5ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV6ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV6ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV7ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV7ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV8ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV8ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV9ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV9ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
            new PluginPageInfo
            {
                Name = LegacyV10ConfigurationPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.html",
            },
            new PluginPageInfo
            {
                Name = LegacyV10ControllerPageName,
                EmbeddedResourcePath = GetType().Namespace + ".Web.subtitle.js",
            },
        };
    }

    internal SubtitleService GetSubtitleService(ISubtitleJobCompletionSink? completionSink = null)
    {
        lock (_serviceLock)
        {
            if (_completionSink == null && completionSink != null)
            {
                _completionSink = completionSink;
            }

            if (_subtitleService != null)
            {
                return _subtitleService;
            }

            var options = ToCoreOptions(Configuration);
            var store = FileJobStore.OpenAsync(_jobStorePath).GetAwaiter().GetResult();
            var executor = new ProcessSubtitleExecutor(options.MaxLogTailBytes);
            var queue = new SubtitleJobQueue(store, executor, options, _completionSink);
            _store = store;
            _subtitleService = new SubtitleService(queue);
            return _subtitleService;
        }
    }

    public override void UpdateConfiguration(MediaBrowser.Model.Plugins.BasePluginConfiguration configuration)
    {
        base.UpdateConfiguration(configuration);
        ResetSubtitleService();
    }

    public override void OnUninstalling()
    {
        ResetSubtitleService();
        base.OnUninstalling();
    }

    private void ResetSubtitleService()
    {
        lock (_serviceLock)
        {
            _subtitleService = null;
            _store = null;
        }
    }

    private static SubtitlePluginOptions ToCoreOptions(PluginConfiguration configuration)
    {
        return new SubtitlePluginOptions
        {
            ExecutablePath = string.IsNullOrWhiteSpace(configuration.ExecutablePath) ? "/usr/local/bin/video_subtitle" : configuration.ExecutablePath,
            ExtraArgs = SplitExtraArgs(configuration.ExtraArgs),
            ConfigPath = configuration.ConfigPath,
            DefaultSourceLanguage = configuration.DefaultSourceLanguage,
            MaxLogTailBytes = configuration.MaxLogTailBytes <= 0 ? 8192 : configuration.MaxLogTailBytes,
        };
    }

    private static List<string> SplitExtraArgs(string? value)
    {
        var result = new List<string>();
        if (string.IsNullOrWhiteSpace(value))
        {
            return result;
        }

        foreach (var line in value!.Split(new[] { '\r', '\n' }, StringSplitOptions.RemoveEmptyEntries))
        {
            var item = line.Trim();
            if (item.Length > 0)
            {
                result.Add(item);
            }
        }

        return result;
    }
}
