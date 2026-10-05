// cmd/shellrecap/main.go
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/ksauraj/shellrecap/internal/models"
)

func main() {
	p := tea.NewProgram(models.InitialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
