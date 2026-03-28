# Meadow BubbleTea

A personal task management TUI app built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea), using a **timebox-first** workflow — you create time blocks in your day, assign task lists to them, and the app sequences your tasks automatically. Access it from a browser via [ttyd](https://github.com/tsl0922/ttyd) or from your phone via Telegram. All data is stored as plain markdown files.

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)

## Screenshots

![Day view — Execute mode](docs/screenshots/day-view.png)
*Day view: timeboxes with sequenced tasks, breaks, and a reserved time block*

![Week view](docs/screenshots/week-view.png)
*Week view: overview of all timeboxes across the week*

![Task list menu](docs/screenshots/task-list-menu.png)
*Task list menu: browse, filter, archive, and manage lists*

## Features

- **Timebox-first workflow** — create time blocks, assign task lists, and the app sequences tasks automatically with breaks when the next task doesn't fit
- **Reserved time blocks** — mark timeboxes for meetings, breaks, or focus time without assigning tasks
- **Task list management** — create, edit, archive, and delete lists with full lifecycle tracking
- **Global notes** — freeform markdown notes accessible with a single keypress
- **Week-based archiving** — completed tasks and timeboxes archived by year and week, with unarchive support
- **Telegram bot** — view your day, manage tasks, and plan timeboxes from your phone
- **Browser-based TUI** — served via [ttyd](https://github.com/tsl0922/ttyd) for access from any browser
- **Plain markdown storage** — all data stored as readable markdown files, no database required

## Quick Start

### Docker (Recommended)

```bash
git clone https://github.com/mhwang-1/meadow-bubbletea.git
cd meadow-bubbletea
docker compose up --build -d
```

Open `http://localhost:34135` in your browser. Edit `docker-compose.yml` to change the bind address, data volume, or add Telegram bot credentials.

### Running Locally

Requires Go 1.24+.

```bash
go build ./cmd/meadow
./meadow tui                     # Start the TUI (uses ./data by default)
DATA=/path/to/data ./meadow tui  # Custom data directory
```

## Documentation

- **[Using the TUI](docs/tui.md)** — modes, views, keyboard shortcuts, task list editor, notes
- **[Telegram Bot](docs/telegram.md)** — setup, commands, inline actions
- **[Data Format](docs/data-format.md)** — file formats, directory layout, conventions

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATA` | No | `./data` | Path to the data directory |
| `TELEGRAM_BOT_TOKEN` | For bot | — | Telegram bot token from @BotFather |
| `TELEGRAM_CHAT_ID` | For bot | — | Comma-separated authorised Telegram chat IDs |
| `FONT_SIZE` | No | `16` | Browser font size (ttyd) |
