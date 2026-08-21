package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	style := lipgloss.NewStyle().Bold(true)
	fmt.Println(style.Render("Total Connect CLI"))
	_ = tea.NewProgram(nil)
}
