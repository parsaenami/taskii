# taskii

A fast, keyboard-driven task manager and dashboard for your terminal — built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Track today's tasks and appointments, see overdue items at a glance, run a Pomodoro timer, jot down notes, and review your progress with a GitHub-style contribution heatmap and weekly/monthly bar charts. Everything is saved locally as JSON — no account, no server, no telemetry.

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-blue)

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

Data is stored in `data/tasks.json`, `data/notes.json`, and `data/settings.json` in the current directory, created automatically on first run.

Flags:

```bash
taskii --mock     # launch with generated sample data instead of your real data
taskii --simple   # single-pane view: greeting + one combined list of tasks, overdue items, and notes
```

## Keybindings

| Key | Action |
| --- | --- |
| `↑/k`, `↓/j` | move selection |
| `tab` / `shift+tab` | switch focused pane |
| `a` | add task / note |
| `space` / `enter` | toggle done (tasks) or open note |
| `d` | delete (asks to confirm) |
| `i` | toggle important |
| `I` | filter: important only |
| `U` | filter: undone only |
| `e` | expand/collapse the notes board |
| `C` | clear all notes (asks to confirm) |
| `p` | start/pause Pomodoro |
| `r` | reset Pomodoro phase |
| `n` | skip Pomodoro phase |
| `t` | cycle color theme |
| `L` | cycle layout |
| `q` / `ctrl+c` | quit |

On the Reports pane, `↑/↓` (or `h/l`) switch between the Week, Month, and Contribution charts instead of moving a selection.

## Features

- **Today / Overdue** — add tasks or timed appointments (type a trailing `14:30` to mark one as an appointment), toggle done, mark important, delete.
- **Notes** — a simple multi-line notes board, expandable to full screen.
- **Pomodoro timer** — start, pause, reset, and skip work/break phases.
- **Reports** — completion progress, a 7-day/monthly bar chart, and a contribution heatmap.
- **Themes** — six built-in color themes (Tokyo Night, Dracula, Nord, Light, Ember, Monokai), cycled with `t` and persisted between runs.
- **Layouts** — multiple pane arrangements, cycled with `L`.

## Development

```bash
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
