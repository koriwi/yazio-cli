package tui

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/koriwi/yazio-cli/internal/api"
	"github.com/koriwi/yazio-cli/internal/models"
)

type diaryModel struct {
	date     time.Time
	entries  []models.DiaryEntry
	goals    *models.GoalsResponse
	totals   *models.DailyNutrient
	loading  bool
	err      string
	width    int
	height   int
	selected int // selected entry index for deletion
	client   *api.Client

	// product cache shared across date navigations
	cache *sync.Map
}

type diaryLoadedMsg struct {
	entries []models.DiaryEntry
	goals   *models.GoalsResponse
	totals  *models.DailyNutrient
	err     string
}

func newDiaryModel(client *api.Client, cache *sync.Map) diaryModel {
	return diaryModel{
		date:   time.Now(),
		client: client,
		cache:  cache,
	}
}

type logoutMsg struct{}

func (m diaryModel) loadDiary() tea.Cmd {
	date := m.date
	client := m.client
	cache := m.cache

	return func() tea.Msg {
		// Fetch consumed items, goals and totals in parallel
		type result struct {
			consumed *models.ConsumedItemsResponse
			goals    *models.GoalsResponse
			totals   *models.DailyNutrient
			err      error
		}
		ch := make(chan result, 1)

		go func() {
			var r result
			var wg sync.WaitGroup
			var mu sync.Mutex

			wg.Add(3)

			go func() {
				defer wg.Done()
				consumed, err := client.GetConsumedItems(date)
				mu.Lock()
				if err != nil && r.err == nil {
					r.err = err
				}
				r.consumed = consumed
				mu.Unlock()
			}()

			go func() {
				defer wg.Done()
				goals, err := client.GetGoals(date)
				mu.Lock()
				if err != nil && r.err == nil {
					r.err = err
				}
				r.goals = goals
				mu.Unlock()
			}()

			go func() {
				defer wg.Done()
				totals, err := client.GetDailyNutrients(date)
				mu.Lock()
				if err != nil && r.err == nil {
					r.err = err
				}
				r.totals = totals
				mu.Unlock()
			}()

			wg.Wait()
			ch <- r
		}()

		r := <-ch
		if r.err != nil {
			return diaryLoadedMsg{err: r.err.Error()}
		}

		// Resolve product details
		entries := resolveEntries(r.consumed, client, cache, date)

		return diaryLoadedMsg{
			entries: entries,
			goals:   r.goals,
			totals:  r.totals,
		}
	}
}

func resolveEntries(consumed *models.ConsumedItemsResponse, client *api.Client, cache *sync.Map, date time.Time) []models.DiaryEntry {
	if consumed == nil {
		return nil
	}

	var entries []models.DiaryEntry
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, cp := range consumed.Products {
		cp := cp
		wg.Add(1)
		go func() {
			defer wg.Done()
			product := fetchProductCached(cp.ProductID, client, cache)
			entry := buildEntry(cp.ID, cp.ProductID, cp.Daytime, cp, product)
			mu.Lock()
			entries = append(entries, entry)
			mu.Unlock()
		}()
	}

	for _, cr := range consumed.RecipePortions {
		cr := cr
		wg.Add(1)
		go func() {
			defer wg.Done()
			product := fetchRecipeCached(cr.RecipeID, client, cache)
			entry := buildRecipeEntry(cr.ID, cr.RecipeID, cr.Daytime, cr.PortionCount, product)
			mu.Lock()
			entries = append(entries, entry)
			mu.Unlock()
		}()
	}

	for _, cp := range consumed.SimpleProducts {
		cp := cp
		wg.Add(1)
		go func() {
			defer wg.Done()
			product := fetchProductCached(cp.ProductID, client, cache)
			entry := buildEntry(cp.ID, cp.ProductID, cp.Daytime, cp, product)
			mu.Lock()
			entries = append(entries, entry)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Sort by meal time order
	order := map[string]int{"breakfast": 0, "lunch": 1, "dinner": 2, "snack": 3}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			oi := order[entries[i].MealTime]
			oj := order[entries[j].MealTime]
			if oi > oj {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	return entries
}

func fetchProductCached(productID string, client *api.Client, cache *sync.Map) *models.ProductResponse {
	if v, ok := cache.Load(productID); ok {
		p, _ := v.(*models.ProductResponse)
		return p
	}
	p, err := client.GetProduct(productID)
	if err != nil {
		return nil
	}
	cache.Store(productID, p)
	return p
}

func fetchRecipeCached(recipeID string, client *api.Client, cache *sync.Map) *models.ProductResponse {
	key := "recipe:" + recipeID
	if v, ok := cache.Load(key); ok {
		p, _ := v.(*models.ProductResponse)
		return p
	}
	p, err := client.GetRecipe(recipeID)
	if err != nil {
		return nil
	}
	cache.Store(key, p)
	return p
}

func buildEntry(consumedID, productID, mealTime string, cp models.ConsumedProduct, p *models.ProductResponse) models.DiaryEntry {
	e := models.DiaryEntry{
		ConsumedID:      consumedID,
		ProductID:       productID,
		MealTime:        mealTime,
		Amount:          cp.Amount,
		Serving:         cp.Serving,
		ServingQuantity: cp.ServingQuantity,
		Name:            productID, // fallback
	}
	amount := cp.Amount
	if p == nil {
		return e
	}
	e.Name = p.Name
	// amount is total grams; nutrients are per gram → multiply directly
	e.Kcal = math.Round(p.Nutrients.EnergyKcal*amount*10) / 10
	e.Protein = math.Round(p.Nutrients.Protein*amount*10) / 10
	e.Carbs = math.Round(p.Nutrients.Carb*amount*10) / 10
	e.Fat = math.Round(p.Nutrients.Fat*amount*10) / 10
	return e
}

func buildRecipeEntry(consumedID, recipeID, mealTime string, portions float64, p *models.ProductResponse) models.DiaryEntry {
	e := models.DiaryEntry{
		ConsumedID: consumedID,
		ProductID:  recipeID,
		MealTime:   mealTime,
		Amount:     portions,
		Serving:    "portion",
		Name:       recipeID,
	}
	if p == nil {
		return e
	}
	e.Name = p.Name
	e.Kcal = math.Round(p.Nutrients.EnergyKcal*portions*10) / 10
	e.Protein = math.Round(p.Nutrients.Protein*portions*10) / 10
	e.Carbs = math.Round(p.Nutrients.Carb*portions*10) / 10
	e.Fat = math.Round(p.Nutrients.Fat*portions*10) / 10
	return e
}

func (m diaryModel) Update(msg tea.Msg) (diaryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if m.loading {
			break
		}
		switch msg.String() {
		case "left", "h":
			m.date = m.date.AddDate(0, 0, -1)
			m.loading = true
			m.err = ""
			m.selected = 0
			return m, m.loadDiary()
		case "right", "l":
			if m.date.Before(time.Now().Truncate(24 * time.Hour)) {
				m.date = m.date.AddDate(0, 0, 1)
				m.loading = true
				m.err = ""
				m.selected = 0
				return m, m.loadDiary()
			}
		case "t":
			m.date = time.Now()
			m.loading = true
			m.err = ""
			m.selected = 0
			return m, m.loadDiary()
		case "j", "down":
			if m.selected < len(m.entries)-1 {
				m.selected++
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
		case "d":
			if len(m.entries) > 0 {
				entry := m.entries[m.selected]
				if entry.ConsumedID != "" {
					m.loading = true
					client := m.client
					id := entry.ConsumedID
					date := m.date
					cache := m.cache
					return m, func() tea.Msg {
						if err := client.DeleteConsumedItem(id); err != nil {
							return diaryLoadedMsg{err: "delete failed: " + err.Error()}
						}
						tmp := diaryModel{date: date, client: client, cache: cache}
						return tmp.loadDiary()()
					}
				}
			}
		case "r":
			m.loading = true
			m.err = ""
			return m, m.loadDiary()
		case "L":
			return m, func() tea.Msg { return logoutMsg{} }
		}

	case diaryLoadedMsg:
		m.loading = false
		if msg.err != "" {
			m.err = msg.err
		} else {
			m.entries = msg.entries
			m.goals = msg.goals
			m.totals = msg.totals
			if m.selected >= len(m.entries) {
				m.selected = max(0, len(m.entries)-1)
			}
		}
	}
	return m, nil
}

func (m diaryModel) View() string {
	if m.loading {
		return "\n  Loading...\n"
	}

	var sb strings.Builder

	// Date navigation header
	dateStr := formatDate(m.date)
	canGoRight := m.date.Before(time.Now().Truncate(24 * time.Hour))
	rightArrow := "→"
	if !canGoRight {
		rightArrow = styleDimmed.Render("→")
	}
	nav := fmt.Sprintf("← %s %s", dateStr, rightArrow)
	sb.WriteString(styleDateNav.Render(nav))
	sb.WriteString("\n\n")

	// Progress bars
	if m.totals != nil || m.goals != nil {
		sb.WriteString(m.renderProgressBars())
		sb.WriteString("\n")
	}

	// Error
	if m.err != "" {
		sb.WriteString(styleError.Render("Error: "+m.err) + "\n")
	}

	// Meal sections
	mealTimes := []string{"breakfast", "lunch", "dinner", "snack"}
	mealEmoji := map[string]string{
		"breakfast": "🌅",
		"lunch":     "☀️ ",
		"dinner":    "🌙",
		"snack":     "🍎",
	}

	availW := m.width
	if availW < 40 {
		availW = 80
	}

	for _, mt := range mealTimes {
		items := filterByMeal(m.entries, mt)
		label := mealEmoji[mt] + " " + models.MealTimeLabel(mt)

		var mealKcal float64
		for _, e := range items {
			mealKcal += e.Kcal
		}

		header := styleMealHeader.Render(fmt.Sprintf("%s  %s", label, styleKcal.Render(fmt.Sprintf("%.0f kcal", mealKcal))))
		sb.WriteString(header + "\n")

		if len(items) == 0 {
			sb.WriteString(styleDimmed.Render("  —") + "\n")
		} else {
			for idx, e := range items {
				// Find global index
				globalIdx := findGlobalIndex(m.entries, e)
				isSelected := globalIdx == m.selected

				name := truncate(e.Name, availW/2)
				serving := formatServing(e.Amount, e.Serving, e.ServingQuantity)
				kcalStr := fmt.Sprintf("%.0f kcal", e.Kcal)
				macros := fmt.Sprintf("P:%.1fg C:%.1fg F:%.1fg", e.Protein, e.Carbs, e.Fat)

				nameW := availW / 2
				servW := 12

				line := fmt.Sprintf("  %s %s %s  %s",
					padRight(name, nameW),
					padRight(serving, servW),
					padRight(kcalStr, 10),
					styleDimmed.Render(macros),
				)

				if isSelected {
					line = styleSelected.Render(line)
				} else {
					line = styleItemName.Render(line)
				}
				sb.WriteString(line + "\n")
				_ = idx
			}
		}
		sb.WriteString("\n")
	}

	// Help
	helpItems := []string{
		"[←/→] date",
		"[↑/↓] select",
		"[a] add",
		"[d] delete",
		"[t] today",
		"[r] refresh",
		"[?] debug",
		"[L] logout",
		"[q] quit",
	}
	sb.WriteString(styleHelp.Render(strings.Join(helpItems, "  ")))

	return sb.String()
}

func (m diaryModel) renderProgressBars() string {
	var totals models.DailyNutrient
	if m.totals != nil {
		totals = *m.totals
	}
	var goals models.GoalsResponse
	if m.goals != nil {
		goals = *m.goals
	}
	if goals.EnergyKcal == 0 {
		goals.EnergyKcal = 2000
	}
	if goals.Protein == 0 {
		goals.Protein = 150
	}
	if goals.Carb == 0 {
		goals.Carb = 250
	}
	if goals.Fat == 0 {
		goals.Fat = 65
	}

	barW := 30
	var sb strings.Builder
	sb.WriteString(renderBar("Calories", totals.Energy, goals.EnergyKcal, "kcal", colorCalories, barW))
	sb.WriteString(renderBar("Protein ", totals.Protein, goals.Protein, "g", colorProtein, barW))
	sb.WriteString(renderBar("Carbs   ", totals.Carb, goals.Carb, "g", colorCarbs, barW))
	sb.WriteString(renderBar("Fat     ", totals.Fat, goals.Fat, "g", colorFat, barW))
	return sb.String()
}

func renderBar(label string, current, goal float64, unit string, color lipgloss.Color, width int) string {
	pct := 0.0
	if goal > 0 {
		pct = current / goal
		if pct > 1 {
			pct = 1
		}
	}
	filled := int(pct * float64(width))
	empty := width - filled

	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(colorSubtle).Render(strings.Repeat("░", empty))

	nums := fmt.Sprintf("%.0f / %.0f %s", current, goal, unit)
	return fmt.Sprintf("  %s  [%s]  %s\n", label, bar, nums)
}

func filterByMeal(entries []models.DiaryEntry, mealTime string) []models.DiaryEntry {
	var out []models.DiaryEntry
	for _, e := range entries {
		if e.MealTime == mealTime {
			out = append(out, e)
		}
	}
	return out
}

func findGlobalIndex(entries []models.DiaryEntry, target models.DiaryEntry) int {
	for i, e := range entries {
		if e.ConsumedID == target.ConsumedID && e.ProductID == target.ProductID {
			return i
		}
	}
	return -1
}

func formatDate(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "Today"
	}
	yesterday := now.AddDate(0, 0, -1)
	if t.Year() == yesterday.Year() && t.YearDay() == yesterday.YearDay() {
		return "Yesterday"
	}
	return t.Format("Mon, Jan 2")
}

func formatServing(amountGrams float64, serving string, qty float64) string {
	switch serving {
	case "gram", "g", "":
		return fmt.Sprintf("%.0fg", amountGrams)
	case "ml":
		return fmt.Sprintf("%.0fml", amountGrams)
	default:
		// Show serving count + label (e.g. "2 cookies", "1 package")
		if qty <= 0 {
			qty = 1
		}
		if qty == 1 {
			return fmt.Sprintf("1 %s", serving)
		}
		return fmt.Sprintf("%.0f %ss", qty, serving)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
