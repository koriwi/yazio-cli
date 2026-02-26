package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/koriwi/yazio-cli/internal/auth"
)

type debugModel struct {
	email      string
	token      string
	configPath string
	scrollY    int
	width      int
	height     int
}

func newDebugModel(token string) debugModel {
	cfg, _ := auth.LoadConfig()
	return debugModel{
		email:      cfg.Email,
		token:      token,
		configPath: auth.ConfigFilePath(),
	}
}

func (m debugModel) Update(msg tea.Msg) (debugModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return backToDiaryMsg{} }
		case "j", "down":
			m.scrollY++
		case "k", "up":
			if m.scrollY > 0 {
				m.scrollY--
			}
		case "g":
			m.scrollY = 0
		}
	}
	return m, nil
}

func (m debugModel) View() string {
	var lines []string

	lines = append(lines, styleHeader.Render("Debug Info"), "")

	lines = append(lines, sectionLabel("Account"))
	lines = append(lines, row("Email", m.email))
	tokenPreview := m.token
	if len(m.token) > 16 {
		tokenPreview = m.token[:8] + "…" + m.token[len(m.token)-8:]
	}
	lines = append(lines, row("Token", tokenPreview))
	lines = append(lines, row("Config file", m.configPath))
	lines = append(lines, "")

	lines = append(lines, styleHelp.Render("[↑/↓] scroll  [g] top  [Esc] back"))

	return applyScroll(lines, m.scrollY, m.height)
}

func applyScroll(lines []string, scrollY, height int) string {
	visible := lines
	if scrollY > 0 {
		if scrollY < len(lines) {
			visible = lines[scrollY:]
		} else {
			visible = lines[len(lines)-1:]
		}
	}
	if height > 2 && len(visible) > height-1 {
		visible = visible[:height-1]
	}
	return strings.Join(visible, "\n")
}

func sectionLabel(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("── " + s + " ──")
}

func row(label, value string) string {
	l := lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("  %-16s", label))
	v := styleItemName.Render(value)
	return l + v
}
