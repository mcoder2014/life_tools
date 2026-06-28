using System;
using System.Threading.Tasks;
using MediaBrowser.Common.Extensions;
using MediaBrowser.Controller.Entities;
using MediaBrowser.Controller.Library;
using MediaBrowser.Controller.Providers;
using MediaBrowser.Model.IO;
using MediaBrowser.Model.Logging;
using MediaBrowser.Model.Services;

namespace LifeTools.Emby.VideoSubtitle.Emby.Services;

internal sealed class EmbySubtitleLibrary : ISubtitleJobCompletionSink
{
    private readonly ILibraryManager _libraryManager;
    private readonly IProviderManager _providerManager;
    private readonly IFileSystem _fileSystem;
    private readonly ILogger _logger;

    public EmbySubtitleLibrary(ILibraryManager libraryManager, IProviderManager providerManager, IFileSystem fileSystem, ILogger logger)
    {
        _libraryManager = libraryManager;
        _providerManager = providerManager;
        _fileSystem = fileSystem;
        _logger = logger;
    }

    public SubmitSubtitleJobRequest Resolve(SubmitSubtitleJob request)
    {
        return Resolve(request.ItemId, request.VideoPath, request.SourceLanguage, request.ForceAsr, request.ForceSplit, request.ForceTranslate, request.ForceRequeue);
    }

    public SubmitSubtitleJobRequest Resolve(SubmitSubtitleBatchItem item)
    {
        return Resolve(item.ItemId, item.VideoPath, item.SourceLanguage, item.ForceAsr, item.ForceSplit, item.ForceTranslate, item.ForceRequeue);
    }

    public Task OnJobCompletedAsync(SubtitleJob job)
    {
        if (job.Status != SubtitleJobStatus.Succeeded)
        {
            return Task.CompletedTask;
        }

        try
        {
            var item = ResolveCompletedItem(job);
            if (item == null)
            {
                return Task.CompletedTask;
            }

            var options = new MetadataRefreshOptions(_fileSystem)
            {
                MetadataRefreshMode = MetadataRefreshMode.Default,
                RefreshPaths = string.IsNullOrWhiteSpace(job.OutputPath) ? new[] { job.VideoPath } : new[] { job.VideoPath, job.OutputPath! },
                EnableSubtitleDownloading = false,
                ForceSave = true,
                Recursive = false,
            };
            _providerManager.QueueRefresh(item.InternalId, options, RefreshPriority.Normal);
            _logger.Info("Queued Emby metadata refresh for subtitle job {0}, item {1}", job.JobId, item.Id);
        }
        catch (Exception ex)
        {
            _logger.Warn("Failed to queue Emby metadata refresh for subtitle job {0}: {1}", job.JobId, ex.Message);
        }

        return Task.CompletedTask;
    }

    private SubmitSubtitleJobRequest Resolve(string? itemId, string? videoPath, string? sourceLanguage, bool forceAsr, bool forceSplit, bool forceTranslate, bool forceRequeue)
    {
        var resolvedPath = string.IsNullOrWhiteSpace(videoPath) ? null : videoPath!.Trim();
        var resolvedItemId = string.IsNullOrWhiteSpace(itemId) ? null : itemId!.Trim();

        if (!string.IsNullOrWhiteSpace(resolvedItemId) && string.IsNullOrWhiteSpace(resolvedPath))
        {
            var item = GetItemById(resolvedItemId!);
            if (item == null)
            {
                throw new ResourceNotFoundException("Emby item not found: " + resolvedItemId);
            }
            if (!IsVideo(item))
            {
                throw new ArgumentException("Emby item is not a video: " + resolvedItemId);
            }
            if (string.IsNullOrWhiteSpace(item.Path))
            {
                throw new ArgumentException("Emby item does not have a local video path: " + resolvedItemId);
            }

            resolvedPath = item.Path;
            resolvedItemId = item.Id.ToString("N");
        }

        return new SubmitSubtitleJobRequest
        {
            ItemId = resolvedItemId,
            VideoPath = resolvedPath,
            SourceLanguage = sourceLanguage,
            ForceAsr = forceAsr,
            ForceSplit = forceSplit,
            ForceTranslate = forceTranslate,
            ForceRequeue = forceRequeue,
        };
    }

    private BaseItem? ResolveCompletedItem(SubtitleJob job)
    {
        if (!string.IsNullOrWhiteSpace(job.ItemId))
        {
            var item = GetItemById(job.ItemId!);
            if (item != null)
            {
                return item;
            }
        }

        if (!string.IsNullOrWhiteSpace(job.VideoPath))
        {
            return _libraryManager.FindByPath(job.VideoPath, false);
        }

        return null;
    }

    private BaseItem? GetItemById(string itemId)
    {
        if (Guid.TryParse(itemId, out var guid))
        {
            return _libraryManager.GetItemById(guid);
        }

        return null;
    }

    private static bool IsVideo(BaseItem item)
    {
        return string.Equals(item.MediaType, "Video", StringComparison.OrdinalIgnoreCase);
    }
}
