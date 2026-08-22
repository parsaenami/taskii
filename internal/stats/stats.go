package stats

import (
	"sort"
	"time"

	"terminal-dashboard/internal/model"
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

type Report struct {
	Today   Progress
	Week    Progress
	Month   Progress
	Heatmap []HeatmapCell // chronological, oldest first, always a multiple of 7 (weeks x 7 days)
	Streak  int
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
		Today:   todayProg,
		Week:    weekProg,
		Month:   monthProg,
		Heatmap: computeHeatmap(tasks, now, 14),
		Streak:  computeStreak(tasks, now),
	}
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
// today, where every task scheduled for that day is Done and there is at
// least one task that day. A day with zero tasks breaks the streak (it's
// not a "free pass") except we allow today itself to be empty/incomplete
// in progress without breaking a streak already earned through yesterday,
// so the user isn't punished mid-day for not having finished yet.
func computeStreak(tasks []model.Task, now time.Time) int {
	byDate := map[string][]model.Task{}
	for _, t := range tasks {
		byDate[t.Date] = append(byDate[t.Date], t)
	}

	dayComplete := func(date string) (complete bool, hasTasks bool) {
		ts, ok := byDate[date]
		if !ok || len(ts) == 0 {
			return false, false
		}
		for _, t := range ts {
			if !t.Done {
				return false, true
			}
		}
		return true, true
	}

	streak := 0
	cursor := now

	todayKey := dayKey(cursor)
	if complete, hasTasks := dayComplete(todayKey); hasTasks && !complete {
		// today started but not finished yet: don't break the streak,
		// just don't count today; start counting from yesterday.
	} else if hasTasks && complete {
		streak++
	}
	cursor = cursor.AddDate(0, 0, -1)

	for {
		key := dayKey(cursor)
		complete, hasTasks := dayComplete(key)
		if !hasTasks || !complete {
			break
		}
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
