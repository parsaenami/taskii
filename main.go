package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"taskii/internal/ui"
)

func main() {
	mock := flag.Bool("mock", false, "run with generated sample data instead of loading/saving data/tasks.json")
	simple := flag.Bool("simple", false, "run a single-pane view: greeting beside one combined list of tasks, overdue items and notes")
	version := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *version {
		fmt.Println("taskii " + ui.Version)
		return
	}

	p := tea.NewProgram(ui.NewApp(ui.Options{Mock: *mock, Simple: *simple}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
