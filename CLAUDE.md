# Meadow-BubbleTea — Development Guide

## Project Overview

Personal task management TUI app using a timebox-first approach. Built with Go and BubbleTea, served via ttyd in the browser, with Telegram bot and REST API integration. All data stored as markdown files.

## Build & Run

```bash
go build ./cmd/meadow          # Build the binary
go test ./...                   # Run all tests
./meadow tui                    # Start the TUI (uses ./data by default)
./meadow serve                  # Start Telegram bot + REST API (requires env vars)
DATA=/path/to/data ./meadow tui # Custom data directory
```

## Architecture

```
cmd/meadow/main.go              # Entry point (tui + serve subcommands)
internal/
  domain/                        # Core types and business logic (no external deps)
    task.go                      # Task, TaskList, duration parsing, Slugify
    timebox.go                   # Timebox, SequenceTasks, ScheduledTask
    week.go                      # Sunday-start week numbering, date formatting
  store/                         # File I/O with flock-based locking
    tasklist.go                  # Store struct, task list CRUD, archive/delete operations
    timebox.go                   # Daily timebox file read/write
    archive.go                   # Archive completed tasks and timebox records
    notes.go                     # Global notes.md read/write
    lock.go                      # WithLock using syscall.Flock
  service/                       # Business logic layer (used by API and Telegram bot)
    service.go                   # Service struct with sync.Mutex, constructor
    timebox.go                   # Timebox CRUD (create, edit, delete, assign, reserve)
    execution.go                 # Mark done, unmark done, archive/unarchive timebox
    tasklist.go                  # Task list CRUD and archive
    query.go                     # Read-only queries (day view, week summary, history, stats)
    notes.go                     # Notes read/write
  api/                           # REST API (HTTP handlers over service layer)
    server.go                    # HTTP server, Bearer token auth middleware
    routes.go                    # Route registration (24 endpoints)
    tasklists.go                 # Task list endpoints
    timeboxes.go                 # Timebox endpoints
    archive.go                   # Archive query endpoints
    notes.go                     # Notes endpoints
  model/                         # BubbleTea TUI models
    root.go                      # RootModel — mode/view switching, key routing, notes editor
    calendar.go                  # Day and week view rendering
    timebox.go                   # Timebox CRUD, mark done, archive
    tasklist_menu.go             # Slash (/) menu overlay with Active/Archived tabs
    tasklist_editor.go           # Task list editor (Edit/Execute/Archive modes, read-only)
    editor.go                    # Nano-like multi-line text editor component
  telegram/                      # Telegram bot (button-driven wizard flows)
    bot.go                       # Bot setup, long polling, chat ID auth, service integration
    wizard.go                    # Wizard state management, callback routing
    keyboards.go                 # Inline keyboard builders for wizard steps
    handlers.go                  # Wizard flow handlers (done, archive, lists, new timebox, etc.)
    views.go                     # Message formatting (no emojis)
  ui/                            # Shared UI components
    styles.go                    # Lip Gloss light-mode colour palette and styles
    components.go                # Header, status bar, pills, time markers
```

## Key Design Decisions

- **Week numbering:** Weeks start on Sunday. Do NOT use Go's `ISOWeek()`. Custom calculation in `domain/week.go`. The week containing 1 Jan belongs to that year.
- **Task sequencing:** Tasks fill timeboxes in order. Break = remaining time when next task doesn't fit. Multiple timeboxes with the same list: earlier fills first.
- **File locking:** Every store write uses `WithLock(path, fn)` with `flock`. Reads don't need locking (atomic writes via temp+rename).
- **Slug generation:** `Slugify()` — lowercase, spaces and `/` to hyphens, strip other special chars. "Work 03/2026" becomes "work-03-2026".
- **Frontmatter:** Parsed manually (no YAML library). Simple `---` delimiters with `key: value` lines.
- **Light mode only:** All styles use a light colour scheme. Colours defined in `ui/styles.go`.
- **Task list lifecycle:** Lists can be archived (moved to `archive/{YYYY}/{WW}/tasklists/`) or deleted (only if no completed tasks exist in any archive week). Archived lists are browsable in a read-only editor via the Archived tab in the slash menu.
- **Notes:** Global `DATA/notes.md` file opened with `-` key; reuses the `EditorModel` component.
- **Service layer:** `internal/service/` consolidates business logic from the TUI. The Telegram bot and REST API call the service layer; the TUI calls the store directly (unchanged). A `sync.Mutex` serialises all write operations.
- **REST API:** `internal/api/` provides HTTP endpoints over the service layer. Uses Go 1.22+ `http.ServeMux` with method routing. Bearer token auth via `API_TOKEN` env var.
- **Telegram wizard:** The Telegram bot uses button-driven wizard flows (inline keyboards) instead of slash commands. All state is encoded in callback data or a short-lived session map.

## Data Directory Layout

```
DATA/
  notes.md                       # Global freeform notes (opened with - key)
  active/
    tasklists/{slug}.md          # YAML frontmatter + one task per line
    timeboxes/{YYYY}-W{WW}-{day}.md  # Daily timeboxes
  archive/
    {YYYY}/{WW}/
      completed.md               # Tasks grouped by list slug with dates
      timeboxes.md               # Archived timebox records
      tasklists/{slug}.md        # Archived task list files
```

## Conventions

- British English spelling and DD Mon YYYY date format
- Task format: `{description} ~{duration}` (e.g. `Review report ~24m`)
- Duration formats: `~24m`, `~1h30m`, `~2h`
- Commented lines start with `#`
- Timebox statuses: `unassigned`, `active`, `archived`

## Environment Variables

- `DATA` — data directory path (default: `./data`)
- `TELEGRAM_BOT_TOKEN` — Telegram bot token (required for `serve`)
- `TELEGRAM_CHAT_ID` — comma-separated allowed chat IDs (required for `serve`)
- `API_TOKEN` — Bearer token for REST API authentication (enables API server when set)
- `API_PORT` — REST API port (default: `34136`)

## Testing

Unit tests cover the domain layer (duration parsing, week calculation, task sequencing), store layer (read/write round-trips for all file types), service layer (all business operations), and API layer (HTTP handler tests with auth). Run with `go test ./...`.

## Docker

```bash
docker compose up --build       # Build and start (ports 34135 + 34136)
```

The container runs ttyd on port 34135 and optionally starts the Telegram bot if `TELEGRAM_BOT_TOKEN` is set. The REST API starts on port 34136 if `API_TOKEN` is set.
