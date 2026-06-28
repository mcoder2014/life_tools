namespace LifeTools.Emby.VideoSubtitle;

public interface ISubtitleJobCompletionSink
{
    Task OnJobCompletedAsync(SubtitleJob job);
}
