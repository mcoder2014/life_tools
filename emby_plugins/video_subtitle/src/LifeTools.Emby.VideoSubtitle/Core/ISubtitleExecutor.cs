namespace LifeTools.Emby.VideoSubtitle;

public interface ISubtitleExecutor
{
    Task<SubtitleExecutionResult> ExecuteAsync(SubtitleCommand command, CancellationToken cancellationToken);
}
