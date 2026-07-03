package main

import (
	"sort"
	"time"
)

const heatmapDays = 183

func BuildOverview(codexHome string, sessions []SessionSummary, heatmapSessions []SessionSummary, sources []SourceStatus, warnings []string) OverviewResponse {
	stats := BuildStats(sessions)
	stats.Heatmap = BuildRecentHeatmap(heatmapSessions, time.Now())
	recent := sessions
	if len(recent) > 8 {
		recent = recent[:8]
	}
	return OverviewResponse{
		CodexHome:      codexHome,
		PrivacyNotice:  "Runs on 127.0.0.1 by default. Codex auth, token, cookie and secret-like fields are excluded or redacted.",
		Sources:        sources,
		Stats:          stats,
		RecentSessions: recent,
		Warnings:       warnings,
	}
}

func BuildStats(sessions []SessionSummary) OverviewStats {
	stats := OverviewStats{TotalSessions: len(sessions)}
	dayCounts := map[string]int{}
	weekCounts := map[string]int{}
	monthCounts := map[string]int{}

	for _, session := range sessions {
		stats.TotalEvents += session.EventCount
		stats.UserMessages += session.UserMessages
		stats.AssistantMessages += session.AssistantMessages
		stats.ToolCalls += session.ToolCalls
		stats.ToolResults += session.ToolResults
		if session.TokenStats.TokenEvents > 0 {
			stats.TokenizedSessions++
			stats.InputTokens += session.TokenStats.Total.InputTokens
			stats.CachedInputTokens += session.TokenStats.Total.CachedInputTokens
			stats.OutputTokens += session.TokenStats.Total.OutputTokens
			stats.ReasoningTokens += session.TokenStats.Total.ReasoningOutputTokens
			stats.TotalTokens += session.TokenStats.Total.TotalTokens
		}

		timeValue := session.UpdatedAt
		if timeValue == "" {
			timeValue = session.StartedAt
		}
		day := dayKey(timeValue)
		if day == "" {
			continue
		}
		dayCounts[day]++
		weekCounts[weekKey(timeValue)]++
		monthCounts[monthKey(timeValue)]++
	}

	stats.ActiveDays = len(dayCounts)
	stats.ByDay = countPoints(dayCounts)
	stats.ByWeek = countPoints(weekCounts)
	stats.ByMonth = countPoints(monthCounts)
	stats.Heatmap = buildHeatmap(dayCounts)
	return stats
}

func BuildRecentHeatmap(sessions []SessionSummary, now time.Time) []HeatmapCell {
	dayCounts := map[string]int{}
	for _, session := range sessions {
		timeValue := session.UpdatedAt
		if timeValue == "" {
			timeValue = session.StartedAt
		}
		day := dayKey(timeValue)
		if day != "" {
			dayCounts[day]++
		}
	}
	end := startOfDay(now)
	start := end.AddDate(0, 0, -heatmapDays+1)
	return buildHeatmapRange(dayCounts, start, end)
}

func countPoints(values map[string]int) []CountPoint {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	points := make([]CountPoint, 0, len(keys))
	for _, key := range keys {
		points = append(points, CountPoint{Key: key, Count: values[key]})
	}
	return points
}

func buildHeatmap(dayCounts map[string]int) []HeatmapCell {
	var minDay time.Time
	var maxDay time.Time
	for key := range dayCounts {
		parsed, ok := parseTime(key)
		if !ok {
			continue
		}
		if minDay.IsZero() || parsed.Before(minDay) {
			minDay = parsed
		}
		if maxDay.IsZero() || parsed.After(maxDay) {
			maxDay = parsed
		}
	}
	if minDay.IsZero() || maxDay.IsZero() || maxDay.Before(minDay) {
		return nil
	}
	if maxDay.Sub(minDay) > 370*24*time.Hour {
		minDay = maxDay.AddDate(0, 0, -370)
	}
	return buildHeatmapRange(dayCounts, minDay, maxDay)
}

func buildHeatmapRange(dayCounts map[string]int, minDay time.Time, maxDay time.Time) []HeatmapCell {
	cells := []HeatmapCell{}
	for day := minDay; !day.After(maxDay); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		cells = append(cells, HeatmapCell{Date: key, Count: dayCounts[key]})
	}
	return cells
}
