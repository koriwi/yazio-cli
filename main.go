package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/koriwi/yazio-cli/internal/auth"
	"github.com/koriwi/yazio-cli/tui"
)

func main() {
	token, _ := auth.LoadToken()

	p := tea.NewProgram(
		tui.New(token != "", token),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
