using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class EmbyWebResourceTests
{
    [Fact]
    public void WebResourcesUseCurrentControllerAndStableHistoryUrl()
    {
        var root = FindProjectRoot();
        var html = File.ReadAllText(Path.Combine(root, "src", "LifeTools.Emby.VideoSubtitle.Emby", "Web", "subtitle.html"));
        var js = File.ReadAllText(Path.Combine(root, "src", "LifeTools.Emby.VideoSubtitle.Emby", "Web", "subtitle.js"));
        var plugin = File.ReadAllText(Path.Combine(root, "src", "LifeTools.Emby.VideoSubtitle.Emby", "Plugin.cs"));

        Assert.Contains("LifeToolsVideoSubtitleJsV11", html);
        Assert.Contains("ConfigurationPageName = \"LifeToolsVideoSubtitleV11\"", plugin);
        Assert.Contains("LegacyV10ConfigurationPageName = \"LifeToolsVideoSubtitleV10\"", plugin);
        Assert.Contains("define([], function () {", js);
        Assert.DoesNotContain("baseView", js);
        Assert.DoesNotContain("define(['baseView'", js);
        Assert.DoesNotContain("modules/viewmanager/baseview.js", js);
        Assert.DoesNotContain("BaseView.apply", js);
        Assert.Contains("byId('ltvsRefreshJobs').onclick = function (e) {", js);
        Assert.Contains("View.prototype.onPause = function () {};", js);
        Assert.Contains("data-ltvs-message=\"jobs\"", html);
        Assert.Contains("function setMessage(el, baseClass, text, cls) {", js);
        Assert.Contains("el.className = baseClass + ' ltvs-message ' + (cls || '');", js);
        Assert.Contains("setMessage(page.querySelector('[data-ltvs-message=\"jobs\"]'), 'ltvs-jobs-message', text, cls);", js);
        Assert.DoesNotContain("page.querySelector('.ltvs-jobs-message')", js);
        Assert.Contains("<button type=\"button\" class=\"ltvs-plain-button\" id=\"ltvsRefreshJobs\">刷新</button>", html);
        Assert.DoesNotContain("id=\"ltvsRefreshJobs\"><span>刷新</span></button>", html);
        Assert.Contains("apiJson('LifeTools/VideoSubtitle/Jobs', 'GET', undefined, { Limit: 50 })", js);
        Assert.Contains("var hasBody = data !== undefined && method !== 'GET' && method !== 'HEAD';", js);
        Assert.Contains("if (hasBody) {", js);
        Assert.DoesNotContain("LifeTools/VideoSubtitle/Jobs?Limit=50", js);
        Assert.Contains("var jobs = result && (result.Jobs || result.jobs) || [];", js);
        Assert.Contains("renderJobs(jobs, '已提交任务，等待刷新历史');", js);
        Assert.Contains("renderJobs([], '加载中...');", js);
        Assert.Contains("byId('ltvsSubmit').onclick = function (e) {", js);
        Assert.Contains("byId('ltvsRefreshJobs').onclick = function (e) {", js);
        Assert.Contains("setTimeout(init, 0);", js);
        Assert.Contains("setTimeout(loadJobs, 1200);", js);
        Assert.Contains("options.timeout = 30000;", js);
        Assert.Contains("byId('ltvsJobsBody').innerHTML = '<tr><td colspan=\"5\">加载历史失败</td></tr>';", js);
    }

    private static string FindProjectRoot()
    {
        var current = new DirectoryInfo(AppContext.BaseDirectory);
        while (current != null)
        {
            if (File.Exists(Path.Combine(current.FullName, "LifeTools.Emby.VideoSubtitle.sln")))
            {
                return current.FullName;
            }

            current = current.Parent;
        }

        throw new InvalidOperationException("Could not find LifeTools.Emby.VideoSubtitle.sln");
    }
}
