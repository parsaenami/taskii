package stats

import (
	"sort"
	"time"

	"taskii/internal/model"
)

const dateFormat = "2006-01-02"

type Progress struct {
	Done  int
	Total int
}

func (p Progress) Percent() float64 {
	if p.Total == 0 {
		return 0
	}
	return float64(p.Done) / float64(p.Total) * 100
}

// HeatmapCell is one day in the contribution-graph-style heatmap.
// Level is a pre-bucketed intensity 0-4 (0 = no completions) so the
// rendering layer never needs to know the max/scale, only the color ramp.
type HeatmapCell struct {
	Date  string
	Done  int
	Level int
}

// DayBar is one column of the daily bar chart: the day's full task count and
// how many of them were completed. Done <= Total always, so a renderer can
// draw one bar Total high and fill the bottom Done of it.
type DayBar struct {
	Date  time.Time
	Done  int
	Total int
}

type Report struct {
	Today   Progress
	Week    Progress
	Month   Progress
	Heatmap []HeatmapCell // chronological, oldest first, always a multiple of 7 (weeks x 7 days)
	Streak  int

	// WeekBars is the last 7 days (oldest first) including today; MonthBars
	// is the current calendar month up to today. Both include days with no
	// tasks, so the x-axis has no gaps.
	WeekBars  []DayBar
	MonthBars []DayBar
}

func dayKey(t time.Time) string {
	return t.Format(dateFormat)
}

func progressFor(tasks []model.Task, in func(d string) bool) Progress {
	var p Progress
	for _, t := range tasks {
		if !in(t.Date) {
			continue
		}
		p.Total++
		if t.Done {
			p.Done++
		}
	}
	return p
}

// Week is a rolling 7-day window (today and the 6 preceding days).
func Compute(tasks []model.Task, now time.Time) Report {
	today := dayKey(now)

	todayProg := progressFor(tasks, func(d string) bool { return d == today })

	weekStart := now.AddDate(0, 0, -6)
	weekProg := progressFor(tasks, func(d string) bool {
		return dateInRange(d, dayKey(weekStart), today)
	})

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthProg := progressFor(tasks, func(d string) bool {
		return dateInRange(d, dayKey(monthStart), today)
	})

	return Report{
		Today:     todayProg,
		Week:      weekProg,
		Month:     monthProg,
		Heatmap:   computeHeatmap(tasks, now, 14),
		Streak:    computeStreak(tasks, now),
		WeekBars:  dayBars(tasks, weekStart, now),
		MonthBars: dayBars(tasks, monthStart, now),
	}
}

// dayBars builds one DayBar per calendar day from start to end inclusive,
// including days with no tasks so the chart's x-axis has no gaps.
func dayBars(tasks []model.Task, start, end time.Time) []DayBar {
	type counts struct{ done, total int }
	byDate := map[string]*counts{}
	for _, t := range tasks {
		c := byDate[t.Date]
		if c == nil {
			c = &counts{}
			byDate[t.Date] = c
		}
		c.total++
		if t.Done {
			c.done++
		}
	}

	// Normalise to midnight so the day count isn't skewed by the times of
	// day in start/end.
	d := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	last := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	var out []DayBar
	for !d.After(last) {
		bar := DayBar{Date: d}
		if c := byDate[dayKey(d)]; c != nil {
			bar.Done, bar.Total = c.done, c.total
		}
		out = append(out, bar)
		d = d.AddDate(0, 0, 1)
	}
	return out
}

// computeHeatmap returns weeks*7 days ending today, aligned so each week
// column runs Sun..Sat (matching GitHub's convention). The first column may
// be partially before the actual `weeks*7` days ago if today isn't a
// Saturday, since we pad back to the most recent Sunday-start boundary.
func computeHeatmap(tasks []model.Task, now time.Time, weeks int) []HeatmapCell {
	byDate := map[string]int{}
	for _, t := range tasks {
		if t.Done {
			byDate[t.Date]++
		}
	}

	// weekday: 0=Sun .. 6=Sat. Grid always starts on a Sunday and covers
	// exactly weeks*7 days, so the last row (this week) may include future
	// dates past today (rendered blank) when today isn't a Saturday.
	todayWeekday := int(now.Weekday())
	start := now.AddDate(0, 0, -todayWeekday-(weeks-1)*7)

	total := weeks * 7
	maxDone := 0
	for d := 0; d < total; d++ {
		day := start.AddDate(0, 0, d)
		if day.After(now) {
			break
		}
		if c := byDate[dayKey(day)]; c > maxDone {
			maxDone = c
		}
	}

	cells := make([]HeatmapCell, 0, total)
	for d := 0; d < total; d++ {
		day := start.AddDate(0, 0, d)
		key := dayKey(day)
		count := byDate[key]
		level := 0
		switch {
		case day.After(now):
			level = -1 // future day, not yet reachable, render blank
		case count == 0:
			level = 0
		case maxDone <= 1:
			level = 4
		default:
			level = 1 + int(float64(count-1)/float64(maxDone-1)*3)
			if level > 4 {
				level = 4
			}
		}
		cells = append(cells, HeatmapCell{Date: key, Done: count, Level: level})
	}
	return cells
}

func dateInRange(d, start, end string) bool {
	return d >= start && d <= end
}

// Streak definition: the number of consecutive days, walking backwards from
// today, on which AT LEAST ONE task was completed. A day with no completions
// breaks it. Today is exempt from breaking a streak already earned through
// yesterday — nothing done *yet* today shouldn't retroactively erase it — but
// it only adds to the count once something is actually completed.
func computeStreak(tasks []model.Task, now time.Time) int {
	doneByDate := map[string]int{}
	for _, t := range tasks {
		if t.Done {
			doneByDate[t.Date]++
		}
	}

	streak := 0
	cursor := now

	if doneByDate[dayKey(cursor)] > 0 {
		streak++
	}
	// Either way, start walking from yesterday: if today has a completion
	// it's counted above; if not, today is skipped rather than breaking.
	cursor = cursor.AddDate(0, 0, -1)

	for doneByDate[dayKey(cursor)] > 0 {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}

	return streak
}

func SortByTime(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].Time < tasks[j].Time
	})
}
