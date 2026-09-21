package ui

import (
	"sort"

	"github.com/parsaenami/taskii/internal/model"
)

// upcomingTasks projects future dates without moving or rewriting stored tasks.
// ISO dates sort chronologically; appointments precede untimed tasks per day.
func (a App) upcomingTasks() []model.Task {
	today := a.now().Format(dateFormat)
	var out []model.Task
	for _, task := range a.tasks {
		if task.Date > today {
			out = append(out, task)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := out[i], out[j]
		if left.Date != right.Date {
			return left.Date < right.Date
		}
		if left.Time == right.Time {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		if left.Time == "" {
			return false
		}
		if right.Time == "" {
			return true
		}
		return left.Time < right.Time
	})
	return a.applyFilters(out)
}

func (a App) activeDayTasks() []model.Task {
	if a.upcoming {
		return a.upcomingTasks()
	}
	if a.timeline {
		return a.timelineTasks()
	}
	return a.todayTasks()
}

func (a App) activeDayTitle() string {
	if a.upcoming {
		return "Upcoming"
	}
	if a.timeline {
		return "Timeline"
	}
	return "Today"
}

func (a *App) setUpcoming(upcoming bool) {
	a.upcoming = upcoming
	a.timeline = false
	a.todaySelected, a.todayScroll = 0, 0
	a.simpleSelected, a.simpleScroll = 0, 0
	if a.simple && upcoming {
		// Upcoming contains tasks only. Tab returns to the mixed list before
		// selecting note input, so a newly added note is always visible.
		a.simpleNoteMode = false
	}
	if !a.simple {
		a.focus = focusToday
	}
	a.clampSelections()
}

func (a *App) setTimeline(timeline bool) {
	if a.simple {
		return
	}
	a.timeline = timeline
	a.upcoming = false
	a.todaySelected, a.todayScroll = 0, 0
	a.focus = focusToday
	a.clampSelections()
}

// moveDayView navigates the visible Today-pane tabs.
func (a *App) moveDayView(delta int) {
	view := 0
	if a.timeline {
		view = 1
	} else if a.upcoming {
		view = 2
	}
	view = max(0, min(2, view+delta))
	switch view {
	case 1:
		a.setTimeline(true)
	case 2:
		a.setUpcoming(true)
	default:
		a.setUpcoming(false)
	}
}

// showTaskDate selects the view that contains an added task. The caller then
// selects its ID, because both daily and future lists have their own ordering.
func (a *App) showTaskDate(date string) {
	a.setUpcoming(date > a.now().Format(dateFormat))
}

// showTask keeps a newly added/edited appointment on Timeline when that is
// already the active view and it still belongs there. All other additions use
// the established date-based Today/Upcoming switching behavior.
func (a *App) showTask(task model.Task) {
	if !a.simple && a.timeline && isTimelineTask(task, a.now()) {
		a.setTimeline(true)
		return
	}
	a.showTaskDate(task.Date)
}
