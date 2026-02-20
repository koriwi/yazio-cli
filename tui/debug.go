package tui

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/koriwi/yazio-cli/internal/api"
	"github.com/koriwi/yazio-cli/internal/auth"
)

type debugEndpoint struct {
	label  string
	path   string
	status int
	body   string
	err    string
}

type userProfile struct {
	UUID      string `json:"uuid"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Country   string `json:"country"`
	Sex       string `json:"sex"`
}

type debugModel struct {
	client     *api.Client
	email      string
	token      string
	configPath string
	jwtClaims  string
	profile    *userProfile
	endpoints  []debugEndpoint
	loading    bool
	scrollY    int
	width      int
	height     int
}

type debugLoadedMsg struct {
	profile   *userProfile
	endpoints []debugEndpoint
}

func newDebugModel(client *api.Client, token string) debugModel {
	cfg, _ := auth.LoadConfig()
	return debugModel{
		client:     client,
		email:      cfg.Email,
		token:      token,
		configPath: auth.ConfigFilePath(),
		jwtClaims:  decodeJWT(token),
		loading:    true,
	}
}

func (m debugModel) load() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		today := time.Now().Format(time.DateOnly)

		// Fetch profile first so we can use country/sex in search probe
		var profile *userProfile
		if raw, _, err2 := client.GetRaw("/v9/user"); err2 == nil {
			var p userProfile
			if json.Unmarshal([]byte(raw), &p) == nil {
				profile = &p
			}
		}

		// Fetch first product from today to probe product nutrient field names
		productProbe := ""
		if raw, _, err2 := client.GetRaw(fmt.Sprintf("/v9/user/consumed-items?date=%s", today)); err2 == nil {
			var cr struct {
				Products []struct {
					ProductID string `json:"product_id"`
				} `json:"products"`
			}
			if json.Unmarshal([]byte(raw), &cr) == nil && len(cr.Products) > 0 {
				productProbe = "/v9/products/" + cr.Products[0].ProductID
			}
		}

		country, sex := "DE", "male"
		if profile != nil {
			if profile.Country != "" {
				country = profile.Country
			}
			if profile.Sex != "" {
				sex = profile.Sex
			}
		}

		paths := []string{
			"/v9/user",
			fmt.Sprintf("/v9/user/consumed-items?date=%s", today),
			fmt.Sprintf("/v9/user/consumed-items/nutrients-daily?start=%s&end=%s", today, today),
			fmt.Sprintf("/v9/user/goals?date=%s", today),
		}
		if productProbe != "" {
			paths = append(paths, productProbe)
		}
		paths = append(paths, fmt.Sprintf(
			"/v9/products/search?query=chicken&language=en&countries=%s&sex=%s", country, sex,
		))

		var results []debugEndpoint

		for _, path := range paths {
			body, status, err := client.GetRaw(path)
			e := debugEndpoint{label: path, path: path, status: status}
			if err != nil {
				e.err = err.Error()
			} else {
				e.body = prettyJSON(body)
			}
			results = append(results, e)
		}
		return debugLoadedMsg{profile: profile, endpoints: results}
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

	case debugLoadedMsg:
		m.loading = false
		m.endpoints = msg.endpoints
		m.profile = msg.profile
	}
	return m, nil
}

func (m debugModel) View() string {
	var lines []string

	title := styleHeader.Render("Debug Info")
	lines = append(lines, title, "")

	// Account section
	lines = append(lines, sectionLabel("Account"))
	lines = append(lines, row("Email", m.email))
	if m.profile != nil {
		lines = append(lines, row("Name", m.profile.FirstName+" "+m.profile.LastName))
		lines = append(lines, row("UUID", m.profile.UUID))
	}
	tokenPreview := ""
	if len(m.token) > 16 {
		tokenPreview = m.token[:8] + "..." + m.token[len(m.token)-8:]
	} else {
		tokenPreview = m.token
	}
	lines = append(lines, row("Token", tokenPreview))
	lines = append(lines, row("Config file", m.configPath))
	lines = append(lines, "")

	// JWT claims
	if m.jwtClaims != "" {
		lines = append(lines, sectionLabel("JWT Claims"))
		for _, l := range strings.Split(m.jwtClaims, "\n") {
			lines = append(lines, "  "+styleDimmed.Render(l))
		}
		lines = append(lines, "")
	}

	// API probe results
	lines = append(lines, sectionLabel("API Endpoint Probe"))
	if m.loading {
		lines = append(lines, styleDimmed.Render("  Probing endpoints..."))
	} else {
		for _, ep := range m.endpoints {
			statusStr := fmt.Sprintf("HTTP %d", ep.status)
			var statusStyle lipgloss.Style
			if ep.err != "" || ep.status >= 400 {
				statusStyle = styleError
			} else {
				statusStyle = lipgloss.NewStyle().Foreground(colorSuccess)
			}
			lines = append(lines, fmt.Sprintf("  %s  %s",
				styleItemName.Render(ep.label),
				statusStyle.Render(statusStr),
			))
			if ep.err != "" {
				lines = append(lines, styleDimmed.Render("    "+ep.err))
			} else if ep.body != "" {
				for _, bl := range strings.Split(ep.body, "\n") {
					lines = append(lines, styleDimmed.Render("    "+bl))
				}
			}
			lines = append(lines, "")
		}
	}

	lines = append(lines, styleHelp.Render("[↑/↓] scroll  [g] top  [Esc] back"))

	// Apply scroll
	visible := lines
	if m.height > 4 && m.scrollY > 0 {
		if m.scrollY < len(lines) {
			visible = lines[m.scrollY:]
		} else {
			visible = lines[len(lines)-1:]
		}
	}
	// Clip to terminal height
	if m.height > 2 && len(visible) > m.height-1 {
		visible = visible[:m.height-1]
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

func prettyJSON(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return s
	}
	return string(b)
}

// decodeJWT tries to extract the payload from a JWT token.
// Returns an empty string if the token is not a valid JWT.
func decodeJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	// JWT uses base64url without padding
	payload := parts[1]
	// Add padding if needed
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// Try RawURLEncoding (no padding)
		data, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	return prettyJSON(string(data))
}
