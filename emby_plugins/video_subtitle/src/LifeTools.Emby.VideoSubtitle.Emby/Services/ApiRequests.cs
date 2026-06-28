using MediaBrowser.Controller.Net;
using MediaBrowser.Model.Services;

namespace LifeTools.Emby.VideoSubtitle.Emby.Services;

[Route("/LifeTools/VideoSubtitle/Jobs", "POST", Summary = "Queues subtitle generation for one video")]
[Authenticated(Roles = "Admin")]
public sealed class SubmitSubtitleJob : IReturn<SubtitleJobResponse>
{
    [ApiMember(Name = "ItemId", Description = "Emby item id", IsRequired = false, DataType = "string", ParameterType = "query", Verb = "POST")]
    public string? ItemId { get; set; }

    [ApiMember(Name = "VideoPath", Description = "Absolute video file path", IsRequired = true, DataType = "string", ParameterType = "query", Verb = "POST")]
    public string? VideoPath { get; set; }

    [ApiMember(Name = "SourceLanguage", Description = "ASR source language", IsRequired = false, DataType = "string", ParameterType = "query", Verb = "POST")]
    public string? SourceLanguage { get; set; }

    public bool ForceAsr { get; set; }

    public bool ForceSplit { get; set; }

    public bool ForceTranslate { get; set; }

    public bool ForceRequeue { get; set; }
}

[Route("/LifeTools/VideoSubtitle/Jobs/{JobId}", "GET", Summary = "Gets a subtitle job")]
[Authenticated(Roles = "Admin")]
public sealed class GetSubtitleJob : IReturn<SubtitleJobResponse>
{
    [ApiMember(Name = "JobId", Description = "Subtitle job id", IsRequired = true, DataType = "string", ParameterType = "path", Verb = "GET")]
    public string JobId { get; set; } = string.Empty;
}

[Route("/LifeTools/VideoSubtitle/Jobs", "GET", Summary = "Lists subtitle jobs")]
[Authenticated(Roles = "Admin")]
public sealed class ListSubtitleJobs : IReturn<System.Collections.Generic.IReadOnlyList<SubtitleJobResponse>>
{
    public int Limit { get; set; } = 50;
}

[Route("/LifeTools/VideoSubtitle/Batches", "POST", Summary = "Queues subtitle generation for multiple explicit videos")]
[Authenticated(Roles = "Admin")]
public sealed class SubmitSubtitleBatch : IReturn<SubtitleBatchResponse>
{
    public System.Collections.Generic.List<SubmitSubtitleBatchItem> Items { get; set; } = new System.Collections.Generic.List<SubmitSubtitleBatchItem>();

    public bool ForceRequeue { get; set; }
}

public sealed class SubmitSubtitleBatchItem
{
    public string? ItemId { get; set; }

    public string? VideoPath { get; set; }

    public string? SourceLanguage { get; set; }

    public bool ForceAsr { get; set; }

    public bool ForceSplit { get; set; }

    public bool ForceTranslate { get; set; }

    public bool ForceRequeue { get; set; }
}

[Route("/LifeTools/VideoSubtitle/Jobs/{JobId}/Cancel", "POST", Summary = "Cancels a queued or running subtitle job")]
[Authenticated(Roles = "Admin")]
public sealed class CancelSubtitleJob : IReturn<CancelSubtitleJobResponse>
{
    [ApiMember(Name = "JobId", Description = "Subtitle job id", IsRequired = true, DataType = "string", ParameterType = "path", Verb = "POST")]
    public string JobId { get; set; } = string.Empty;
}

public sealed class CancelSubtitleJobResponse
{
    public bool Canceled { get; set; }
}
