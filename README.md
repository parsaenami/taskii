<img src="docs/images/logo.png" alt="taskii logo" width="72">

# taskii

A fast, keyboard-driven task manager and dashboard for your terminal — built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Track today's tasks and appointments, see overdue items at a glance, run a Pomodoro timer, jot down notes, and review your progress with a GitHub-style contribution heatmap and weekly/monthly bar charts. Everything is saved locally as JSON — no account, no server, no telemetry.

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-blue)

<table>
<tr>
<td width="50%"><img src="docs/images/screenshot-normal.png" alt="taskii normal mode"><p align="center"><em>Normal mode — Today, Overdue, Reports, Pomodoro, and Notes panes</em></p></td>
<td width="50%"><img src="docs/images/screenshot-simple.png" alt="taskii simple mode"><p align="center"><em>Simple mode — one combined list of tasks, appointments, and notes</em></p></td>
</tr>
</table>

## Install

**Homebrew** (macOS/Linux):

```bash
brew install parsaenami/tap/taskii
```

**Install with curl** (macOS/Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/parsaenami/taskii/main/install.sh | sh
```

**Download a binary** from the [releases page](https://github.com/parsaenami/taskii/releases) (macOS, Linux, Windows — amd64/arm64), then put it on your `PATH`.

**Or build from source** (requires [Go](https://go.dev) 1.26+):

```bash
go install github.com/parsaenami/taskii@latest
```

**Or clone and run:**

```bash
git clone https://github.com/parsaenami/taskii.git
cd taskii
go run .
```

## Run

```bash
taskii
```

Data is stored automatically in the standard XDG locations, independent of the
current working directory:

- Tasks, notes, and routines: `$XDG_DATA_HOME/taskii/tasks.json`,
  `$XDG_DATA_HOME/taskii/notes.json`, and `$XDG_DATA_HOME/taskii/routines.json`
- Settings: `$XDG_CONFIG_HOME/taskii/settings.json`

When the XDG environment variables are unset, the platform defaults are used
(for example, `~/.local/share` and `~/.config` on Linux). On normal startup,
taskii automatically validates and copies missing legacy files from the current
working directory's `data/` directory. Existing XDG files win and sources are
never changed. Use `--import-data` for another legacy directory.

Flags:

```bash
taskii --mock     # launch with generated sample data instead of your real data
taskii --simple   # single-pane view: greeting + one combined list of tasks, overdue items, and notes
taskii --import-data /path/to/data  # merge tasks, notes, routines, and settings
taskii --export /path/to/export-directory  # create taski_data/ with all saved data
```

The import command processes each recognized JSON file independently. Source-
only task, note, and routine IDs are added; identical IDs are duplicates, and conflicting
IDs are skipped in favor of existing XDG records. Settings import only when no
XDG settings file exists. Errors are reported and the command exits nonzero.

The export command creates `/path/to/export-directory/taski_data/` containing
`tasks.json`, `notes.json`, `routines.json`, and `settings.json`. It does not overwrite an
existing `taski_data` directory.

Settings > Calendar controls which weekdays count as workdays for workday
routines and which day begins calendar-week reports. Defaults are Monday-Friday
workdays and a Monday week start. Calendar edits are applied only when Settings
is saved; changing workdays first settles outstanding routine days against the
previous workday definition, while changing week start never rewrites history.

## Keybindings

Bindings are contextual: actions that do not apply to the focused pane are
inactive. Press `?` at any time for the complete in-app shortcuts reference.

| Key | Action |
| --- | --- |
| `↑/↓` or `j/k` | Move or scroll the current selection |
| `tab` / `shift+tab` | Switch focused pane in normal mode; `tab` switches Task/Note input in `--simple` mode |
| `a` | Add a task from Today, or add a note from Notes |
| `space` | Toggle a Today task/routine; move an Overdue task to Today |
| `enter` | Edit a selected task/note; toggle a selected Today routine |
| `s` | Skip a selected Today routine; restore a skipped routine in the manager |
| `d` | Delete the selected task or note after confirmation |
| `i` | Toggle important on the selected task |
| `I` / `U` | Toggle the important-only / undone-only task filters |
| `C` | Switch Today/Upcoming; clear Notes when Notes is focused (with confirmation) |
| `e` | Expand/shrink Notes; edit a routine in the routine manager |
| `R` | Open the routine manager, including in `--simple` mode |
| `p` / `r` / `n` | Pause/resume, reset, or skip the Pomodoro phase |
| `S` | Open Settings for themes, layouts, Pomodoro, and calendar preferences |
| `?` | Open/close the categorized shortcuts reference |
| `q` / `ctrl+c` | Quit |

On the Reports pane, `←/→` (or `h/l`) switch between the **7 Days**,
**Month**, **Contribution**, and **Routines** charts. On Routines, `↑/↓`
(or `j/k`) scroll the per-routine rows while the weekday columns stay fixed.

## Routines

Press `R` to open the routine manager. `a` creates a routine, `enter` or `e`
edits the selected one, and `d` deletes its definition **and history** after
confirmation. The manager shows today's state, including skipped routines;
select a skipped routine and press `s` to restore it for today. `esc` closes it.

In the editor, type a title, use `tab`/`↑`/`↓` to move between fields, and
`←`/`→` to choose **every day**, **workdays**, or **custom**. For a custom
schedule, select days with `space` in your configured week-start order. Press
`ctrl+s` to save or `esc` to cancel. A title and at least one custom day are
required. Workdays and the week start are configured in Settings > Calendar.

Today's scheduled routines appear above tasks (and above the combined list in
`--simple`). Select one and press `space` or `enter` to toggle its completion,
or `s` to skip it for today. Skipped routines disappear until restored from the
manager; completed ones remain visible. Routines never enter Overdue or
Upcoming and are unaffected by task filters. Past scheduled days with no
decision become missed on launch or after midnight. Editing a schedule settles
previous days using the old schedule before applying the new one to today and
future dates. Mock mode demonstrates all three schedule types without saving.

The Reports pane's **Routines** chart uses the configured calendar week. It
shows weekly done/skipped/missed totals, follow-through (completed divided by
completed plus missed), and a seven-day status row for every routine.

## Scheduled tasks

Press `a` in the task pane and append a date, a time, or both:

| Input | Result |
| --- | --- |
| `Read a chapter` | Task for today |
| `Call Alex 14:30` | Appointment for today at 14:30 |
| `Read a chapter 09-10` | Task for September 10 |
| `Call Alex 09-10 14:30` | Appointment for September 10 at 14:30 |
| `Submit report !5d` | Task with a deadline five days from today |
| `Call Alex 09-10 14:30 !1d` | Scheduled appointment due tomorrow |

Dates use `MM-DD` and times use `HH:MM` (24-hour clock). The date is the next
occurrence of that month and day, including today. A date that has already
passed rolls forward to next year; `02-29` selects the next valid leap day.
Impossible dates, such as `02-30`, show an error and keep the input for correction.
Date and time tokens must be at the end of the title, in that order when both
are present. A trailing `MM-DD` token is interpreted as a schedule date.

Append `!Nd` to set a deadline `N` calendar days from today: `!0d` is today,
`!1d` is tomorrow, and `!5d` is five days away. The annotation is removed from
the task title and stored as an absolute date, so it counts down naturally.
Deadline tasks remain in Today until completed and show a right-aligned phrase
such as `today`, `tomorrow`, `in 5 days`, `yesterday`, or `5 days ago`.

`Shift+C` opens **Upcoming**, sorted by date and then appointment time, with
untimed tasks after appointments on the same day. Each row shows its full date.
Press it again to return to Today (or the current combined list in `--simple`).
On the Notes pane, `Shift+C` retains its existing clear-board action.
Adding a task switches to the view containing its date.

Scheduled tasks automatically appear in Today on their date. Unfinished tasks
from earlier dates appear in Overdue using the existing behavior. These lists
refresh while the app is running; no restart or rescheduling is needed.
The saved JSON format is unchanged: the scheduled day is stored in `Task.Date`.

## Features

- **Today / Overdue** — add tasks or timed appointments, set deadlines with a trailing `!Nd`, toggle done, mark important, delete.
- **Routines** — recurring every-day, workday, or custom-weekday activities with daily completion, skip, and missed history.
- **Upcoming** — schedule tasks with a trailing `MM-DD`, optionally followed by `HH:MM`, and browse future dates with `Shift+C`.
- **Notes** — a simple multi-line notes board, expandable to full screen.
- **Pomodoro timer** — start, pause, reset, and skip work/break phases.
- **Reports** — task completion with 7-day/monthly bars and a contribution heatmap, plus a configurable-week routine matrix and follow-through summary.
- **Themes** — 7 curated core themes, 340+ searchable terminal themes, and extensible custom JSON/JSONL theme support, all available from Settings (`S`).
- **Layouts** — multiple pane arrangements selected and previewed from Settings (`S`).

## Custom Themes

You can add your own custom palettes by placing `.json` or `.jsonl` files in any of the following directories:
- **Project local**: `data/themes/`
- **Linux**: `$XDG_CONFIG_HOME/taskii/themes/` or `~/.config/taskii/themes/`
- **macOS**: `~/Library/Application Support/taskii/themes/` or `~/.config/taskii/themes/`
- **Windows**: `%AppData%\taskii\themes\`

### Example: Taskii Theme Format (`theme.json`)
```json
{
  "name": "Custom Palette",
  "bg": "#121815",
  "pane_bg": "#18201c",
  "panel": "#202a25",
  "border": "#31423a",
  "border_focus": "#52b788",
  "text": "#d8f3dc",
  "muted": "#74c69d",
  "accent": "#52b788",
  "green": "#74c69d",
  "warning": "#ffe6a7",
  "danger": "#e63946",
  "purple": "#b5838d"
}
```

### Example: Terminal / Bubbletint Tint Format (`tint.json`)
```json
{
  "display_name": "My Terminal Tint",
  "id": "my_tint",
  "dark": true,
  "bg": "#1a1b26",
  "fg": "#c0caf5",
  "red": "#f7768e",
  "green": "#9ece6a",
  "yellow": "#e0af68",
  "blue": "#7aa2f7"
}
```

*(You can also bundle multiple themes into a single JSON array or a `.jsonl` file with one JSON theme object per line).*

## Development

```bash
go test ./...    # tests
go build ./...   # build
go vet ./...     # static checks
gofmt -l .       # formatting check
```

## Support

If taskii is useful to you, consider supporting its development:

- Inside Iran: [donito.me/parsaenami](https://donito.me/parsaenami)
- Outside Iran: [buymeacoffee.com/parsaenami](https://buymeacoffee.com/parsaenami)

## License

[MIT](LICENSE)
