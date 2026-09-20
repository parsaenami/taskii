package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/parsaenami/taskii/internal/model"
	"github.com/parsaenami/taskii/internal/ui"
)

func main() {
	setUsage(flag.CommandLine)
	mock, simple, version, importDir, exportDir := registerFlags(flag.CommandLine)
	flag.CommandLine.Parse(os.Args[1:])

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

func registerFlags(fs *flag.FlagSet) (mock, simple, version *bool, importDir, exportDir *string) {
	mock = fs.Bool("mock", false, "run with generated sample data instead of loading/saving real data")
	simple = fs.Bool("simple", false, "run a single-pane view: greeting beside one combined list of tasks, overdue items and notes")
	version = fs.Bool("version", false, "print the version and exit")
	importDir = fs.String("import-data", "", "merge tasks.json, notes.json, routines.json, and settings.json from DIRECTORY into Taskii's data")
	exportDir = fs.String("export", "", "write tasks.json, notes.json, routines.json, and settings.json to DIRECTORY/taski_data")
	return
}

func setUsage(fs *flag.FlagSet) {
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintln(out, "Usage: taskii [options]")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Taskii is a terminal task manager.")
		fmt.Fprintln(out, "Import and export commands run without launching the dashboard.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Options:")
		fs.PrintDefaults()
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Examples:")
		fmt.Fprintln(out, "  taskii --import-data /path/to/data")
		fmt.Fprintln(out, "  taskii --export /path/to/backup")
	}
}

func printImportReport(r model.ImportReport) {
	fmt.Printf("tasks: added=%d duplicate=%d conflict=%d\n", r.TasksAdded, r.TasksDuplicate, r.TasksConflict)
	fmt.Printf("notes: added=%d duplicate=%d conflict=%d\n", r.NotesAdded, r.NotesDuplicate, r.NotesConflict)
	fmt.Printf("routines: added=%d duplicate=%d conflict=%d\n", r.RoutinesAdded, r.RoutinesDuplicate, r.RoutinesConflict)
	if r.SettingsResult == "" {
		r.SettingsResult = "not provided"
	}
	fmt.Println("settings:", r.SettingsResult)
	for _, err := range r.Errors {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
