package ui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// dimANSI darkens every truecolor foreground/background SGR sequence in an
// already-rendered ANSI string toward black by factor (0 = unchanged, 1 =
// black), leaving everything else — text, cursor movement, other SGR codes
// — untouched. Used to show the app dimmed behind a modal instead of hiding
// it behind a solid backdrop.
func dimANSI(s string, factor float64) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != 0x1b {
			r, size := utf8.DecodeRuneInString(s[i:])
			b.WriteRune(r)
			i += size
			continue
		}
		j := i + 1
		for j < len(s) && s[j] != 'm' {
			j++
		}
		if j >= len(s) {
			b.WriteString(s[i:])
			break
		}
		j++ // include the trailing 'm'
		b.WriteString(dimSGR(s[i:j], factor))
		i = j
	}
	return b.String()
}

// dimSGR rewrites one CSI...m escape sequence, darkening any 38;2;r;g;b or
// 48;2;r;g;b (truecolor fg/bg) triplet it contains and passing every other
// parameter through unchanged.
func dimSGR(seq string, factor float64) string {
	if !strings.Contains(seq, ";2;") {
		return seq
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	parts := strings.Split(inner, ";")

	out := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		if (parts[i] == "38" || parts[i] == "48") && i+4 < len(parts) && parts[i+1] == "2" {
			r := dimComponent(parts[i+2], factor)
			g := dimComponent(parts[i+3], factor)
			bl := dimComponent(parts[i+4], factor)
			out = append(out, parts[i], "2", r, g, bl)
			i += 4
			continue
		}
		out = append(out, parts[i])
	}
	return "\x1b[" + strings.Join(out, ";") + "m"
}

func dimComponent(s string, factor float64) string {
	v, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	v = int(float64(v) * (1 - factor))
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return strconv.Itoa(v)
}

// compositeOver overlays `top` (a small rectangular block, e.g. a modal),
// centered, on top of `base` (a full page already padded to width x height).
// Unlike lipgloss.Place, which pads its whitespace with a solid fill color,
// this splices top's own lines directly into base's — so the base content
// outside the overlay's footprint stays exactly as rendered (dimmed, in the
// settings-modal case) instead of being replaced.
func compositeOver(base, top string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	topLines := strings.Split(top, "\n")

	topWidth := 0
	for _, l := range topLines {
		if w := lineWidth(l); w > topWidth {
			topWidth = w
		}
	}
	topHeight := len(topLines)

	rowOff := (height - topHeight) / 2
	colOff := (width - topWidth) / 2
	if rowOff < 0 {
		rowOff = 0
	}
	if colOff < 0 {
		colOff = 0
	}

	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	for i, tl := range topLines {
		row := rowOff + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		left := ansiSlice(baseLines[row], 0, colOff)
		right := ansiSlice(baseLines[row], colOff+lineWidth(tl), width)
		baseLines[row] = left + tl + right
	}

	return strings.Join(baseLines, "\n")
}

// lineWidth is the display width of one already-styled line, ignoring ANSI
// escape sequences.
func lineWidth(s string) int {
	w := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := i + 1
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w += runewidth.RuneWidth(r)
		i += size
	}
	return w
}

// ansiSlice extracts the display-cell range [start, end) from an
// already-styled line, carrying forward whatever SGR state was active at
// `start` so the extracted slice renders correctly on its own, and
// re-terminating with a reset. Cells past the end of the line are treated
// as blank. Used to preserve the dimmed background on either side of the
// modal when splicing it into a line.
func ansiSlice(s string, start, end int) string {
	if end <= start {
		return ""
	}
	var b strings.Builder
	var lastSGR string
	col := 0
	wroteAny := false

	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := i + 1
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++
			}
			seq := s[i:j]
			lastSGR = seq
			if col >= start && col < end {
				b.WriteString(seq)
			}
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := runewidth.RuneWidth(r)
		if col >= start && col < end {
			if !wroteAny && lastSGR != "" {
				b.WriteString(lastSGR)
			}
			wroteAny = true
			b.WriteString(s[i : i+size])
		}
		col += rw
		i += size
		if col >= end {
			break
		}
	}

	// Pad any remaining cells (line was shorter than `end`) with plain
	// spaces carrying whatever styling was last active, so the slice always
	// spans exactly end-start cells.
	if col < end {
		if lastSGR != "" {
			b.WriteString(lastSGR)
		}
		b.WriteString(strings.Repeat(" ", end-col))
	}

	if wroteAny || col < end {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}
