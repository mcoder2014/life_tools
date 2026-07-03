package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildStatsGroupsByDayWeekMonthAndHeatmap(t *testing.T) {
	sessions := []SessionSummary{
		{ID: "a", UpdatedAt: "2026-07-01T10:00:00Z", EventCount: 3, UserMessages: 1, AssistantMessages: 1, ToolCalls: 1, TokenStats: TokenStats{TokenEvents: 1, Total: TokenUsage{InputTokens: 1000, CachedInputTokens: 300, OutputTokens: 90, ReasoningOutputTokens: 10, TotalTokens: 1100}}},
		{ID: "b", UpdatedAt: "2026-07-01T12:00:00Z", EventCount: 2, UserMessages: 1, AssistantMessages: 1, ToolResults: 1},
		{ID: "c", UpdatedAt: "2026-07-08T10:00:00Z", EventCount: 4, UserMessages: 2, AssistantMessages: 1, TokenStats: TokenStats{TokenEvents: 1, Total: TokenUsage{InputTokens: 2000, CachedInputTokens: 500, OutputTokens: 180, ReasoningOutputTokens: 20, TotalTokens: 2200}}},
	}

	stats := BuildStats(sessions)
	require.Equal(t, 3, stats.TotalSessions)
	require.Equal(t, 9, stats.TotalEvents)
	require.Equal(t, 2, stats.ActiveDays)
	require.Equal(t, 4, stats.UserMessages)
	require.Equal(t, 3, stats.AssistantMessages)
	require.Equal(t, 2, stats.TokenizedSessions)
	require.Equal(t, int64(3000), stats.InputTokens)
	require.Equal(t, int64(800), stats.CachedInputTokens)
	require.Equal(t, int64(270), stats.OutputTokens)
	require.Equal(t, int64(30), stats.ReasoningTokens)
	require.Equal(t, int64(3300), stats.TotalTokens)
	require.Equal(t, []CountPoint{{Key: "2026-07-01", Count: 2}, {Key: "2026-07-08", Count: 1}}, stats.ByDay)
	require.Equal(t, []CountPoint{{Key: "2026-07", Count: 3}}, stats.ByMonth)
	require.NotEmpty(t, stats.Heatmap)
	require.Equal(t, "2026-07-01", stats.Heatmap[0].Date)
	require.Equal(t, "2026-07-08", stats.Heatmap[len(stats.Heatmap)-1].Date)
}

func TestBuildRecentHeatmapUsesSixMonthWindowIndependentOfFilteredStats(t *testing.T) {
	filtered := []SessionSummary{
		{ID: "filtered", UpdatedAt: "2026-07-03T10:00:00Z"},
	}
	all := []SessionSummary{
		{ID: "old", UpdatedAt: "2026-01-02T10:00:00Z"},
		{ID: "current", UpdatedAt: "2026-07-03T10:00:00Z"},
	}

	stats := BuildStats(filtered)
	stats.Heatmap = BuildRecentHeatmap(all, time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC))

	require.Equal(t, 1, stats.TotalSessions)
	require.Len(t, stats.Heatmap, heatmapDays)
	require.Equal(t, "2026-01-02", stats.Heatmap[0].Date)
	require.Equal(t, 1, stats.Heatmap[0].Count)
	require.Equal(t, "2026-07-03", stats.Heatmap[len(stats.Heatmap)-1].Date)
	require.Equal(t, 1, stats.Heatmap[len(stats.Heatmap)-1].Count)
}

func TestParseFilterKeepsInvalidDatesNonFatal(t *testing.T) {
	filter := parseFilter("bad", "2026-07-03", "demo", 10)
	require.False(t, filter.hasFrom)
	require.True(t, filter.hasTo)
	require.Equal(t, "demo", filter.Query)
}
