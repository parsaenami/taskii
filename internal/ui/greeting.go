package ui

import (
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// taskiiBanner is a hardcoded 3-row block-letter "TASKII" wordmark, each
// glyph hand-drawn on a 3-row grid using a mix of full and half-block
// characters (█▄▀) for thicker, more legible strokes at only 3 rows tall.
// Iterated with the user sample-by-sample until approved.
var taskiiBanner = []string{
	"█▀▀▀█ ▄▀▀▀▄ ▄▀▀▀▀ █ ▄▀ ▀█▀ ▀█▀",
	"  █   █▀▀▀█  ▀▀▀▄ █▀▄   █   █ ",
	"  ▀   ▀   ▀ ▀▀▀▀  ▀  ▀ ▀▀▀ ▀▀▀",
}

// greetingContentLines is renderGreeting's fixed output height (banner rows,
// blank, date line, greeting line), used by View() to carve a fixed budget
// out of the right column the same way pomoContentLines does for Pomodoro.
var greetingContentLines = len(taskiiBanner) + 3

// currentUsername resolves the OS user for the greeting pane. user.Current()
// can fail in some sandboxed/containerized environments, so it falls back to
// the USER/USERNAME env vars, and gives up gracefully (empty string) rather
// than showing an error — a missing username just means a slightly shorter
// greeting, not a broken one.
func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if v := os.Getenv("USER"); v != "" {
		return v
	}
	if v := os.Getenv("USERNAME"); v != "" {
		return v
	}
	return ""
}

func timeGreeting(now time.Time) string {
	switch h := now.Hour(); {
	case h < 12:
		return "Good morning"
	case h < 18:
		return "Good afternoon"
	default:
		return "Good evening"
	}
}

// renderGreeting builds the small logo/date/greeting banner shown above the
// Reports pane. The banner's glyph rows are colored with the accent color
// against the pane's own background (not a solid accent-filled block like
// appTitleStyle) — a filled rectangle behind hand-drawn block letters would
// hide their shape instead of showing it.
//
// height is the pane's content-line budget; View() caps the greeting pane's
// outer height (bodyHeight/3) independently of greetingContentLines, and
// lipgloss's own Height() is a floor not a cap (documented elsewhere in this
// codebase), so unconditionally rendering all banner rows would silently
// overflow a capped pane instead of shrinking to fit it — trim the banner
// from the bottom rows first (keeps the more identifying top of the
// letterforms) before dropping the date/greeting lines. width is likewise
// respected: the banner is wider than the plain "TASKII" text it replaces,
// so on a narrow terminal it falls back to the plain word instead of
// getting clipped mid-letter by the pane's own width wrap.
func renderGreeting(now time.Time, username string, width, height int) string {
	greetLine := timeGreeting(now)
	if username != "" {
		greetLine += ", " + username
	}

	banner := taskiiBanner
	bannerWidth := lipgloss.Width(banner[0])
	logoStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorPaneBg)

	var logoLines []string
	if width > 0 && bannerWidth > width {
		logoLines = []string{logoStyle.Render("TASKII")}
	} else {
		// banner + blank + date + greeting = len(taskiiBanner)+3 lines
		// wanted; shrink the banner first if height is tighter than that.
		if want := len(taskiiBanner) + 3; height > 0 && height < want {
			bannerBudget := height - 3
			if bannerBudget < 0 {
				bannerBudget = 0
			}
			if bannerBudget < len(banner) {
				banner = banner[:bannerBudget]
			}
		}
		for _, row := range banner {
			logoLines = append(logoLines, logoStyle.Render(row))
		}
	}

	blankStyle := lipgloss.NewStyle().Background(colorPaneBg)
	lines := append([]string{}, logoLines...)
	lines = append(lines,
		blankStyle.Render(""),
		statLabelStyle.Render(now.Format("Monday, January 2, 2006")),
		statLabelStyle.Render(greetLine),
	)
	if height > 0 && len(lines) > height {
		lines = lines[len(lines)-height:]
	}

	// Pad each line to width manually rather than relying on the outer
	// Width().Render() below to do it: every line here already carries its
	// own ANSI styling (ending in its own reset), and lipgloss only pads a
	// pre-styled string's start/end — it doesn't background-fill padding
	// added after an embedded reset, leaving unbackgrounded gaps on every
	// line shorter than width (same class of bug fixed in today.go/reports.go).
	if width > 0 {
		for i, l := range lines {
			if pad := width - lipgloss.Width(l); pad > 0 {
				lines[i] = l + blankStyle.Render(strings.Repeat(" ", pad))
			}
		}
	}

	return strings.Join(lines, "\n")
}
