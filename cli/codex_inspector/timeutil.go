package main

import (
	"fmt"
	"strings"
	"time"
)

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseFilter(from string, to string, query string, limit int) QueryFilter {
	filter := QueryFilter{
		From:  strings.TrimSpace(from),
		To:    strings.TrimSpace(to),
		Query: strings.TrimSpace(query),
		Limit: limit,
	}
	if _, ok := parseTime(filter.From); ok {
		filter.hasFrom = true
	}
	if _, ok := parseTime(filter.To); ok {
		filter.hasTo = true
	}
	return filter
}

func inRange(value string, filter QueryFilter) bool {
	t, ok := parseTime(value)
	if !ok {
		return true
	}
	if filter.hasFrom {
		from, _ := parseTime(filter.From)
		if t.Before(startOfDay(from)) {
			return false
		}
	}
	if filter.hasTo {
		to, _ := parseTime(filter.To)
		if t.After(endOfDay(to)) {
			return false
		}
	}
	return true
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return startOfDay(t).AddDate(0, 0, 1).Add(-time.Nanosecond)
}

func dayKey(value string) string {
	t, ok := parseTime(value)
	if !ok {
		return ""
	}
	return t.Format("2006-01-02")
}

func weekKey(value string) string {
	t, ok := parseTime(value)
	if !ok {
		return ""
	}
	year, week := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}

func monthKey(value string) string {
	t, ok := parseTime(value)
	if !ok {
		return ""
	}
	return t.Format("2006-01")
}
