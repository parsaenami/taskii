package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"taskii/internal/model"
	"taskii/internal/ui"
)

func main() {
	mock := flag.Bool("mock", false, "run with generated sample data instead of loading/saving real data")
	simple := flag.Bool("simple", false, "run a single-pane view: greeting beside one combined list of tasks, overdue items and notes")
	version := flag.Bool("version", false, "print the version and exit")
	importDir := flag.String("import-data", "", "merge legacy data from a directory containing tasks.json, notes.json, or settings.json")
	exportDir := flag.String("export", "", "export tasks, notes, and settings into DIRECTORY/taski_data")
	flag.Parse()

	if *version {
		fmt.Println("taskii " + ui.Version)
		return
	}
	if *importDir != "" {
		report, err := model.ImportData(*importDir)
		printImportReport(report)
		if err != nil || len(report.Errors) > 0 {
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			os.Exit(1)
		}
		return
	}
	if *exportDir != "" {
		path, err := model.ExportData(*exportDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println("Exported Taskii data to", path)
		return
	}

	startupWarning := ""
	if !*mock {
		startupWarning = model.MigrateLegacyData().WarningText()
	}

	p := tea.NewProgram(ui.NewApp(ui.Options{Mock: *mock, Simple: *simple, StartupWarning: startupWarning}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printImportReport(r model.ImportReport) {
	fmt.Printf("tasks: added=%d duplicate=%d conflict=%d\n", r.TasksAdded, r.TasksDuplicate, r.TasksConflict)
	fmt.Printf("notes: added=%d duplicate=%d conflict=%d\n", r.NotesAdded, r.NotesDuplicate, r.NotesConflict)
	if r.SettingsResult == "" {
		r.SettingsResult = "not provided"
	}
	fmt.Println("settings:", r.SettingsResult)
	for _, err := range r.Errors {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
