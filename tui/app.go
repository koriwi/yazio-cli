package tui

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/koriwi/yazio-cli/internal/api"
	"github.com/koriwi/yazio-cli/internal/auth"
	"github.com/koriwi/yazio-cli/internal/models"
)

type page int

const (
	pageLogin   page = 0
	pageDiary   page = 1
	pageAddMeal page = 2
	pageDebug   page = 3
)

type profileLoadedMsg struct{ profile *models.UserProfile }

type App struct {
	page    page
	login   loginModel
	diary   diaryModel
	addMeal addMealModel
	debug   debugModel
	client  *api.Client
	token   string
	profile *models.UserProfile
	cache   *sync.Map
	width   int
	height  int
}

func fetchProfile(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		p, _ := client.GetProfile()
		return profileLoadedMsg{profile: p}
	}
}

func New(loggedIn bool, token string) *App {
	cache := &sync.Map{}
	var p page
	var diary diaryModel
	var client *api.Client

	if loggedIn {
		client = api.New(token)
		diary = newDiaryModel(client, cache)
		p = pageDiary
	} else {
		p = pageLogin
	}

	return &App{
		page:   p,
		login:  newLoginModel(),
		diary:  diary,
		cache:  cache,
		client: client,
		token:  token,
	}
}

func (a *App) Init() tea.Cmd {
	if a.page == pageDiary {
		a.diary.loading = true
		return tea.Batch(a.diary.loadDiary(), fetchProfile(a.client))
	}
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.login.width, a.login.height = msg.Width, msg.Height
		a.diary.width, a.diary.height = msg.Width, msg.Height
		if a.page == pageAddMeal {
			a.addMeal.width, a.addMeal.height = msg.Width, msg.Height
		}

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		if a.page == pageDiary && msg.String() == "q" {
			return a, tea.Quit
		}

		// Debug page
		if a.page == pageDiary && msg.String() == "?" {
			a.debug = newDebugModel(a.client, a.token)
			a.debug.width, a.debug.height = a.width, a.height
			a.page = pageDebug
			return a, a.debug.load()
		}

		// Page-specific add key
		if a.page == pageDiary && msg.String() == "a" {
			a.addMeal = newAddMealModel(a.client, a.cache, a.diary.date, a.profile)
			a.addMeal.width, a.addMeal.height = a.width, a.height
			a.addMeal.loading = true
			a.page = pageAddMeal
			cmds = append(cmds, a.addMeal.loadRecent())
			return a, tea.Batch(cmds...)
		}

	case profileLoadedMsg:
		a.profile = msg.profile

	// Login flow
	case loginSuccessMsg:
		a.token = msg.token
		a.client = api.New(msg.token)
		a.diary = newDiaryModel(a.client, a.cache)
		a.diary.width, a.diary.height = a.width, a.height
		a.diary.loading = true
		a.page = pageDiary
		return a, tea.Batch(a.diary.loadDiary(), fetchProfile(a.client))

	// Add meal transitions
	case backToDiaryMsg:
		a.page = pageDiary
		a.diary.loading = true
		return a, a.diary.loadDiary()

	case addedMealMsg:
		a.page = pageDiary
		// Reload diary to show new item
		a.diary.date = a.addMeal.date
		a.diary.loading = true
		return a, a.diary.loadDiary()

	case editEntryMsg:
		a.addMeal = newEditMealModel(a.client, a.cache, a.diary.date, a.profile, msg.entry)
		a.addMeal.width, a.addMeal.height = a.width, a.height
		a.page = pageAddMeal
		return a, a.addMeal.doFetchProduct(msg.entry.ProductID)

	case logoutMsg:
		auth.ClearToken()
		a.token = ""
		a.client = nil
		a.profile = nil
		a.cache = &sync.Map{}
		a.page = pageLogin
		a.login = newLoginModel()
		a.login.width, a.login.height = a.width, a.height
		return a, nil
	}

	// Delegate to current page
	switch a.page {
	case pageLogin:
		var cmd tea.Cmd
		a.login, cmd = a.login.Update(msg)
		cmds = append(cmds, cmd)

	case pageDiary:
		var cmd tea.Cmd
		a.diary, cmd = a.diary.Update(msg)
		cmds = append(cmds, cmd)

	case pageAddMeal:
		var cmd tea.Cmd
		a.addMeal, cmd = a.addMeal.Update(msg)
		cmds = append(cmds, cmd)

	case pageDebug:
		var cmd tea.Cmd
		a.debug, cmd = a.debug.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) View() string {
	switch a.page {
	case pageLogin:
		return a.login.View()
	case pageDiary:
		return a.diary.View()
	case pageAddMeal:
		return a.addMeal.View()
	case pageDebug:
		return a.debug.View()
	}
	return ""
}

