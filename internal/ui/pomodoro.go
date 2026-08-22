package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type pomodoroPhase int

const (
	phaseWork pomodoroPhase = iota
	phaseShortBreak
	phaseLongBreak
)

const (
	workDuration       = 25 * time.Minute
	shortBreakDuration = 5 * time.Minute
	longBreakDuration  = 15 * time.Minute
	longBreakEvery     = 4
)

type pomodoroTickMsg time.Time

type pomodoro struct {
	phase     pomodoroPhase
	remaining time.Duration
	running   bool
	completed int // completed work sessions, drives the long-break cadence
}

func newPomodoro() pomodoro {
	return pomodoro{
		phase:     phaseWork,
		remaining: workDuration,
	}
}

func pomodoroTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return pomodoroTickMsg(t)
	})
}

func (p pomodoro) phaseDuration() time.Duration {
	switch p.phase {
	case phaseShortBreak:
		return shortBreakDuration
	case phaseLongBreak:
		return longBreakDuration
	default:
		return workDuration
	}
}

func (p *pomodoro) reset() {
	p.remaining = p.phaseDuration()
}

// advance moves to the next phase in the work/break cycle. Every 4th
// completed work session earns a long break instead of a short one.
func (p *pomodoro) advance() {
	if p.phase == phaseWork {
		p.completed++
		if p.completed%longBreakEvery == 0 {
			p.phase = phaseLongBreak
		} else {
			p.phase = phaseShortBreak
		}
	} else {
		p.phase = phaseWork
	}
	p.reset()
}

// tick returns true when it caused the phase to advance, so the caller
// can trigger a notification without duplicating the countdown logic.
func (p *pomodoro) tick() bool {
	if !p.running {
		return false
	}
	p.remaining -= time.Second
	if p.remaining <= 0 {
		p.advance()
		return true
	}
	return false
}

// notifyPhaseChange fires an OS notification announcing the phase that was
// just entered. Shelling out to the native notifier (osascript/notify-send)
// avoids pulling in a third-party dependency for a single-user CLI tool.
func notifyPhaseChange(phase pomodoroPhase) tea.Cmd {
	title := "Pomodoro"
	message := "Break's over — back to work!"
	if phase != phaseWork {
		message = "Focus session complete — time for a break!"
	}

	return func() tea.Msg {
		switch runtime.GOOS {
		case "darwin":
			script := fmt.Sprintf(`display notification %q with title %q`, message, title)
			_ = exec.Command("osascript", "-e", script).Run()
		case "linux":
			_ = exec.Command("notify-send", title, message).Run()
		}
		return nil
	}
}

func (p pomodoro) phaseName() string {
	switch p.phase {
	case phaseShortBreak:
		return "Short Break"
	case phaseLongBreak:
		return "Long Break"
	default:
		return "Focus"
	}
}

func (p pomodoro) phaseColor() lipgloss.Color {
	switch p.phase {
	case phaseShortBreak, phaseLongBreak:
		return colorGreen
	default:
		return colorAccent
	}
}

func formatMMSS(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func renderPomodoro(p pomodoro, width int) string {
	color := p.phaseColor()
	phaseStyle := lipgloss.NewStyle().Bold(true).Foreground(color).Background(colorPaneBg)

	status := "paused"
	if p.running {
		status = "running"
	}

	timerStyle := lipgloss.NewStyle().Bold(true).Foreground(color).Background(colorPaneBg)
	timer := timerStyle.Render(formatMMSS(p.remaining))

	barWidth := width - 4
	if barWidth < 5 {
		barWidth = 5
	}
	pct := 1 - float64(p.remaining)/float64(p.phaseDuration())
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	bar := renderGradientBar(pct*100, barWidth, color)

	sessions := fmt.Sprintf("Sessions completed: %d", p.completed)

	lines := []string{
		titleStyle.Width(width).Render("Pomodoro"),
		"",
		// The separating spaces are rendered through a background-carrying
		// style rather than written raw: a bare "  " between two
		// independently-styled segments sits outside both of their SGR
		// spans and falls back to the terminal's default background.
		phaseStyle.Render(p.phaseName()) + statLabelStyle.Render("  ("+status+")"),
		timer,
		bar,
		"",
		statLabelStyle.Render(sessions),
	}
	return strings.Join(lines, "\n")
}
