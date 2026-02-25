package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/koriwi/yazio-cli/internal/api"
	"github.com/koriwi/yazio-cli/internal/auth"
)

type debugProbe struct {
	label  string
	path   string
	status int
	body   string
	err    string
}

type debugAttempt struct {
	label    string
	endpoint string
	body     string
	status   int
	response string
	err      string
	found    bool // item appeared in consumed-items after POST
	deleted  bool
}

type debugAddTest struct {
	productID    string
	attempts     []debugAttempt
	postConsumed string // consumed-items snapshot after all attempts
}

type debugProfile struct {
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
	profile    *debugProfile
	probes     []debugProbe
	addTest    *debugAddTest
	loading    bool
	scrollY    int
	width      int
	height     int
}

type debugLoadedMsg struct {
	profile *debugProfile
	probes  []debugProbe
	addTest *debugAddTest
}

func newDebugModel(client *api.Client, token string) debugModel {
	cfg, _ := auth.LoadConfig()
	return debugModel{
		client:     client,
		email:      cfg.Email,
		token:      token,
		configPath: auth.ConfigFilePath(),
		loading:    true,
	}
}

func (m debugModel) load() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		today := time.Now().Format(time.DateOnly)

		// Fetch profile first
		var profile *debugProfile
		if raw, _, err := client.GetRaw("/v9/user"); err == nil {
			var p debugProfile
			if json.Unmarshal([]byte(raw), &p) == nil {
				profile = &p
			}
		}

		// Find a product ID for the add test
		testProductID := ""
		if raw, _, err := client.GetRaw(fmt.Sprintf("/v9/user/consumed-items?date=%s", today)); err == nil {
			var cr struct {
				Products []struct {
					ProductID string `json:"product_id"`
				} `json:"products"`
			}
			if json.Unmarshal([]byte(raw), &cr) == nil && len(cr.Products) > 0 {
				testProductID = cr.Products[0].ProductID
			}
		}
		if testProductID == "" {
			country, sex := "DE", "male"
			if profile != nil && profile.Country != "" {
				country = profile.Country
			}
			if profile != nil && profile.Sex != "" {
				sex = profile.Sex
			}
			if raw, _, err := client.GetRaw(fmt.Sprintf(
				"/v9/products/search?query=apple&language=en&countries=%s&sex=%s", country, sex,
			)); err == nil {
				var results []struct {
					ID string `json:"id"`
				}
				if json.Unmarshal([]byte(raw), &results) == nil && len(results) > 0 {
					testProductID = results[0].ID
				}
			}
		}

		// GET probes
		country, sex := "DE", "male"
		if profile != nil && profile.Country != "" {
			country = profile.Country
		}
		if profile != nil && profile.Sex != "" {
			sex = profile.Sex
		}
		probePaths := []struct{ label, path string }{
			{"user profile", "/v9/user"},
			{"consumed today", fmt.Sprintf("/v9/user/consumed-items?date=%s", today)},
			{"nutrients today", fmt.Sprintf("/v9/user/consumed-items/nutrients-daily?start=%s&end=%s", today, today)},
			{"goals today", fmt.Sprintf("/v9/user/goals?date=%s", today)},
			{"search probe", fmt.Sprintf("/v9/products/search?query=apple&language=en&countries=%s&sex=%s", country, sex)},
		}
		if testProductID != "" {
			probePaths = append(probePaths, struct{ label, path string }{
				"product details", "/v9/products/" + testProductID,
			})
		}
		var probes []debugProbe
		for _, p := range probePaths {
			body, status, err := client.GetRaw(p.path)
			ep := debugProbe{label: p.label, path: p.path, status: status}
			if err != nil {
				ep.err = err.Error()
			} else {
				ep.body = truncateLines(prettyJSON(body), 15)
			}
			probes = append(probes, ep)
		}

		// Add meal test: try several request formats in sequence
		var addTest *debugAddTest
		if testProductID != "" {
			at := &debugAddTest{productID: testProductID}

			// Fetch product to get its actual defined servings
			actualServing := ""
			actualAmount := 0.0
			if raw, _, e := client.GetRaw("/v9/products/" + testProductID); e == nil {
				var prod struct {
					Servings []struct {
						Amount  float64 `json:"amount"`
						Serving string  `json:"serving"`
					} `json:"servings"`
				}
				if json.Unmarshal([]byte(raw), &prod) == nil && len(prod.Servings) > 0 {
					actualServing = prod.Servings[0].Serving
					actualAmount = prod.Servings[0].Amount
				}
			}

			type attempt struct {
				label    string
				endpoint string
				body     string
			}

			candidates := []attempt{
				{"gram serving", "/v9/user/consumed-items", fmt.Sprintf(
					`{"product_id":%q,"date":%q,"daytime":"snack","amount":100,"serving":"gram","serving_quantity":1,"type":"product"}`,
					testProductID, today,
				)},
			}

			// Add attempt using the product's actual defined serving (if available)
			if actualServing != "" {
				candidates = append(candidates, attempt{
					label:    fmt.Sprintf("actual serving (%s)", actualServing),
					endpoint: "/v9/user/consumed-items",
					body: fmt.Sprintf(
						`{"product_id":%q,"date":%q,"daytime":"snack","amount":%g,"serving":%q,"serving_quantity":1,"type":"product"}`,
						testProductID, today, actualAmount, actualServing,
					),
				})
			}

			for _, c := range candidates {
				respBody, status, err := client.PostRaw(c.endpoint, c.body)
				a := debugAttempt{
					label:    c.label,
					endpoint: c.endpoint,
					body:     prettyJSON(c.body),
					status:   status,
					response: prettyJSON(respBody),
				}
				if err != nil {
					a.err = err.Error()
				}

				// If 2xx, check whether item was actually added
				if status >= 200 && status < 300 {
					createdID := ""
					var created struct {
						ID string `json:"id"`
					}
					if json.Unmarshal([]byte(respBody), &created) == nil {
						createdID = created.ID
					}
					if raw, _, e := client.GetRaw(fmt.Sprintf("/v9/user/consumed-items?date=%s", today)); e == nil {
						var cr struct {
							Products []struct {
								ID        string `json:"id"`
								ProductID string `json:"product_id"`
								Daytime   string `json:"daytime"`
							} `json:"products"`
						}
						if json.Unmarshal([]byte(raw), &cr) == nil {
							for _, p := range cr.Products {
								if p.ProductID == testProductID && p.Daytime == "snack" {
									if createdID == "" {
										createdID = p.ID
									}
									a.found = true
									break
								}
							}
						}
					}
					if createdID != "" {
						if client.DeleteConsumedItem(createdID) == nil {
							a.deleted = true
						}
					}
				}

				at.attempts = append(at.attempts, a)

				// Stop as soon as we find a working format
				if a.found {
					break
				}
			}

			// Final consumed-items snapshot
			if raw, _, e := client.GetRaw(fmt.Sprintf("/v9/user/consumed-items?date=%s", today)); e == nil {
				at.postConsumed = truncateLines(prettyJSON(raw), 20)
			}
			addTest = at
		}

		return debugLoadedMsg{profile: profile, probes: probes, addTest: addTest}
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
		m.profile = msg.profile
		m.probes = msg.probes
		m.addTest = msg.addTest
	}
	return m, nil
}

func (m debugModel) View() string {
	var lines []string

	lines = append(lines, styleHeader.Render("Debug Info"), "")

	// Account
	lines = append(lines, sectionLabel("Account"))
	lines = append(lines, row("Email", m.email))
	if m.profile != nil {
		lines = append(lines, row("Name", m.profile.FirstName+" "+m.profile.LastName))
		lines = append(lines, row("UUID", m.profile.UUID))
		lines = append(lines, row("Country / Sex", m.profile.Country+" / "+m.profile.Sex))
	}
	tokenPreview := m.token
	if len(m.token) > 16 {
		tokenPreview = m.token[:8] + "…" + m.token[len(m.token)-8:]
	}
	lines = append(lines, row("Token", tokenPreview))
	lines = append(lines, row("Config file", m.configPath))
	lines = append(lines, "")

	if m.loading {
		lines = append(lines, styleDimmed.Render("  Loading…"))
		lines = append(lines, "", styleHelp.Render("[↑/↓] scroll  [g] top  [Esc] back"))
		return applyScroll(lines, m.scrollY, m.height)
	}

	// Add meal test
	lines = append(lines, sectionLabel("Add Meal Test"))
	if m.addTest == nil {
		lines = append(lines, styleDimmed.Render("  No product found for test"))
	} else {
		at := m.addTest
		lines = append(lines, row("Product ID", at.productID))
		lines = append(lines, "")

		successStyle := lipgloss.NewStyle().Foreground(colorSuccess)

		for _, a := range at.attempts {
			statusStr := fmt.Sprintf("HTTP %d", a.status)
			statusStyle := successStyle
			if a.err != "" || a.status >= 400 {
				statusStyle = styleError
			}

			// One-line summary per attempt
			result := statusStyle.Render(statusStr)
			if a.err != "" {
				result += "  " + styleError.Render(a.err)
			} else if a.found && a.deleted {
				result += "  " + successStyle.Render("✓ item added & deleted — THIS FORMAT WORKS")
			} else if a.found {
				result += "  " + successStyle.Render("✓ item added (delete failed)")
			} else if a.status >= 200 && a.status < 300 {
				result += "  " + styleDimmed.Render("accepted but item not found in consumed-items")
			}

			lines = append(lines, fmt.Sprintf("  %s  %s  %s",
				styleDimmed.Render(a.label),
				styleDimmed.Render(a.endpoint),
				result,
			))

			// Show request body only for failed/interesting attempts
			if !a.found {
				for _, l := range strings.Split(a.body, "\n") {
					lines = append(lines, "    "+styleDimmed.Render(l))
				}
				if a.response != "" && a.response != `""` && a.response != "null" {
					lines = append(lines, "    "+styleDimmed.Render("response: "+a.response))
				}
			}
			lines = append(lines, "")
		}

		if at.postConsumed != "" {
			lines = append(lines, styleDimmed.Render("  consumed-items after test:"))
			for _, l := range strings.Split(at.postConsumed, "\n") {
				lines = append(lines, "    "+styleDimmed.Render(l))
			}
		}
	}
	lines = append(lines, "")

	// GET probes
	lines = append(lines, sectionLabel("API Probes"))
	for _, ep := range m.probes {
		statusStr := fmt.Sprintf("HTTP %d", ep.status)
		statusStyle := lipgloss.NewStyle().Foreground(colorSuccess)
		if ep.err != "" || ep.status >= 400 {
			statusStyle = styleError
		}
		lines = append(lines, fmt.Sprintf("  %s  %s  %s",
			styleDimmed.Render(ep.label),
			styleItemName.Render(ep.path),
			statusStyle.Render(statusStr),
		))
		if ep.err != "" {
			lines = append(lines, styleError.Render("    "+ep.err))
		} else {
			for _, l := range strings.Split(ep.body, "\n") {
				lines = append(lines, "    "+styleDimmed.Render(l))
			}
		}
		lines = append(lines, "")
	}

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

func truncateLines(s string, max int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= max {
		return s
	}
	return strings.Join(lines[:max], "\n") + fmt.Sprintf("\n  … (%d more lines)", len(lines)-max)
}
