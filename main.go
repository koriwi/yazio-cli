package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/koriwi/yazio-cli/internal/api"
	"github.com/koriwi/yazio-cli/internal/auth"
	"github.com/koriwi/yazio-cli/tui"
)

func main() {
	refresh := flag.Bool("refresh", false, "exchange the stored refresh token for a new access token")
	flag.Parse()

	if *refresh {
		cfg, err := auth.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
			os.Exit(1)
		}
		if cfg.RefreshToken == "" {
			fmt.Fprintf(os.Stderr, "no refresh token stored — log in through the app first\n")
			os.Exit(1)
		}
		c := api.New("")
		resp, err := c.RefreshAccessToken(cfg.RefreshToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "refresh failed: %v\n", err)
			os.Exit(1)
		}
		// Prefer the new refresh token; fall back to the old one if the server didn't rotate it.
		newRefresh := resp.RefreshToken
		if newRefresh == "" {
			newRefresh = cfg.RefreshToken
		}
		if err := auth.SaveToken(cfg.Email, resp.AccessToken, newRefresh); err != nil {
			fmt.Fprintf(os.Stderr, "failed to save token: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("token refreshed")
		return
	}

	cfg, _ := auth.LoadConfig()

	p := tea.NewProgram(
		tui.New(cfg.Token != "", cfg.Token, cfg.Email, cfg.RefreshToken),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
