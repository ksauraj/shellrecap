// cmd/shellrecap/main.go
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/models"
)

func main() {
	// Ask the terminal for its background color now, before the TUI takes
	// over the input, so the theme can pick its light or dark palette
	lipgloss.HasDarkBackground()

	p := tea.NewProgram(models.InitialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
