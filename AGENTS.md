# AGENTS.md

Decision log for the `terminal-dashboard` project. Updated after every step/change.

## Project summary

Full-screen terminal task manager / dashboard. Sections:
1. **Today** — today's tasks & appointments, mark done/undone, delete.
2. **Overdue** — previous days' undone tasks, shown in a distinct (warning) color.
3. **Reports** — progress for today/week/month + extra charts (7-day bar chart, streak).

Data persists to a JSON file in the project directory (`data/tasks.json`), loaded on every launch.

## Tech stack decisions

- **Language**: Go (chosen by user over Rust/ratatui, Python/Textual, Node/Ink).
- **TUI framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm-architecture TUI).
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss) for layout/colors.
- **Components**: [Bubbles](https://github.com/charmbracelet/bubbles) (list, textinput) where useful.
- Why: modern, actively maintained, produces visually polished dashboards, compiles to a single static binary — easy to run with `go run .` or build.

## Effort-tier workflow (per user instruction)

- **Project structure & planning** → done at high effort, directly in the orchestrating session.
- **Feature development/implementation** → delegated to medium-effort subagents.
- **Low-level tasks** (file read/write helpers, lint, type/build checks, doc touch-ups, bug fixes) → delegated to Haiku 4.5 high-effort subagents.
- This file is updated after every step/change with what was done and why.

## Project structure (as scaffolded)

```
terminal-dashboard/
├── AGENTS.md
├── go.mod
├── main.go
├── internal/
│   ├── model/       # Task struct + JSON store (load/save)
│   ├── ui/          # Bubble Tea root model, panes, theme, keybindings
│   └── stats/       # completion % / streak calculations for reports
├── data/
│   └── tasks.json   # runtime data file (gitignored)
└── .gitignore
```

## Data model (planned)

```go
type Task struct {
    ID        string
    Title     string
    Done      bool
    Date      string // YYYY-MM-DD, the day the task belongs to
    Time      string // optional HH:MM for appointments, "" if none
    CreatedAt time.Time
    DoneAt    *time.Time
}
```

Store: whole-file JSON array, saved synchronously after every mutation (add/toggle/delete). Simple and safe at this scale (personal single-user todo list).

## Layout (planned)

Two-column full-screen layout:
- Left column, top: **Today** pane (add/toggle/delete).
- Left column, bottom: **Overdue** pane (read-mostly: toggle/delete allowed, no add — tasks are inherited from prior days), rendered in a warning/orange-red color.
- Right column: **Reports** pane — today/week/month completion progress bars, 7-day completion bar chart, current streak.

Keybindings: `a` add, `space`/`enter` toggle done, `d` delete, `↑/↓` or `j/k` navigate, `tab` switch focus between Today/Overdue, `q`/`ctrl+c` quit.

## Implementation details (as built)

- **Streak definition**: walking backwards from today, a day counts toward the streak only if it has ≥1 task AND all of that day's tasks are Done. A day with zero tasks breaks the streak. Today itself doesn't break an already-earned streak while still in progress (incomplete or empty) — it just isn't counted until finished.
- **Week boundary**: rolling last 7 days (today + 6 prior), not calendar Mon–Sun. Chosen so "This Week" progress and the 7-day bar chart describe the same window.
- **Month boundary**: calendar month (1st of current month through today).
- **Add-task flow**: single inline text input (`a` to open, Enter confirms, Esc cancels). Typing a trailing `HH:MM` token (e.g. `Buy milk 14:30`) is auto-detected as the appointment time and stripped from the title — no separate time prompt.
- Dependencies: `charmbracelet/bubbletea` v1.3.10, `charmbracelet/lipgloss` v1.1.0, `charmbracelet/bubbles` v1.0.0.
- Theme: Tokyo Night–inspired dark palette, rounded borders, focus-aware border colors, overdue tasks rendered in a warning/amber-red color distinct from today's tasks.
- Known minor inconsistency: `internal/ui/keys.go` defines a `key.Binding` keymap, but `app.go` currently switches on raw `msg.String()` instead of `key.Matches`. Functionally equivalent, just not wired through the abstraction — candidate cleanup for a future pass.
- `go build ./...` and `go vet ./...` both clean; `gofmt` applied.

## Change log

- **2026-08-19** — Initialized git repo, `go mod init terminal-dashboard`, created directory skeleton (`internal/model`, `internal/ui`, `internal/stats`, `data`), `.gitignore`, and this AGENTS.md. Stack decision (Go + Bubble Tea) made via user choice among 4 presented options.
- **2026-08-19** — Implemented full app via medium-effort subagent: `main.go`, `internal/model/{task,store}.go`, `internal/stats/stats.go`, `internal/ui/{theme,keys,today,overdue,reports,app}.go`. Today/Overdue/Reports panes, JSON persistence to `data/tasks.json`, full keybinding set (a/space/enter/d/tab/j-k/arrows/q). Build and vet clean.
- **2026-08-19** — Ran the app in a tmux-wrapped terminal to visually verify. Found a real layout bug: `lipgloss.Style.Width(n)` sets *content* width only — the actual rendered width is `n + border + padding` (4 extra columns for `paneStyle`'s rounded border + `Padding(0,1)`), and `.Height(n)` similarly adds 2 extra lines for the border. Callers were passing intended *outer* dimensions straight into `.Width()/.Height()`, so every pane (Today, Overdue, Reports) rendered 4 columns wider / 2 lines taller than intended — the Reports pane's right border was pushed off-screen at 160 columns.
- **2026-08-19** — Fixed via a low-level Haiku subagent: `internal/ui/overdue.go`'s `renderPane()` and the Reports pane sizing in `internal/ui/app.go` (`View()`) now subtract the 4-column/2-line border+padding overhead before calling `.Width()/.Height()`, so callers keep thinking in outer/intended dimensions. Verified independently (build, vet, and manual tmux visual checks at 100x30 / 160x45 / 220x50 — all pane borders fully closed and aligned; also confirmed add/toggle/delete/persist round-trip and overdue-task warning-color rendering work correctly end-to-end). Test data cleaned up; `data/tasks.json` does not exist until first real run (gitignored).

## Status

Feature-complete and manually verified: Today/Overdue/Reports panes render correctly at multiple terminal sizes, add/toggle/delete work and persist to `data/tasks.json`, overdue tasks render in the warning color, reports (today/week/month progress, 7-day chart, streak) update live. Known minor cleanup item (non-blocking): `internal/ui/keys.go`'s `key.Binding` map isn't wired through `key.Matches` in `app.go` (raw string switch used instead, same effective behavior).

To run: `go run .` from the project root, or `go build -o terminal-dashboard . && ./terminal-dashboard`. Quit with `q`.

## Feature batch 2 (planned 2026-08-19)

User-requested additions, planned at high effort, implemented via medium-effort subagents in two passes (data/model/list changes first, then Pomodoro + polish), verified by a low-effort pass:

1. **Bug**: selected-row highlight background overflows 2 chars past the task line onto the next line — root cause: `selectedStyle`/row rendering not pinned to an explicit content width before applying background color. Fix: render each task line to an exact known width (pad/truncate) before applying `selectedStyle`.
2. **GitHub-style contribution heatmap**: add a calendar heatmap (weeks × days grid, shaded by completion count) to Reports, similar to GitHub's contribution graph / Claude Code's model-usage chart. Keep the existing "Last 7 Days" bar chart or fold it into the heatmap — implementer's call, document the choice.
3. **Scrollable Today/Overdue lists**: cap visible rows to a fixed viewport height per pane; j/k/arrows scroll the viewport when selection moves past the visible window. Needs a `scrollOffset` per list in `App` state.
4. **Pomodoro section**: new pane with work/break countdown timer driven by `tea.Tick`. Needs new state (phase: work/short-break/long-break, remaining duration, running/paused) and keybindings (start/pause/reset/skip).
5. **Important tag**: `Task.Important bool` field (new JSON field, backward compatible — missing/omitted defaults false on old data), toggle keybinding (`i`), visually distinct style (star prefix + accent color).
6. **Filter: important-only** — keybinding to toggle showing only important tasks in Today/Overdue.
7. **Filter: undone-only** — keybinding to toggle showing only undone tasks (mainly relevant for Today, since Overdue is already undone-only by definition).
8. **Prettier charts**: improve bar chart / progress bar rendering — smoother gradient blocks, better spacing/alignment, more visually polished report styling overall.

## Feature batch 2 — implementation (2026-08-19)

Implemented by a medium-effort subagent, then independently verified (and two rendering bugs fixed) by the orchestrating session.

**New/changed files**: `internal/model/task.go` (+`Important bool`), `internal/stats/stats.go` (+`HeatmapCell`/`computeHeatmap`, replaced `DayCount`/`Last7`), `internal/ui/theme.go` (+`importantStyle`), `internal/ui/keys.go` (+important/filter/pomodoro bindings), `internal/ui/today.go` (rewritten: exact-width row rendering, viewport scrolling), `internal/ui/overdue.go` (`renderPane` skips empty title; width/height math fixed, see below), `internal/ui/reports.go` (gradient bars, heatmap renderer, height-aware layout), `internal/ui/app.go` (scroll state, filter state, pomodoro wiring, two-row right column), `internal/ui/pomodoro.go` (new: phase state machine, tick-driven countdown).

**Design decisions**:
1. Selection-highlight bug — fixed by padding/truncating each row to an exact content width before applying the background style.
2. Heatmap **replaces** the old 7-day bar chart (not additive) — same "how active am I" story, far more history (14 weeks), reads at a glance from block density. Sun–Sat grid, 5-level color ramp.
3. Scrolling — independent `scroll`/`selected` index pair per list (today/overdue), minimal-movement viewport sync, "↑ N more / ↓ N more" indicator when overflowing.
4. Pomodoro — `p` pause/resume, `r` reset phase, `n` skip phase. 25/5/15 min work/short/long-break, long break every 4th work session. Own pane stacked below Reports in the right column.
5. Important tag — `★` prefix + amber (`colorWarning`), distinct from overdue-red and done-muted. `i` toggles it on the selected task.
6/7. Filters — independently toggleable, not mutually exclusive (`I` important, `U` undone), since "important AND undone" is a realistic combined view. Applied to both Today and Overdue; shown in pane titles as e.g. `[Important, Undone]`.
8. Charts — sub-character gradient fill via eighth-block Unicode glyphs, percent-driven green/amber/red coloring instead of one flat color.

**Bugs found during independent verification (not caught by the subagent's own tmux pass) and fixed by the orchestrating session**:
- **Heatmap level-0 color bug**: empty/no-activity cells rendered as solid `██` blocks in `colorMuted` (a medium-bright slate), which reads as "filled" rather than "empty" against the dark pane background — a heatmap with zero data looked fully lit up. Fixed by changing `heatmapRamp[0]` from `colorMuted` to `colorPanel` (near-background) so level 0 visually recedes, matching GitHub's own near-invisible empty-cell convention.
- **Pane width/height overflow bug** (root cause of a visible layout misalignment — Reports/Pomodoro panes rendering 2-6 lines taller than the Today/Overdue column, borders no longer lining up): `internal/ui/overdue.go`'s `renderPane` computed `.Width(width - 4)`, incorrectly assuming lipgloss's `Style.Width()` needs both border (2 cols) AND padding (2 cols) subtracted from the intended outer width. Empirically verified (via temporary Go tests) that `.Width()` already carves padding OUT of its own value — only the border sits outside it — so the correct subtraction is `width - 2`, not `width - 4`. The old formula under-sized the actual pane by 2 columns, which combined with a second bug (see next) caused long lines to wrap and silently grow pane height past its intended budget. Fixed to `contentWidth := width - 2`.
- **`renderReports` bar-width miscalculation**: `renderProgressRow` appends a percentage + count suffix (up to ~13-14 visible chars, e.g. `" 100.0% (12/34)"`) after the gradient bar on the same line, but `barWidth` was computed as `width - 4`, leaving no room for that suffix — the combined line exceeded the pane's content width by a few columns, triggering word-wrap inside lipgloss and (compounding with the pane-width bug above) inflating the whole pane's rendered height. Fixed by reserving `width - 14` for the bar instead.
- **`renderReports`'s heatmap-fit guard undercounted lines**: `remaining := height - len(sections)` counted string *elements*, not rendered *lines* — several `sections` entries are multi-line strings (e.g. each progress row is 2 lines), so the guard overestimated available headroom before deciding whether to render the heatmap. Fixed by counting actual `\n`-delimited lines per section.

All four fixes were verified with temporary Go unit tests (measuring `lipgloss.Width`/`lipgloss.Height` on isolated render calls to pin down exactly which line/box was overflowing) plus a fresh round of tmux visual verification at 160×24 and 160×50: confirmed all four pane borders close and align at both sizes, confirmed 15+ tasks trigger the "↓ N more" scroll indicator, confirmed the selection-highlight background terminates exactly at the pane's inner edge (checked via raw ANSI capture), confirmed important-tagging, the independent/combinable Important+Undone filters, and the Pomodoro start/pause/countdown all work end-to-end, and confirmed data reloads correctly across a restart. Temporary test files and build artifacts were removed after verification; `go build`, `go vet`, and `gofmt -l` are clean.

**Updated keybindings**: `a` add · `space`/`enter` toggle done · `d` delete · `i` toggle important · `I` filter important-only · `U` filter undone-only · `tab` switch pane · `↑/↓` `j/k` navigate (scrolls viewport) · `p` pomodoro pause/resume · `r` pomodoro reset phase · `n` pomodoro skip phase · `q`/`ctrl+c` quit.

## Status

Feature-complete for batches 1 and 2, independently verified end-to-end via tmux at multiple terminal sizes. No known layout bugs remaining. Minor non-blocking cleanup item carried over: `internal/ui/keys.go`'s `key.Binding` map isn't wired through `key.Matches` in `app.go` (raw string switch used instead, same effective behavior).

## Feature batch 3 (planned 2026-08-19)

User-requested, done step by step per explicit request (implement + verify each before moving to the next, not batched):

1. **Theme option** — in-app keybinding (`t`) cycles themes live; choice persists across runs (saved alongside task data). 3-4 built-in palettes: current dark (Tokyo Night), a light theme, plus 1-2 popular dev palettes (Dracula and/or Nord).
2. **`--mock` CLI flag** — launches with a generated realistic fake task set (mix of today/overdue, done/undone, important, tasks and appointments) instead of loading/saving `data/tasks.json`, so mock runs never touch real data.
3. **Task vs Appointment entry types** — new `Kind` field on the task model (task/appointment). Appointments render with `{ }`/`{x}` instead of `[ ]`/`[x]` and are the only entries with an optional time field (per user decision — tasks no longer carry a time, only appointments do). Needs an add-flow decision (prompt for kind, or a keybinding to add-as-appointment vs add-as-task) and backward-compat handling for old saved data that has `Time` set on non-appointment entries.
4. **Section title background** — add a background color to pane title bars (Today/Overdue/Reports/Pomodoro).
5. **Bug: scroll indicator changes section height** — `visibleRowsFor` in app.go doesn't reserve a line for the "N more" indicator, so panes grow by 1 line whenever it appears. Fix: always reserve that line in the height budget (render a blank line when the indicator isn't needed) so pane height is constant regardless of scroll state.
6. **Bottom keybinding guide** — restyle for readability: group related keys, consistent visual separators/coloring, possibly multi-line.
7. **Bug: heatmap not visible** — `renderReports`'s fit guard (`remaining >= 13` lines of headroom) is too strict for realistic terminal sizes, so the heatmap silently disappears with no explanation. Fix: lower the threshold and/or show a clear "resize to see activity chart" fallback instead of silently vanishing.

### Progress (done directly by the orchestrating session, step by step)

- **Item 5 fixed**: `renderTaskList` (internal/ui/today.go) now always emits the scroll-indicator line — blank when there's nothing to scroll — instead of only appending it when needed. `visibleRowsFor` (internal/ui/app.go) reserves that line unconditionally in its row budget. Empty-list case also emits a matching blank second line so `(no tasks)` is the same height as a populated list. Verified via tmux: Today pane's bottom border sits on the same row with 0 tasks and with 15 tasks (indicator showing).
- **Item 7 fixed**: two compounding causes. (a) The right column's Reports/Pomodoro height split was a flat 3:5, starving Reports of room for 3 progress bars + a 7-row heatmap grid on realistic terminal sizes — changed to give Pomodoro a fixed budget (its content is always exactly 7 lines) and let Reports claim the remainder. (b) The heatmap's own layout cost 11 lines (separate legend row + blank framing); moved the legend inline onto the heatmap's last (Saturday) row, cutting it to 9 lines, and lowered/recomputed the fit-guard threshold in `renderReports` to match. Also added week-trimming so the heatmap narrows (fewest-recent-weeks-first, keeping "today" visible) rather than just refusing to render when the pane is a bit too narrow. Now visible starting around 160×40 terminals; still omitted below ~35 body rows since a 7-weekday-row grid has an irreducible minimum height — that's an inherent space trade-off, not a bug.
- **Item 4 done**: `titleStyle` (internal/ui/theme.go) now sets `Background(colorPanel)` plus `Foreground(colorText)` (was accent-colored text on transparent background). `renderPane` (internal/ui/overdue.go) and the inline title lines in `renderReports`/`renderPomodoro` now call `titleStyle.Width(...)` with the pane's text-safe content width so the background fills the full title row instead of just hugging the title text. Applies uniformly to all four panes (Today, Overdue, Reports, Pomodoro) since they share one `titleStyle`. Verified via tmux with raw ANSI capture — background color present, full-width, no overflow introduced.

- **Item 1 done**: `internal/ui/theme.go` restructured around a `Theme` struct (full palette + a 5-step `HeatmapRamp`) and a `themes` slice with 4 built-ins: Tokyo Night (default), Dracula, Nord, Light. All the package-level `color*`/`*Style` vars used throughout `internal/ui` are now assigned (not `const`) and rebuilt by `applyTheme(t Theme)` — so every other UI file keeps referencing them unchanged; only theme.go needed structural changes. `t` cycles themes (`internal/ui/keys.go` + the `"t"` case in `updateNormal`, internal/ui/app.go), shows a transient "Theme: NAME" status in the header, and persists the choice to a new `data/settings.json` (`internal/model/settings.go`, mirrors the existing `store.go` load/save pattern) so it's restored on next launch. Verified via tmux: cycled all 4 themes, confirmed border/accent ANSI color codes actually change per theme, confirmed the choice survives an app restart.

- **Item 2 done**: `--mock` CLI flag (`main.go`, parsed with `flag`, passed via a new `ui.Options{Mock: bool}` struct — `NewApp` now takes `Options` instead of no args). When set, `NewApp` calls `mockTasks(now)` (new `internal/ui/mock.go`) instead of `model.Load()`, and `App.noPersist` is set so `persist()` (and the `t` theme-cycle handler's settings save) becomes a no-op — mock runs never read or write `data/tasks.json` or `data/settings.json`. Mock data: 6 today tasks (mixed done/undone/important/appoint­ment-style times), 2 overdue items, plus ~6 weeks of backfilled history (mostly-done, occasional gaps) so This Week/This Month progress and the heatmap have real-looking variation instead of all zeros.
  - Caught and fixed two bugs during this step's own verification: (a) the mock generator's title-selection index was derived from the same `id` counter as its done/undone check, so — because only 1-in-5 ids were undone and that residue class mod 10 always mapped to the same 1-2 titles — every overdue item showed one of only two repeating titles; fixed by deriving the title index from `offset`+`i` instead, which isn't correlated with the done-check's modulus. (b) `renderProgressRow`'s bar-width reservation (a flat `width-14`) was still occasionally too narrow — realistic mock data pushed counts to double digits (e.g. `(17/26)`), whose suffix is 15 chars wide, one more than reserved, causing exactly the same "wraps and inflates pane height" failure mode fixed earlier for the empty-data case. Fixed properly this time by having `renderProgressRow` compute its own suffix width from the actual formatted pct+counts string and size the bar from what's left, instead of the caller guessing a constant.

- **Item 3 done**: `Task` (internal/model/task.go) gained a `Kind` field (`KindTask`/`KindAppointment`, `json:"kind,omitempty"`) and an `IsAppointment()` helper. Per user decision, only appointments carry a time — the existing single add-input flow already auto-detects a trailing `HH:MM` token, so that same detection now also sets `Kind: KindAppointment` (internal/ui/app.go's `addTask`); no new keybinding needed. Appointments render with `{ }`/`{x}` instead of `[ ]`/`[x]` and a new `appointmentStyle` (purple, theme.go) instead of the plain task color — precedence in `renderTaskLine` (internal/ui/today.go) is done/overdue state first, then important, then appointment styling, so a done or overdue appointment still reads primarily as done/overdue. Backward compatible: old saved tasks with no `kind` field unmarshal to `Kind: ""`, and `IsAppointment()` returns false for anything that isn't exactly `KindAppointment`, so they render as plain `[ ]` tasks with no migration needed — verified by hand-crafting a pre-Kind JSON file and confirming it loads and renders correctly. `internal/ui/mock.go` updated so mock entries with a time are generated as appointments, consistent with the real add-flow.

- **Item 6 done**: bottom keybinding guide rewritten from one flat "·"-separated string into grouped, styled segments (new `internal/ui/helpbar.go`): keys are bold accent-colored, labels muted, groups (Task / View / Pomodoro / App) separated by a dim "│" within a group and a wider gap between groups, with the group name as an italic muted prefix. `renderHelpBar` wraps group-by-group onto a second line when the full bar doesn't fit the terminal width (verified: wraps at 160 cols, fits on one line at 220 cols), rather than truncating or overflowing.
  - This required a real fix to keep the layout correct: since the help bar's line count now varies with terminal width, the two places that compute the body panes' available height (`visibleRowsFor` and `View()`) previously both hardcoded `a.height - 6`, assuming a fixed 1-line help bar. Replaced with a shared `App.chromeLines()` that measures the actual rendered help bar height via `lipgloss.Height()`, so pane sizing and the help bar's real height never disagree — verified at both 160×40 (2-line help) and 220×40 (1-line help) that all pane borders still align with no clipping.

## Status

All 7 items from this batch are implemented and independently verified via tmux at multiple terminal sizes (160×24, 160×40, 220×40), plus a `--mock` run and a hand-crafted pre-Kind data file for backward-compatibility. `go build`, `go vet`, and `gofmt -l` are clean. No known open bugs.

## Follow-up: appointment hint in add mode (2026-08-19)

User asked: while in "add task" mode (`a`), inform the user they can type a trailing `HH:MM` to add an appointment instead of a task — the placeholder text already said this, but placeholders disappear the instant you start typing, so the hint was invisible mid-entry.

Fix: `App.helpGroups()` (internal/ui/app.go) now includes a third, key-less entry in the adding-mode help group — `"end with HH:MM to add it as an appointment"` — alongside the existing `enter confirm` / `esc cancel`. `renderHelpBar` (internal/ui/helpbar.go) was extended to treat an empty `helpKey.key` as a plain note (skips the bold key styling and the space that would otherwise precede the label) rather than requiring every help entry to be a "press X" pair. Verified via tmux: hint shows in the bottom bar throughout the whole time the add-input is focused (not just before typing), fits on one line down to 90-column terminals, and typing/confirming an appointment still works correctly.

## Feature batch 4 (2026-08-20) — 3 parallel subagents

User requested 4 tasks done in parallel via subagents. Two of the four (section backgrounds + orange theme) both needed to edit `theme.go`, so — per user's choice when asked — those two were merged into one subagent to avoid a guaranteed file conflict, giving 3 subagents total: [1] greeting section + header removal (owns app.go + new greeting.go), [2] Pomodoro completion notification (owns pomodoro.go + minimal app.go tick-handling), [3] section backgrounds + orange theme (owns theme.go only). Each was briefed with an explicit "only edit these files" boundary to prevent collisions. All three ran concurrently, all three finished clean, and the orchestrating session did a final integration pass (build/vet/gofmt + tmux visual check across all four features together) rather than trusting each agent's isolated verification alone.

1. **Header removed, greeting section added**: the old top bar (`"Terminal Dashboard" + date`) is gone. A new bordered "greeting" pane (new `internal/ui/greeting.go`) sits above Reports in the right column: **"Trak"** wordmark (reuses `appTitleStyle` — bold text on an accent-colored background, the existing TUI approximation of a blocky logo treatment), the date (same `"Monday, January 2, 2006"` format as before), and a time-of-day greeting with the OS username (`currentUsername()` tries `os/user.Current()`, falls back to `USER`/`USERNAME` env vars, degrades to no username rather than erroring). Height budgeting follows the same fixed-content-line pattern already established for the Pomodoro pane (`pomoContentLines`/`pomoHeight` in `View()`) — a small fixed budget carved out before Reports claims the remainder. `App.chromeLines()` updated to drop the now-removed header/blank-line chrome. The theme-cycle status message (previously shown in the removed header) was relocated to the error-line row since it had nowhere else to go.
2. **Pomodoro completion notification**: `pomodoro.tick()` (internal/ui/pomodoro.go) now returns `bool` (did it just cause a phase advance), and `Update()`'s `pomodoroTickMsg` case (internal/ui/app.go) fires `notifyPhaseChange(a.pomo.phase)` as a `tea.Cmd` — batched alongside the next tick, non-blocking — whenever that's true. Also fires on manual skip (`n`). Implementation shells out via `os/exec` (no new dependency): `osascript -e 'display notification ...'` on macOS, `notify-send` on Linux, silent no-op elsewhere; any command error is discarded rather than surfaced, since this is a nice-to-have. Message text: "Focus session complete — time for a break!" leaving work, "Break's over — back to work!" leaving a break. Verified: `osascript` executes successfully standalone in this environment (exit 0), the notification path doesn't block the UI (confirmed via a temporarily-shortened timer duration reaching zero live in tmux, then reverted), and rapid manual skips (`n`) stay responsive.
3. **Section backgrounds + Ember (orange) theme**: `Theme` struct gained a `PaneBg` field — deliberately a *different*, subtler shade than `Panel` (already used for title bars and the selected-row highlight) so panes read as distinct filled cards without everything collapsing into the same color; roughly halfway between each theme's `Bg` and `Panel`. Applied via `Background(colorPaneBg)` on `paneStyle`/`paneFocusStyle` (internal/ui/theme.go only — confirmed via lipgloss that a background on a bordered+padded style fills only the interior, border glyphs keep their own color). All 4 existing themes got a `PaneBg` value; a 5th theme, **"Ember"**, was added: warm dark palette, `Accent`/`BorderFocus` = saturated orange `#e0703a`/`#ff8c42`, `Warning` deliberately kept more yellow-amber (`#f2b544`) so it doesn't muddy against the orange accent in the same palette, `HeatmapRamp` kept green-based (GitHub's own convention, safer/more legible than a theme-hued ramp). Verified via tmux + raw ANSI capture: pane-content background color is present and distinct from title-bar background across all 5 themes; cycling `t` through all 5 doesn't crash; Ember's orange border/accent (`#ff8c42` = ANSI `255;140;65`) and pane fills render correctly.

**Integration verification** (orchestrating session, after all 3 subagents completed): `go build ./...`, `go vet ./...`, `gofmt -l .` all clean with zero manual reconciliation needed — the file-boundary briefing avoided any conflict. tmux checks at 160×45 and 160×30 with `--mock` confirm all four changes work together: no header, greeting pane renders correctly, pane backgrounds visible in raw ANSI, cycling to Ember shows the orange theme correctly, manual pomodoro skip (`n`) advances phase and increments session count with the UI staying responsive, and `osascript` notification delivery confirmed working standalone in this environment.

## Status

All 4 items from this batch are implemented, integrated, and verified together. No known open bugs.

## Follow-up: background bleed bug + keybinding guide simplification (2026-08-20)

User reported two problems with the section-background feature from batch 4: (1) some cells inside panes still lacked the background color, and (2) converting a task to an appointment or marking it important made the background — including the row-selection highlight — disappear entirely for that row.

**Root cause** (found via direct investigation, not delegated): lipgloss only emits a style's SGR codes once, at the very start/end of the string passed to `Render()`. `renderTaskLine` (internal/ui/today.go) was pre-rendering the time (`timeStyle.Render(...)`) and importance star (`importantStyle.Render(...)`) as their own independently-styled substrings — each ending in its own reset sequence — then splicing them as plain text into a larger `line` string that got wrapped in a SECOND `Render()` call for the row's main style (including the selection/pane background). That outer `Render()` does NOT re-apply itself after an inner span's embedded reset, so everything from the colored time/star onward lost its background — reading as a "hole" in the fill, and, when the row was selected, as the highlight disappearing partway through the line. Confirmed via raw ANSI byte inspection (`tmux capture-pane -e` piped through `od -c`) rather than guessing — the `\x1b[49m` (background reset) from the inner span's own close was visible sitting right before the un-backgrounded tail of the line.

**Fix, `internal/ui/today.go`**: `renderTaskLine` rewritten to never re-wrap already-styled text. Every segment (prefix+checkbox, time, star, title, and padding) is now rendered as a self-contained span carrying its own foreground AND the row's background, then concatenated as final output with no further `Render()` pass over the assembled string. Width/truncation math (`fitSegmentsToWidth`, new) is done on the PLAIN unstyled pieces up front, before any styling, so the ellipsis-truncation logic never has to reason about ANSI-embedded strings. `fitToWidth` (the original width helper) is kept for the plain hint/indicator lines in `renderTaskList`, which don't have this multi-segment problem.

**Fix, `internal/ui/theme.go`**: the same "leaf style has no background" issue existed structurally across every shared style used for text inside a pane — `taskStyle`, `doneStyle`, `overdueStyle`, `overdueDoneStyle`, `importantStyle`, `appointmentStyle`, `timeStyle`, `statLabelStyle`, `statValueStyle`, `inputPromptStyle` all gained `Background(colorPaneBg)`. Split `helpStyle` (kept transparent — used only for the bottom bar, which sits outside any pane and should keep the terminal's own background) from a new `hintStyle` (same muted look, but WITH `colorPaneBg`) for in-pane muted text like the "(no tasks)" and scroll-indicator lines in `internal/ui/today.go`.

**Fix, `internal/ui/reports.go` and `internal/ui/pomodoro.go`**: the same ad-hoc `lipgloss.NewStyle().Foreground(...)` pattern (no background) existed for the gradient progress-bar glyphs, heatmap cells/legend, percentage text, Pomodoro phase name, and Pomodoro timer digits — all given `Background(colorPaneBg)` alongside their existing foreground.

Verified via tmux + raw ANSI byte inspection (not just visual screenshots) across: an appointment row with a time, an important-starred row, and a row that's both selected AND important/appointment simultaneously (the exact combination the user reported) — confirmed the background color is now continuous across the entire row with zero drop-outs in every case, and the selection highlight (`colorPanel`) survives through colored sub-spans exactly like the pane background (`colorPaneBg`) does for unselected rows. Also re-verified the full dashboard layout still renders correctly end-to-end (all panes, `--mock` data, theme cycling) with no regressions from these changes.

## Follow-up: keybinding guide simplified (2026-08-20)

User asked: bottom keybinding bar should show only the keys (no group labels), with keys in brackets, e.g. `[a] add task`.

`internal/ui/helpbar.go`'s `renderHelpBar` rewritten: dropped the group-name prefix entirely (the `helpGroup.name` field is kept on the struct since app.go's `helpGroups()` call sites still use it to organize which keys are grouped together in source, but it's no longer rendered) and changed key formatting from bold-key-then-label to `[key] label`. The removed `helpGroupStyle` was deleted as dead code. Wrapping logic changed from group-by-group to entry-by-entry (since there are no more groups to keep together), so a `[key] label` pair never splits across the two lines the bar wraps to on narrower terminals. Verified via tmux at 160×40: bar now reads e.g. `[a] add │ [space/enter] toggle │ [d] delete │ ...` with no "Task:"/"View:"/etc. prefixes, wraps cleanly, and the add-mode hint (`end with HH:MM to add it as an appointment`) still renders correctly as an unbracketed plain note alongside the bracketed `[enter]`/`[esc]` keys.

## Follow-up: hardcoded TRAK block-letter banner (2026-08-20)

User asked to replace the plain "Trak" text logo in the greeting section with a hardcoded ASCII/Unicode block-art banner for the word "TRAK", in the spirit of a reference image (a blocky monospace-letter wordmark). Asked and confirmed with the user: 5 rows tall.

`internal/ui/greeting.go`: added `trakBanner []string` — a hand-designed 5-row, 27-column block-letter "TRAK" wordmark using `█` (full block), each glyph drawn on its own 5-row bitmap and composed with a 2-column gap between letters (built and column-verified with a disposable Go generator script in the scratchpad before hardcoding, to guarantee the letters align correctly — hand-typing the spacing directly is error-prone). Styled with the theme's accent color (bold, `colorAccent`) against the pane's own background — not a solid accent-filled block like `appTitleStyle` (which the old "Trak" text logo reused), since a filled rectangle behind hand-drawn block letters would hide their shape rather than show it.

`greetingContentLines` changed from a `const` to a `var` (`len(trakBanner) + 3`, i.e. banner rows + blank + date + greeting) so the height budgeting in `View()` (internal/ui/app.go) automatically tracks the banner's actual size instead of a stale hardcoded number.

**Bug found and fixed during this change's own verification**: growing the banner from 1 line (plain "Trak" text) to 5 lines exposed the same class of bug documented earlier in this file for Reports — `renderGreeting` unconditionally rendered all content lines regardless of the height actually granted to its pane, and lipgloss's `Style.Height()` is a floor, not a cap, so on shorter terminals (where `greetHeight` gets capped to `bodyHeight/3`) the pane silently overflowed past its box instead of shrinking to fit, pushing content off the top of the visible terminal entirely. Fixed by making `renderGreeting` height-aware (new `width, height` signature) — it now trims the banner rows first (from the bottom, keeping the more identifying top of the letterforms) when the budget is tight, then trims the date/greeting lines if even that isn't enough room. Verified via tmux at 160×30 (severely constrained — greeting pane correctly shows only the date line, banner hidden, no overflow), 160×36 and 160×45 (full banner shows, borders align correctly in all cases).

## Follow-up: banner shrunk to 3 rows, renamed to TASKII (2026-08-20)

User felt the 5-row banner was too tall and asked for a 3-row version using a wider mix of block/half-block characters (█ ▀ ▄ ▌ ▐), plus renamed the app from "Trak" to "TASKII".

First attempt (a `S`/`K`-heavy design using `▄███▄` style glyphs) was rejected as "horribly ugly." Per explicit user instruction, redid this by hand-drawing ONE sample at a time directly in the chat (no file edits) and asking for approval before touching code, iterating: fixed the `K` glyph on request, tried lowercase-style `i`s (rejected, user wanted the capital-I version from an earlier sample kept), then got final approval on:
```
█▀▀▀█ ▄▀▀▀▄ ▄▀▀▀▀ █ ▄▀ ▀█▀ ▀█▀
  █   █▀▀▀█  ▀▀▀▄ █▀▄   █   █
  ▀   ▀   ▀ ▀▀▀▀  ▀  ▀ ▀▀▀ ▀▀▀
```
Only wired into `internal/ui/greeting.go` (`taskiiBanner`) once approved — no code changes made during the iteration itself.

`renderGreeting`'s existing height/width-safety logic (trims banner rows under a tight height budget, falls back to plain "TASKII" text on narrow terminals) needed no changes — the new banner is the same 3-row, ~30-col shape as the design it replaced. Verified via tmux at 160×45: banner renders correctly, reads clearly, borders align.

## Follow-up: greeting pane background gaps fixed (2026-08-20)

User reported the greeting section still had cells without the pane background color. Root cause was the same class of bug documented earlier for `today.go`/`reports.go`: `renderGreeting` (internal/ui/greeting.go) rendered each line (banner rows, date, greeting) as a self-contained styled string, then relied on a single outer `lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))` call to pad every line up to the pane's width — but since each line already ends in its own SGR reset, lipgloss's padding (added after that reset) got no background, leaving the trailing whitespace on every non-full-width line unbackgrounded.

Fixed by padding each line manually before joining: a small `blankStyle` (background only) renders the exact number of trailing spaces needed to reach `width`, appended directly onto each already-styled line, so the padding carries the correct background itself instead of depending on an outer wrap that can't reach past an embedded reset. The final `strings.Join` result is returned directly (no more outer `Width().Render()` call). Verified via tmux + raw ANSI byte inspection across all three of `renderGreeting`'s paths: the full banner at 160 cols, the narrow-width "TASKII" plain-text fallback at 80 cols, and a height-trimmed case — confirmed `colorPaneBg` (`48;2;30;32;48` in Tokyo Night) is present continuously after every line's own reset, all the way to the pane border, in every case.

## Follow-up: page-wide background, plus a real init-order bug found along the way (2026-08-20)

User asked for the background color to apply to the WHOLE page, not just inside pane sections — the gap between the left/right columns, the error/status line, and the bottom help bar were all still showing the terminal's raw default background.

**Fix 1 — page-level padding** (`internal/ui/app.go`'s `View()`): added `colorBg` (the app's base/page background, distinct from `colorPaneBg` used inside panes) via a `pageBg` style, applied to the error/status line, and a `padLines` helper that right-pads every line of `body`, `errLine`, and `helpLine` to `a.width` with `colorBg`-styled spaces individually, BEFORE joining them together. This mattered for two separate reasons, not just one: (a) the by-now-familiar issue where a line carrying its own embedded ANSI can't have background backfilled by a later outer `Render()` call past its own reset; and (b) a previously-undiscovered one — `lipgloss.JoinVertical` pads shorter lines up to the widest sibling's width using its OWN plain, unstyled spaces, so even a line that already had correctly-backgrounded trailing padding got MORE, unstyled padding appended on top by `JoinVertical` itself whenever a sibling line (`body`, much wider than `errLine`/`helpLine`) was wider. Pre-padding every line to the same final width before the `JoinVertical` call removes its padding from the equation entirely, since it then has nothing left to pad.

**Fix 2 — a real bug, not a padding issue**: `internal/ui/helpbar.go` used to declare `helpKeyStyle`/`helpLabelStyle`/`helpSepStyle` as their own package-level `var` block, built once via `lipgloss.NewStyle().Foreground(colorAccent)...` etc. at package-load time. This is a genuine Go initialization-order bug: package-level `var` initializers all run before any `init()` function, and `colorAccent`/`colorMuted`/`colorBg`/`colorBorder` only get their real values inside `theme.go`'s `applyTheme()`, called from `theme.go`'s own `init()` — which runs AFTER `helpbar.go`'s var block already captured them as their zero value. The three help-bar styles were therefore permanently colorless (foreground AND background) from the very first paint onward, and never got rebuilt on subsequent theme changes either, since nothing outside `applyTheme()`'s own reassignment block was ever re-run. Fixed by moving all three into `theme.go`'s shared style var block and their construction into `applyTheme()`, exactly like every other themed style in the app. Confirmed via a temporary Go test that comparing `helpKeyStyle.Render("[a]")` before vs. after the fix went from a bare `[1m[a][0m` (no color at all) to a fully-combined `[1;38;2;121;162;247;48;2;26;27;38m[a][0m` (bold + accent fg + page bg).

**Verification note**: direct Go-level testing of `App.View()`'s raw output (a temporary test calling `.View()` and inspecting the exact bytes) confirms the help bar line is now fully, correctly backgrounded end-to-end — every segment carries `colorBg` in its SGR code. However, `tmux capture-pane -e`'s re-serialized ANSI output for that same live-rendered line still shows what looks like split/missing background codes on some segments, which could not be conclusively attributed to either a real remaining rendering bug or to tmux's own capture-pane ANSI re-encoding (multiplexers are known to omit "unchanged" SGR parameters between cells when reconstructing escape sequences for capture output, which wouldn't reflect an actual visual defect). Asked the user to visually confirm in a real terminal rather than continuing to guess via automated tooling — the underlying Go-level bug is fixed and verified either way.

## Follow-up: black rectangle artifacts (Light theme), and a real renderPane bug found while fixing it (2026-08-20)

User attached a screenshot (Light theme, empty task lists) showing solid BLACK rectangles at several points — after "(no tasks)", after "0.0%", after "Current Streak:", after "Focus" — clearly a different symptom from the earlier missing-background gaps (those would show the terminal's own background, which in a screenshot of a black-background terminal running the pale Light theme would ALSO look black, so this needed careful diagnosis to confirm it wasn't just the same class of bug in a new spot vs. something else, like a stray cursor).

Ruled out a terminal-cursor-position theory (Bubble Tea already auto-hides the real cursor at TTY init; a temporary test program confirmed it stays hidden) and confirmed via a direct Go-level test that `renderPane` (`internal/ui/overdue.go`) was the culprit: its final `style.Width(contentWidth).Height(contentHeight).Render(content)` call — where `content` is a multi-line string ALREADY full of embedded ANSI from every caller — auto-pads any line shorter than `contentWidth` and adds blank lines up to `contentHeight`, but that auto-padding is PLAIN, unstyled space, not `colorPaneBg`-filled, when the wrapped content already carries its own styling. This is the same family of bug fixed repeatedly elsewhere in this file, but this time inside the shared pane-wrapping function all six panes (Today/Overdue/Reports/Pomodoro/Greeting, everything) route through — so it affected every pane's empty-state text, every short/blank row, and the tail end of every pane shorter than its box.

First fix attempt — pad `content`'s lines to `contentWidth`/`contentHeight` manually (mirroring the fix pattern used everywhere else in this session) and THEN still call `style.Render(content)` for the border — introduced a NEW, worse bug: a correctly-produced single-line, ANSI-heavy row (e.g. "bar + 33.3% + (2/6)", one styled 59-cell string) got spuriously split across two separate output lines purely by being passed through `Style.Render()` a second time after already containing complex nested Bold+Foreground+Background ANSI. Confirmed by isolating: the identical padding-then-`Render()` logic worked correctly against a plain unstyled string, but broke specifically on real, heavily-styled task-list/progress-bar content — this lipgloss version's width/alignment pass is unreliable on already-ANSI content passed through a *second* `Style.Render()` call, not just for padding (as documented in every earlier fix in this file) but for basic line integrity too.

Final fix: `renderPane` no longer calls `Style.Render()` on the assembled content at all. It builds the bordered box BY HAND — manually padding every content line to `contentWidth`/`contentHeight` with `colorPaneBg`-styled spaces (as before), then wrapping with hand-written border characters (`╭─╮`/`│`/`╰─╯`, matching `lipgloss.RoundedBorder()`'s exact glyph set) styled with a plain `Foreground(colorBorder or colorBorderFocus)` style applied only to the border glyphs themselves — never re-running lipgloss's own width/alignment/border pipeline over pre-styled content. `contentWidth`/`contentHeight` math changed slightly to account for this (now computed as border+padding both subtracted up front, `width-4`/`height-2`, matching what every caller already assumed when computing safe text-wrap widths like `rightWidth-4` — no caller changes were needed).

Verified via a battery of direct Go-level tests (plain-content round-trip, real ANSI-heavy progress-row round-trip, full `App.View()` byte inspection) plus tmux visual checks in both Tokyo Night and Light themes with `--mock` data and with a genuinely empty (non-mock) task list matching the user's screenshot exactly: no line-doubling/splitting anywhere, no black or otherwise-unstyled gaps anywhere — every background code present in every captured line is either `colorPaneBg` (inside a pane) or `colorBg` (the page-level gaps), confirmed via `grep`-ing the raw ANSI capture for `48;2;` codes and checking for anything unexpected (particularly `0;0;0`/black, which never appears).

## Follow-up: the last three unstyled-gap sources, found by an automated scanner (2026-08-20)

User re-reported black rectangles with a screenshot after the previous round. The remaining causes were found by writing a **scanner** rather than eyeballing captures: a helper (`findUnstyledRuns`) that walks a rendered line byte by byte, tracks whether a background SGR is currently active, and flags any run of 2+ spaces sitting outside one. Driving that over the full `App.View()` across 5 themes × 5 terminal sizes × 4 UI modes turned a vague "some characters are black" report into an exact, exhaustive list — this is the approach to reach for if this class of bug ever resurfaces, instead of inspecting `od -c` output by hand.

Three distinct root causes, all instances of the same underlying rule (**anything that lands between or after independently-styled ANSI segments has no background unless something explicitly gives it one**):

1. **`lipgloss.JoinVertical` inside `renderPane`** (`internal/ui/overdue.go`) — the title/body join padded every line up to the widest line's width with its own PLAIN spaces. Because the styled title row is normally the widest, every shorter body line arrived at `renderPane`'s own styled-padding loop already exactly `contentWidth` wide, so `pad` computed to 0 and the styled padding never ran. The plain spaces `JoinVertical` inserted were the black rectangles after short content like `(no tasks)`. Fixed by prepending the title to a plain `[]string` slice instead of calling `JoinVertical`, so this function's existing per-line styled padding is the only thing that ever pads.
2. **`renderPomodoro`'s phase line** (`internal/ui/pomodoro.go`) — `fmt.Sprintf("%s  %s", phaseStyle.Render(...), statLabelStyle.Render(...))` put two raw spaces *between* two independently-styled segments, outside both SGR spans. Fixed by folding the separator into the second segment's own `Render` call.
3. **The add-task textinput** (`internal/ui/app.go`) — two separate problems. It sizes itself from its placeholder unless given an explicit `Width`, so at narrow terminals (90 cols) it rendered 54 columns wide inside a 50-column content area, dragging the whole layout 5 columns past the terminal width and making `JoinHorizontal` insert unstyled filler between the columns. And its own internal styles (text/placeholder/prompt/cursor) carried no background, plus it pads out to its `Width` with plain spaces. Fixed by setting `a.input.Width` to the actual available space, assigning all four widget styles with `colorPaneBg` (done in `View()`, not `NewApp`, so they follow theme changes), and `TrimRight`-ing the widget's own trailing filler before re-padding it ourselves. `View()` has a value receiver, so mutating `a.input` there only affects the frame being rendered.

Final state: the scanner reports zero unstyled gaps across all 5 themes × 5 sizes × 4 modes, and a live tmux run in the Light theme (both normal and add-task mode) scans clean too. Typed input values were separately checked to confirm the `TrimRight` doesn't disturb real content. Temporary test files were removed; `go build`, `go vet`, and `gofmt -l` are clean.

## Add-task input line changed width as soon as you started typing (2026-08-20)

User reported that entering add mode and pressing the first key made the input line jump by ~2 characters. The previous entry's fix for this widget (setting `Width`, then `TrimRight`-ing its trailing filler) was treating symptoms; reading `bubbles@v1.0.0/textinput/textinput.go` showed why it couldn't work.

`textinput.View()` has **two branches that size themselves differently**:

- `placeholderView()` (taken while the value is empty) pads to `m.Width`, but the width-clamping it applies to a typed value is never applied to the placeholder itself — a placeholder longer than `Width` is emitted in full.
- The typed branch pads to `m.Width` and then appends a one-cell cursor block *past* that padding, so its real width is `Width + 1`.

So the line was 91 cells wide showing the 54-char placeholder in an 80-col terminal, then snapped to a correct 80 on the first keystroke. The old `TrimRight(view, " ")` was also a no-op: the widget pads through `TextStyle`, so the trailing spaces come out wrapped in SGR codes and the string ends in an escape sequence, not a space.

Fixes, all in `internal/ui/app.go`:

1. **Width is set in `Update`, not `View`.** The widget derives its horizontal-scroll offsets from `Width` inside `handleOverflow`, which runs from `Update`; a `View`-time assignment can never affect scrolling. New helpers `leftPaneWidth()` and `inputFieldWidth()` are shared by both so the two sides can't drift.
2. **`inputFieldWidth()` reserves everything that isn't the value**: pane interior (`leftPaneWidth()-4`), the app's own `"+ "` prompt, the widget's `Prompt`, and one cell for the cursor block. Under-reserving here doesn't wrap the line — it pushes it one column past the pane border, because `renderPane` clamps height but not width.
3. **The placeholder is clipped** to `inputFieldWidth()` via the existing `fitToWidth` helper (which ellipsizes by display width, not rune count).
4. **`Width` is zeroed at render time** so neither branch emits its own plain-space padding; the trailing fill is done here with a `colorPaneBg` style, as everywhere else in this file.

Verification: an instrumented test rendered the input line at 4 terminal sizes across 61 successive keystrokes (empty → past the field width, into horizontal scrolling) and recorded the rendered width at every step. Before: 3–4 distinct widths per size. After: **`distinct=1` at every size** — one constant width across the whole sequence. A separate test confirmed typed values still round-trip exactly (including the `HH:MM` → appointment parse) and that `View()`'s mutation of the frame-local placeholder doesn't leak back into the model. Confirmed live in tmux at 80×30 across all four states (placeholder, 1 char, 8 chars, horizontally scrolling); the border column does not move.

**Testing note worth remembering:** under `go test` stdout is not a TTY, so lipgloss degrades to the Ascii profile and emits **no SGR codes at all**. A background-gap scanner run in a test will therefore report every single space as an unstyled gap. Call `lipgloss.SetColorProfile(termenv.TrueColor)` (and re-run `applyTheme`) first, or the results are meaningless.

## Pane borders (and four more raw-space gaps) had no background (2026-08-20)

User reported the section borders didn't carry the background color. `renderPane` (`internal/ui/overdue.go`) built its hand-rolled border with `lipgloss.NewStyle().Foreground(borderColor)` and no `Background`, so every border glyph cell fell through to the terminal default — each pane was outlined in black.

Fixed by adding `.Background(colorPaneBg)` to `borderStyle`. **PaneBg, not Bg**: the border is the pane's own outermost edge, so filling it with the pane background makes each box read as one solid surface against the deliberately-different page background (the two differ in every theme, e.g. Tokyo Night `#1a1b26` page vs `#1e2030` pane).

Auditing for the same root cause (**a raw space between two independently-styled spans is outside both and gets the terminal default background**) turned up four more instances that the earlier scanner runs had missed:

1. `renderProgressRow` (`internal/ui/reports.go`) — `fmt.Sprintf("%s %s %s", bar, pct, counts)`. The two separator spaces were raw. Folded into the following segment's own `Render` (`" "+pct`, `" "+counts`); the existing `suffix` width calculation already counted both spaces, so no layout change.
2. `renderReports`' streak line — same pattern, `fmt.Sprintf("%s %s", label, value)`.
3. `renderHeatmap` — `b.WriteString(" ")` after the weekday label. Folded into the label's span.
4. `renderHelpBar` (`internal/ui/helpbar.go`) — `helpKeyStyle.Render("["+key+"]") + " " + helpLabelStyle.Render(label)`.

Note `paneStyle`/`paneFocusStyle` in theme.go are now dead (only mentioned in a comment) since `renderPane` was hand-rolled; left in place rather than widening this change's scope.

**Methodology correction worth remembering.** Mid-investigation the tmux `capture-pane -e` output appeared to show `[a]` in the help bar rendering with bold+foreground but *no* background, which looked like a real bug and sent me looking for an init-order problem in `helpKeyStyle`. It wasn't real: a direct Go-level probe showed the app emits `\x1b[1;38;2;121;162;247;48;2;26;27;38m[a]\x1b[0m` — background present and correct. `capture-pane -e` had re-serialized the single combined SGR into separate `\x1b[1m\x1b[38;...m` sequences and dropped the background parameter. This is the same `capture-pane -e` unreliability already noted earlier in this file — **when the two disagree, trust the Go-level `View()` bytes, not the tmux capture.**

Final verification therefore scans `App.View()` output directly rather than a terminal capture: 5 themes × 4 sizes × 2 modes (normal and add-task) = 40 full renders, walking every line and tracking background-active state. Result: **TOTAL UNSTYLED CELLS: 0**. `go build`, `go vet`, `gofmt -l` clean; temporary test files removed.

## Section titles moved onto the top border (2026-08-20)

User asked for titles on the left of the top border (`--- title ---------`) instead of as a row inside the box.

`renderPane` (`internal/ui/overdue.go`) now splices the title into the top border run: `╭` + `─ ` + title + ` ` + remaining dashes + `╮`. Built as separate border/title/border spans concatenated (never re-wrapped), per this file's standing rule. New `paneTitleStyle` in theme.go carries `colorPaneBg` — matching the border run it sits in — rather than reusing `titleStyle`, whose `colorPanel` background would read as a block pasted onto the border line.

Applied consistently to all four titled panes. Reports and Pomodoro previously drew their own title *inside* the body (they passed `title=""`); those rows were removed and the titles passed to `renderPane` instead, otherwise the app would have had two different title treatments. Freed body rows were returned to content:
- `pomoContentLines` 7 → 5 (title row + its blank spacer).
- `visibleRowsFor` no longer reserves `titleLines = 1`, so Today/Overdue each show one more task.

**Three pre-existing layout bugs surfaced while verifying this**, all found by sweeping every terminal width 70–200 × 5 heights and asserting every rendered line equals the terminal width:

1. **`renderPane` never clamped width.** It padded short lines but let long ones through, so any overlong body line silently widened the pane and dragged the whole `JoinHorizontal` layout past the terminal edge. Added a truncating branch plus `truncateANSI`, which cuts to a display width while copying ANSI escapes through (so styling and background survive) and re-terminating with a reset. Hand-rolled for the same reason the border is: lipgloss's own truncation is unreliable on already-styled multi-segment content.
2. **The heatmap legend wasn't budgeted.** `availableWeeks` sized the grid as `(width-3)/2`, counting only the grid, but the legend rides inline on the Saturday row and adds 27 cells — so that row always overflowed. At height ≥ 40 (where the heatmap renders at all) this pushed the layout up to **26 columns** past the terminal. Now `heatmapWidth`/`heatmapLegendWidth` make the legend explicit: it's kept only if it still leaves a legible grid, otherwise dropped and the full width goes to history. `renderHeatmap` takes a `withLegend` param.
3. **The help bar wrapped one cell too wide.** The wrap test compared the bare candidate against `width`, but every emitted line is `margin + lineText`, so wrapped lines ran to `width+1`. This was the `+1` overflow at ~50 scattered widths. Now the margin is subtracted from the budget up front.

Also fixed in the same pass: my first version of the border title miscounted its own decoration (`decor = 4` while emitting 3 cells: `─`, ` `, ` `), making every titled top border exactly one cell short. Caught by a direct `renderPane` width-contract test rather than by eye.

Verification: every rendered line equals the terminal width across **131 widths × 5 heights (655 renders)** — previously 50+ sizes overflowed. Background-gap scanner re-run after the border/title changes: **0 unstyled cells** across 5 themes × 5 sizes × 2 modes. Live tmux checks at 110×30 (columns align, no gutter drift) and 130×46 (heatmap, inline legend, and streak all render inside the pane). `go build`, `go vet`, `gofmt -l` clean; temp tests removed.

## Delete confirmation, Monokai theme, layouts, and version badge (2026-08-21)

Four user-requested features.

**1. Delete confirmation.** Added `modeConfirmDelete`. `d` now enters the mode instead of deleting (and only when something is actually selected, so the prompt never asks about nothing); `y`/`Y`/`enter` confirms, **anything else cancels** — chosen over matching only `n`/`esc` so a stray keypress can't destroy a task. The prompt takes over the status line and names the task (`Delete "Review pull requests"?`), and `helpGroups()` swaps to a two-entry bar. New `selectedTask()` helper returns the focused task or nil.

**2. Monokai theme + Ember default.** Added Monokai (Accent is the cyan `#66d9ef`, not the pink, so it stays distinct from Danger — same reasoning that keeps Ember's Accent and Warning apart). The default is now Ember via a new `defaultThemeName` constant and `setDefaultTheme()`, used by both `init()` and `setThemeByName`'s fallback. Named rather than indexed so reordering `themes` can't silently change the default.

**3. Layouts.** New `internal/ui/layout.go` with three arrangements — `layoutTasksLeft` (previous behavior), `layoutTasksRight` (mirrored), `layoutStacked` (short full-width info row of Greeting/Reports/Pomodoro side by side, tasks filling the rest). Cycled with `L`, persisted in settings.

The important structural change is `geometry()`: **every size decision for a frame is now computed in one place**, and both `View()` and `visibleRowsFor()` read from it. Previously each duplicated the height math with a comment warning they must be kept in sync — with three layouts that duplication would have been a guaranteed source of off-by-one scroll bugs. `leftPaneWidth()` now delegates to it too, since the task panes span the full width in the stacked layout rather than a 3:5 column.

The inter-column gutter is now an explicit `gutterColumn()` of background-carrying spaces. It was previously implicit in `JoinHorizontal`'s padding, which would have been a full-height stripe of unstyled cells.

**4. Version badge.** `ui.Version` rendered at the right end of the banner's first row — on that row rather than its own line because the greeting pane's height budget is fixed. Only drawn when at least 2 cells of gap remain (roughly ≥110 columns); below that it's suppressed rather than crowding the wordmark.

**Bug found and fixed along the way:** `SaveSettings` was called with a freshly-built `model.Settings{Theme: name}`, so persisting the theme would have blanked the layout (and vice versa) — settings are written as a whole struct. Replaced with an `a.saveSettings()` method that always sends the complete current state. Also added a truncating branch to `View()`'s `padLines`, the last remaining pad-only-never-clamp site (a long enough task title in the delete prompt would have pushed the page past the terminal edge).

Verification: every rendered line equals the terminal width across **3 layouts × 6 themes × 131 widths × 4 heights**, with **0 unstyled background cells**. Delete flow tested at the model level: `d` leaves the count unchanged and shows the prompt, `n` and `esc` cancel, `y` deletes (6→5). Live tmux runs confirmed all three layouts, the delete prompt, theme cycling, and that theme+layout round-trip through `data/settings.json` together across a restart.

## Stacked layout: taller info row, tasks side by side (2026-08-21)

Follow-up to the layouts work. Two changes to `layoutStacked` only; the two column layouts are untouched.

**Tasks side by side.** Today and Overdue now sit in one row, each `a.width/2` wide (Overdue absorbs the odd column) and each getting the FULL task-row height rather than half of it.

**Info row sized to fit Reports.** Previously the row was sized to the greeting's fixed content, which left Reports too short for its heatmap and streak — the things the row mostly exists to show. Reports' full height is now an exported-in-package constant, `reportsFullContentLines = 8 + heatmapCost + 2` (three 2-line progress rows + two blanks, the heatmap block, blank + streak), and `heatmapCost` was lifted out of the function body so both can reference it. The stacked layout sizes its info row from that.

**The cap was the actual bug.** Setting the desired height wasn't enough: the existing `bodyHeight/2` cap silently clipped 21 rows down to 18 on a 40-row terminal, leaving Reports 3 lines short of the heatmap guard — so the first fix looked like it had failed. `/2` made sense when the task panes were stacked and needed half the screen; now that they're side by side they need far fewer rows, so the cap is a fixed `minTaskRows = 8` reservation instead. That lets the info row reach full size whenever the terminal can afford it.

Worth noting for future debugging: the symptom (no heatmap) had two plausible causes — insufficient width for the grid+legend, or insufficient height. Checking the width arithmetic first ruled it out (36 content cells was ample) and pointed straight at the cap, rather than guessing.

Verification: no layout overflows the terminal in either dimension across **3 layouts × 6 themes × 131 widths × 5 heights**, with **0 unstyled background cells**. A separate check confirms the stacked layout's Reports shows the heatmap at every width from 35 rows up, and degrades to progress-rows-plus-streak below that rather than clipping.

## Pomodoro redesign + centered greeting (2026-08-21)

**Pomodoro.** The countdown is the thing you glance at from across the room, so it now renders as 3-row block glyphs (`bigDigits`, same visual language as the TASKII banner) instead of a plain `25:00` that read as just another stat line. Everything is centered. Two other changes: the `(paused)`/`(running)` parenthetical became a status pip — dim for paused, phase-colored for running — and `Sessions completed: N` became `sessionDots()`, four filled/empty pips, since the cadence (which of 4 you're on) is what matters mid-session, not the lifetime total.

All ten digits plus the colon are hand-drawn. The colon's dots sit on the OUTER rows (`" ▄ ", "   ", " ▀ "`) rather than adjacent ones — the first version had both on rows 0-1 and read as top-heavy against the digits.

**Greeting centered.** Content is now centered both ways. Horizontal padding is split left/right and vertical padding above/below, both applied by hand with a background-carrying style rather than lipgloss's `Align`, for the usual reason (re-rendering pre-styled content leaves the added padding unbackgrounded). The version badge moved from the banner's right edge onto the date line (`Friday, August 21, 2026  ·  v1.0.0`) — pinning it right would have broken the centering it now sits inside.

**Regression this caused, and the fix.** Growing Pomodoro's content from 5 lines to 9 pushed the heatmap out of Reports in layout 1: Reports takes whatever height the other two panes leave, so a fixed-height Pomodoro spends the column's slack before Reports can see it. The heatmap's threshold moved from ~40 rows to 45. Caught by a check that greps the rendered view for `Activity (last` / `Current Streak` across sizes — not by eye.

Fixed by making Pomodoro yield instead of shrinking the timer back: `geometry()` now sizes it AFTER the greeting and caps it at the slack remaining once Reports' full height is reserved, down to a floor of `pomoMinContentLines` (= `pomoContentLines - 3`). `renderPomodoro` takes a height and sheds its three blank spacers from the end when short, so shrinking costs whitespace rather than information — verified at 100×32, where the spacers vanish but timer, bar and dots all survive. Layout 1 is back to showing the heatmap at 40 rows.

(At 100 columns the heatmap is still absent at every height: 36 cells are needed for a 3-week grid plus the 27-cell legend and only 35 are available. That's the pre-existing width guard, not this change — confirmed by arithmetic before concluding.)

Verification: **0 width overflows, 0 height overflows, 0 unstyled background cells** across 3 layouts × 6 themes × 131 widths × 5 heights. Separately confirmed all 10 digits render at a uniform 19 cells, and that the greeting's vertical centering is exactly balanced (equal blank rows above and below) at pane heights 6/8/10/14. Live tmux checks of the running timer, a phase change to Short Break, and the session dots filling.

## Pomodoro: centered, 6-pip full cycle, explicit run status (2026-08-21)

**Centered both ways.** Same hand-rolled approach as `renderGreeting` — horizontal padding split left/right by `centerLine`, vertical padding split above/below (renderPane pads only at the bottom, which left the block top-aligned in a taller pane). Visible in the stacked layout, where the Pomodoro pane is taller than its content.

**6 pips = the whole cycle.** The user asked for 25→5→25→5→25→15, which is a change to the CYCLE, not just the display: `longBreakEvery` went 4 → 3, and the pips now represent every *phase* of a cycle (3 work + 3 breaks) rather than just work sessions, so breaks are visible too. `cyclePosition()` maps (completed, phase) onto [0, 6).

Getting the glyphs right took a second pass. The first version used `●` for work and `◦` for break regardless of progress, with color alone marking completion — so at cycle start it rendered `◉ ◦ ● ◦ ● ◦`, where the untouched work pips looked identical to completed ones. Now the **glyph encodes progress** (done `●`/`◦`, current `◉`, pending `○`/`·`) and **size encodes kind** (large = work, small = break), so it reads correctly without relying on color: `◉ · ○ · ○ ·` at the start, filling left to right.

**Run status spelled out.** `PAUSED`/`RUNNING` on its own line under the timer. The previous design conveyed this only through the header pip's color, which is too quiet for the question this pane gets asked most — and invisible to anyone who can't distinguish the two hues.

**Regression caught and fixed (second time for this same mechanism).** Adding the status line took Pomodoro to 10 content lines and pushed Reports' heatmap from 40 rows back up to 45 in layout 1. Root cause was `pomoMinContentLines` being *derived* as `pomoContentLines - 3`, so every line added to Pomodoro silently raised its floor too, consuming slack Reports needs. Now stated as an explicit `= 6` (header, 3 digit rows, status, dots) with a comment explaining why it must not be derived. `renderPomodoro` gained a second degradation step to reach it: blank spacers first, then the **progress bar** — the big countdown already conveys progress, whereas phase/timer/status/dots each carry information nothing else shows.

Verified the ladder directly at heights 10→5: spacers shed from 10 down to 7, bar drops at 6, and phase/timer/status/dots survive at every step.

Verification: **0 width overflows, 0 height overflows, 0 unstyled cells, and the run status present in every render** across 3 layouts × 6 themes × 131 widths × 5 heights. Cycle logic traced over two full cycles confirming 25/5/25/5/25/15 and correct pip progression. Reports back to showing the heatmap at 40 rows in layout 1.

## Pomodoro digit glyph revisions (2026-08-21)

Reworked six of the ten block digits in `bigDigits` — 9, 8, 6, 5, 3, 2 — sample-by-sample with the user approving each before moving to the next. The originals mixed `▄` and `▀` inconsistently across digits, so shapes that should have shared a baseline (e.g. 2/3/5's bottom bar) didn't line up. The new set uses `▀▀▀` as a common bottom row on all six, giving the whole numeral set a consistent baseline. Digit widths are unchanged (3 cells each, 19 per "MM:SS"), so no layout math is affected.

## Pomodoro: session summary replaces cycle pips, status text removed (2026-08-21)

Two removals per user request, both net simplifications.

**Pips → session summary.** The six-pip cycle row is gone, replaced by a single line: `Sessions 0  ·  Short break next`. The running count is the durable number (the pips reset every cycle and lost it), and the break indicator is colored — muted for short, green for long — so the upcoming long break stands out. `cyclePosition()` and `sessionDots()` were deleted; `nextBreakIsLong()` and `sessionSummary()` replace them.

During a break the label drops the "next" framing and names the break actually in progress, since "Short break next" while sitting in that break would be wrong. Traced across a full cycle to confirm: count increments per completed focus session, "Long break next" appears on the third work session (the one that earns it), and break phases self-describe.

**Status text → pip only.** The `PAUSED`/`RUNNING` line is gone; the header pip's color carries run state alone (dim paused, phase-colored running). Noted for the record: I'd argued the opposite when adding it last turn — that a color-only cue is too quiet and inaccessible to anyone who can't distinguish the hues. That reasoning still holds in the abstract, but the user's call stands, and the pause key toggling it deliberately makes it far less of an issue in practice.

Heights: `pomoContentLines` 10 → 9, `pomoMinContentLines` 6 → 5 (header, 3 digit rows, summary). The comment on why the floor must stay an explicit count rather than being derived is retained — that's what caused the heatmap regression twice.

Verification: **0 width overflows, 0 height overflows, 0 unstyled cells, summary present in every render**, across 3 layouts × 6 themes × 131 widths × 5 heights. Explicitly asserted that layout 1 still shows the heatmap at 40 rows, confirming the two freed lines didn't perturb the Reports budget in the other direction.

## Pomodoro key hints moved into its own pane (2026-08-21)

The `p`/`r`/`n` controls left the global help bar and now render on the Pomodoro pane's last row, next to what they act on. The global bar dropped from three lines to two at common widths as a result, giving the body panes a row back.

Implementation notes:

- **Pinned, not centered.** The hints are appended AFTER the vertical centering, and the centering budget is `height-1`, so centring the timer block never drags the hints up into the middle of a tall pane. The shrink loop's budget is likewise `height-1`.
- **New `paneKeyStyle`/`paneKeyLabelStyle`** in theme.go: same look as the global bar's key styles but carrying `colorPaneBg` instead of `colorBg`. Reusing the page-level styles inside a pane would punch page background through the pane surface.
- **Narrow-width fallback.** Below ~37 cells the labels are dropped for bare `[p]  [r]  [n]` (13 cells) rather than truncating — a terse complete hint beats a clipped one. This is what the stacked layout's 38-wide pane gets.

Heights: `pomoContentLines` 9 → 10, `pomoMinContentLines` 5 → 6, since the hint row is never droppable.

Verification: shrink ladder checked directly at heights 12→5 — the hints are the last line at every height, and rendered widths stay uniform. Full sweep across 3 layouts × 6 themes × 131 widths × 5 heights: **0 width overflows, 0 height overflows, 0 unstyled cells, hints and session summary present in every render**, and layout 1 still shows the heatmap at 40 rows (confirming Pomodoro reclaiming a line didn't re-break the Reports budget).

## Notes section (2026-08-21)

A persistent bullet board. Named **Notes** rather than Whiteboard: "whiteboard" implies free-form 2D placement, which this isn't — it's a scrolling text buffer — and the shorter title matters on a border in the stacked layout's ~40-wide pane.

**Model.** `internal/model/note.go`, persisted to `data/notes.json`. `Body` may contain newlines and has no length limit, so nothing may assume one note is one line.

**Rendering** (`internal/ui/notes.go`). `wrapPlain` wraps on word boundaries (hard-breaking a single over-wide word), operating on PLAIN text so styling can be applied per-line afterwards and no escape sequence is ever split. `noteLines` flattens notes into display rows tagged with their note index, indenting continuation rows under the bullet so a wrapped note reads as one block.

**The key structural point: scrolling is in DISPLAY LINES, selection is in NOTES.** A wrapped note occupies several rows, so `syncScroll` resolves the selected note's line span (`noteLineSpan`) before comparing against the scroll offset — treating the two as the same unit scrolls to the wrong row the moment any note wraps. When a note is taller than the pane, its start is preferred over its end.

**Layout.** Under Pomodoro in the column layouts; a third task-row column in the stacked layout (Today/Overdue/Notes each a third, Notes absorbing the rounding remainder). Unlike Pomodoro, Notes does *not* fully yield to Reports — a requested section that only appears on very tall terminals isn't much of a section — so it takes a fixed share capped at a quarter of the info column, and Reports absorbs the cost. That's the right trade because `renderReports` already degrades gracefully (dropping heatmap, then streak), whereas a 1-row Notes pane is simply unusable; below `notesMinContentLines` it's omitted entirely rather than rendered too small.

**Editing.** Multi-line via bubbles `textarea`. Enter saves, esc cancels, and **shift+enter / alt+enter / ctrl+j** insert a newline. Shift+enter is what was asked for, but most terminals send an identical byte sequence for enter and shift+enter — it's genuinely indistinguishable without kitty-protocol support — so the other two are bound alongside it as portable fallbacks. An empty body deletes rather than storing a blank bullet.

**Widget restyling.** The textarea's styles run through `Inline(true)`, which strips backgrounds, and it pads with unstyled spaces — so its output cannot be made to carry `colorPaneBg` from the outside by styling alone (the scanner found 44k unstyled cells on the first attempt). Fixed by stripping the widget's ANSI and re-rendering each line as background-carrying spans. That also discards the cursor's inverse video, so the caret is redrawn from the model's own `Line()`/`ColumnOffset` instead.

**Focus plumbing.** `focusedPane` gained `focusNotes`, and the `currentSelected`/`setCurrentSelected`/`currentScroll`/`setCurrentScroll` helpers were converted from `if focusToday {...} else {...}` to explicit switches — the two-pane `else` would have silently made Notes behave as Overdue. Tab cycles all three (shift+tab reverses).

Keys when Notes is focused: `a` add, `enter` edit, `d` delete (confirmed), `C` clear board (confirmed).

Verification: **0 width overflows, 0 height overflows, 0 unstyled cells, Notes present at every size where it should be** — 3 layouts × 6 themes × 44 widths × 5 heights × 4 modes (normal / notes-focused / editing / clear-prompt). Live tmux runs confirmed wrapping, display-line scrolling, alt+enter newlines, edit-existing, the delete prompt naming the note's first line, and a multi-line note round-tripping through `data/notes.json` across a restart.

Note on test brittleness: an assertion checking for the accent color as a literal `48;2;122;162;247` failed even though rendering was correct — lipgloss emits `121;162;247` for `#7aa2f7`. Prefer structural assertions over exact color literals.

## Column layouts: Notes is now the flexible pane, Reports fixed (2026-08-21)

Inverted the height policy in layouts 1 and 2 per user request. Reports was taking `bodyHeight - everything else` (flexible); now Reports is fixed and Notes absorbs the remainder.

This is the right way round on the merits too: Reports' content is a bounded set of blocks, so height beyond what it renders is pure blank filler, whereas Notes is unbounded and every extra row shows another bullet.

**Reports snaps between two sizes rather than scaling.** The first attempt simply clamped Reports to `remaining - notesMin`, which produced the worst of both: at 44 rows Reports got 14 content lines — more than the 8 it needs for the progress rows, but less than the 19 the heatmap requires — so it rendered short and padded 5 blank rows while Notes sat at its 3-line minimum. Because its blocks are all-or-nothing (the heatmap needs 11 lines on top of the base rows, or nothing), Reports now takes either `reportsFullContentLines` or `reportsMinContentLines` and never anything between. Everything else goes to Notes.

Consequence worth stating: on mid-height terminals (~40-48 rows in layout 1) the heatmap now gives way to a taller Notes board, where before it was kept. That follows directly from making Notes flexible — the column can't fit both — and it reverses on taller terminals, where Reports gets its full height and Notes still takes the rest.

Verification adds a standing assertion that **Reports is never allocated more height than its content uses** (rendering it at the granted height and comparing line counts), which is what would catch this class of regression. Sweep across 3 layouts × 6 themes × 44 widths × 6 heights × 3 modes: **0 width overflows, 0 height overflows, 0 unstyled cells, 0 filler rows.**

## Fourth layout: Three Column (2026-08-21)

`layoutThreeColumn` — info | tasks | notes at 1/4 : 2/4 : 1/4, with two single-column gutters (tasks absorb the rounding remainder so the row sums to exactly `a.width`). Notes owns its column at full height; Today/Overdue split the middle column as in the two-column layouts.

**Reports is flexible again here.** In layouts 1/2 Reports is fixed and Notes takes the slack, because they share the info column. In this layout Notes has a column of its own and isn't competing, so there's no reason to pin Reports — it takes the info column's remainder after greeting and Pomodoro.

**Heatmap legend placement is now chosen from the width**, replacing the `withLegend bool` with a `legendPlacement` enum (`legendNone` / `legendInline` / `legendBelow`):

- **Inline** (on the Saturday row) costs ~27 cells of grid but no height — the right trade in a wide pane, where height is the scarcer resource. Used when at least 6 weeks still fit beside it.
- **Below** the grid costs one line but no width. In the quarter-width info column an inline legend would leave room for barely any history, so the legend moves under the chart — which is what the user asked for, and it's derived from width rather than special-cased on the layout, so any narrow Reports pane gets the same treatment.

Concretely: at a quarter of 160 columns the heatmap now shows a full **14 weeks** where an inline legend would have left about 3. Measured transition: legend below from 27–38 cells of content width, inline from 44 up.

`heatmapHeight(legend)` was added alongside `heatmapWidth(weeks, legend)` so the height guard accounts for the extra row, and the legend body was factored into `renderHeatmapLegend()` shared by both placements.

Verification: **0 width overflows, 0 height overflows, 0 unstyled cells, Notes present at every three-column size** — 4 layouts × 6 themes × 44 widths × 6 heights × 3 modes. Legend placement checked independently across 9 widths, confirming the rendered block always fits its pane. Live tmux confirmed the 1/4:2/4:1/4 split, the below-grid legend in the new layout, and the inline legend still used in layout 1.

## Three small fixes: notes/task key bleed, doubled confirmation, stale add hint (2026-08-21)

**1. `i` acted on Overdue while Notes was focused.** The reported symptom (marking the last overdue row important) was a surface of a deeper hazard: `currentList()` still had the old two-pane shape — `if focusToday { today } else { overdue }` — so with Notes focused it returned the OVERDUE list while `currentSelected()` returned `notesSelected`. Every task operation reachable from the notes board was therefore acting on an arbitrary overdue row, not just `i`.

Fixed at both levels: `currentList()` now switches explicitly and returns nil for `focusNotes`, and the `i` handler is guarded. The sibling helpers (`currentSelected`/`setCurrentSelected`/`currentScroll`/`setCurrentScroll`) were converted to explicit switches when Notes was added; `currentList` was missed. Worth remembering: an `else` branch standing in for "the other pane" is a latent bug the moment a third pane exists.

**2. Two confirmations at once.** The status line showed `[y] yes / [any other key] cancel` while the help bar simultaneously showed `[y] delete │ [n/esc] cancel` — same question, two styles, and the help-bar copy was also wrong (any key cancels, not just n/esc). Removed the help-bar branches entirely; the status-line prompt is now the single source, and its hint is built by `confirmHint()` from `helpKeyStyle`/`helpLabelStyle` so it carries the help bar's bracketed-key styling.

**3. `[a] add` advertised on the Overdue pane.** Adding is Today-only, so the hint is dropped from the Task group when Overdue has focus rather than advertising a dead key.

Verification: behavioural assertions for each (important-set unchanged after `i` in Notes; `currentList` empty there; exactly one `[y]` and no `[n/esc]` on screen during a confirm; `[a] add` present on Today and absent on Overdue), plus a full sweep across 4 layouts × 6 themes × 44 widths × 6 heights × **6 modes** (now including both confirm prompts and the clear prompt): **0 width overflows, 0 height overflows, 0 unstyled cells, 0 duplicated confirmations.**

## Confirmation prompts: help bar emptied, without the layout jumping (2026-08-21)

While a confirmation is up the help bar is now blank — every other binding is inert until the prompt is answered, so listing them advertised keys that do nothing. `helpGroups()` returns nil for `modeConfirmDelete`/`modeConfirmClearNotes`.

**That alone caused a worse bug than it fixed.** `chromeLines()` measures the help bar's actual rendered height, so blanking the bar handed those rows to the panes: opening a prompt made every pane grow by 2-3 lines and answering it shrank them back. The layout jumped underneath the question, and the visible task count changed mid-decision (`↓ 8 more` → `↓ 7 more`).

Two changes fix it:
- `chromeLines()` now measures against a copy of the model forced to `modeNormal`, so the help bar's footprint is reserved regardless of what the current mode displays.
- `View()` pads the rendered help bar back up to that reserved height with empty rows, or the page would come up short of the terminal and leave unpainted lines at the bottom. (`padLines` then fills them with the page background.)

The general rule this is an instance of: **anything that reserves space must be measured from a fixed reference, not from the current frame's content** — the same shape as the scroll-indicator line that's always emitted, and the pinned Pomodoro key row.

Verification adds a standing assertion that **`geometry()` is identical with and without a prompt open**, compared against a baseline captured in the same focus state. (A first version compared against a Today-focused baseline and reported 480 false jumps for the notes-focused cases — focus changes the help bar's height, so the baseline has to match.) Full sweep across 4 layouts × 6 themes × 44 widths × 6 heights: **0 width overflows, 0 height mismatches, 0 unstyled cells, 0 stale bindings during a confirm, 0 geometry jumps.**

## Notes: full-screen expand (2026-08-21)

`e` toggles the Notes board between its pane and the whole body area. Notes-only binding — it does nothing from the other panes.

**State.** `App.notesExpanded`, deliberately NOT persisted: it's a transient working state, not a preference like theme or layout. Toggling calls `syncScroll()`, since the viewport height changes and the selected note may fall outside it.

**Geometry.** `geometry()` short-circuits when expanded: Notes gets `a.width` x `bodyHeight`, every other pane keeps zero height. `View()` short-circuits to match, so the hidden panes aren't built at all rather than being rendered at zero size.

**Key gating.** A single `expandedAllowedKeys` set at the top of `updateNormal` filters everything that isn't a notes key or app-wide (`t`/`L`/`q`). Done as one gate rather than a check per handler, so a future binding can't silently leak in. `tab` is excluded — there's no other pane to focus — which also keeps focus pinned to Notes, the invariant `geometry()` and `View()` both test on.

The reason the other panes' keys must be dead: they'd act invisibly. Pressing `p` would start a Pomodoro that isn't on screen, `i` would flag a task in a hidden list.

**Refactor.** The tail of `View()` (status/prompt line, help bar, per-line width padding) became `assemblePage(body, helpLine)`, shared by the normal layouts and the expanded view — so prompts, the status line and the background padding behave identically in both instead of the expanded path re-implementing them.

Help bar while expanded lists the notes keys plus `[e] shrink`, drops `[tab] switch pane`, and keeps only the app-wide group.

Verification: behavioural assertions that `p`/`r`/`n`/`i`/`I`/`U`/`tab` all leave the model untouched while expanded (pomodoro not running, filters unset, focus and expansion unchanged), that only the Notes pane renders, that notes keys and `t` still work there, and that `e` restores. Full sweep across 4 layouts × 6 themes × 22 widths × 6 heights × 5 modes (including expanded + editing + confirm): **0 width overflows, 0 height mismatches, 0 unstyled cells, 0 leaks** of another pane or its key hints into an expanded frame.

## Layout 4: info column split into equal thirds (2026-08-21)

The three-column layout's first column now gives Greeting, Reports and Pomodoro exactly a third of the body height each, replacing the fixed-content-then-remainder scheme it inherited from the two-column layouts. Reports takes the ≤2-row rounding remainder, since it's the only one of the three whose content actually scales with height — Greeting and Pomodoro are fixed-size and just centre themselves in whatever they're given.

**Tradeoff worth stating:** capping Reports at a third costs it the heatmap on mid-size terminals. It previously appeared from ~46 rows in this layout; now it needs ~70, because Reports can no longer absorb the column's slack. That follows directly from equal thirds and is the requested behaviour, not a regression — but it's the reason the other three layouts were deliberately left alone.

Verification adds a standing assertion that in layout 4 `greetHeight == pomoHeight`, `reportsHeight` exceeds them by at most 2, and the three sum to exactly `bodyHeight`. Full sweep across 4 layouts × 6 themes × 22 widths × 6 heights × 3 modes: **0 width overflows, 0 height mismatches, 0 unstyled cells, 0 thirds violations.** The other layouts' geometry was separately confirmed unchanged.

## Simple mode (--simple) (2026-08-21)

`--simple` renders one full-screen view: the greeting in a fixed left column, a vertical divider, and beside it a single list merging today's tasks, overdue tasks and notes in `created_at` order.

**No data-structure change**, as required. `simpleEntry` is a purely view-level projection over the existing `[]model.Task` and `[]model.Note`; adds still write to `data/tasks.json` and `data/notes.json` with their existing schemas. Verified by round-tripping one of each and inspecting both files.

**Merge rule.** The task half is exactly the union of what the Today and Overdue panes show (today's items, plus anything still undone from an earlier day), so the list is the two panes plus Notes, not a third definition of "current".

**Selection vs scrolling.** Same distinction as the notes board: selection counts ENTRIES, the scroll offset counts DISPLAY LINES, since notes wrap and tasks don't. `simpleLines` tags each row with its entry index so a wrapped note highlights as a block.

**Tab switches the input type** rather than panes — `[a] add task` / `[a] add note` — since there are no panes to switch between. The other panes' bindings (pomodoro, filters, layout, focus) are simply absent from `updateSimple` rather than being gated off, because those panes don't exist in this mode at all.

**Refactor.** The inline note-editor block in `renderNotesPane` became `renderNoteEditor(width)`, shared with simple mode. It takes its background from `a.simple` — `colorBg` on the page surface here, `colorPaneBg` inside the Notes pane — since simple mode has no panes. Normal mode was separately swept afterwards to confirm the extraction changed nothing.

Note on `--mock`: mock notes have a zero `CreatedAt`, so they sort to the top of the simple list. That's the fixture, not the ordering — real notes carry a real timestamp and interleave correctly.

Verification: 6 themes × 36 widths × 5 heights × 4 modes (normal / adding task / adding note / confirm) — **0 width overflows, 0 height mismatches, 0 unstyled cells, 0 ordering violations** — plus an assertion that the merged list contains exactly `len(todayTasks) + len(overdueTasks)` tasks and `len(notes)` notes. Normal mode re-swept across 4 layouts × 6 themes × 22 widths × 3 heights × 3 modes: also clean.

## Simple mode: page side padding (2026-08-21)

A `simplePadding = 2` margin on each side of the simple-mode page. There's no pane border in this mode to hold content off the terminal edge, so the padding does that job.

**Taken out of the budget at the source.** New `simpleContentWidth()` = `a.width - 2*simplePadding`, and `simpleGreetingWidth`/`simpleListWidth` derive from it rather than from `a.width` — otherwise the margins would be spent twice and push the page past the terminal.

**Chrome is indented to match.** The status and help lines would otherwise sit flush against the edge while the body is inset. `indentLines(s, own)` subtracts the margin a line already carries (`renderHelpBar` prefixes one space) so body, status and help all land on the same column. The body is excluded — it carries its own margins already, and indenting it here would apply them twice.

**Two pad-only-never-clamp sites fixed**, both found by asserting the margin is present on every body row rather than by eye:
- `renderSimpleGreeting` padded short lines but not long ones. The date line ("Friday, August 21, 2026", 23 cells) is wider than the banner-derived column on a narrow terminal, so it overflowed, pushed the divider right and ate the right margin.
- The column assembly in `renderSimple` now clamps as well as pads, so a too-wide column can't shift the divider.

This is the same class as the `renderPane` and `padLines` fixes documented earlier: **anything that pads to a budget must also truncate to it**, or the budget isn't a budget.

Verification: 6 themes × 38 widths (from 50) × 5 heights × 4 modes — **0 width overflows, 0 height mismatches, 0 unstyled cells, 0 rows missing either margin**. Normal mode re-swept: also clean.

Test-boundary note: the padding assertion first reported 40 false failures on the delete-confirm line, because it assumed a 3-line footer while the help bar wraps to 2 at narrow widths. Use `chromeLines()` to find where the body ends, not a hardcoded count.

## Simple mode: note rows aligned with task rows (2026-08-21)

Note rows now share the task rows' gutter exactly: a 1-cell selection indicator, a space, a 3-cell marker field, a space — so `•` sits centred under the `[ ]`/`{ }` brackets and every body starts at the same column.

```
>  •  Standup moved to 10:15
> { } 09:30 ★ Review pull requests
> [ ] Write weekly status update
```

- `simpleGutter = 1+1+3+1` and `simpleNoteMarker = " • "` (padded to the checkbox's 3 cells) replace the old bare 2-cell `"• "` bullet.
- Notes now render the `>` selection indicator, which they previously lacked.
- Wrapped continuation rows keep the whole gutter blank, so they hang under the body rather than under the marker.
- The note branch also switched from `colorPaneBg` to `colorBg`: simple mode has no panes, so pane background was the wrong surface (it happened not to show as a gap because both are painted, but it was inconsistent with the rest of the page).

Verification adds an alignment assertion — for both row kinds, cols 1 and 5 are spaces and the marker occupies cols 2-4 — alongside the existing width/height/gap/padding checks. 6 themes × 38 widths × 5 heights × 3 modes: **all zero**. Normal mode re-swept: clean.

Test-brittleness note (second time this shape has bitten): the alignment assertion first reported 5 false failures because it indexed the plain string by BYTE while `•` is 3 bytes — the rows were correct all along. Index by rune when asserting on column positions.

## Simple mode: top padding, single indicator on wrapped notes, Option+Enter (2026-08-21)

**1. One-line top margin.** `simpleTopPadding = 1`, taken out of the height budget by a new `simpleBodyHeight()` rather than added on top — otherwise the page would overflow the terminal. Both the row budget (`simpleVisibleRows`) and the column padding now derive from that one helper, so they can't disagree about how tall the body is. The blank row spans the full width above everything, so the divider starts level with the content instead of running to the top edge.

**2. `>` repeated on every line of a wrapped note.** The marker was gated on `dl.first` but the selection indicator wasn't, so a 3-line note showed three indicators and read as three selected entries. Now both are.

**3. Option+Enter for a newline — already worked; verified rather than assumed.** macOS sends Option+Enter one of two ways depending on the terminal's "use Option as Meta" setting. Probed both with a throwaway bubbletea program:
- ESC-prefixed Enter → reported as `alt+enter`
- raw linefeed → reported as `ctrl+j`

Both were already bound, so the key needed no code change; confirmed end-to-end by driving both encodings into the editor and seeing two newlines. What did change is the labelling, which said "shift+enter" — misleading, since most terminals send shift+enter identically to plain Enter and can't distinguish them. Help text and placeholder now read `opt/alt+enter`, and the handler carries a comment recording both encodings.

Verification adds two standing assertions — the first rendered row is always blank, and a selected note produces exactly one `>` however many lines it wraps to — alongside the existing width/height/gap/side-padding checks. 6 themes × 38 widths × 5 heights × 3 modes: **all zero**.

## Simple mode: TASK/NOTE tab bar, and the newline-binding reality (2026-08-21)

**1. Mode indicator.** The task/note input mode was only discoverable from the help bar at the bottom of the screen. Now a two-tab strip sits above the list: the active tab is filled with the accent colour and marked `▸`, the inactive one muted. `simpleTabsHeight = 2` (row + rule) is reserved out of `simpleVisibleRows`.

**2. Newline bindings — what's actually possible.** The user asked whether shift+enter / alt+enter / ctrl+j would work, plus a custom combo for full coverage. The answer, worth recording because it constrains any future keybinding work:

- **ctrl+j** — real control character (0x0A). Universal.
- **ctrl+n** — added as a mnemonic alias. Universal, same reason.
- **alt/option+enter** — ESC-prefixed. Verified previously with a key probe: macOS Option+Enter arrives as `alt+enter` with Option-as-Meta on, and as `ctrl+j` with it off, so it works either way.
- **shift+enter** — *cannot* work in most terminals, and no binding fixes it. Legacy terminal encoding has no representation for modifier+Enter: Terminal.app sends bytes for shift+Enter that are byte-identical to plain Enter. It only reaches the app under the kitty keyboard protocol (kitty, WezTerm, Ghostty, foot) or iTerm2 with a manual mapping. Left bound for those terminals, but removed from the hints.

**A "custom combination" cannot add coverage** — the limit is what the terminal can encode, not what the app listens for. Any `<modifier>+enter` hits the same wall. Only ctrl+letter, alt+key, and plain keys reliably reach a TUI, so `ctrl+n` was the only kind of addition worth making.

Hints changed from `shift+enter / alt+enter` to `ctrl+j / opt+enter`: advertising a binding that silently does nothing on the user's likely terminal is worse than not mentioning it.

Verification: all three universal bindings confirmed to insert `\n` at the model level (value ends up `"ab\ncd"`), with Enter still saving. Sweep adds an assertion that both tabs render with exactly one active caret. 6 themes × 38 widths × 5 heights × 4 modes: **all zero**.

## Simple mode: stop using the pane background (2026-08-21)

User reported a background-coloured strip on the row right after the last list item. The cause: that row is the always-emitted scroll indicator, rendered through `hintStyle`, which carries `colorPaneBg` — the shade meant for the inside of a bordered pane. Simple mode has no panes, so against the page background it read as a stray highlight.

**Auditing rather than spot-fixing found it was systemic.** Counting distinct `48;2;r;g;b` codes in a rendered frame showed `PaneBg` appearing **77 times**, not once: every task row too, because `renderTaskLine` hardcoded `colorPaneBg`. Task rows and note rows were sitting on different shades throughout the list.

Fixed by making the surface explicit rather than assumed:
- `renderTaskLine` takes a `surface lipgloss.Color` parameter — `colorPaneBg` from the panes, `colorBg` from simple mode. Every segment is re-based onto it, since the shared styles (`taskStyle`, `doneStyle`, …) all bake in the pane shade.
- The scroll indicator, the `(nothing yet)` empty message, and the `+ ` input prompt are all re-based the same way.

Standing invariants added to the sweep: **`PaneBg` never appears in simple mode**, and **it always appears in normal mode** (guarding against over-correcting and stripping it from the panes).

Two test-methodology notes, both previously recorded and both re-encountered here:
- The invariant first reported failures on Dracula/Ember/Monokai because it compared emitted SGR values against the theme's hex literal exactly; lipgloss's colour conversion is off by one per channel (`#2d2f3d` → `44,47,60`). Compare with a ±1 tolerance.
- `tmux capture-pane -e` showed rows with no background codes at all, suggesting unpainted cells. A Go-level scan reported **zero** unstyled cells — the capture was dropping codes, as documented earlier in this file. Trust the `View()` bytes.

Verification: 6 themes × 26 widths × 5 heights × 4 modes — **0 width overflows, 0 height mismatches, 0 unstyled cells, 0 PaneBg in simple, 0 themes missing PaneBg in normal.**

## Reports rebuilt: count on the label, stacked daily bar chart, chart switcher (2026-08-21)

**Today's progress** now shows the count beside the label (`Today  2/6`) instead of a percentage after the bar. The count moved to the label line so the bar gets the full row width and the number you actually want reads first. The Week/Month progress rows were dropped — the bar chart supersedes them.

**New stacked bar chart** (`internal/ui/barchart.go`). One column per day; the bar's full height is that day's task count and the bottom portion, in green, is how many were completed. Half-block glyphs (`▄`) give each row two units of resolution. `stats.DayBar{Date, Done, Total}` and `dayBars()` are new; they include days with **no tasks** so the x-axis has no gaps.

x-axis labels are written into a plain character buffer positioned under their column and styled once at the end. Per-column padding truncated a two-digit day to `"."` when the column was 1-2 cells wide; letting a label overflow into the following (unlabelled) column is correct here.

**Chart switcher.** `Week / Month / Contribution`, with the heatmap moved under Contribution. Month thins its x-labels by width (every 2/3/5 days). Below 29 cells the tab labels fall back to initials — truncated mid-word, the selector stops being readable exactly where switching still matters.

**Reports is now a focusable pane.** `focusReports` joins the cycle (`focusCount = 4`); arrow/hjkl keys move between charts there instead of between rows, and the help bar switches to chart bindings plus the current chart's name. Item keys (`a`, `d`, `i`, space/enter) are explicitly inert on that pane.

While adding it, the `currentSelected`/`setCurrentSelected`/`currentScroll`/`setCurrentScroll` helpers were made fully explicit — their `default` branches meant "Overdue", so a fourth focus value would silently have operated on the Overdue list. Same latent bug that `currentList` had when Notes was added.

**Streak rule changed** to "at least one task completed that day", from "every task that day completed". Today is still exempt from breaking a streak earned through yesterday, but only adds to the count once something is actually done.

Verification: stats asserted directly against a hand-built fixture (7 week bars including empty days, `Done <= Total` everywhere, streak = 2 with a gap day, an empty today not breaking it, a partial day still counting). Bar rendering asserted to emit both the done and open colours. Sweep across 4 layouts × 6 themes × 17 widths × 5 heights × 3 charts × focused/unfocused: **0 width overflows, 0 height mismatches, 0 unstyled cells, 0 missing selectors, 0 item-key leaks**.

Test-methodology note, hit twice more here: comparing emitted SGR values against a theme's hex literal fails — lipgloss's conversion is off by one per channel (`#9ece6a` → `158;206;105`, not `106`). Both "failures" were the assertions, not the renderer. Always compare colours with a ±1 tolerance.

## Reports: streak removed, chart centred, percentage restored (2026-08-22)

**Streak removed** from the pane; `streakText` deleted with it. `stats.Report.Streak` and `computeStreak` are left in place — they're cheap, correct, and the natural home if the streak ever returns. The height constants were re-derived rather than decremented: `reportsHeaderLines = 2+1+1+1` (progress row, blank, selector, blank), with full = header + heatmap and min = header alone.

**Percentage** back beside today's bar (`█████░░░  33.3%`), carved out of the bar's own width so the row total is unchanged. The count stays on the label line.

**Chart centred** via `centerBlock`, which pads every line by the SAME amount rather than centring each independently — per-line centring would have broken the chart's internal alignment, floating bars off their axis labels.

**Bug this surfaced:** the legend vanished at every height. `renderBarChart` reserved 2 rows (axis + labels) but appended a third (the legend), returning `height+1` lines; `renderPane` truncated the overflow, eating the legend silently. It had been masked before because the streak line absorbed the extra row. Now the legend row is reserved up front, and a standing assertion checks `renderBarChart` never exceeds its budget — `lines == height` exactly at every size tested.

Also fixed: a month label that would run past the axis is now skipped rather than clipped, since "22" rendering as "2" reads as day 2.

Verification: 4 layouts × 6 themes × 17 widths × 5 heights × 3 charts — **0 width overflows, 0 height mismatches, 0 unstyled cells, 0 streak occurrences, 0 chart height overruns.**

## Heatmap legend moved below the chart (2026-08-22)

The contribution heatmap's "less ██ ██ ██ ██ ██ more" key now always sits on
its own row beneath the grid, centred under it.

- `legendInline` (the variant that rode the Saturday row) is **deleted**, along
  with the width-based choice between placements. It cost ~27 cells of grid
  width and made the key's position depend on the column, so the same chart
  read differently in different layouts. `legendPlacement` is now just
  `legendNone` / `legendBelow`.
- `heatmapWidth` no longer widens for an inline legend; it takes the max of the
  grid width and the key's own width, since the key can now be the widest line.
- **Height constants split.** `heatmapCost` used to mean both "the grid's line
  count" and "the whole block's cost" — fine while the legend was inline, wrong
  once it takes a row of its own. Now `heatmapGridLines = 7` (one row per
  weekday) and `heatmapCost = heatmapGridLines + 1`. Everything sizes off
  `reportsFullContentLines`/`reportsMinContentLines`, so this propagates to
  every layout automatically.
- The old `heatmapCost = 9` over-counted by two (it budgeted a blank + label row
  that `renderHeatmap` never emitted). That slack is what silently absorbed the
  new legend row; tightening the constant makes budget == actual exactly, so a
  future extra row fails loudly instead of being truncated away. Same bug class
  as the bar-chart legend last change.
- The legend is centred within the block before `centerBlock` shifts the whole
  block, since `centerBlock` moves every line by the same amount to keep bars
  over their axis labels.

Degradation is unchanged and intentional: the key is dropped when the column is
narrower than the key itself, or when the pane is one line short of holding
grid + legend. `renderReports` was verified to never exceed its budget at any
height.

Verified by sweep over 4 layouts x 6 themes x widths 70-200 x heights 24-50:
`BAD_WIDTH=0 BAD_HEIGHT=0 GAP=0 INLINE=0` (INLINE asserts the key never shares
a line with a weekday row).
