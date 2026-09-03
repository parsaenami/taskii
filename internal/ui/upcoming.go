package ui

import (
	"sort"

	"taskii/internal/model"
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
	return a.todayTasks()
}

func (a App) activeDayTitle() string {
	if a.upcoming {
		return "Upcoming"
	}
	return "Today"
}

func (a App) upcomingSwitchLabel() string {
	if a.upcoming {
		if a.simple {
			return "current"
		}
		return "today"
	}
	return "upcoming"
}

func (a *App) toggleUpcoming() {
	a.setUpcoming(!a.upcoming)
}

func (a *App) setUpcoming(upcoming bool) {
	a.upcoming = upcoming
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

// showTaskDate selects the view that contains an added task. The caller then
// selects its ID, because both daily and future lists have their own ordering.
func (a *App) showTaskDate(date string) {
	a.setUpcoming(date > a.now().Format(dateFormat))
}
