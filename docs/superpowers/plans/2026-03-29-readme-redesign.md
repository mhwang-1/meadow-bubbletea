# README Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure README into a concise front page with three doc files and three screenshots using mock data.

**Architecture:** Thin README.md links to docs/tui.md, docs/telegram.md, docs/data-format.md. Screenshots captured via VHS with mock data in a temporary directory, stored in docs/screenshots/.

**Tech Stack:** Markdown, VHS (charmbracelet/vhs), Go (for building the app)

---

### Task 1: Create docs/tui.md

**Files:**
- Create: `docs/tui.md`

- [ ] **Step 1: Write docs/tui.md**

```markdown
# Using the TUI

## Modes and Views

The TUI has two **modes** and two **views**, giving you four combinations:

|              | Day                    | Week                  |
|--------------|------------------------|-----------------------|
| **Execute**  | Work through tasks     | Week overview         |
| **Plan**     | Create/edit timeboxes  | Week overview         |

- Press `Tab` to switch between **Execute** and **Plan** mode.
- Press `Shift+Tab` to switch between **Day** and **Week** view.

The current mode and view are shown in the header.

## Day View: Planning Your Day

Switch to **Plan** mode (`Tab`) in **Day** view:

1. **Create a timebox** — press `n`, then type the time range (e.g. `09:00-11:00`) and press `Enter`. Timeboxes must be at least 15 minutes and cannot overlap.
2. **Assign a task list** — select the timebox with `Up`/`Down`, press `/` to open the task list picker, type to filter, and press `Enter` to assign.
3. **Edit timebox times** — select a timebox and press `Enter`, then type new times.
4. **Delete a timebox** — select a timebox and press `d`, then confirm with `y`.
5. **Reserve a time block** — select an unassigned timebox and press `r` to mark it as reserved (for meetings, breaks, focus time). You'll be prompted for an optional note. Press `r` again to unreserve.

Reserved timeboxes cannot have task lists assigned to them. They appear with a filled square marker (■) and amber styling.

Navigate between days with `Left`/`Right`. Jump to today with `t`.

## Day View: Executing Tasks

Switch to **Execute** mode (`Tab`) in **Day** view:

- Tasks from the assigned list are sequenced into each timebox automatically. If a task doesn't fit in the remaining time, a break is inserted.
- Press `x` to mark the current task as done. It moves to the completed section and is removed from the task list.
- Press `u` to undo the most recently completed task in the selected timebox. The task is restored to the task list.
- Press `a` to archive a completed timebox. Completed tasks are recorded in the weekly archive. If there are pending tasks, you'll be asked to confirm.
- Press `U` to unarchive a previously archived timebox. The timebox is reactivated; completed tasks remain in the archive.

## Week View

Press `Shift+Tab` to see the week overview. It shows a seven-column grid with one column per day (Sunday through Saturday):

- Each timebox is shown with its time range, assigned list name (or "■ Rsvd" for reserved), and a status indicator (▶ active, ✓ archived).
- Below the grid, a **Task Lists** section shows all lists with weekly stats: total time, completed, scheduled, and unscheduled.

Navigate days with `Left`/`Right`. Press `Enter` to jump into a specific day's Day view. Press `t` to jump to today.

Weeks start on **Sunday**. Week 1 of a year is the week containing 1 January.

## Task List Menu

Press `/` from any main view to open the task list picker overlay.

### Active Tab

The default tab shows all active task lists:

- **Type to filter** — case-insensitive substring match on list names.
- **Select a list** — `Up`/`Down` to navigate, `Enter` to select. In Plan mode with an unassigned timebox, this assigns the list. Otherwise, it opens the task list editor.
- **Create a new list** — arrow down to `[+ New Task List]` and press `Enter`, then type the name.
- **Archive a list** — press `a` on the selected list. Only allowed if the list is not assigned to any active timebox. The list is moved to `archive/{YYYY}/{WW}/tasklists/`.
- **Delete a list** — press `d` on the selected list. Only allowed if the list has no completed tasks in any archive week and is not assigned to an active timebox.

### Archived Tab

Press `Tab` to switch to the Archived tab:

- **Year pills** at the top — `Left`/`Right` to navigate between years.
- **Week headers** with date ranges (e.g. `W13 · 22–28 Mar 2026`).
- **Archived lists** grouped by week, sorted alphabetically.
- Press `Enter` to open an archived list in read-only Archive mode.

| Key | Action | Tab |
|-----|--------|-----|
| Type | Filter lists by name | Active |
| `Up` / `Down` | Navigate list | Both |
| `Enter` | Select / view list | Both |
| `a` | Archive selected list | Active |
| `d` | Delete selected list | Active |
| `Tab` | Switch Active / Archived tab | Both |
| `Left` / `Right` | Navigate year pills | Archived |
| `Esc` | Close menu | Both |

## Task List Editor

Select a list from the menu and press `Enter` (or select it in Execute mode) to open the full-screen editor. The editor has three sub-modes, cycled with `Tab`:

### Edit Mode

A nano-like text editor for the raw task list:

- Each line is one task in the format: `Description ~duration` (e.g. `Review report ~24m`)
- Lines starting with `#` are comments (excluded from scheduling)
- Duration formats: `~24m`, `~1h30m`, `~2h`

### Execute Mode

Mark tasks as done from within the editor:

- `Up`/`Down` to navigate tasks
- `Enter` or `x` to mark the selected task as done
- Stats are shown at the bottom (total, scheduled, unscheduled, completed)

### Archive Mode

Browse completed tasks grouped by year and week:

- `Up`/`Down` to navigate year pills
- `Left`/`Right` to navigate week pills
- `j`/`k` to select individual completed tasks
- `u` to unmark (restore) a completed task back to the active list

When viewing an archived task list (opened from the Archived tab), Archive mode is **read-only** — the `u` (unmark) key is disabled and a `(read-only)` label is shown.

### Editor Shortcuts

| Key | Action |
|-----|--------|
| `Tab` | Cycle sub-mode (Edit / Execute / Archive) |
| `Ctrl+O` | Save |
| `Ctrl+X` | Close |
| `Ctrl+K` | Cut line |
| `Ctrl+U` | Paste line |
| `Esc` | Close (prompts to save if modified) |

## Global Notes

Press `-` from any view to open a full-screen editor for your global notes file (`DATA/notes.md`). The editor uses the same controls as the task list editor (Ctrl+O to save, Ctrl+X or Esc to close).

## Keyboard Reference

### Main View

| Key | Action | Context |
|-----|--------|---------|
| `Tab` | Toggle mode (Execute / Plan) | All views |
| `Shift+Tab` | Toggle view (Day / Week) | All views |
| `Left` / `Right` | Navigate days or weekdays | All views |
| `Up` / `Down` | Navigate timeboxes | Day view |
| `t` | Jump to today | All views |
| `/` | Open task list menu | All views |
| `-` | Open global notes editor | All views |
| `Enter` | Enter day (Week); edit times (Plan+Day) | Week / Plan+Day |
| `n` | Create new timebox | Plan mode, Day view |
| `d` | Delete timebox | Plan mode, Day view |
| `r` | Toggle reserved time block | Plan mode, Day view |
| `x` | Mark current task done | Execute mode, Day view |
| `u` | Undo most recent done task | Execute mode, Day view |
| `a` | Archive timebox | Day view |
| `U` | Unarchive timebox | Day view |
| `q` / `Ctrl+C` | Quit | All views |

### Task List Menu

| Key | Action |
|-----|--------|
| Type | Filter lists by name |
| `Up` / `Down` | Navigate list |
| `Enter` | Select list |
| `a` | Archive list (Active tab) |
| `d` | Delete list (Active tab) |
| `Tab` | Switch Active / Archived tab |
| `Left` / `Right` | Navigate year pills (Archived tab) |
| `Esc` | Close menu |

### Task List Editor

| Key | Action |
|-----|--------|
| `Tab` | Cycle sub-mode (Edit / Execute / Archive) |
| `Ctrl+O` | Save |
| `Ctrl+X` | Close |
| `Ctrl+K` | Cut line |
| `Ctrl+U` | Paste line |
| `Up` / `Down` | Navigate tasks (Execute); navigate years (Archive) |
| `Left` / `Right` | Navigate weeks (Archive) |
| `j` / `k` | Select task (Archive) |
| `x` / `Enter` | Mark task done (Execute) |
| `u` | Unmark task (Archive, active lists only) |
| `Esc` | Close (prompts to save if modified) |
```

- [ ] **Step 2: Commit**

```bash
git add docs/tui.md
git commit -m "Add TUI documentation (docs/tui.md)"
```

---

### Task 2: Create docs/telegram.md

**Files:**
- Create: `docs/telegram.md`

- [ ] **Step 1: Write docs/telegram.md**

```markdown
# Telegram Bot

## Setup

1. Message [@BotFather](https://t.me/BotFather) on Telegram and create a new bot with `/newbot`. Copy the token it gives you.

2. Find your chat ID. The easiest way is to message [@userinfobot](https://t.me/userinfobot) — it replies with your ID.

3. Set the environment variables and start the bot:

```bash
export TELEGRAM_BOT_TOKEN=123456:ABC-DEF...
export TELEGRAM_CHAT_ID=987654321
./meadow serve
```

Or add them to your `.env` file for Docker Compose (see the [Quick Start](../README.md#quick-start) in the README).

You can authorise multiple chat IDs by separating them with commas:

```bash
TELEGRAM_CHAT_ID=111111,222222,333333
```

The bot silently ignores messages from unauthorised chats.

## Viewing Your Day and Week

**`/today`** — shows today's timeboxes with scheduled tasks, completed tasks, and progress. Use the inline `Prev` and `Next` buttons to navigate between days.

**`/week`** — shows a summary of the current week with all timeboxes.

## Managing Tasks

**`/lists`** — shows all task lists with task count and total duration.

**`/list work`** — shows tasks in a list (fuzzy matches "work" to e.g. "Work 03/2026").

**`/add work Review Q1 figures ~30m`** — adds a task to the matching list.

**`/done review`** — marks the first task matching "review" as done (searches across all lists). The `/today` view also shows inline "Done" buttons for quick task completion.

## Planning Timeboxes

**`/plan today 09:00-11:00`** — creates a timebox. Timeboxes must be at least 15 minutes and cannot overlap.

**`/assign today 09:00 work`** — assigns a task list to the timebox starting at 09:00.

**`/archive today 09:00`** — archives the timebox starting at 09:00. The `/today` view also shows inline "Archive" buttons.

## Command Reference

| Command | Description |
|---------|-------------|
| `/today` | Show today's timeboxes with inline navigation |
| `/week` | Show current week summary |
| `/lists` | Show all task lists with stats |
| `/list {name}` | Show tasks in a list (fuzzy match) |
| `/done {task}` | Mark a task as done (fuzzy match) |
| `/add {list} {desc} ~{time}` | Add a task to a list |
| `/plan {day} {start}-{end}` | Create a timebox |
| `/assign {day} {time} {list}` | Assign a list to a timebox |
| `/archive {day} {time}` | Archive a timebox |
| `/help` | Show available commands |

**Day formats for `{day}`:** `today`, `tomorrow`, `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun`, or `YYYY-MM-DD`.

## Running the Bot

**Standalone:**

```bash
export TELEGRAM_BOT_TOKEN=your-token
export TELEGRAM_CHAT_ID=your-chat-id
./meadow serve
```

**Docker:** The bot starts automatically alongside the TUI if `TELEGRAM_BOT_TOKEN` is set in the environment. See the [Quick Start](../README.md#quick-start) in the README.
```

- [ ] **Step 2: Commit**

```bash
git add docs/telegram.md
git commit -m "Add Telegram bot documentation (docs/telegram.md)"
```

---

### Task 3: Create docs/data-format.md

**Files:**
- Create: `docs/data-format.md`

- [ ] **Step 1: Write docs/data-format.md**

````markdown
# Data Format

All data is stored as plain markdown files in the data directory (`./data` by default, configurable via the `DATA` environment variable).

## Task Lists

Stored in `active/tasklists/{slug}.md`. Each file has YAML-like frontmatter and one task per line:

```markdown
---
name: Work 03/2026
---
Review Q1 report ~24m
Create visit maps ~1h30m
# This task is commented out and excluded from scheduling ~24m
```

- **Format:** `Description ~duration`
- **Duration formats:** `~24m`, `~1h30m`, `~2h`
- **Comments:** Lines starting with `#` are excluded from scheduling
- **Slugs:** Derived from the name — lowercase, spaces and `/` to hyphens, other special characters stripped. "Work 03/2026" becomes `work-03-2026`.

## Daily Timeboxes

Stored per day in `active/timeboxes/{YYYY}-W{WW}-{Day}.md`. Each timebox is a `##` section:

```markdown
---
date: 2026-04-01
---

## 09:00-11:00
tasklist: engineering-04-2026
status: active
completed:
  - Fix login timeout bug ~30m

## 12:00-13:00
tag: reserved
note: Lunch
status: active

## 14:00-16:00
tasklist: marketing-04-2026
status: active

## 17:00-18:00
status: unassigned
```

### Timebox Fields

| Field | Description |
|-------|-------------|
| `tasklist` | Slug of the assigned task list (absent for unassigned/reserved) |
| `status` | `unassigned`, `active`, or `archived` |
| `tag` | `reserved` for time blockers (absent otherwise) |
| `note` | Optional text for reserved timeboxes (absent otherwise) |
| `completed` | List of tasks marked done during this timebox |

## Global Notes

Stored at `notes.md` in the data directory root. Plain markdown with no frontmatter.

## Directory Layout

```
data/
  notes.md                           # Global freeform notes
  active/
    tasklists/
      work-03-2026.md                # Task list files (slug derived from name)
      journal-03-2026.md
    timeboxes/
      2026-W14-Wed.md                # Daily timebox files
      2026-W14-Thu.md
  archive/
    2026/
      13/                            # Year/week number
        completed.md                 # Archived completed tasks
        timeboxes.md                 # Archived timebox records
        tasklists/
          work-02-2026.md            # Archived task list files
```

## Conventions

- **Spelling:** British English throughout
- **Date format:** DD Mon YYYY (e.g. 29 Mar 2026)
- **Week numbering:** Weeks start on Sunday. Week 1 of a year is the week containing 1 January. Do not use ISO week numbering.
- **Slug generation:** Lowercase, spaces and `/` replaced with hyphens, other special characters stripped.
````

- [ ] **Step 2: Commit**

```bash
git add docs/data-format.md
git commit -m "Add data format documentation (docs/data-format.md)"
```

---

### Task 4: Create mock data and capture screenshots

**Files:**
- Create: `docs/screenshots/` directory
- Create: mock data files in a temp directory
- Create: VHS tape files
- Create: `docs/screenshots/day-view.png`
- Create: `docs/screenshots/week-view.png`
- Create: `docs/screenshots/task-list-menu.png`

- [ ] **Step 1: Install VHS**

```bash
go install github.com/charmbracelet/vhs@latest
```

Verify: `vhs --version` should print a version number.

If VHS is not available or fails, fall back to running the TUI with mock data and using a terminal screenshot tool, or create the screenshots manually.

- [ ] **Step 2: Build the app**

```bash
go build -o meadow-screenshot ./cmd/meadow
```

- [ ] **Step 3: Create mock data directory and files**

Create a temporary directory with mock data. The directory structure must match what the app expects:

```bash
mkdir -p /tmp/meadow-screenshots/active/tasklists
mkdir -p /tmp/meadow-screenshots/active/timeboxes
mkdir -p /tmp/meadow-screenshots/archive/2026/14
```

**File: /tmp/meadow-screenshots/active/tasklists/engineering-04-2026.md**

```markdown
---
name: Engineering 04/2026
---
Write API rate limiter ~1h
Review pull requests ~45m
Update error handling docs ~20m
```

**File: /tmp/meadow-screenshots/active/tasklists/marketing-04-2026.md**

```markdown
---
name: Marketing 04/2026
---
Review Q1 report ~24m
Update landing page copy ~45m
Draft newsletter ~30m
Prepare campaign brief ~1h
```

**File: /tmp/meadow-screenshots/active/timeboxes/2026-W14-Wed.md**

```markdown
---
date: 2026-04-01
---

## 09:00-11:00
tasklist: engineering-04-2026
status: active
completed:
  - Fix login timeout bug ~30m

## 12:00-13:00
tag: reserved
note: Lunch
status: active

## 14:00-16:00
tasklist: marketing-04-2026
status: active
```

**File: /tmp/meadow-screenshots/active/timeboxes/2026-W14-Mon.md**

```markdown
---
date: 2026-03-30
---

## 09:00-12:00
tasklist: engineering-04-2026
status: archived
completed:
  - Set up CI pipeline ~45m
  - Refactor auth middleware ~1h

## 13:00-15:00
tasklist: marketing-04-2026
status: active
```

**File: /tmp/meadow-screenshots/active/timeboxes/2026-W14-Tue.md**

```markdown
---
date: 2026-03-31
---

## 10:00-11:30
tasklist: engineering-04-2026
status: active
```

**File: /tmp/meadow-screenshots/active/timeboxes/2026-W14-Thu.md**

```markdown
---
date: 2026-04-02
---

## 09:00-10:00
tag: reserved
note: Team standup
status: active

## 10:00-12:00
tasklist: engineering-04-2026
status: active
```

**File: /tmp/meadow-screenshots/active/timeboxes/2026-W14-Fri.md**

```markdown
---
date: 2026-04-03
---

## 14:00-16:00
tasklist: marketing-04-2026
status: active
```

- [ ] **Step 4: Write VHS tape for day view screenshot**

**File: /tmp/meadow-screenshots/day-view.tape**

The app opens in Execute mode, Day view showing today. Navigate to Wed 01 Apr 2026 (the day with the richest mock data). Calculate the number of `Right` key presses from the current date — e.g. if today is 29 Mar 2026, press `Right` 3 times.

```
Output docs/screenshots/day-view.png
Set Shell "bash"
Set Width 900
Set Height 600
Set FontSize 14
Set Theme { "name": "Meadow", "background": "#FAF5FF", "foreground": "#3B0764", "cursor": "#7C3AED", "selectionBackground": "#E9D5FF" }

Type "DATA=/tmp/meadow-screenshots ./meadow-screenshot tui"
Enter
Sleep 1s
Right
Sleep 200ms
Right
Sleep 200ms
Right
Sleep 500ms
Screenshot docs/screenshots/day-view.png
```

- [ ] **Step 5: Write VHS tape for week view screenshot**

**File: /tmp/meadow-screenshots/week-view.tape**

Navigate to the same week and switch to Week view (Shift+Tab):

```
Output docs/screenshots/week-view.png
Set Shell "bash"
Set Width 1100
Set Height 600
Set FontSize 14
Set Theme { "name": "Meadow", "background": "#FAF5FF", "foreground": "#3B0764", "cursor": "#7C3AED", "selectionBackground": "#E9D5FF" }

Type "DATA=/tmp/meadow-screenshots ./meadow-screenshot tui"
Enter
Sleep 1s
Right
Sleep 200ms
Right
Sleep 200ms
Right
Sleep 200ms
Shift+Tab
Sleep 500ms
Screenshot docs/screenshots/week-view.png
```

- [ ] **Step 6: Write VHS tape for task list menu screenshot**

**File: /tmp/meadow-screenshots/task-list-menu.tape**

Open the task list menu (/):

```
Output docs/screenshots/task-list-menu.png
Set Shell "bash"
Set Width 900
Set Height 600
Set FontSize 14
Set Theme { "name": "Meadow", "background": "#FAF5FF", "foreground": "#3B0764", "cursor": "#7C3AED", "selectionBackground": "#E9D5FF" }

Type "DATA=/tmp/meadow-screenshots ./meadow-screenshot tui"
Enter
Sleep 1s
Type "/"
Sleep 500ms
Screenshot docs/screenshots/task-list-menu.png
```

- [ ] **Step 7: Run VHS to capture screenshots**

```bash
mkdir -p docs/screenshots
vhs /tmp/meadow-screenshots/day-view.tape
vhs /tmp/meadow-screenshots/week-view.tape
vhs /tmp/meadow-screenshots/task-list-menu.tape
```

Verify each PNG exists and looks correct. If VHS fails or the screenshots don't look right, adjust the tape files (timing, dimensions, navigation).

- [ ] **Step 8: Clean up**

```bash
rm -f meadow-screenshot
rm -rf /tmp/meadow-screenshots
```

- [ ] **Step 9: Commit**

```bash
git add docs/screenshots/
git commit -m "Add TUI screenshots with mock data"
```

---

### Task 5: Rewrite README.md

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Replace README.md content**

```markdown
# Meadow BubbleTea

A personal task management app built around **timeboxing** — you create time blocks in your day, assign task lists to them, and the app sequences your tasks automatically. Access it from a browser or Telegram. All data is stored as plain markdown files.

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
git clone https://github.com/hwang/meadow-bubbletea.git
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
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "Rewrite README as concise front page with doc links"
```

---

### Task 6: Verify and final commit

**Files:**
- All files from previous tasks

- [ ] **Step 1: Verify all links work**

Check that all relative links in README.md resolve correctly:

```bash
test -f docs/tui.md && echo "tui.md OK" || echo "tui.md MISSING"
test -f docs/telegram.md && echo "telegram.md OK" || echo "telegram.md MISSING"
test -f docs/data-format.md && echo "data-format.md OK" || echo "data-format.md MISSING"
test -f docs/screenshots/day-view.png && echo "day-view.png OK" || echo "day-view.png MISSING"
test -f docs/screenshots/week-view.png && echo "week-view.png OK" || echo "week-view.png MISSING"
test -f docs/screenshots/task-list-menu.png && echo "task-list-menu.png OK" || echo "task-list-menu.png MISSING"
```

All six should print "OK".

- [ ] **Step 2: Verify README line count**

```bash
wc -l README.md
```

Expected: approximately 70-90 lines (under the 120-line target).

- [ ] **Step 3: Review the full README**

```bash
cat README.md
```

Confirm: header, screenshots, features, quick start, documentation links, environment variables — all present and no broken markdown.

- [ ] **Step 4: Run tests to ensure nothing is broken**

```bash
go test ./...
```

Expected: all tests pass. The documentation changes should not affect any tests.
