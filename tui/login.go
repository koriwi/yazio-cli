package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/koriwi/yazio-cli/internal/api"
	"github.com/koriwi/yazio-cli/internal/auth"
)

type loginModel struct {
	email    textinput.Model
	password textinput.Model
	focused  int // 0=email, 1=password
	err      string
	loading  bool
	width    int
	height   int
}

type loginSuccessMsg struct {
	token        string
	email        string
	refreshToken string
}
type loginErrMsg struct{ err string }

func newLoginModel() loginModel {
	email := textinput.New()
	email.Placeholder = "you@example.com"
	email.Focus()
	email.CharLimit = 128
	email.Width = 40

	pass := textinput.New()
	pass.Placeholder = "password"
	pass.EchoMode = textinput.EchoPassword
	pass.EchoCharacter = '•'
	pass.CharLimit = 128
	pass.Width = 40

	return loginModel{email: email, password: pass, focused: 0}
}

func doLogin(email, password string) tea.Cmd {
	return func() tea.Msg {
		c := api.New("")
		resp, err := c.Login(email, password)
		if err != nil {
			return loginErrMsg{err: err.Error()}
		}
		if err := auth.SaveToken(email, resp.AccessToken, resp.RefreshToken); err != nil {
			return loginErrMsg{err: "saved token but failed to write config: " + err.Error()}
		}
		return loginSuccessMsg{token: resp.AccessToken, email: email, refreshToken: resp.RefreshToken}
	}
}

func (m loginModel) Update(msg tea.Msg) (loginModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if m.loading {
			break
		}
		switch msg.String() {
		case "tab", "down":
			m.focused = (m.focused + 1) % 2
			if m.focused == 0 {
				m.email.Focus()
				m.password.Blur()
			} else {
				m.password.Focus()
				m.email.Blur()
			}
		case "shift+tab", "up":
			m.focused = (m.focused + 1) % 2
			if m.focused == 0 {
				m.email.Focus()
				m.password.Blur()
			} else {
				m.password.Focus()
				m.email.Blur()
			}
		case "enter":
			if m.focused == 0 {
				m.focused = 1
				m.password.Focus()
				m.email.Blur()
			} else {
				if m.email.Value() == "" || m.password.Value() == "" {
					m.err = "Email and password are required"
					break
				}
				m.loading = true
				m.err = ""
				cmds = append(cmds, doLogin(m.email.Value(), m.password.Value()))
			}
		}
	}

	var cmd tea.Cmd
	m.email, cmd = m.email.Update(msg)
	cmds = append(cmds, cmd)
	m.password, cmd = m.password.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m loginModel) View() string {
	logo := styleHeader.Render("YAZIO CLI")

	subtitle := styleDimmed.Render("Track your meals from the terminal")

	emailLabel := "Email"
	passLabel := "Password"
	if m.focused == 0 {
		emailLabel = styleSelected.Render("> " + emailLabel)
	} else {
		emailLabel = "  " + emailLabel
	}
	if m.focused == 1 {
		passLabel = styleSelected.Render("> " + passLabel)
	} else {
		passLabel = "  " + passLabel
	}

	form := lipgloss.JoinVertical(lipgloss.Left,
		emailLabel,
		styleInput.Width(m.email.Width).Render(m.email.View()),
		"",
		passLabel,
		styleInput.Width(m.password.Width).Render(m.password.View()),
	)

	var status string
	if m.loading {
		status = styleDimmed.Render("Logging in...")
	} else if m.err != "" {
		status = styleError.Render("✗ "+m.err)
	} else {
		status = styleDimmed.Render("Press Enter to login • Tab to switch fields")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		logo,
		subtitle,
		"",
		form,
		"",
		status,
	)

	box := styleBorder.Render(content)

	if m.width > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return fmt.Sprintf("\n%s\n", strings.TrimSpace(box))
}
