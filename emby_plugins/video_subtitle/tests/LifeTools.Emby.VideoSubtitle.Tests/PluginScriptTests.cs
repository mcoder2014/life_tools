using System.Diagnostics;
using Xunit;

namespace LifeTools.Emby.VideoSubtitle.Tests;

public sealed class PluginScriptTests
{
    [Fact]
    public void ScriptsExistAndCanShowHelp()
    {
        var root = FindProjectRoot();

        Assert.True(File.Exists(Path.Combine(root, "build.sh")));
        Assert.True(File.Exists(Path.Combine(root, "install.sh")));
        Assert.Equal(0, RunBash(Path.Combine(root, "build.sh"), "--help").ExitCode);
        Assert.Equal(0, RunBash(Path.Combine(root, "install.sh"), "--help").ExitCode);
    }

    [Fact]
    public void InstallHelpDocumentsSupportedOptions()
    {
        var root = FindProjectRoot();
        var result = RunBash(Path.Combine(root, "install.sh"), "--help");

        Assert.Equal(0, result.ExitCode);
        Assert.Contains("--plugins-dir DIR", result.Output);
        Assert.Contains("--dll PATH", result.Output);
        Assert.Contains("--restart", result.Output);
        Assert.Contains("LifeTools.Emby.VideoSubtitle.Emby.dll", result.Output);
    }

    [Fact]
    public void InstallCopiesExplicitDllToPluginsDirWithoutRestart()
    {
        var root = FindProjectRoot();
        var temp = Path.Combine(Path.GetTempPath(), "life-tools-emby-plugin-install-" + Guid.NewGuid());
        var sourceDir = Path.Combine(temp, "src");
        var pluginsDir = Path.Combine(temp, "plugins");
        var sourceDll = Path.Combine(sourceDir, "LifeTools.Emby.VideoSubtitle.Emby.dll");
        try
        {
            Directory.CreateDirectory(sourceDir);
            File.WriteAllText(sourceDll, "fake plugin dll");

            var result = RunBash(Path.Combine(root, "install.sh"), "--dll", sourceDll, "--plugins-dir", pluginsDir, "--no-restart");

            Assert.Equal(0, result.ExitCode);
            Assert.True(File.Exists(Path.Combine(pluginsDir, "LifeTools.Emby.VideoSubtitle.Emby.dll")));
            Assert.Equal("fake plugin dll", File.ReadAllText(Path.Combine(pluginsDir, "LifeTools.Emby.VideoSubtitle.Emby.dll")));
            Assert.Contains("installed plugin", result.Output);
        }
        finally
        {
            if (Directory.Exists(temp))
            {
                Directory.Delete(temp, recursive: true);
            }
        }
    }

    private static (int ExitCode, string Output) RunBash(string script, params string[] args)
    {
        var psi = new ProcessStartInfo
        {
            FileName = "/bin/bash",
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
        };
        psi.ArgumentList.Add(script);
        foreach (var arg in args)
        {
            psi.ArgumentList.Add(arg);
        }

        using var process = Process.Start(psi) ?? throw new InvalidOperationException("failed to start bash");
        var stdout = process.StandardOutput.ReadToEnd();
        var stderr = process.StandardError.ReadToEnd();
        process.WaitForExit();
        return (process.ExitCode, stdout + stderr);
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
