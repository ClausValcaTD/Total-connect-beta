package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ClausValcaTD/Total-connect-beta/cli/internal/tui"
)

func main() {
	// Connect to gRPC server
	client, err := tui.NewClient(":50051")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to gRPC server: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure tc-server is running on :50051\n")
		os.Exit(1)
	}

	// Launch TUI
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
