package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds the base palette; every derived *Style below is rebuilt from
// one of these when the active theme changes, so a single setTheme call is
// enough to re-skin the whole app without touching any other file.
type Theme struct {
	Name string

	Bg          lipgloss.Color
	Panel       lipgloss.Color
	Border      lipgloss.Color
	BorderFocus lipgloss.Color
	Text        lipgloss.Color
	Muted       lipgloss.Color
	Accent      lipgloss.Color
	Green       lipgloss.Color
	Warning     lipgloss.Color
	Danger      lipgloss.Color
	Purple      lipgloss.Color
	AppTitleFg  lipgloss.Color

	// PaneBg fills pane content areas (the body inside each bordered box).
	// Deliberately a shade between Bg and Panel, not equal to Panel itself —
	// Panel is already used for title bars and the selected-row highlight,
	// so reusing it here would make panes, titles, and selection all blend
	// into one flat surface instead of reading as distinct layers.
	PaneBg lipgloss.Color

	// HeatmapRamp is a 5-step no-activity..most-activity color ramp. Level 0
	// should read as "empty" against Panel, not as a dim-but-visible fill.
	HeatmapRamp [5]lipgloss.Color
}

var themes = []Theme{
	{
		Name: "Tokyo Night", Bg: "#1a1b26", Panel: "#24283b", Border: "#414868",
		BorderFocus: "#7aa2f7", Text: "#c0caf5", Muted: "#565f89", Accent: "#7aa2f7",
		Green: "#9ece6a", Warning: "#e0af68", Danger: "#f7768e", Purple: "#bb9af7",
		AppTitleFg: "#ffffff", PaneBg: "#1e2030",
		HeatmapRamp: [5]lipgloss.Color{"#24283b", "#3d5a4c", "#5a8a6a", "#79c088", "#9ece6a"},
	},
	{
		Name: "Dracula", Bg: "#282a36", Panel: "#343746", Border: "#44475a",
		BorderFocus: "#bd93f9", Text: "#f8f8f2", Muted: "#6272a4", Accent: "#bd93f9",
		Green: "#50fa7b", Warning: "#f1fa8c", Danger: "#ff5555", Purple: "#ff79c6",
		AppTitleFg: "#282a36", PaneBg: "#2d2f3d",
		HeatmapRamp: [5]lipgloss.Color{"#343746", "#2d5a3d", "#3a8a52", "#43b866", "#50fa7b"},
	},
	{
		Name: "Nord", Bg: "#2e3440", Panel: "#3b4252", Border: "#4c566a",
		BorderFocus: "#88c0d0", Text: "#e5e9f0", Muted: "#616e88", Accent: "#88c0d0",
		Green: "#a3be8c", Warning: "#ebcb8b", Danger: "#bf616a", Purple: "#b48ead",
		AppTitleFg: "#2e3440", PaneBg: "#333a48",
		HeatmapRamp: [5]lipgloss.Color{"#3b4252", "#455147", "#586e4d", "#7c9a5c", "#a3be8c"},
	},
	{
		Name: "Light", Bg: "#fafafa", Panel: "#eeeeee", Border: "#c9c9c9",
		BorderFocus: "#3b6ea5", Text: "#2a2a2a", Muted: "#767676", Accent: "#3b6ea5",
		Green: "#2e7d32", Warning: "#b5730a", Danger: "#c62828", Purple: "#7b4fa3",
		AppTitleFg: "#ffffff", PaneBg: "#f2f2f2",
		HeatmapRamp: [5]lipgloss.Color{"#eeeeee", "#cfe3cf", "#a8d0a8", "#7ab97a", "#4ba14b"},
	},
	{
		// Warm dark theme. Accent is a saturated orange (border focus / selection);
		// Warning stays more yellow-amber so the two don't muddy together in the
		// same palette the way two orange-ish tones would.
		Name: "Ember", Bg: "#241c1a", Panel: "#2f2522", Border: "#4a3a34",
		BorderFocus: "#ff8c42", Text: "#f0e4d8", Muted: "#8a7268", Accent: "#e0703a",
		Green: "#8bc34a", Warning: "#f2b544", Danger: "#e5484d", Purple: "#c68bd6",
		AppTitleFg: "#241c1a", PaneBg: "#2a211e",
		HeatmapRamp: [5]lipgloss.Color{"#2f2522", "#3d5a3d", "#4f8a4f", "#69b869", "#8bc34a"},
	},
	{
		// Classic Monokai: warm near-black ground with the signature pink /
		// green / cyan / orange accents. Accent is the cyan rather than the
		// pink, so it stays distinct from Danger (pink-red) the way Ember
		// keeps Accent and Warning apart.
		Name: "Monokai", Bg: "#272822", Panel: "#3e3d32", Border: "#54544a",
		BorderFocus: "#66d9ef", Text: "#f8f8f2", Muted: "#90907e", Accent: "#66d9ef",
		Green: "#a6e22e", Warning: "#fd971f", Danger: "#f92672", Purple: "#ae81ff",
		AppTitleFg: "#272822", PaneBg: "#2d2e27",
		HeatmapRamp: [5]lipgloss.Color{"#3e3d32", "#4a5a2a", "#6b8a2f", "#8bb32e", "#a6e22e"},
	},
}

// defaultThemeName is the theme used on first run, before any settings file
// exists. Named rather than indexed so reordering `themes` can't silently
// change the default.
const defaultThemeName = "Ember"

var themeIndex int

func currentTheme() Theme {
	return themes[themeIndex]
}

// cycleTheme advances to the next built-in theme and rebuilds every derived
// style, returning the new theme's name for a status message.
func cycleTheme() string {
	themeIndex = (themeIndex + 1) % len(themes)
	applyTheme(themes[themeIndex])
	return themes[themeIndex].Name
}

// setThemeByName selects a theme by name (used to restore a persisted
// choice at startup); falls back to index 0 silently if not found, since an
// unrecognized name in an old data file shouldn't block startup.
func setThemeByName(name string) {
	for i, t := range themes {
		if t.Name == name {
			themeIndex = i
			applyTheme(t)
			return
		}
	}
	setDefaultTheme()
}

// setDefaultTheme selects defaultThemeName, falling back to the first theme
// only if that name somehow isn't present.
func setDefaultTheme() {
	for i, t := range themes {
		if t.Name == defaultThemeName {
			themeIndex = i
			applyTheme(t)
			return
		}
	}
	themeIndex = 0
	applyTheme(themes[0])
}

var (
	colorBg          lipgloss.Color
	colorPanel       lipgloss.Color
	colorBorder      lipgloss.Color
	colorBorderFocus lipgloss.Color
	colorText        lipgloss.Color
	colorMuted       lipgloss.Color
	colorAccent      lipgloss.Color
	colorGreen       lipgloss.Color
	colorWarning     lipgloss.Color
	colorDanger      lipgloss.Color
	colorPurple      lipgloss.Color
	colorPaneBg      lipgloss.Color
	heatmapRamp      []lipgloss.Color

	titleStyle       lipgloss.Style
	paneTitleStyle   lipgloss.Style
	appTitleStyle    lipgloss.Style
	paneStyle        lipgloss.Style
	paneFocusStyle   lipgloss.Style
	taskStyle        lipgloss.Style
	doneStyle        lipgloss.Style
	overdueStyle     lipgloss.Style
	overdueDoneStyle lipgloss.Style
	importantStyle   lipgloss.Style
	appointmentStyle lipgloss.Style
	selectedStyle    lipgloss.Style
	timeStyle        lipgloss.Style
	helpStyle        lipgloss.Style
	hintStyle        lipgloss.Style
	statLabelStyle   lipgloss.Style
	statValueStyle   lipgloss.Style
	inputPromptStyle lipgloss.Style

	helpKeyStyle   lipgloss.Style
	helpLabelStyle lipgloss.Style
	helpSepStyle   lipgloss.Style

	paneKeyStyle      lipgloss.Style
	paneKeyLabelStyle lipgloss.Style
)

func init() {
	setDefaultTheme()
}

func applyTheme(t Theme) {
	colorBg = t.Bg
	colorPanel = t.Panel
	colorBorder = t.Border
	colorBorderFocus = t.BorderFocus
	colorText = t.Text
	colorMuted = t.Muted
	colorAccent = t.Accent
	colorGreen = t.Green
	colorWarning = t.Warning
	colorDanger = t.Danger
	colorPurple = t.Purple
	colorPaneBg = t.PaneBg
	heatmapRamp = t.HeatmapRamp[:]

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		Background(colorPanel).
		Padding(0, 1)

	// paneTitleStyle is for titles that sit ON a pane's top border rather
	// than in a row inside it. It carries colorPaneBg (matching the border
	// run it's spliced into, so the line reads continuously) and no padding
	// — renderPane supplies the surrounding "─ " / " ─" itself.
	paneTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		Background(colorPaneBg)

	appTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.AppTitleFg).
		Background(colorAccent).
		Padding(0, 2)

	paneStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Background(colorPaneBg).
		Padding(0, 1)

	paneFocusStyle = paneStyle.
		BorderForeground(colorBorderFocus)

	taskStyle = lipgloss.NewStyle().Foreground(colorText).Background(colorPaneBg)

	doneStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)

	overdueStyle = lipgloss.NewStyle().Foreground(colorDanger).Background(colorPaneBg)

	overdueDoneStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)

	importantStyle = lipgloss.NewStyle().Foreground(colorWarning).Bold(true).Background(colorPaneBg)

	appointmentStyle = lipgloss.NewStyle().Foreground(colorPurple).Background(colorPaneBg)

	selectedStyle = lipgloss.NewStyle().
		Background(colorPanel).
		Bold(true)

	timeStyle = lipgloss.NewStyle().Foreground(colorPurple).Background(colorPaneBg)

	// helpStyle colors the bottom help bar's left margin space, which sits
	// outside any pane, so it gets the page-level background (colorBg)
	// rather than colorPaneBg — unlike hintStyle below, which is the same
	// muted look used INSIDE panes (empty-list text, scroll indicators) and
	// needs the pane background instead. Every visible cell on screen
	// should carry one of these two backgrounds; none should be left at
	// the terminal's default.
	helpStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg)

	hintStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)

	statLabelStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)

	statValueStyle = lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(colorPaneBg)

	inputPromptStyle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Background(colorPaneBg)

	// These three (bottom help bar) previously lived in helpbar.go as their
	// own package-level var block, initialized ONCE at package load using
	// whatever colorAccent/colorMuted/colorBg/colorBorder held at THAT
	// moment — which, per Go's init order (package vars initialize before
	// any init() func runs), was always their zero value, since those
	// colors only get set here inside applyTheme. The bug was invisible in
	// isolated tests that constructed styles fresh, but any style built
	// outside applyTheme's rebuild cycle silently rendered with no color at
	// all. Moved here so cycling themes (or the very first paint) always
	// rebuilds these along with everything else.
	helpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorBg)
	helpLabelStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg)
	helpSepStyle = lipgloss.NewStyle().Foreground(colorBorder).Background(colorBg)

	// Pane-scoped variants of the two help styles, for key hints rendered
	// INSIDE a bordered pane (the Pomodoro footer) rather than in the
	// page-level bar — same look, but carrying colorPaneBg so they sit on
	// the pane's surface instead of punching page background through it.
	paneKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorPaneBg)
	paneKeyLabelStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)
}
