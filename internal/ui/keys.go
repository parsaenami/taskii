package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up             key.Binding
	Down           key.Binding
	Tab            key.Binding
	Add            key.Binding
	Toggle         key.Binding
	Delete         key.Binding
	Quit           key.Binding
	Enter          key.Binding
	Esc            key.Binding
	Important      key.Binding
	FilterImp      key.Binding
	FilterUndone   key.Binding
	PomodoroToggle key.Binding
	PomodoroReset  key.Binding
	PomodoroSkip   key.Binding
	Theme          key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch pane"),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add task"),
	),
	Toggle: key.NewBinding(
		key.WithKeys(" ", "enter"),
		key.WithHelp("space/enter", "toggle done"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
	),
	Important: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "toggle important"),
	),
	FilterImp: key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("I", "filter important"),
	),
	FilterUndone: key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "filter undone"),
	),
	PomodoroToggle: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "start/pause pomodoro"),
	),
	PomodoroReset: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reset phase"),
	),
	PomodoroSkip: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "skip phase"),
	),
	Theme: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "cycle theme"),
	),
}
