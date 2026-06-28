namespace LifeTools.Emby.VideoSubtitle;

public interface ISubtitleJobStore
{
    Task UpsertJobAsync(SubtitleJob job);

    Task<SubtitleJob?> GetJobAsync(string jobId);

    Task<IReadOnlyList<SubtitleJob>> ListJobsAsync(int limit);

    Task<SubtitleJob?> FindActiveJobByVideoPathAsync(string videoPath);
}
