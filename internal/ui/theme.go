package ui

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/lipgloss"
	tint "github.com/lrstanley/bubbletint/v2"
)

// ThemeSource identifies where a theme originated.
type ThemeSource string

const (
	SourceCurated    ThemeSource = "Curated"
	SourceBubbletint ThemeSource = "Bubbletint"
	SourceCustom     ThemeSource = "Custom"
)

// Theme holds the base palette; every derived *Style below is rebuilt from
// one of these when the active theme changes, so a single setTheme call is
// enough to re-skin the whole app without touching any other file.
type Theme struct {
	Name   string      `json:"name"`
	Source ThemeSource `json:"source,omitempty"`

	Bg          lipgloss.Color `json:"bg"`
	Panel       lipgloss.Color `json:"panel"`
	Border      lipgloss.Color `json:"border"`
	BorderFocus lipgloss.Color `json:"border_focus"`
	Text        lipgloss.Color `json:"text"`
	Muted       lipgloss.Color `json:"muted"`
	Accent      lipgloss.Color `json:"accent"`
	Green       lipgloss.Color `json:"green"`
	Warning     lipgloss.Color `json:"warning"`
	Danger      lipgloss.Color `json:"danger"`
	Purple      lipgloss.Color `json:"purple"`
	AppTitleFg  lipgloss.Color `json:"app_title_fg"`

	// PaneBg fills pane content areas (the body inside each bordered box).
	// Deliberately a shade between Bg and Panel, not equal to Panel itself —
	// Panel is already used for title bars and the selected-row highlight,
	// so reusing it here would make panes, titles, and selection all blend
	// into one flat surface instead of reading as distinct layers.
	PaneBg lipgloss.Color `json:"pane_bg"`

	// HeatmapRamp is a 5-step no-activity..most-activity color ramp. Level 0
	// should read as "empty" against Panel, not as a dim-but-visible fill.
	HeatmapRamp [5]lipgloss.Color `json:"heatmap_ramp"`
}

var curatedThemes = []Theme{
	{
		Name: "Tokyo Night", Source: SourceCurated, Bg: "#1a1b26", Panel: "#24283b", Border: "#414868",
		BorderFocus: "#7aa2f7", Text: "#c0caf5", Muted: "#565f89", Accent: "#7aa2f7",
		Green: "#9ece6a", Warning: "#e0af68", Danger: "#f7768e", Purple: "#bb9af7",
		AppTitleFg: "#ffffff", PaneBg: "#1e2030",
		HeatmapRamp: [5]lipgloss.Color{"#24283b", "#3d5a4c", "#5a8a6a", "#79c088", "#9ece6a"},
	},
	{
		Name: "Dracula", Source: SourceCurated, Bg: "#282a36", Panel: "#343746", Border: "#44475a",
		BorderFocus: "#bd93f9", Text: "#f8f8f2", Muted: "#6272a4", Accent: "#bd93f9",
		Green: "#50fa7b", Warning: "#f1fa8c", Danger: "#ff5555", Purple: "#ff79c6",
		AppTitleFg: "#282a36", PaneBg: "#2d2f3d",
		HeatmapRamp: [5]lipgloss.Color{"#343746", "#2d5a3d", "#3a8a52", "#43b866", "#50fa7b"},
	},
	{
		Name: "Nord", Source: SourceCurated, Bg: "#2e3440", Panel: "#3b4252", Border: "#4c566a",
		BorderFocus: "#88c0d0", Text: "#e5e9f0", Muted: "#616e88", Accent: "#88c0d0",
		Green: "#a3be8c", Warning: "#ebcb8b", Danger: "#bf616a", Purple: "#b48ead",
		AppTitleFg: "#2e3440", PaneBg: "#333a48",
		HeatmapRamp: [5]lipgloss.Color{"#3b4252", "#455147", "#586e4d", "#7c9a5c", "#a3be8c"},
	},
	{
		Name: "Light", Source: SourceCurated, Bg: "#fafafa", Panel: "#eeeeee", Border: "#c9c9c9",
		BorderFocus: "#3b6ea5", Text: "#2a2a2a", Muted: "#767676", Accent: "#3b6ea5",
		Green: "#2e7d32", Warning: "#b5730a", Danger: "#c62828", Purple: "#7b4fa3",
		AppTitleFg: "#ffffff", PaneBg: "#f2f2f2",
		HeatmapRamp: [5]lipgloss.Color{"#eeeeee", "#cfe3cf", "#a8d0a8", "#7ab97a", "#4ba14b"},
	},
	{
		// Warm dark theme. Accent is a saturated orange (border focus / selection);
		// Warning stays more yellow-amber so the two don't muddy together in the
		// same palette the way two orange-ish tones would.
		Name: "Ember", Source: SourceCurated, Bg: "#241c1a", Panel: "#2f2522", Border: "#4a3a34",
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
		Name: "Monokai", Source: SourceCurated, Bg: "#272822", Panel: "#3e3d32", Border: "#54544a",
		BorderFocus: "#66d9ef", Text: "#f8f8f2", Muted: "#90907e", Accent: "#66d9ef",
		Green: "#a6e22e", Warning: "#fd971f", Danger: "#f92672", Purple: "#ae81ff",
		AppTitleFg: "#272822", PaneBg: "#2d2e27",
		HeatmapRamp: [5]lipgloss.Color{"#3e3d32", "#4a5a2a", "#6b8a2f", "#8bb32e", "#a6e22e"},
	},
	{
		// Everforest: warm, natural green-based theme with soft contrast.
		// Accent is the signature sage green (#a7c080), with warm yellow warning
		// and soft red danger accents on a dark earthy forest ground.
		Name: "Everforest", Source: SourceCurated, Bg: "#272e33", Panel: "#343f44", Border: "#475258",
		BorderFocus: "#a7c080", Text: "#d3c6aa", Muted: "#7a8478", Accent: "#a7c080",
		Green: "#a7c080", Warning: "#dbbc7f", Danger: "#e67e80", Purple: "#d699b6",
		AppTitleFg: "#272e33", PaneBg: "#2d353b",
		HeatmapRamp: [5]lipgloss.Color{"#343f44", "#3d5248", "#516f58", "#78a06f", "#a7c080"},
	},
}

// themes maintains the curated list for fast cycling
var themes = curatedThemes

var (
	themeRegistry map[string]Theme
	allThemes     []Theme
	activeTheme   Theme
	themeIndex    int
)

// defaultThemeName is the theme used on first run, before any settings file
// exists. Named rather than indexed so reordering `themes` can't silently
// change the default.
const defaultThemeName = "Ember"

func currentTheme() Theme {
	if activeTheme.Name != "" {
		return activeTheme
	}
	return curatedThemes[themeIndex]
}

// allAvailableThemes returns the complete catalog of all curated, custom, and bubbletint themes.
func allAvailableThemes() []Theme {
	return allThemes
}

// cycleTheme advances to the next curated theme and rebuilds every derived
// style, returning the new theme's name for a status message.
func cycleTheme() string {
	themeIndex = (themeIndex + 1) % len(curatedThemes)
	activeTheme = curatedThemes[themeIndex]
	applyTheme(activeTheme)
	return activeTheme.Name
}

// cycleThemePrev moves to the previous curated theme.
func cycleThemePrev() string {
	themeIndex = (themeIndex - 1 + len(curatedThemes)) % len(curatedThemes)
	activeTheme = curatedThemes[themeIndex]
	applyTheme(activeTheme)
	return activeTheme.Name
}

// setThemeByName selects a theme by name (used to restore a persisted
// choice at startup); checks curated themes first, then the unified registry.
// Falls back to default if not found.
func setThemeByName(name string) {
	norm := strings.ToLower(strings.TrimSpace(name))
	for i, t := range curatedThemes {
		if strings.ToLower(t.Name) == norm {
			themeIndex = i
			activeTheme = t
			applyTheme(t)
			return
		}
	}
	if t, ok := themeRegistry[norm]; ok {
		activeTheme = t
		applyTheme(t)
		return
	}
	setDefaultTheme()
}

// setDefaultTheme selects defaultThemeName, falling back to the first theme
// only if that name somehow isn't present.
func setDefaultTheme() {
	for i, t := range curatedThemes {
		if t.Name == defaultThemeName {
			themeIndex = i
			activeTheme = t
			applyTheme(t)
			return
		}
	}
	themeIndex = 0
	activeTheme = curatedThemes[0]
	applyTheme(curatedThemes[0])
}

// colorLuminance calculates the relative luminance of a hex color.
func colorLuminance(hex string) float64 {
	r, g, b, ok := parseHexColor(hex)
	if !ok {
		return 0.5
	}
	channel := func(c int) float64 {
		v := float64(c) / 255.0
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

// tintToTheme adapts any standard bubbletint.Tint into taskii's Theme architecture.
func tintToTheme(t *tint.Tint) Theme {
	if t == nil {
		return curatedThemes[0]
	}
	name := t.DisplayName
	if name == "" {
		name = t.ID
	}

	colorOr := func(c *tint.Color, fallback string) string {
		if c != nil {
			return c.Hex()
		}
		return fallback
	}

	bgHex := colorOr(t.Bg, "#1a1b26")
	fgHex := colorOr(t.Fg, "#ffffff")

	bgLum := colorLuminance(bgHex)
	isDark := t.Dark || bgLum < 0.5

	var paneBgHex, panelHex string
	if isDark {
		paneBgHex = string(blendColor(lipgloss.Color(bgHex), "#ffffff", 0.04))
		panelHex = string(blendColor(lipgloss.Color(bgHex), "#ffffff", 0.10))
	} else {
		paneBgHex = string(blendColor(lipgloss.Color(bgHex), "#000000", 0.03))
		panelHex = string(blendColor(lipgloss.Color(bgHex), "#000000", 0.07))
	}

	borderHex := colorOr(t.BrightBlack, string(blendColor(lipgloss.Color(bgHex), lipgloss.Color(fgHex), 0.20)))

	var accentHex string
	if t.BrightBlue != nil {
		accentHex = t.BrightBlue.Hex()
	} else if t.Blue != nil {
		accentHex = t.Blue.Hex()
	} else if t.BrightCyan != nil {
		accentHex = t.BrightCyan.Hex()
	} else if t.Cyan != nil {
		accentHex = t.Cyan.Hex()
	} else if t.Purple != nil {
		accentHex = t.Purple.Hex()
	} else if t.BrightGreen != nil {
		accentHex = t.BrightGreen.Hex()
	} else if t.Green != nil {
		accentHex = t.Green.Hex()
	} else {
		accentHex = fgHex
	}

	borderFocusHex := accentHex
	greenHex := colorOr(t.Green, colorOr(t.BrightGreen, "#9ece6a"))
	warnHex := colorOr(t.Yellow, colorOr(t.BrightYellow, "#e0af68"))
	dangerHex := colorOr(t.Red, colorOr(t.BrightRed, "#f7768e"))
	purpleHex := colorOr(t.Purple, colorOr(t.BrightPurple, "#bb9af7"))
	mutedHex := colorOr(t.BrightBlack, string(blendColor(lipgloss.Color(fgHex), lipgloss.Color(bgHex), 0.45)))

	accentLum := colorLuminance(accentHex)
	var appTitleFgHex string
	if accentLum > 0.35 {
		appTitleFgHex = bgHex
	} else {
		appTitleFgHex = "#ffffff"
	}

	cPanel := lipgloss.Color(panelHex)
	cGreen := lipgloss.Color(greenHex)
	heatmap := [5]lipgloss.Color{
		cPanel,
		blendColor(cPanel, cGreen, 0.25),
		blendColor(cPanel, cGreen, 0.50),
		blendColor(cPanel, cGreen, 0.75),
		cGreen,
	}

	return Theme{
		Name:        name,
		Source:      SourceBubbletint,
		Bg:          lipgloss.Color(bgHex),
		Panel:       lipgloss.Color(panelHex),
		Border:      lipgloss.Color(borderHex),
		BorderFocus: lipgloss.Color(borderFocusHex),
		Text:        lipgloss.Color(fgHex),
		Muted:       lipgloss.Color(mutedHex),
		Accent:      lipgloss.Color(accentHex),
		Green:       lipgloss.Color(greenHex),
		Warning:     lipgloss.Color(warnHex),
		Danger:      lipgloss.Color(dangerHex),
		Purple:      lipgloss.Color(purpleHex),
		AppTitleFg:  lipgloss.Color(appTitleFgHex),
		PaneBg:      lipgloss.Color(paneBgHex),
		HeatmapRamp: heatmap,
	}
}

// loadCustomThemes searches data/themes and cross-platform XDG config/data directories for JSON/JSONL theme files.
func loadCustomThemes() []Theme {
	var loaded []Theme
	dirs := []string{
		"data/themes",
		filepath.Join(xdg.ConfigHome, "taskii", "themes"),
		filepath.Join(xdg.DataHome, "taskii", "themes"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "taskii", "themes"))
	}

	seenDirs := make(map[string]bool)
	for _, dir := range dirs {
		if seenDirs[dir] {
			continue
		}
		seenDirs[dir] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".json" && ext != ".jsonl" {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			if ext == ".jsonl" {
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					if th, ok := parseSingleCustomTheme([]byte(line)); ok {
						loaded = append(loaded, th)
					}
				}
			} else {
				var themeList []Theme
				if err := json.Unmarshal(data, &themeList); err == nil && len(themeList) > 0 && themeList[0].Name != "" {
					for _, th := range themeList {
						th.Source = SourceCustom
						loaded = append(loaded, th)
					}
					continue
				}
				var tintList []*tint.Tint
				if err := json.Unmarshal(data, &tintList); err == nil && len(tintList) > 0 {
					for _, t := range tintList {
						th := tintToTheme(t)
						th.Source = SourceCustom
						loaded = append(loaded, th)
					}
					continue
				}
				if th, ok := parseSingleCustomTheme(data); ok {
					loaded = append(loaded, th)
				}
			}
		}
	}
	return loaded
}

func parseSingleCustomTheme(data []byte) (Theme, bool) {
	var th Theme
	if err := json.Unmarshal(data, &th); err == nil && th.Name != "" && th.Bg != "" && th.Text != "" {
		th.Source = SourceCustom
		if th.PaneBg == "" {
			th.PaneBg = th.Bg
		}
		if th.Panel == "" {
			th.Panel = th.PaneBg
		}
		if th.HeatmapRamp[0] == "" {
			cPanel := th.Panel
			cGreen := th.Green
			if cGreen == "" {
				cGreen = th.Accent
			}
			th.HeatmapRamp = [5]lipgloss.Color{
				cPanel,
				blendColor(cPanel, cGreen, 0.25),
				blendColor(cPanel, cGreen, 0.50),
				blendColor(cPanel, cGreen, 0.75),
				cGreen,
			}
		}
		return th, true
	}

	var t tint.Tint
	if err := json.Unmarshal(data, &t); err == nil && (t.DisplayName != "" || t.ID != "") {
		th := tintToTheme(&t)
		th.Source = SourceCustom
		return th, true
	}
	return Theme{}, false
}

// initThemeRegistry initializes the registry combining curated, custom, and bubbletint palettes.
func initThemeRegistry() {
	themeRegistry = make(map[string]Theme)
	allThemes = nil

	// 1. Curated themes
	for _, t := range curatedThemes {
		norm := strings.ToLower(strings.TrimSpace(t.Name))
		themeRegistry[norm] = t
		allThemes = append(allThemes, t)
	}

	// 2. Custom themes
	custom := loadCustomThemes()
	for _, t := range custom {
		norm := strings.ToLower(strings.TrimSpace(t.Name))
		if _, exists := themeRegistry[norm]; !exists {
			themeRegistry[norm] = t
			allThemes = append(allThemes, t)
		}
	}

	// 3. Bubbletint default tints (sorted alphabetically)
	bubbleTints := tint.DefaultTints()
	sort.Slice(bubbleTints, func(i, j int) bool {
		return strings.ToLower(bubbleTints[i].DisplayName) < strings.ToLower(bubbleTints[j].DisplayName)
	})

	for _, bt := range bubbleTints {
		th := tintToTheme(bt)
		norm := strings.ToLower(strings.TrimSpace(th.Name))
		if _, exists := themeRegistry[norm]; !exists {
			themeRegistry[norm] = th
			allThemes = append(allThemes, th)
		}
		idNorm := strings.ToLower(strings.TrimSpace(bt.ID))
		if _, exists := themeRegistry[idNorm]; !exists {
			themeRegistry[idNorm] = th
		}
	}
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
	initThemeRegistry()
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
