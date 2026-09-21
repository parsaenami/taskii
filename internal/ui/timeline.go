package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/parsaenami/taskii/internal/model"
)

const (
	timelinePointMarker = "●"
	timelineRangeMarker = "┃"
	timelineRailMarker  = "│"
	// Event rows begin with a one-cell selection indicator plus a separating
	// space before their HH:MM label. Rail rows reserve those same two cells so
	// the vertical axis runs through point/range markers instead of sitting two
	// columns to their left.
	timelineAxisPrefix = "  "
)

// isTimelineTask is deliberately stricter than IsAppointment: Timeline is a
// clock view for appointments actually scheduled today, not deadline tasks
// projected into Today or malformed legacy records that happen to carry Time.
func isTimelineTask(task model.Task, now time.Time) bool {
	return task.IsAppointment() && task.Date == now.Format(dateFormat) && isTimeLike(task.Time)
}

func (a App) timelineTasks() []model.Task {
	now := a.now()
	var out []model.Task
	for _, task := range a.tasks {
		if isTimelineTask(task, now) {
			out = append(out, task)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Time == out[j].Time {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].Time < out[j].Time
	})
	return a.applyFilters(out)
}

func timelineMinutes(clock string) int {
	if !isTimeLike(clock) {
		return -1
	}
	return int(clock[0]-'0')*600 + int(clock[1]-'0')*60 + int(clock[3]-'0')*10 + int(clock[4]-'0')
}

func timelineClock(minute int) string {
	minute = max(0, min(1439, minute))
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

// timelineWindow avoids a wasteful midnight-to-midnight rail. It includes now,
// every event start and valid range end, adds an hour of context, and keeps a
// minimum three-hour span so the NOW line can visibly move between ticks.
func timelineWindow(tasks []model.Task, now time.Time) (int, int) {
	nowMinute := now.Hour()*60 + now.Minute()
	lo, hi := nowMinute, nowMinute
	for _, task := range tasks {
		start := timelineMinutes(task.Time)
		if start >= 0 {
			lo, hi = min(lo, start), max(hi, start)
		}
		end := timelineMinutes(task.EndTime)
		if end > start {
			hi = max(hi, end)
		}
	}
	lo = max(0, lo-60)
	hi = min(1439, hi+60)
	if hi-lo < 180 {
		missing := 180 - (hi - lo)
		lo = max(0, lo-missing/2)
		hi = min(1439, hi+(missing-missing/2))
		if hi-lo < 180 {
			if lo == 0 {
				hi = min(1439, 180)
			} else {
				lo = max(0, hi-180)
			}
		}
	}
	return lo, hi
}

func timelineRowFor(minute, start, end, height int) int {
	if height <= 1 || end <= start {
		return 0
	}
	minute = max(start, min(end, minute))
	return (minute - start) * (height - 1) / (end - start)
}

func timelineNowLine(now time.Time, width int, note string) string {
	bg := colorPaneBg
	style := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(bg)
	text := now.Format("15:04") + " ━ NOW"
	if note != "" && width-lipgloss.Width(text)-3 >= lipgloss.Width(note) {
		text += "  " + note
	}
	if width > lipgloss.Width(text) {
		text += " " + strings.Repeat("━", width-lipgloss.Width(text)-1)
	}
	return padPanelLine(style.Render(fitToWidth(text, width)), width, bg)
}

func timelineAxisLine(minute, width int) string {
	bg := colorPaneBg
	axis := lipgloss.NewStyle().Foreground(colorMuted).Background(bg)
	line := axis.Render(timelineAxisPrefix + timelineClock(minute) + " " + timelineRailMarker)
	return padPanelLine(line, width, bg)
}

func timelineRailLine(width int) string {
	style := lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)
	return padPanelLine(style.Render(timelineAxisPrefix+"      "+timelineRailMarker), width, colorPaneBg)
}

// timelineRailRows labels only whole hours. Labelling every interpolated row
// produced clock values such as 08:34 and 09:08 when the available height did
// not divide the time window evenly; those are mathematically accurate but
// visually imply meaningful 34-minute intervals. Unlabelled rail cells keep
// the proportional spacing while exact times remain on events and NOW.
func timelineRailRows(start, end, width, height int) []string {
	rows := make([]string, height)
	for row := range rows {
		rows[row] = timelineRailLine(width)
	}
	if height < 1 || end <= start {
		return rows
	}
	firstHour := ((start + 59) / 60) * 60
	lastRow := -1
	for minute := firstHour; minute <= end; minute += 60 {
		row := timelineRowFor(minute, start, end, height)
		// A compressed rail can map adjacent hour marks onto one terminal row.
		// Keep the earlier label rather than repeatedly replacing the same cell.
		if row == lastRow {
			continue
		}
		rows[row] = timelineAxisLine(minute, width)
		lastRow = row
	}
	return rows
}

func timelineEventLine(task model.Task, selected bool, width int) string {
	bg := colorPaneBg
	if selected {
		bg = colorPanel
	}
	base := taskStyle.Copy().Background(bg)
	timeSty := timeStyle.Copy().Background(bg)
	markerSty := appointmentStyle.Copy().Bold(true).Background(bg)
	starSty := importantStyle.Copy().Background(bg)
	if task.Done {
		base = doneStyle.Copy().Background(bg)
		timeSty = base
		markerSty = base
		starSty = base
	}
	if selected {
		base = base.Bold(true)
		timeSty = timeSty.Bold(true)
		markerSty = markerSty.Bold(true)
	}

	marker := timelinePointMarker
	end := timelineMinutes(task.EndTime)
	if end > timelineMinutes(task.Time) {
		marker = timelineRangeMarker
	}
	prefix := " "
	if selected {
		prefix = ">"
	}
	star := ""
	if task.Important {
		star = "★ "
	}
	suffix := ""
	if marker == timelineRangeMarker {
		suffix = " → " + task.EndTime
	}

	fixed := lipgloss.Width(prefix+" "+task.Time+" "+marker+"  ") + lipgloss.Width(star) + lipgloss.Width(suffix)
	titleWidth := max(0, width-fixed)
	title := fitToWidth(task.Title, titleWidth)

	var line strings.Builder
	line.WriteString(base.Render(prefix + " "))
	line.WriteString(timeSty.Render(task.Time))
	line.WriteString(base.Render(" "))
	line.WriteString(markerSty.Render(marker))
	line.WriteString(base.Render("  "))
	if star != "" {
		line.WriteString(starSty.Render(star))
	}
	line.WriteString(renderTitleWithTags(title, base, bg))
	if suffix != "" {
		line.WriteString(timeSty.Render(suffix))
	}
	return padPanelLine(line.String(), width, bg)
}

func timelineMessageLine(message string, width int) string {
	message = fitToWidth(message, width)
	left := max(0, (width-lipgloss.Width(message))/2)
	style := lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)
	return padPanelLine(style.Render(strings.Repeat(" ", left)+message), width, colorPaneBg)
}

// renderTimeline always returns exactly height rows. With enough room it maps
// events and NOW onto a scaled day rail. If the pane is too short to give every
// event its own rail row, it degrades to a scrollable agenda while pinning the
// current-time line, so overlapping/same-start appointments remain selectable.
func renderTimeline(tasks []model.Task, selected, scroll int, focused bool, width, height int, now time.Time) string {
	width = max(1, width)
	height = max(1, height)
	if len(tasks) == 0 {
		start, end := timelineWindow(nil, now)
		nowRow := timelineRowFor(now.Hour()*60+now.Minute(), start, end, height)
		messageRow := height / 2
		if messageRow == nowRow {
			if messageRow+1 < height {
				messageRow++
			} else if messageRow > 0 {
				messageRow--
			}
		}
		rows := timelineRailRows(start, end, width, height)
		rows[nowRow] = timelineNowLine(now, width, "")
		if height > 1 {
			rows[messageRow] = timelineMessageLine("No appointments today", width)
		}
		return strings.Join(rows, "\n")
	}

	// The scaled rail needs one row for NOW in addition to one independently
	// selectable row per event. Below that threshold use the compact viewport.
	if height < 5 || len(tasks)+1 > height || width < 24 {
		capacity := max(0, height-1)
		scroll = max(0, min(scroll, max(0, len(tasks)-capacity)))
		end := min(len(tasks), scroll+capacity)
		note := ""
		above, below := scroll, len(tasks)-end
		switch {
		case above > 0 && below > 0:
			note = fmt.Sprintf("↑%d ↓%d", above, below)
		case above > 0:
			note = fmt.Sprintf("↑%d", above)
		case below > 0:
			note = fmt.Sprintf("↓%d", below)
		}
		rows := []string{timelineNowLine(now, width, note)}
		for i := scroll; i < end; i++ {
			rows = append(rows, timelineEventLine(tasks[i], focused && i == selected, width))
		}
		blank := lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", width))
		for len(rows) < height {
			rows = append(rows, blank)
		}
		return strings.Join(rows[:height], "\n")
	}

	start, end := timelineWindow(tasks, now)
	nowRow := timelineRowFor(now.Hour()*60+now.Minute(), start, end, height)
	available := make([]int, 0, height-1)
	for row := 0; row < height; row++ {
		if row != nowRow {
			available = append(available, row)
		}
	}

	// Choose distinct, monotonic rows nearest each event's scaled position.
	// Reserving enough available rows for the remaining events ensures every
	// same-start/overlapping appointment gets a visible row of its own.
	eventRows := make([]int, len(tasks))
	previous := -1
	for i, task := range tasks {
		desired := timelineRowFor(timelineMinutes(task.Time), start, end, height)
		idx := sort.SearchInts(available, desired)
		if idx == len(available) || (idx > 0 && desired-available[idx-1] <= available[idx]-desired) {
			idx--
		}
		idx = max(previous+1, idx)
		idx = min(idx, len(available)-(len(tasks)-i))
		eventRows[i] = available[idx]
		previous = idx
	}

	rows := timelineRailRows(start, end, width, height)
	rows[nowRow] = timelineNowLine(now, width, "")
	for i, row := range eventRows {
		rows[row] = timelineEventLine(tasks[i], focused && i == selected, width)
	}
	return strings.Join(rows, "\n")
}
