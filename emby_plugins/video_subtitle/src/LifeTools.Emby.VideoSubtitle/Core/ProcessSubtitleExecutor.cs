using System.Diagnostics;
using System.Threading.Tasks;
using System.Text;

namespace LifeTools.Emby.VideoSubtitle;

public sealed class ProcessSubtitleExecutor : ISubtitleExecutor
{
    private readonly int _maxLogTailBytes;

    public ProcessSubtitleExecutor(int maxLogTailBytes = 8192)
    {
        _maxLogTailBytes = Math.Max(1024, maxLogTailBytes);
    }

    public async Task<SubtitleExecutionResult> ExecuteAsync(SubtitleCommand command, CancellationToken cancellationToken)
    {
        if (command == null)
        {
            throw new ArgumentNullException(nameof(command));
        }

        var startInfo = new ProcessStartInfo
        {
            FileName = command.ExecutablePath,
            UseShellExecute = false,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            CreateNoWindow = true,
        };
        startInfo.Arguments = JoinArguments(command.Arguments);

        using var process = new Process { StartInfo = startInfo, EnableRaisingEvents = true };
        if (!process.Start())
        {
            throw new InvalidOperationException("failed to start subtitle process");
        }

        var stdoutTask = process.StandardOutput.ReadToEndAsync();
        var stderrTask = process.StandardError.ReadToEndAsync();

        try
        {
            await WaitForExitAsync(process, cancellationToken);
            var stdout = await stdoutTask;
            var stderr = await stderrTask;
            return new SubtitleExecutionResult(
                process.ExitCode,
                ResolveOutputPath(command, stdout),
                Tail(stdout),
                Tail(stderr));
        }
        catch (OperationCanceledException)
        {
            TryKill(process);
            throw;
        }
    }

    private string? ResolveOutputPath(SubtitleCommand command, string stdout)
    {
        var fromLog = stdout
            .Split(new[] { '\n' }, StringSplitOptions.RemoveEmptyEntries)
            .Select(line => line.Trim()).LastOrDefault(line => line.StartsWith("subtitle written:", StringComparison.OrdinalIgnoreCase));
        if (fromLog != null)
        {
            var value = fromLog.Substring("subtitle written:".Length).Trim();
            if (!string.IsNullOrWhiteSpace(value))
            {
                return value;
            }
        }

        return command.ExpectedOutputPath;
    }

    private string Tail(string value)
    {
        if (string.IsNullOrEmpty(value))
        {
            return string.Empty;
        }

        var bytes = Encoding.UTF8.GetBytes(value);
        if (bytes.Length <= _maxLogTailBytes)
        {
            return value;
        }

        var start = bytes.Length - _maxLogTailBytes;
        while (start < bytes.Length && (bytes[start] & 0b1100_0000) == 0b1000_0000)
        {
            start++;
        }

        return Encoding.UTF8.GetString(bytes, start, bytes.Length - start);
    }

    private static string JoinArguments(IEnumerable<string> arguments)
    {
        return string.Join(" ", arguments.Select(QuoteArgument));
    }

    private static string QuoteArgument(string argument)
    {
        if (argument.Length == 0)
        {
            return "\"\"";
        }

        if (!argument.Any(char.IsWhiteSpace) && argument.IndexOf('"') < 0 && argument.IndexOf('\\') < 0)
        {
            return argument;
        }

        var result = new StringBuilder();
        result.Append('"');
        var backslashes = 0;
        foreach (var ch in argument)
        {
            if (ch == '\\')
            {
                backslashes++;
                continue;
            }

            if (ch == '"')
            {
                result.Append('\\', backslashes * 2 + 1);
                result.Append(ch);
                backslashes = 0;
                continue;
            }

            if (backslashes > 0)
            {
                result.Append('\\', backslashes);
                backslashes = 0;
            }

            result.Append(ch);
        }

        if (backslashes > 0)
        {
            result.Append('\\', backslashes * 2);
        }

        result.Append('"');
        return result.ToString();
    }

    private static void TryKill(Process process)
    {
        try
        {
            if (!process.HasExited)
            {
                process.Kill();
            }
        }
        catch (InvalidOperationException)
        {
        }
    }

    private static Task WaitForExitAsync(Process process, CancellationToken cancellationToken)
    {
        var completion = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        process.Exited += (sender, args) => completion.TrySetResult(true);

        if (process.HasExited)
        {
            completion.TrySetResult(true);
        }

        if (cancellationToken.CanBeCanceled)
        {
            cancellationToken.Register(() => completion.TrySetCanceled());
        }

        return completion.Task;
    }
}
