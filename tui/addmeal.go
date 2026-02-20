package tui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/koriwi/yazio-cli/internal/api"
	"github.com/koriwi/yazio-cli/internal/models"
)

type addMealTab int

const (
	tabRecent   addMealTab = 0
	tabSearch   addMealTab = 1
)

type addMealStep int

const (
	stepBrowse   addMealStep = 0
	stepAmount   addMealStep = 1
	stepMealTime addMealStep = 2
	stepConfirm  addMealStep = 3
)

type addMealModel struct {
	tab     addMealTab
	step    addMealStep
	client  *api.Client
	cache   *sync.Map
	date    time.Time
	profile *models.UserProfile // for search params (country, sex)
	width   int
	height  int

	// Browse state
	recent  []models.ProductResponse
	results []models.ProductResponse
	listIdx int
	loading bool
	err     string

	// Search state
	searchInput textinput.Model

	// Selected product + entry details
	selected    *models.ProductResponse
	amountInput textinput.Model
	mealTimeIdx int // index into models.MealTimes
}

type recentLoadedMsg struct {
	products []models.ProductResponse
	err      string
}

type searchResultsMsg struct {
	products []models.ProductResponse
	err      string
}

type addSuccessMsg struct{}
type addErrMsg struct{ err string }

func newAddMealModel(client *api.Client, cache *sync.Map, date time.Time, profile *models.UserProfile) addMealModel {
	search := textinput.New()
	search.Placeholder = "Search foods..."
	search.CharLimit = 128
	search.Width = 40

	amount := textinput.New()
	amount.Placeholder = "100"
	amount.CharLimit = 10
	amount.Width = 15

	return addMealModel{
		tab:         tabRecent,
		client:      client,
		cache:       cache,
		date:        date,
		profile:     profile,
		searchInput: search,
		amountInput: amount,
		mealTimeIdx: 0,
	}
}

func (m addMealModel) loadRecent() tea.Cmd {
	client := m.client
	cache := m.cache
	date := m.date

	return func() tea.Msg {
		seen := map[string]bool{}
		var products []models.ProductResponse

		// Look at the last 7 days of consumed items
		for i := 0; i < 7; i++ {
			d := date.AddDate(0, 0, -i)
			consumed, err := client.GetConsumedItems(d)
			if err != nil {
				continue
			}
			for _, cp := range consumed.Products {
				if seen[cp.ProductID] {
					continue
				}
				seen[cp.ProductID] = true
				product := fetchProductCached(cp.ProductID, client, cache)
				if product != nil {
					products = append(products, *product)
				}
			}
			if len(products) >= 20 {
				break
			}
		}
		return recentLoadedMsg{products: products}
	}
}

func (m addMealModel) doSearch(query string) tea.Cmd {
	client := m.client
	country, sex := "DE", "male"
	if m.profile != nil {
		if m.profile.Country != "" {
			country = m.profile.Country
		}
		if m.profile.Sex != "" {
			sex = m.profile.Sex
		}
	}

	return func() tea.Msg {
		products, err := client.SearchProducts(query, country, sex)
		if err != nil {
			return searchResultsMsg{err: err.Error()}
		}
		return searchResultsMsg{products: products}
	}
}

func (m addMealModel) doAddMeal() tea.Cmd {
	product := m.selected
	amount, _ := strconv.ParseFloat(m.amountInput.Value(), 64)
	if amount <= 0 {
		amount = 100
	}
	mealTime := models.MealTimes[m.mealTimeIdx]
	serving := "gram"
	if len(product.Servings) > 0 {
		serving = product.Servings[0].Serving
	}
	client := m.client
	date := m.date

	return func() tea.Msg {
		err := client.AddConsumedItem(models.AddConsumedRequest{
			ProductID:       product.ID,
			Date:            date.Format(time.DateOnly),
			Daytime:         mealTime,
			Amount:          amount,
			Serving:         serving,
			ServingQuantity: 1,
		})
		if err != nil {
			return addErrMsg{err: err.Error()}
		}
		return addSuccessMsg{}
	}
}

func (m addMealModel) currentList() []models.ProductResponse {
	if m.tab == tabSearch {
		return m.results
	}
	return m.recent
}

func (m addMealModel) Update(msg tea.Msg) (addMealModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case recentLoadedMsg:
		m.loading = false
		if msg.err != "" {
			m.err = msg.err
		} else {
			m.recent = msg.products
		}

	case searchResultsMsg:
		m.loading = false
		if msg.err != "" {
			m.err = msg.err
		} else {
			m.results = msg.products
			m.listIdx = 0
		}

	case tea.KeyMsg:
		switch m.step {
		case stepBrowse:
			switch msg.String() {
			case "ctrl+c", "q", "esc":
				return m, func() tea.Msg { return backToDiaryMsg{} }

			case "tab":
				m.tab = (m.tab + 1) % 2
				m.listIdx = 0
				m.err = ""
				if m.tab == tabRecent && len(m.recent) == 0 {
					m.loading = true
					cmds = append(cmds, m.loadRecent())
				} else if m.tab == tabSearch {
					m.searchInput.Focus()
				}

			case "j", "down":
				list := m.currentList()
				if m.tab == tabSearch && m.searchInput.Focused() {
					// Navigate to list
					if len(list) > 0 {
						m.searchInput.Blur()
					}
				} else if m.listIdx < len(list)-1 {
					m.listIdx++
				}

			case "k", "up":
				if m.listIdx > 0 {
					m.listIdx--
				} else if m.tab == tabSearch {
					m.searchInput.Focus()
				}

			case "enter":
				if m.tab == tabSearch && m.searchInput.Focused() {
					// Perform search
					q := m.searchInput.Value()
					if q != "" {
						m.loading = true
						m.err = ""
						cmds = append(cmds, m.doSearch(q))
					}
					break
				}
				list := m.currentList()
				if m.listIdx < len(list) {
					p := list[m.listIdx]
					m.selected = &p
					m.step = stepAmount
					m.amountInput.SetValue("100")
					m.amountInput.Focus()
					m.err = ""
				}
			}

		case stepAmount:
			switch msg.String() {
			case "esc":
				m.step = stepBrowse
				m.selected = nil
				m.amountInput.Blur()
			case "enter":
				_, err := strconv.ParseFloat(m.amountInput.Value(), 64)
				if err != nil || m.amountInput.Value() == "" {
					m.err = "Enter a valid number"
					break
				}
				m.err = ""
				m.step = stepMealTime
				m.amountInput.Blur()
			}

		case stepMealTime:
			switch msg.String() {
			case "esc":
				m.step = stepAmount
				m.amountInput.Focus()
			case "left", "h":
				if m.mealTimeIdx > 0 {
					m.mealTimeIdx--
				}
			case "right", "l":
				if m.mealTimeIdx < len(models.MealTimes)-1 {
					m.mealTimeIdx++
				}
			case "enter":
				m.step = stepConfirm
				m.loading = true
				cmds = append(cmds, m.doAddMeal())
			}

		case stepConfirm:
			// handled by addSuccessMsg / addErrMsg
		}

	case addSuccessMsg:
		return m, func() tea.Msg { return addedMealMsg{} }

	case addErrMsg:
		m.loading = false
		m.step = stepMealTime
		m.err = msg.err
	}

	// Update text inputs
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	cmds = append(cmds, cmd)
	m.amountInput, cmd = m.amountInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m addMealModel) View() string {
	var sb strings.Builder

	sb.WriteString(styleHeader.Render("Add Meal") + "\n")

	switch m.step {
	case stepBrowse:
		// Tab bar
		tabs := []string{"Recent", "Search"}
		for i, t := range tabs {
			if addMealTab(i) == m.tab {
				sb.WriteString(styleTabActive.Render(t))
			} else {
				sb.WriteString(styleTab.Render(t))
			}
		}
		sb.WriteString("\n\n")

		if m.tab == tabSearch {
			sb.WriteString("  " + styleInput.Render(m.searchInput.View()) + "\n\n")
		}

		if m.loading {
			sb.WriteString(styleDimmed.Render("  Loading...") + "\n")
		} else if m.err != "" {
			sb.WriteString(styleError.Render("  "+m.err) + "\n")
		} else {
			list := m.currentList()
			if len(list) == 0 {
				if m.tab == tabRecent {
					sb.WriteString(styleDimmed.Render("  No recent foods") + "\n")
				} else {
					sb.WriteString(styleDimmed.Render("  Type to search and press Enter") + "\n")
				}
			} else {
				maxVisible := m.height - 12
				if maxVisible < 5 {
					maxVisible = 5
				}
				start := 0
				if m.listIdx >= maxVisible {
					start = m.listIdx - maxVisible + 1
				}
				end := start + maxVisible
				if end > len(list) {
					end = len(list)
				}
				for i := start; i < end; i++ {
					p := list[i]
					kcalPer100 := fmt.Sprintf("%.0f kcal/100g", p.Nutrients.EnergyKcal*100)
					name := truncate(p.Name, m.width-20)
					line := fmt.Sprintf("  %s  %s", padRight(name, m.width/2), styleDimmed.Render(kcalPer100))
					if i == m.listIdx && !m.searchInput.Focused() {
						line = styleSelected.Render(line)
					}
					sb.WriteString(line + "\n")
				}
			}
		}

		sb.WriteString("\n")
		sb.WriteString(styleHelp.Render("[Tab] switch tab  [↑/↓] navigate  [Enter] select  [Esc] back"))

	case stepAmount:
		if m.selected != nil {
			sb.WriteString(fmt.Sprintf("  %s\n\n", styleItemName.Render(m.selected.Name)))
		}
		sb.WriteString("  Amount:\n")
		sb.WriteString("  " + styleInput.Render(m.amountInput.View()))

		serving := "g"
		if m.selected != nil && len(m.selected.Servings) > 0 {
			serving = m.selected.Servings[0].Serving
		}
		sb.WriteString("  " + styleDimmed.Render(serving) + "\n\n")

		if m.err != "" {
			sb.WriteString(styleError.Render("  "+m.err) + "\n\n")
		}
		sb.WriteString(styleHelp.Render("[Enter] next  [Esc] back"))

	case stepMealTime:
		if m.selected != nil {
			amount, _ := strconv.ParseFloat(m.amountInput.Value(), 64)
			kcal := m.selected.Nutrients.EnergyKcal * amount
			sb.WriteString(fmt.Sprintf("  %s  —  %.0fg  —  %.0f kcal\n\n",
				styleItemName.Render(m.selected.Name), amount, kcal))
		}
		sb.WriteString("  Meal:\n  ")
		for i, mt := range models.MealTimes {
			label := models.MealTimeLabel(mt)
			if i == m.mealTimeIdx {
				sb.WriteString(styleSelected.Render(" "+label+" "))
			} else {
				sb.WriteString(styleDimmed.Render(" "+label+" "))
			}
			if i < len(models.MealTimes)-1 {
				sb.WriteString("  ")
			}
		}
		sb.WriteString("\n\n")
		if m.err != "" {
			sb.WriteString(styleError.Render("  "+m.err) + "\n\n")
		}
		sb.WriteString(styleHelp.Render("[←/→] meal  [Enter] add  [Esc] back"))

	case stepConfirm:
		sb.WriteString(styleDimmed.Render("  Adding meal...") + "\n")
	}

	return sb.String()
}

// Messages for page transitions
type backToDiaryMsg struct{}
type addedMealMsg struct{}
