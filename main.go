package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"terminal-dashboard/internal/ui"
)

func main() {
	mock := flag.Bool("mock", false, "run with generated sample data instead of loading/saving data/tasks.json")
	flag.Parse()

	p := tea.NewProgram(ui.NewApp(ui.Options{Mock: *mock}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
