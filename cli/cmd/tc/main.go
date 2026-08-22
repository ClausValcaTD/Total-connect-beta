package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
    "github.com/ClausValcaTD/Total-connect-beta/cli/internal/tui"
)

func main() {
	// FIX #1 + #2: NewClient now exists and returns *Model
	client, err := tui.NewClient(":50051")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize TUI: %v\n", err)
		os.Exit(1)
	}

	// NewModel unwraps *Model into the value tea.NewProgram expects
	p := tea.NewProgram(
		tui.NewModel(client),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
