using System;
using System.Linq;
using System.Threading.Tasks;
using MediaBrowser.Common;
using MediaBrowser.Common.Extensions;
using MediaBrowser.Controller.Library;
using MediaBrowser.Controller.Providers;
using MediaBrowser.Model.IO;
using MediaBrowser.Model.Logging;
using MediaBrowser.Model.Services;

namespace LifeTools.Emby.VideoSubtitle.Emby.Services;

public sealed class SubtitleApiService : IService
{
    private readonly IApplicationHost _applicationHost;
    private readonly EmbySubtitleLibrary _library;

    public SubtitleApiService(IApplicationHost applicationHost, ILibraryManager libraryManager, IProviderManager providerManager, IFileSystem fileSystem, ILogManager logManager)
    {
        _applicationHost = applicationHost;
        _library = new EmbySubtitleLibrary(libraryManager, providerManager, fileSystem, logManager.GetLogger(Plugin.PluginName));
    }

    public Task<SubtitleJobResponse> Post(SubmitSubtitleJob request)
    {
        return Plugin.GetSubtitleService(_library).SubmitAsync(_library.Resolve(request));
    }

    public Task<SubtitleBatchResponse> Post(SubmitSubtitleBatch request)
    {
        return Plugin.GetSubtitleService(_library).SubmitBatchAsync(new SubmitSubtitleBatchRequest
        {
            ForceRequeue = request.ForceRequeue,
            Items = request.Items.Select(_library.Resolve).ToList(),
        });
    }

    public async Task<SubtitleJobResponse> Get(GetSubtitleJob request)
    {
        var response = await Plugin.GetSubtitleService(_library).GetAsync(request.JobId);
        if (response == null)
        {
            throw new ResourceNotFoundException("subtitle job not found");
        }

        return response;
    }

    public Task<System.Collections.Generic.IReadOnlyList<SubtitleJobResponse>> Get(ListSubtitleJobs request)
    {
        return Plugin.GetSubtitleService(_library).ListAsync(request.Limit);
    }

    public async Task<CancelSubtitleJobResponse> Post(CancelSubtitleJob request)
    {
        return new CancelSubtitleJobResponse
        {
            Canceled = await Plugin.GetSubtitleService(_library).CancelAsync(request.JobId),
        };
    }

    private Plugin Plugin
    {
        get
        {
            var plugin = _applicationHost.Plugins.OfType<Plugin>().FirstOrDefault();
            if (plugin == null)
            {
                throw new InvalidOperationException(Plugin.PluginName + " is not loaded");
            }

            return plugin;
        }
    }
}
