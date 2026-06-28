using System.Globalization;
using Microsoft.Data.Sqlite;

namespace LifeTools.Emby.VideoSubtitle;

public sealed class SqliteJobStore : ISubtitleJobStore, IAsyncDisposable
{
    private readonly SqliteConnection _connection;
    private readonly SemaphoreSlim _lock = new(1, 1);

    private SqliteJobStore(SqliteConnection connection)
    {
        _connection = connection;
    }

    public static async Task<SqliteJobStore> OpenAsync(string dbPath)
    {
        if (string.IsNullOrWhiteSpace(dbPath))
        {
            throw new ArgumentException("database path is required", nameof(dbPath));
        }

        if (dbPath != ":memory:")
        {
            var directory = Path.GetDirectoryName(Path.GetFullPath(dbPath));
            if (!string.IsNullOrEmpty(directory))
            {
                Directory.CreateDirectory(directory);
            }
        }

        var connection = new SqliteConnection($"Data Source={dbPath}");
        await connection.OpenAsync();

        var store = new SqliteJobStore(connection);
        await store.CreateSchemaAsync();
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
            using var command = _connection.CreateCommand();
            command.CommandText = """
                INSERT INTO subtitle_jobs (
                    job_id,
                    batch_id,
                    item_id,
                    video_path,
                    source_language,
                    force_asr,
                    force_split,
                    force_translate,
                    status,
                    created_at,
                    started_at,
                    finished_at,
                    output_path,
                    exit_code,
                    stdout_tail,
                    stderr_tail,
                    error_message
                ) VALUES (
                    $job_id,
                    $batch_id,
                    $item_id,
                    $video_path,
                    $source_language,
                    $force_asr,
                    $force_split,
                    $force_translate,
                    $status,
                    $created_at,
                    $started_at,
                    $finished_at,
                    $output_path,
                    $exit_code,
                    $stdout_tail,
                    $stderr_tail,
                    $error_message
                )
                ON CONFLICT(job_id) DO UPDATE SET
                    batch_id = excluded.batch_id,
                    item_id = excluded.item_id,
                    video_path = excluded.video_path,
                    source_language = excluded.source_language,
                    force_asr = excluded.force_asr,
                    force_split = excluded.force_split,
                    force_translate = excluded.force_translate,
                    status = excluded.status,
                    created_at = excluded.created_at,
                    started_at = excluded.started_at,
                    finished_at = excluded.finished_at,
                    output_path = excluded.output_path,
                    exit_code = excluded.exit_code,
                    stdout_tail = excluded.stdout_tail,
                    stderr_tail = excluded.stderr_tail,
                    error_message = excluded.error_message;
                """;
            AddJobParameters(command, job);
            await command.ExecuteNonQueryAsync();
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
            using var command = _connection.CreateCommand();
            command.CommandText = SelectColumns + " WHERE job_id = $job_id LIMIT 1";
            command.Parameters.AddWithValue("$job_id", jobId);

            using var reader = await command.ExecuteReaderAsync();
            if (!await reader.ReadAsync())
            {
                return null;
            }

            return ReadJob(reader);
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
            using var command = _connection.CreateCommand();
            command.CommandText = SelectColumns + " ORDER BY datetime(created_at) DESC, job_id DESC LIMIT $limit";
            command.Parameters.AddWithValue("$limit", safeLimit);

            using var reader = await command.ExecuteReaderAsync();
            var jobs = new List<SubtitleJob>();
            while (await reader.ReadAsync())
            {
                jobs.Add(ReadJob(reader));
            }

            return jobs;
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
            using var command = _connection.CreateCommand();
            command.CommandText = SelectColumns + """
                 WHERE video_path = $video_path
                   AND status IN ($queued, $running)
                 ORDER BY datetime(created_at) ASC
                 LIMIT 1
                """;
            command.Parameters.AddWithValue("$video_path", videoPath);
            command.Parameters.AddWithValue("$queued", SubtitleJobStatus.Queued.ToString());
            command.Parameters.AddWithValue("$running", SubtitleJobStatus.Running.ToString());

            using var reader = await command.ExecuteReaderAsync();
            if (!await reader.ReadAsync())
            {
                return null;
            }

            return ReadJob(reader);
        }
        finally
        {
            _lock.Release();
        }
    }

    public ValueTask DisposeAsync()
    {
        _lock.Dispose();
        _connection.Dispose();
        return default;
    }

    private async Task CreateSchemaAsync()
    {
        await _lock.WaitAsync();
        try
        {
            using var command = _connection.CreateCommand();
            command.CommandText = """
                CREATE TABLE IF NOT EXISTS subtitle_jobs (
                    job_id TEXT PRIMARY KEY,
                    batch_id TEXT NULL,
                    item_id TEXT NULL,
                    video_path TEXT NOT NULL,
                    source_language TEXT NULL,
                    force_asr INTEGER NOT NULL,
                    force_split INTEGER NOT NULL,
                    force_translate INTEGER NOT NULL,
                    status TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    started_at TEXT NULL,
                    finished_at TEXT NULL,
                    output_path TEXT NULL,
                    exit_code INTEGER NULL,
                    stdout_tail TEXT NULL,
                    stderr_tail TEXT NULL,
                    error_message TEXT NULL
                );

                CREATE INDEX IF NOT EXISTS idx_subtitle_jobs_video_status
                    ON subtitle_jobs(video_path, status);

                CREATE INDEX IF NOT EXISTS idx_subtitle_jobs_created_at
                    ON subtitle_jobs(created_at);
                """;
            await command.ExecuteNonQueryAsync();
        }
        finally
        {
            _lock.Release();
        }
    }

    private static void AddJobParameters(SqliteCommand command, SubtitleJob job)
    {
        command.Parameters.AddWithValue("$job_id", job.JobId);
        AddNullable(command, "$batch_id", job.BatchId);
        AddNullable(command, "$item_id", job.ItemId);
        command.Parameters.AddWithValue("$video_path", job.VideoPath);
        AddNullable(command, "$source_language", job.SourceLanguage);
        command.Parameters.AddWithValue("$force_asr", job.ForceAsr ? 1 : 0);
        command.Parameters.AddWithValue("$force_split", job.ForceSplit ? 1 : 0);
        command.Parameters.AddWithValue("$force_translate", job.ForceTranslate ? 1 : 0);
        command.Parameters.AddWithValue("$status", job.Status.ToString());
        command.Parameters.AddWithValue("$created_at", Format(job.CreatedAt));
        AddNullable(command, "$started_at", Format(job.StartedAt));
        AddNullable(command, "$finished_at", Format(job.FinishedAt));
        AddNullable(command, "$output_path", job.OutputPath);
        AddNullable(command, "$exit_code", job.ExitCode);
        AddNullable(command, "$stdout_tail", job.StdoutTail);
        AddNullable(command, "$stderr_tail", job.StderrTail);
        AddNullable(command, "$error_message", job.ErrorMessage);
    }

    private static SubtitleJob ReadJob(SqliteDataReader reader)
    {
        return SubtitleJob.Hydrate(
            reader.GetString(0),
            ReadNullableString(reader, 1),
            ReadNullableString(reader, 2),
            reader.GetString(3),
            ReadNullableString(reader, 4),
            reader.GetInt32(5) != 0,
            reader.GetInt32(6) != 0,
            reader.GetInt32(7) != 0,
            (SubtitleJobStatus)Enum.Parse(typeof(SubtitleJobStatus), reader.GetString(8)),
            ParseRequiredDate(reader.GetString(9)),
            ParseNullableDate(ReadNullableString(reader, 10)),
            ParseNullableDate(ReadNullableString(reader, 11)),
            ReadNullableString(reader, 12),
            reader.IsDBNull(13) ? null : reader.GetInt32(13),
            ReadNullableString(reader, 14),
            ReadNullableString(reader, 15),
            ReadNullableString(reader, 16));
    }

    private static string? ReadNullableString(SqliteDataReader reader, int ordinal)
    {
        return reader.IsDBNull(ordinal) ? null : reader.GetString(ordinal);
    }

    private static DateTimeOffset ParseRequiredDate(string value)
    {
        return DateTimeOffset.Parse(value, CultureInfo.InvariantCulture, DateTimeStyles.RoundtripKind);
    }

    private static DateTimeOffset? ParseNullableDate(string? value)
    {
        return string.IsNullOrWhiteSpace(value) ? null : ParseRequiredDate(value!);
    }

    private static string Format(DateTimeOffset value)
    {
        return value.ToUniversalTime().ToString("O", CultureInfo.InvariantCulture);
    }

    private static string? Format(DateTimeOffset? value)
    {
        return value.HasValue ? Format(value.Value) : null;
    }

    private static void AddNullable(SqliteCommand command, string name, object? value)
    {
        command.Parameters.AddWithValue(name, value ?? DBNull.Value);
    }

    private const string SelectColumns = """
        SELECT
            job_id,
            batch_id,
            item_id,
            video_path,
            source_language,
            force_asr,
            force_split,
            force_translate,
            status,
            created_at,
            started_at,
            finished_at,
            output_path,
            exit_code,
            stdout_tail,
            stderr_tail,
            error_message
        FROM subtitle_jobs
        """;
}
