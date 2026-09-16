package ui

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"taskii/internal/model"
)

func TestParseTaskInputAnnotations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		raw   string
		want  parsedTask
		notOK bool
	}{
		{
			name: "plain task",
			raw:  "write the report",
			want: parsedTask{title: "write the report", kind: model.KindTask},
		},
		{
			name: "point in time",
			raw:  "standup 09:00",
			want: parsedTask{title: "standup", time: "09:00", kind: model.KindAppointment},
		},
		{
			name: "time range",
			raw:  "design review 14:00-15:30",
			want: parsedTask{title: "design review", time: "14:00", endTime: "15:30", kind: model.KindAppointment},
		},
		{
			// Tags are RECORDED but stay in the title, in the position they
			// were typed — the stored text is the sentence the user wrote.
			name: "single tag",
			raw:  "fix login #api",
			want: parsedTask{title: "fix login #api", kind: model.KindTask, tags: []string{"api"}},
		},
		{
			// Tags are self-delimiting, so unlike a time they're recognised
			// anywhere in the line rather than only at the end.
			name: "tags mid-sentence",
			raw:  "review #api docs #urgent today",
			want: parsedTask{title: "review #api docs #urgent today", kind: model.KindTask, tags: []string{"api", "urgent"}},
		},
		{
			name: "tags and range together",
			raw:  "planning #ops #q4 11:00-12:30",
			want: parsedTask{title: "planning #ops #q4", time: "11:00", endTime: "12:30", kind: model.KindAppointment, tags: []string{"ops", "q4"}},
		},
		{
			// The recorded tag LIST dedupes case-insensitively, but the
			// title keeps every word exactly as typed.
			name: "duplicate tags collapse in the list only",
			raw:  "ship #api fix #API again",
			want: parsedTask{title: "ship #api fix #API again", kind: model.KindTask, tags: []string{"api"}},
		},
		{
			// A time is only recognised as the LAST field, so an interior
			// number or clock time stays part of the title.
			name: "interior time stays in title",
			raw:  "meet at 09:00 downtown",
			want: parsedTask{title: "meet at 09:00 downtown", kind: model.KindTask},
		},
		{
			name: "bare hash is punctuation",
			raw:  "issue # 42",
			want: parsedTask{title: "issue # 42", kind: model.KindTask},
		},
		{
			// A backwards or zero-length range is likelier a typo or literal
			// text than an event, so it stays in the title.
			name: "backwards range is not a range",
			raw:  "shift 17:00-09:00",
			want: parsedTask{title: "shift 17:00-09:00", kind: model.KindTask},
		},
		{
			name: "zero length range is not a range",
			raw:  "blip 09:00-09:00",
			want: parsedTask{title: "blip 09:00-09:00", kind: model.KindTask},
		},
		{
			name: "invalid clock stays in title",
			raw:  "call 25:00",
			want: parsedTask{title: "call 25:00", kind: model.KindTask},
		},
		{name: "empty", raw: "   ", notOK: true},
		{
			// Tags remain in the title, so a line of nothing but tags still
			// has text to show — only a truly empty entry is rejected.
			name: "tags only is still a title",
			raw:  "#api 09:00",
			want: parsedTask{title: "#api", time: "09:00", kind: model.KindAppointment, tags: []string{"api"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseTaskInput(tc.raw)
			if ok == tc.notOK {
				t.Fatalf("ok = %v, want %v", ok, !tc.notOK)
			}
			if tc.notOK {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseTaskInput(%q)\n got %+v\nwant %+v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestTaskInputRoundTrip pins that every annotation survives an edit. The
// editor re-parses whatever it shows on save, so an annotation missing from
// taskInputText would be silently dropped just by opening a task.
func TestTaskInputRoundTrip(t *testing.T) {
	for _, raw := range []string{
		"write the report",
		"standup 09:00",
		"design review 14:00-15:30",
		"fix login #api",
		"review #api docs #urgent today",
		"ship it #api #infra 16:00",
		"planning #ops 11:00-12:30",
	} {
		p, ok := parseTaskInput(raw)
		if !ok {
			t.Fatalf("parse failed for %q", raw)
		}
		task := model.Task{Title: p.title, Time: p.time, EndTime: p.endTime, Kind: p.kind, Tags: p.tags}

		again, ok := parseTaskInput(taskInputText(task, testNow()))
		if !ok {
			t.Fatalf("re-parse failed for %q", raw)
		}
		if !reflect.DeepEqual(p, again) {
			t.Errorf("%q did not round-trip\n first: %+v\nsecond: %+v", raw, p, again)
		}
	}
}

func TestEditUpdatesAndClearsAnnotations(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.tasks = []model.Task{{
		ID: "x", Title: "Planning #ops", Time: "11:00", EndTime: "12:30",
		Kind: model.KindAppointment, Tags: []string{"ops"}, Date: "2026-09-13",
	}}
	a.focus = focusToday

	// Opening the editor shows every annotation, so what's on screen is the
	// whole truth about the task. The tag is already part of the title.
	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if got := a.input.Value(); got != "Planning #ops 11:00-12:30" {
		t.Fatalf("editor value = %q", got)
	}

	// Deleting annotations from the text removes them from the task.
	a.input.SetValue("Planning")
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	got := a.tasks[0]
	if got.Time != "" || got.EndTime != "" || len(got.Tags) != 0 || got.IsAppointment() {
		t.Errorf("annotations not cleared on edit: %+v", got)
	}
}

func TestRenderShowsRangeAndTags(t *testing.T) {
	plain := func(s string) string { return ansiRe.ReplaceAllString(s, "") }
	row := func(task model.Task, w int) string {
		return plain(renderTaskLine(task, false, false, w, colorPaneBg, testNow()))
	}

	ranged := model.Task{Title: "Review", Time: "14:00", EndTime: "15:30", Kind: model.KindAppointment}
	if got := row(ranged, 50); !strings.Contains(got, "14:00-15:30") {
		t.Errorf("range not rendered: %q", got)
	}

	// Tags stay exactly where they were typed — the row's plain text is the
	// title verbatim, with no reordering.
	tagged := model.Task{Title: "review #api docs #urgent", Tags: []string{"api", "urgent"}}
	got := row(tagged, 50)
	if !strings.Contains(got, "review #api docs #urgent") {
		t.Errorf("title should render verbatim, tags in place: %q", got)
	}
}

// TestTagsAreColoredInPlace checks the one thing the plain-text assertions
// above cannot: that the "#tag" words carry the tag colour while the rest of
// the title keeps the row's own, without the position changing.
func TestTagsAreColoredInPlace(t *testing.T) {
	// lipgloss strips every escape sequence when it can't see a TTY, which
	// is the case under `go test` — so the profile is forced here or the
	// assertions below would pass vacuously against plain text.
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	// Foreground SGR for a hex colour, as lipgloss emits it.
	fgSeq := func(c lipgloss.Color) string {
		r, g, b := hexToRGB(string(c))
		return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
	}
	tagSeq := fgSeq(colorAccent)

	styled := renderTaskLine(
		model.Task{Title: "review #api docs", Tags: []string{"api"}},
		false, false, 50, colorPaneBg, testNow())

	// The tag word carries the tag colour.
	if !strings.Contains(styled, tagSeq+";48;2") {
		t.Errorf("tag colour absent from the row: %q", styled)
	}
	// ...and the words around it carry a DIFFERENT one, so the tag reads as
	// distinct rather than the whole title being recoloured.
	fgRuns := map[string]bool{}
	for _, m := range regexp.MustCompile(`38;2;\d+;\d+;\d+`).FindAllString(styled, -1) {
		fgRuns[m] = true
	}
	if len(fgRuns) < 2 {
		t.Errorf("expected the tag to differ from the surrounding title, saw only %v", fgRuns)
	}

	// Position is unchanged: the plain text is the title verbatim.
	if plain := ansiRe.ReplaceAllString(styled, ""); !strings.Contains(plain, "review #api docs") {
		t.Errorf("title text altered: %q", plain)
	}

	// A title with no tag gets no tag-coloured span at all.
	untagged := renderTaskLine(model.Task{Title: "review docs"}, false, false, 50, colorPaneBg, testNow())
	if strings.Contains(untagged, tagSeq+";48;2") {
		t.Errorf("untagged row should carry no tag colouring: %q", untagged)
	}
}

// hexToRGB converts "#rrggbb" to its components.
func hexToRGB(hex string) (int, int, int) {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

// A range must not disturb the day's ordering: Time stays a bare HH:MM, so
// appointments still sort by start time.
func TestRangeSortsByStartTime(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.now = testNow
	a.tasks = []model.Task{
		{ID: "c", Title: "Late", Time: "16:00", Kind: model.KindAppointment, Date: "2026-09-13"},
		{ID: "a", Title: "Early range", Time: "09:00", EndTime: "17:00", Kind: model.KindAppointment, Date: "2026-09-13"},
		{ID: "b", Title: "Mid", Time: "12:00", Kind: model.KindAppointment, Date: "2026-09-13"},
	}
	var order []string
	for _, task := range a.todayTasks() {
		order = append(order, task.ID)
	}
	if !reflect.DeepEqual(order, []string{"a", "b", "c"}) {
		t.Errorf("order = %v, want [a b c]", order)
	}
}
