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

// Built-in defaults, used whenever a setting hasn't been customized. A full
// cycle is work/break/work/break/work/long-break — three focus sessions, the
// last of which earns the long break.
const (
	defaultWorkMinutes       = 25
	defaultShortBreakMinutes = 5
	defaultLongBreakMinutes  = 15
	defaultLongBreakEvery    = 3
)

type pomodoroTickMsg time.Time

type pomodoro struct {
	phase     pomodoroPhase
	remaining time.Duration
	running   bool
	completed int // completed work sessions, drives the long-break cadence

	// Configurable durations, in minutes, plus the long-break cadence and
	// whether the next phase starts automatically. Editable from the
	// Settings modal; zero values are never stored here (newPomodoro and the
	// settings modal always fill in the defaults), so phaseDuration never
	// needs to fall back itself.
	workMinutes       int
	shortBreakMinutes int
	longBreakMinutes  int
	longBreakEvery    int
	autoStartNext     bool
}

func newPomodoro() pomodoro {
	p := pomodoro{
		workMinutes:       defaultWorkMinutes,
		shortBreakMinutes: defaultShortBreakMinutes,
		longBreakMinutes:  defaultLongBreakMinutes,
		longBreakEvery:    defaultLongBreakEvery,
	}
	p.phase = phaseWork
	p.remaining = p.phaseDuration()
	return p
}

func pomodoroTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return pomodoroTickMsg(t)
	})
}

func (p pomodoro) phaseDuration() time.Duration {
	switch p.phase {
	case phaseShortBreak:
		return time.Duration(p.shortBreakMinutes) * time.Minute
	case phaseLongBreak:
		return time.Duration(p.longBreakMinutes) * time.Minute
	default:
		return time.Duration(p.workMinutes) * time.Minute
	}
}

func (p *pomodoro) reset() {
	p.remaining = p.phaseDuration()
}

// advance moves to the next phase in the work/break cycle. Every 4th
// completed work session earns a long break instead of a short one.
func (p *pomodoro) advance() {
	every := p.longBreakEvery
	if every < 1 {
		every = defaultLongBreakEvery
	}
	if p.phase == phaseWork {
		p.completed++
		if p.completed%every == 0 {
			p.phase = phaseLongBreak
		} else {
			p.phase = phaseShortBreak
		}
	} else {
		p.phase = phaseWork
	}
	p.reset()
	p.running = p.autoStartNext
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

// notificationSound is the macOS system sound played with the alert. Any name
// in /System/Library/Sounds works; Glass is the conventional notification
// chime. A phase change is easy to miss in a background terminal, so the alert
// is deliberately audible rather than silent.
const notificationSound = "Glass"

// notifyPhaseChange fires an OS notification announcing the phase that was
// just entered. Shelling out to the native notifier (osascript/notify-send)
// avoids pulling in a third-party dependency for a single-user CLI tool.
//
// Sound is requested explicitly on both platforms: `display notification`
// without `sound name` is SILENT, and notify-send likewise only shows a
// visual bubble. On Linux the sound is a separate command because notify-send
// has no sound flag — the freedesktop sound theme is played via paplay/canberra
// when present, and simply skipped when neither is installed.
func notifyPhaseChange(phase pomodoroPhase) tea.Cmd {
	title := "Pomodoro"
	message := "Break's over — back to work!"
	if phase != phaseWork {
		message = "Focus session complete — time for a break!"
	}

	return func() tea.Msg {
		switch runtime.GOOS {
		case "darwin":
			script := fmt.Sprintf(`display notification %q with title %q sound name %q`,
				message, title, notificationSound)
			_ = exec.Command("osascript", "-e", script).Run()
		case "linux":
			_ = exec.Command("notify-send", title, message).Run()
			playSoundLinux()
		}
		return nil
	}
}

// playSoundLinux makes the alert audible on Linux, trying each known player in
// turn and stopping at the first that succeeds. All of them are optional, so a
// machine with none installed still gets the visual notification — the sound
// is an enhancement, never a hard requirement.
func playSoundLinux() {
	candidates := []struct {
		bin  string
		args []string
	}{
		// canberra-gtk-play speaks the freedesktop sound theme directly and is
		// the closest analogue to what a desktop notification normally plays.
		{"canberra-gtk-play", []string{"-i", "message-new-instant"}},
		{"paplay", []string{"/usr/share/sounds/freedesktop/stereo/complete.oga"}},
		{"aplay", []string{"-q", "/usr/share/sounds/alsa/Front_Center.wav"}},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.bin); err != nil {
			continue
		}
		if err := exec.Command(c.bin, c.args...).Run(); err == nil {
			return
		}
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

// bigDigits renders the timer as 3-row block glyphs, in the same visual
// language as the TASKII banner in the greeting pane. The countdown is the
// one thing you glance at from across the room, so it gets the weight; a
// plain "25:00" read as just another stat line.
var bigDigits = map[rune][3]string{
	'0': {"█▀█", "█ █", "▀▀▀"},
	'1': {" ▄█", "  █", "  ▀"},
	'2': {"▀▀█", "█▀▀", "▀▀▀"},
	'3': {"▀▀█", " ▀█", "▀▀▀"},
	'4': {"█ █", "▀▀█", "  ▀"},
	'5': {"█▀▀", "▀▀█", "▀▀▀"},
	'6': {"█▀▀", "█▀█", "▀▀▀"},
	'7': {"▀▀█", "  █", "  ▀"},
	'8': {"█▀█", "█▀█", "▀▀▀"},
	'9': {"█▀█", "▀▀█", "▀▀▀"},
	// Dots on the outer rows rather than adjacent ones, so the colon reads
	// as vertically balanced against the digits instead of top-heavy.
	':': {" ▄ ", "   ", " ▀ "},
}

// bigTimerRows converts "MM:SS" into its 3 block rows, or returns nil when a
// character has no glyph (so callers can fall back to the plain text).
func bigTimerRows(s string) []string {
	rows := make([]string, 3)
	for _, r := range s {
		g, ok := bigDigits[r]
		if !ok {
			return nil
		}
		for i := 0; i < 3; i++ {
			if rows[i] != "" {
				rows[i] += " "
			}
			rows[i] += g[i]
		}
	}
	return rows
}

// nextBreakIsLong reports whether the break after the current work session
// will be the long one — i.e. this is the last work session of the cycle.
// During a break it describes the break already in progress.
func (p pomodoro) nextBreakIsLong() bool {
	if p.phase != phaseWork {
		return p.phase == phaseLongBreak
	}
	every := p.longBreakEvery
	if every < 1 {
		every = defaultLongBreakEvery
	}
	return (p.completed+1)%every == 0
}

// sessionSummary is the one-line footer: how many focus sessions are done,
// and which kind of break comes next. Replaces the per-phase pip row — the
// running total is the durable number, and the pips restated the phase name
// already shown above them.
func sessionSummary(p pomodoro) string {
	label := "Short break next"
	color := colorMuted
	if p.nextBreakIsLong() {
		label = "Long break next"
		color = colorGreen
	}
	if p.phase != phaseWork {
		// Mid-break the "next" framing would be wrong — name the break that
		// is actually running.
		label = "Short break"
		if p.phase == phaseLongBreak {
			label = "Long break"
		}
	}

	count := fmt.Sprintf("%d", p.completed)
	return statLabelStyle.Render("Sessions ") +
		lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(colorPaneBg).Render(count) +
		statLabelStyle.Render("  ·  ") +
		lipgloss.NewStyle().Foreground(color).Background(colorPaneBg).Render(label)
}

// pomodoroKeys are the controls shown at the bottom of the Pomodoro pane
// rather than in the global help bar, so they sit next to what they act on.
var pomodoroKeys = []helpKey{
	{"p", "pause/resume"}, {"r", "reset"}, {"n", "skip"},
}

// renderPomodoroKeys lays the controls out as "[k] label" entries. Falls back
// to bare "[p] [r] [n]" when the labels won't fit, since a truncated hint is
// worse than a terse complete one.
func renderPomodoroKeys(width int) string {
	build := func(withLabels bool) string {
		var b strings.Builder
		for i, k := range pomodoroKeys {
			if i > 0 {
				b.WriteString(paneKeyLabelStyle.Render("  "))
			}
			b.WriteString(paneKeyStyle.Render("[" + k.key + "]"))
			if withLabels {
				b.WriteString(paneKeyLabelStyle.Render(" " + k.label))
			}
		}
		return b.String()
	}
	full := build(true)
	if width <= 0 || lipgloss.Width(full) <= width {
		return full
	}
	return build(false)
}

// centerLine pads a pre-styled line to width with background-carrying spaces
// on both sides. Written by hand rather than using lipgloss's own alignment
// because that re-renders already-ANSI content, which drops backgrounds past
// embedded resets (documented repeatedly in this codebase).
func centerLine(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	blank := lipgloss.NewStyle().Background(colorPaneBg)
	left := gap / 2
	return blank.Render(strings.Repeat(" ", left)) + s +
		blank.Render(strings.Repeat(" ", gap-left))
}

func renderPomodoro(p pomodoro, width, height int) string {
	color := p.phaseColor()
	phaseStyle := lipgloss.NewStyle().Bold(true).Foreground(color).Background(colorPaneBg)

	timerStyle := lipgloss.NewStyle().Bold(true).Foreground(color).Background(colorPaneBg)
	timerText := formatMMSS(p.remaining)

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

	// The header pip carries the run state on its own: dim when paused, the
	// phase color when running. No spelled-out PAUSED/RUNNING line — the
	// pause key toggles it deliberately, so the pip is enough of a reminder.
	pipColor := colorMuted
	if p.running {
		pipColor = color
	}
	header := lipgloss.NewStyle().Foreground(pipColor).Background(colorPaneBg).Render("● ") +
		phaseStyle.Render(p.phaseName())

	// No title row here — renderPane draws "Pomodoro" on the top border.
	lines := []string{header, ""}

	// Big digits when they fit, plain text when the pane is too narrow.
	if rows := bigTimerRows(timerText); rows != nil && lipgloss.Width(rows[0]) <= width {
		for _, r := range rows {
			lines = append(lines, timerStyle.Render(r))
		}
	} else {
		lines = append(lines, timerStyle.Render(timerText))
	}

	barIdx := len(lines) + 1
	lines = append(lines, "", bar, "", sessionSummary(p))

	// Shrink to fit rather than letting the content overflow its box, since
	// renderPane truncates and would cut the session summary off entirely.
	// Blank spacers go first (from the end, so the timer keeps its breathing
	// room longest); if that isn't enough, the progress bar goes too — the big
	// countdown already conveys progress, whereas the phase, timer and session
	// summary each carry information nothing else shows.
	// Budget excludes the pinned key-hint row appended at the very end.
	if height > 0 {
		for len(lines) > height-1 {
			dropped := -1
			for i := len(lines) - 1; i >= 0; i-- {
				if lines[i] == "" {
					dropped = i
					break
				}
			}
			if dropped < 0 && barIdx >= 0 && barIdx < len(lines) {
				dropped = barIdx
				barIdx = -1
			}
			if dropped < 0 {
				break
			}
			if barIdx > dropped {
				barIdx--
			}
			lines = append(lines[:dropped], lines[dropped+1:]...)
		}
	}

	for i, l := range lines {
		lines[i] = centerLine(l, width)
	}

	// The key hints are pinned to the pane's last row. They're appended AFTER
	// the vertical centering below, and the centering budget excludes their
	// row, so centring the timer block never drags the hints up into the
	// middle of the pane.
	keys := centerLine(renderPomodoroKeys(width), width)

	// Vertical centering: split the leftover rows above and below. renderPane
	// pads only at the bottom, which would leave the block top-aligned in a
	// taller pane. Same approach as renderGreeting.
	if body := height - 1; body > len(lines) {
		blankRow := centerLine("", width)
		gap := body - len(lines)
		top := gap / 2
		padded := make([]string, 0, body)
		for i := 0; i < top; i++ {
			padded = append(padded, blankRow)
		}
		padded = append(padded, lines...)
		for len(padded) < body {
			padded = append(padded, blankRow)
		}
		lines = padded
	}

	return strings.Join(append(lines, keys), "\n")
}
