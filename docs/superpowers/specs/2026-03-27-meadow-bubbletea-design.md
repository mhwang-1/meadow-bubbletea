# Meadow-BubbleTea — Design Specification

A personal task management TUI app built with Go and BubbleTea, served in the browser via ttyd, with Telegram bot integration. Database-free — all data stored as markdown files.

## 1. Overview

**Problem:** Managing tasks and time requires constant decision-making about what to do next. Existing tools either lack structure (plain to-do lists) or are over-engineered (full project management suites).

**Solution:** Meadow-BubbleTea uses a timebox-first approach. The user creates time blocks, assigns task lists to them, and the app automatically sequences tasks within each timebox. The user simply follows what comes next.

**Access methods:**
- **Browser TUI** — open `{IP}:34135` on any device for the full terminal interface
- **Telegram bot** — commands and inline buttons for on-the-go access

**Deployment:** Docker Compose, single container, UID:GID 1000:1000, behind Tailscale (no built-in auth).

## 2. User Interface

### 2.1 Mode & View Switching

- **Tab** switches between **Plan** and **Execute** modes
- **Shift+Tab** switches between **Day** and **Week** views
- Default on launch: **Day Execute** mode

### 2.2 Day Execute View

Displays today's timeboxes with their assigned tasks in a vertical timeline.

```
🌿 Meadow                    Plan [Execute]     Week [Day]     Fri, 27 Mar 2026 · W13

── 09:00 ── ── ── ── ── ── ── ── ── ── ── ──

│ 09:00-11:00 Work 03/2026 96/120
│   ✓ 09:00 Internal KD - Review Q1 report ~24m
│   ✓ 09:24 Visit Report - Draft intro ~24m
│   ▶ 09:48 Visit Report - Create maps for Lojing ~24m        ← current task (highlighted)
│   ○ 10:12 Visit Report - Finish slides #1-#4 ~24m
│   ⏸ 10:36 Break ~24m                                        ← auto-calculated

── 14:00 ── ── ── ── ── ── ── ── ── ── ── ──

│ 14:00-15:30 Journal 12/2025 90/90 ✓ archived                ← dimmed, read-only

── 16:00 ── ── ── ── ── ── ── ── ── ── ── ──

│ 16:00-17:00 (unassigned) 0/60                                ← press / to assign

↑↓ navigate · Enter select · / task lists · Tab plan/execute · Shift+Tab day/week · a archive · d delete · x mark done
```

**Key elements:**
- `{scheduled minutes}/{timebox duration}` shown in timebox header (e.g., `96/120`)
- Completed tasks: `✓`, current task: `▶` (highlighted), pending: `○`
- Break is auto-calculated when remaining timebox time < next task duration
- Archived timeboxes are dimmed and read-only
- Unassigned timeboxes prompt to assign a task list
- `←→` navigates to previous/next day

### 2.3 Day Plan View

Compact timebox summaries (no individual tasks shown). Used for creating, editing, assigning, archiving, and deleting timeboxes.

```
│ 09:00-11:00 Work 03/2026 96/120
│   4 tasks · 96m scheduled · 24m break

│ 16:00-17:00 (unassigned) 0/60                ← dashed border, selected
│   Press / to assign · Enter to edit times · d to delete
```

**Timebox creation (two methods):**
1. **Navigate + key:** Arrow to a time slot, press `n` to create a timebox at that time
2. **Manual entry:** Press `n` anywhere, type start and end times (e.g., `22:00-23:30`)

**Minimum timebox duration:** 15 minutes.

**Shortcuts:** `n` new timebox · `Enter` edit times · `/` assign task list · `a` archive · `d` delete · `←→` prev/next day

### 2.4 Week Execute View

Seven-column grid (Sun–Sat) with timebox summaries per day, plus a task list overview below.

```
🌿 Meadow                    Plan [Execute]     [Week] Day     W13 · 22–28 Mar 2026

 Sun 22    Mon 23       Tue 24       Wed 25        Thu 26         Fri 27        Sat 28
 ───────   ───────      ───────      ───────       ───────        ───────       ───────
  —        09-11 Jrn ✓  09-12 Work   22-23:30 Work 10-11 Gitea   09-11 Work ▶   —
           14-16 Work   144/180      72/90         48/60          96/120
           96/120                                  19-21 Work     14-15:30 Jrn ✓
                                                   96/120         16-17 (unasgn)

── Task Lists (W13) ──────────────────────────────────────────────────────────
 Gitea            2h48m   0h✓   0h⏱    2h48m○
 Journal 12/2025  6h48m   1h36m✓  0h⏱  5h12m○
 Journal 01/2026  27h12m  0h✓   0h⏱    27h12m○
 Work 03/2026     8h      5h12m✓  2h48m⏱  0h○

↑↓←→ navigate · Enter open day · Tab plan/execute · Shift+Tab day view · / task lists
```

**Task list overview:**
- Shown below the calendar
- Sorted alphabetically
- Columns: total time, completed (✓), scheduled (⏱), unscheduled (○)
- All stats are for the current week only

### 2.5 Week Plan View

Same 7-column grid as week execute, but used for planning. Navigate to a day and press Enter to switch to day plan view for that day.

### 2.6 Slash Menu (Task List Picker)

Pressing `/` from any view opens a searchable overlay listing all task lists alphabetically.

```
┌─ Task Lists ──────────────────────────────┐
│ > ___________  (type to filter)           │
│                                           │
│   Gitea                                   │
│     Total: 2h48m | Sched: 0h | Unsched:  │
│     2h48m | Done: 0h                      │
│                                           │
│   Journal 12/2025                         │
│     Total: 6h48m | Sched: 0h | Unsched:  │
│     6h48m | Done: 1h36m                   │
│                                           │
│   Work 03/2026                            │
│     Total: 8h | Sched: 2h48m | Unsched:  │
│     0h | Done: 5h12m                      │
│                                           │
│   [+ New Task List]                       │
│                                           │
│ Enter select · n new · Esc close          │
└───────────────────────────────────────────┘
```

**Context-dependent behaviour:**
- From **Plan mode** on an unassigned timebox: selecting a task list assigns it to that timebox
- From **any view** otherwise: selecting a task list opens the task list editor

### 2.7 Task List Editor

Entered by selecting a task list from the `/` menu (or assigning to a timebox then pressing Enter on it). Three modes switched with **Tab**:

#### 2.7.1 Edit Mode (default)

Full nano-like text editor for modifying the task list.

**Task format:** `{description} ~{time}`

- Time supports minutes (`~24m`) and hours+minutes (`~1h30m`, `~2h`)
- Lines starting with `#` are commented out (excluded from time calculations)
- Invalid format shows an inline error message
- Stats bar: `Total: 8h | Scheduled: 2h48m | Unscheduled: 5h12m | Completed: 0h`

**Keybindings:** Ctrl+O save · Ctrl+X close · Ctrl+K cut line · Ctrl+U paste · arrow keys for cursor movement

#### 2.7.2 Execute Mode

Checklist view for marking tasks done spontaneously (without creating a timebox).

- Navigate with ↑↓, press Enter or `x` to mark done
- Completed tasks move to the archive for the current week
- Commented (`#`) items are hidden
- Press Esc to close

#### 2.7.3 Archive Mode

View completed tasks grouped by year and week, with year/week pill navigation at the top.

```
[2026]  [2025]                                    ← year pills
[W13] [W12] [W11] [W10]                          ← week pills for selected year

── 2026 · Week 13 (22–28 Mar) ──
  ✓ Internal KD - Review Q1 report ~24m          Fri, 27 Mar
  ✓ Visit Report - Draft intro ~24m              Fri, 27 Mar

── 2026 · Week 12 (15–21 Mar) ──
  ✓ Internal KD - Submit expense reports ~36m    Mon, 16 Mar
  ✓ Visit Report - Complete Lojing 2 maps ~24m   Wed, 18 Mar
```

- Press `u` on a task to unmark it (moves back to the active task list)
- Press Esc to close

## 3. Timebox Lifecycle

### 3.1 States

- **Unassigned** — created with start/end time, no task list assigned
- **Active** — task list assigned, tasks are sequenced within the timebox
- **Archived** — frozen; completed/done tasks stay, open items redistributed

### 3.2 Task Sequencing

When a task list is assigned to a timebox:
1. Tasks are placed in order from the task list
2. Start times are calculated: first task starts at timebox start, each subsequent task starts after the previous one's duration
3. If remaining timebox time < next task's duration, a "Break" is inserted for the remaining time
4. `{scheduled}/{total}` shows how many minutes of tasks fit vs total timebox duration

**Multiple timeboxes with the same task list:** Tasks fill the earlier timebox first. Once it's full (remaining time < next task), subsequent tasks go to the next timebox chronologically.

### 3.3 Archiving

When a timebox is archived:
- The timebox is marked as archived (displayed dimmed, read-only)
- Completed tasks (marked done) stay in the archived timebox
- Open/pending tasks are removed and redistributed to the next chronological timebox assigned to the **same task list**
- If no future timebox exists for that task list, open tasks return to the unscheduled pool

### 3.4 Deleting

When a timebox is deleted:
- The timebox is permanently removed (irreversible)
- Open/pending tasks are redistributed to the next timebox for the same task list (same as archiving)
- No record of the timebox remains

### 3.5 Marking Tasks Done

Tasks can be marked done from:
1. **Day Execute view** — navigate to a task in a timebox, press `x`
2. **Task list Execute mode** — spontaneous completion without a timebox

Completed tasks are moved to the archive for the current week (based on when they were marked done).

## 4. Data Model

### 4.1 Directory Structure

```
DATA/
├── active/
│   ├── tasklists/
│   │   ├── work-03-2026.md
│   │   ├── journal-12-2025.md
│   │   └── gitea.md
│   └── timeboxes/
│       ├── 2026-W13-fri.md
│       ├── 2026-W13-thu.md
│       └── ...
└── archive/
    └── 2026/
        └── 13/
            ├── completed.md
            └── timeboxes.md
```

### 4.2 Task List File (`active/tasklists/{slug}.md`)

```markdown
---
name: Work 03/2026
---
Internal KD - Ask Claude how to draw a polyshape ~24m
Visit Report - Create the maps for Lojing 1 24 ~24m
Visit Report - Finish slides #1-#4 ~24m
Visit Report - Finish slides #5-#8 and send them off ~36m
Internal KD - File Malaysia travel claim ~36m
Internal KD - Take out the boards ~24m
# Blossom Agritech - Call John ~24m
```

- YAML frontmatter with display name
- One task per line: `{description} ~{duration}`
- Lines starting with `#` are commented out
- Duration: `~24m`, `~1h30m`, `~2h`
- Filename slug derived from name (lowercase, spaces→hyphens)

### 4.3 Daily Timeboxes File (`active/timeboxes/{YYYY}-W{WW}-{day}.md`)

```markdown
---
date: 2026-03-27
---
## 09:00-11:00
tasklist: work-03-2026
status: active

## 14:00-15:30
tasklist: journal-12-2025
status: archived
completed:
  - Internal KD - Review Q1 report ~24m
  - Visit Report - Draft intro ~24m

## 16:00-17:00
status: unassigned
```

- One file per day (only created when timeboxes exist for that day)
- Each `##` section is a timebox with start-end times
- `tasklist` references the task list file slug
- `status`: `unassigned`, `active`, or `archived`
- Archived timeboxes store their completed tasks inline

### 4.4 Archive Files (`archive/{YYYY}/{WW}/`)

**`completed.md`** — completed tasks grouped by task list:
```markdown
## work-03-2026
Internal KD - Review Q1 report ~24m | 2026-03-27
Visit Report - Draft intro ~24m | 2026-03-27

## journal-12-2025
Write reflection on week 12 ~30m | 2026-03-23
```

**`timeboxes.md`** — archived timebox records (for historical week views):
```markdown
## 2026-03-27
### 14:00-15:30
tasklist: journal-12-2025
completed:
  - Write reflection on week 12 ~30m
  - Organise notes from reading ~30m
  - Plan next week's journal prompts ~30m
```

### 4.5 Week Numbering

- Weeks start on **Sunday**
- Week numbers follow ISO 8601 convention but with Sunday as the first day
- This week (27 Mar 2026) is **Week 13 of 2026**

## 5. Application Architecture

### 5.1 Go Module Structure

```
meadow-bubbletea/
├── cmd/meadow/main.go              # Entry point (subcommands: tui, serve)
├── internal/
│   ├── model/                       # BubbleTea models
│   │   ├── root.go                  # Root model (mode/view switching, routing)
│   │   ├── calendar.go              # Day/week calendar views
│   │   ├── timebox.go               # Timebox creation/editing
│   │   ├── tasklist_menu.go         # Slash menu (task list picker)
│   │   ├── tasklist_editor.go       # Edit/Archive/Execute modes
│   │   └── editor.go               # Nano-like text editor component
│   ├── store/                       # File I/O, markdown parsing
│   │   ├── tasklist.go              # Read/write task list files
│   │   ├── timebox.go              # Read/write timebox files
│   │   ├── archive.go              # Read/write archive files
│   │   └── lock.go                 # File locking for concurrent access
│   ├── domain/                      # Core types and business logic
│   │   ├── task.go                  # Task, TaskList types
│   │   ├── timebox.go              # Timebox type, scheduling logic
│   │   └── week.go                 # Week calculation (Sunday start)
│   ├── ui/                          # Shared UI components
│   │   ├── styles.go               # Lip Gloss styles (light theme)
│   │   └── components.go           # Reusable UI pieces (status bar, pills)
│   └── telegram/                    # Telegram bot
│       ├── bot.go                   # Bot setup, polling, auth
│       ├── handlers.go              # Command handlers
│       └── views.go                 # Message formatting, inline keyboards
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

### 5.2 BubbleTea Model Hierarchy

```
RootModel
├── mode: Plan | Execute
├── view: Day | Week
├── CalendarModel (day/week rendering)
│   └── TimeboxModel[] (individual timeboxes)
├── TaskListMenuModel (/ overlay)
└── TaskListEditorModel
    ├── EditorModel (nano-like, Edit mode)
    ├── ExecuteModel (checklist, Execute mode)
    └── ArchiveModel (completed tasks, Archive mode)
```

### 5.3 Subcommands

- `meadow tui` — starts the BubbleTea TUI (what ttyd spawns per browser connection)
- `meadow serve` — starts the Telegram bot as a long-running daemon

### 5.4 Concurrent Access

Both the TUI (multiple ttyd sessions) and the Telegram bot read/write the same files. File-level locking (`flock`) ensures safe concurrent writes. The `store/lock.go` module provides a `WithLock(path, fn)` helper.

### 5.5 Dependencies

- `charmbracelet/bubbletea` — TUI framework
- `charmbracelet/lipgloss` — styling (light theme)
- `charmbracelet/bubbles` — reusable components (viewport, textinput)
- `gopkg.in/yaml.v3` — YAML frontmatter parsing
- `go-telegram-bot-api/telegram-bot-api/v5` — Telegram bot API

## 6. Telegram Bot

### 6.1 Authentication

- Environment variables: `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`
- All messages from non-whitelisted chat IDs are silently ignored

### 6.2 Commands

| Command | Description |
|---------|-------------|
| `/today` | Show today's timeboxes with inline action buttons |
| `/week` | Show current week overview |
| `/lists` | Show all task lists with stats |
| `/list {name}` | Show tasks in a specific list |
| `/done {task}` | Mark a task as done (fuzzy match) |
| `/add {list} {description} ~{time}` | Add a task to a list |
| `/plan {day} {start}-{end}` | Create a timebox |
| `/assign {day} {time} {list}` | Assign a task list to a timebox |
| `/archive {day} {time}` | Archive a timebox |
| `/help` | Show available commands |

### 6.3 Inline Buttons

Messages from `/today` and `/week` include inline keyboard buttons:

- **Task buttons:** `[✓ Done]` next to each current/pending task
- **Navigation:** `[← Prev Day]` `[Next Day →]` `[Week View]`
- **Timebox actions:** `[Archive]` `[Delete]`
- **Task list:** `[View List]` `[Add Task]`

### 6.4 Notifications (optional, future)

Potential future feature: the bot sends a notification when a timebox starts or when it's time to move to the next task. Not in initial scope.

## 7. Docker Deployment

### 7.1 Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o meadow ./cmd/meadow

FROM alpine:3.19
RUN apk add --no-cache ttyd
COPY --from=builder /build/meadow /app/meadow
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh
USER 1000:1000
ENTRYPOINT ["/app/entrypoint.sh"]
```

### 7.2 Entrypoint Script

```bash
#!/bin/sh
# Start Telegram bot daemon (if token is set)
if [ -n "$TELEGRAM_BOT_TOKEN" ]; then
  /app/meadow serve &
fi

# Start ttyd serving the TUI
exec ttyd --port 34135 --writable /app/meadow tui
```

### 7.3 docker-compose.yml

```yaml
services:
  meadow:
    build: .
    ports:
      - "34135:34135"
    user: "1000:1000"
    environment:
      - TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN:-}
      - TELEGRAM_CHAT_ID=${TELEGRAM_CHAT_ID:-}
    volumes:
      - data:/data
    restart: unless-stopped

volumes:
  data:
    name: DATA
```

## 8. UI Theme

- **Light mode** colour scheme throughout
- Uses Lip Gloss for consistent styling
- British English spelling and date format (e.g., "Fri, 27 Mar 2026")
- Colour coding: gold/amber for active timeboxes, purple for archived, blue for unassigned, green for completed

## 9. Verification

### 9.1 Build & Run

```bash
go build ./cmd/meadow
./meadow tui                    # Test TUI locally
docker compose up --build       # Test full deployment
```

### 9.2 Functional Tests

1. Create a task list via `/` menu → verify file created in `DATA/active/tasklists/`
2. Add tasks in edit mode → verify file content matches expected format
3. Create a timebox in plan mode → verify file created in `DATA/active/timeboxes/`
4. Assign task list to timebox → verify tasks are sequenced correctly
5. Mark tasks done → verify they move to archive
6. Archive a timebox → verify open tasks redistribute to next timebox for same list
7. Delete a timebox → verify it's gone and open tasks redistribute
8. Week view → verify task list overview stats are correct
9. Telegram: `/today` → verify message with inline buttons
10. Telegram: mark done via button → verify task archived

### 9.3 Edge Cases

- Timebox shorter than any task (all break)
- Last timebox for a task list archived (open tasks return to unscheduled pool)
- Empty task list assigned to timebox
- Concurrent TUI + Telegram writes to same file
- Week boundary (Sunday start)
