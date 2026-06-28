using System.Globalization;
using System.Text;

namespace LifeTools.Emby.VideoSubtitle;

public sealed class FileJobStore : ISubtitleJobStore
{
    private const int FieldCount = 17;

    private readonly string _filePath;
    private readonly SemaphoreSlim _lock = new(1, 1);
    private readonly Dictionary<string, SubtitleJob> _jobs = new Dictionary<string, SubtitleJob>(StringComparer.Ordinal);

    private FileJobStore(string filePath)
    {
        _filePath = filePath;
    }

    public static async Task<FileJobStore> OpenAsync(string filePath)
    {
        if (string.IsNullOrWhiteSpace(filePath))
        {
            throw new ArgumentException("job file path is required", nameof(filePath));
        }

        var directory = Path.GetDirectoryName(Path.GetFullPath(filePath));
        if (!string.IsNullOrEmpty(directory))
        {
            Directory.CreateDirectory(directory);
        }

        var store = new FileJobStore(filePath);
        await store.LoadAsync();
        return store;
    }

    public async Task UpsertJobAsync(SubtitleJob job)
    {
        if (job == null)
        {
            throw new ArgumentNullException(nameof(job));
        }

        await _lock.WaitAsync();
        try
        {
            _jobs[job.JobId] = job;
            SaveLocked();
        }
        finally
        {
            _lock.Release();
        }
    }

    public async Task<SubtitleJob?> GetJobAsync(string jobId)
    {
        if (string.IsNullOrWhiteSpace(jobId))
        {
            return null;
        }

        await _lock.WaitAsync();
        try
        {
            return _jobs.TryGetValue(jobId, out var job) ? job : null;
        }
        finally
        {
            _lock.Release();
        }
    }

    public async Task<IReadOnlyList<SubtitleJob>> ListJobsAsync(int limit)
    {
        var safeLimit = Math.Max(1, Math.Min(limit, 500));

        await _lock.WaitAsync();
        try
        {
            return _jobs.Values
                .OrderByDescending(job => job.CreatedAt)
                .ThenByDescending(job => job.JobId, StringComparer.Ordinal)
                .Take(safeLimit)
                .ToList();
        }
        finally
        {
            _lock.Release();
        }
    }

    public async Task<SubtitleJob?> FindActiveJobByVideoPathAsync(string videoPath)
    {
        if (string.IsNullOrWhiteSpace(videoPath))
        {
            return null;
        }

        await _lock.WaitAsync();
        try
        {
            return _jobs.Values
                .Where(job => string.Equals(job.VideoPath, videoPath, StringComparison.Ordinal))
                .Where(job => job.Status == SubtitleJobStatus.Queued || job.Status == SubtitleJobStatus.Running)
                .OrderBy(job => job.CreatedAt)
                .FirstOrDefault();
        }
        finally
        {
            _lock.Release();
        }
    }

    private async Task LoadAsync()
    {
        if (!File.Exists(_filePath))
        {
            return;
        }

        var lines = File.ReadAllLines(_filePath);
        foreach (var line in lines)
        {
            if (string.IsNullOrWhiteSpace(line))
            {
                continue;
            }

            var job = TryParseJob(line);
            if (job != null)
            {
                _jobs[job.JobId] = job;
            }
        }

        await Task.CompletedTask;
    }

    private void SaveLocked()
    {
        var lines = _jobs.Values
            .OrderBy(job => job.CreatedAt)
            .ThenBy(job => job.JobId, StringComparer.Ordinal)
            .Select(Serialize)
            .ToArray();
        File.WriteAllLines(_filePath, lines);
    }

    private static string Serialize(SubtitleJob job)
    {
        return string.Join("\t", new[]
        {
            Encode(job.JobId),
            Encode(job.BatchId),
            Encode(job.ItemId),
            Encode(job.VideoPath),
            Encode(job.SourceLanguage),
            job.ForceAsr ? "1" : "0",
            job.ForceSplit ? "1" : "0",
            job.ForceTranslate ? "1" : "0",
            job.Status.ToString(),
            Format(job.CreatedAt),
            Format(job.StartedAt) ?? string.Empty,
            Format(job.FinishedAt) ?? string.Empty,
            Encode(job.OutputPath),
            job.ExitCode.HasValue ? job.ExitCode.Value.ToString(CultureInfo.InvariantCulture) : string.Empty,
            Encode(job.StdoutTail),
            Encode(job.StderrTail),
            Encode(job.ErrorMessage),
        });
    }

    private static SubtitleJob? TryParseJob(string line)
    {
        var fields = line.Split('\t');
        if (fields.Length != FieldCount)
        {
            return null;
        }

        if (!Enum.TryParse(fields[8], out SubtitleJobStatus status))
        {
            return null;
        }

        if (!TryParseRequiredDate(fields[9], out var createdAt))
        {
            return null;
        }

        return SubtitleJob.Hydrate(
            DecodeRequired(fields[0]),
            Decode(fields[1]),
            Decode(fields[2]),
            DecodeRequired(fields[3]),
            Decode(fields[4]),
            fields[5] == "1",
            fields[6] == "1",
            fields[7] == "1",
            status,
            createdAt,
            ParseNullableDate(fields[10]),
            ParseNullableDate(fields[11]),
            Decode(fields[12]),
            ParseNullableInt(fields[13]),
            Decode(fields[14]),
            Decode(fields[15]),
            Decode(fields[16]));
    }

    private static string Encode(string? value)
    {
        return string.IsNullOrEmpty(value) ? string.Empty : Convert.ToBase64String(Encoding.UTF8.GetBytes(value));
    }

    private static string? Decode(string value)
    {
        return string.IsNullOrEmpty(value) ? null : Encoding.UTF8.GetString(Convert.FromBase64String(value));
    }

    private static string DecodeRequired(string value)
    {
        return Decode(value) ?? string.Empty;
    }

    private static bool TryParseRequiredDate(string value, out DateTimeOffset result)
    {
        return DateTimeOffset.TryParse(value, CultureInfo.InvariantCulture, DateTimeStyles.RoundtripKind, out result);
    }

    private static DateTimeOffset? ParseNullableDate(string value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return null;
        }

        return DateTimeOffset.Parse(value, CultureInfo.InvariantCulture, DateTimeStyles.RoundtripKind);
    }

    private static int? ParseNullableInt(string value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return null;
        }

        return int.Parse(value, CultureInfo.InvariantCulture);
    }

    private static string Format(DateTimeOffset value)
    {
        return value.ToUniversalTime().ToString("O", CultureInfo.InvariantCulture);
    }

    private static string? Format(DateTimeOffset? value)
    {
        return value.HasValue ? Format(value.Value) : null;
    }
}
