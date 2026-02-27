# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

yazio-cli is a terminal UI (TUI) for the YAZIO fitness/nutrition app, built by reverse-engineering YAZIO's undocumented API. Written in Go using the Bubble Tea TUI framework.

## Build & Run

```bash
go build -o yazio-cli .   # Build binary
./yazio-cli               # Run
./yazio-cli --refresh     # Refresh stored tokens (for cron use)
```

No test suite or linter is configured. Standard `go vet ./...` applies.

## Architecture

### State Machine (tui/app.go)

The app is a Bubble Tea multi-page state machine with four pages:
- `pageLogin` (0) — Login form
- `pageDiary` (1) — Main food diary view
- `pageAddMeal` (2) — Add/edit meal form
- `pageDebug` (3) — Raw API debug interface (accessed via `?` key)

`tui/app.go` owns global state (token, user profile, window size, product cache) and routes `tea.Msg` to the active page model.

### Key Files

| File | Role |
|------|------|
| `main.go` | Entry point; loads auth config, starts Bubble Tea |
| `tui/app.go` | App state machine, page routing, global keybindings |
| `tui/diary.go` | Food diary view (~564 lines); day navigation, selection, delete |
| `tui/addmeal.go` | Product search + add/edit meal form (~664 lines) |
| `tui/styles.go` | Shared lipgloss styles and color scheme |
| `tui/login.go` | Login form component |
| `tui/debug.go` | Raw API testing UI |
| `internal/api/client.go` | HTTP client for YAZIO API; handles 401 + token refresh |
| `internal/auth/auth.go` | Token storage at `~/.config/yazio-cli/config.json` |
| `internal/models/types.go` | API response structs and `DiaryEntry` model |

### API Client (internal/api/client.go)

Base URL: `https://yzapi.yazio.com` (v15 endpoints)

The `Client` struct handles automatic 401 recovery by refreshing the access token transparently. Key methods: `Login`, `RefreshAccessToken`, `GetConsumedItems`, `GetDailyNutrients`, `GetGoals`, `SearchProducts`, `AddConsumedItem`, `DeleteConsumedItem`, `GetProfile`.

`diary.go` loads diary data in parallel goroutines (consumed items + goals + nutrients) using `sync.WaitGroup`.

### Authentication

- Config stored at `~/.config/yazio-cli/config.json` (perms 0600)
- OAuth client ID/secret are hardcoded defaults, overridable via `YAZIO_CLIENT_ID` / `YAZIO_CLIENT_SECRET` env vars

### Data Flow

```
main.go → tui.New() → app.go (state machine)
  ├─ login.go → api.Login()
  ├─ diary.go → api.GetConsumedItems / GetGoals / GetDailyNutrients
  └─ addmeal.go → api.SearchProducts / AddConsumedItem / DeleteConsumedItem
```

## API Reference

`yazio-api.yaml` — Swagger spec for the YAZIO API (useful for understanding request/response shapes)
`yazio-api-research.md` — Community notes on undocumented API endpoints
