# README Redesign Spec

## Goal

Restructure the README into a concise front page with detailed documentation split into separate files. Add three screenshots using mock data. Update all content to reflect recently added features.

## Audience

Potential users (deploying and using Meadow) and potential contributors (understanding and modifying the codebase).

## Structure

### README.md (~100-120 lines)

Thin front page with:

1. **Header** — one-paragraph description, Go badge
2. **Screenshots** — three screenshots stacked vertically with one-line captions:
   - Day view (Execute mode)
   - Week view (Execute mode)
   - Task list menu
3. **Features** — bullet list of key capabilities:
   - Timebox-first workflow (create time blocks, assign task lists, auto-sequence tasks)
   - Reserved time blocks (meetings, breaks, focus time)
   - Task list management with archive/delete lifecycle
   - Global freeform notes
   - Week-based archiving with unarchive support
   - Telegram bot for mobile access
   - Browser-based TUI via ttyd
   - Plain markdown storage (no database)
4. **Quick Start**
   - Docker (Recommended): clone, `docker compose up --build`, open browser (~5 lines)
   - Running Locally: `go build`, `./meadow tui` (~3 lines)
5. **Documentation** — links to:
   - `docs/tui.md` — Using the TUI
   - `docs/telegram.md` — Telegram Bot
   - `docs/data-format.md` — Data Format
6. **Environment Variables** — table with DATA, TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID, FONT_SIZE

### docs/tui.md

Full TUI documentation:

- **Modes and Views** — 2x2 table (Execute/Plan x Day/Week), Tab and Shift+Tab switching
- **Day View: Planning Your Day** — create (n), edit (Enter), delete (d) timeboxes; assign lists (/); reserved timeboxes (r) with optional notes; navigate days (Left/Right), today (t)
- **Day View: Executing Tasks** — task sequencing explanation (fills in order, breaks when next doesn't fit); mark done (x), undo done (u); archive (a), unarchive (U)
- **Week View** — what's displayed per day, navigation (Left/Right, Enter, t), week numbering (Sunday start, week containing 1 Jan = week 1)
- **Task List Menu** — open with /; Active tab (filter, select, archive with `a`, delete with `d`); Archived tab (Tab to toggle, browse by year/week, open in read-only mode); menu keyboard shortcuts
- **Task List Editor** — three sub-modes (Edit/Execute/Archive) cycled with Tab; Edit mode (nano-like, task format, comments, durations); Execute mode (mark done); Archive mode (browse completed, unmark for active lists, read-only for archived); editor shortcuts (Ctrl+O, Ctrl+X, Ctrl+K, Ctrl+U, Esc)
- **Global Notes** — open with `-`, loads DATA/notes.md, same editor controls
- **Keyboard Reference** — comprehensive table with Key / Action / Context columns covering all shortcuts: Tab, Shift+Tab, Left/Right, Up/Down, t, /, -, Enter, n, d, r, a, U, x, u, q/Ctrl+C, and menu/editor-specific keys

### docs/telegram.md

Full Telegram bot documentation:

- **Setup** — create bot via @BotFather, find chat ID via @userinfobot, set env vars, multiple chat IDs (comma-separated), unauthorised chats silently ignored
- **Viewing Your Day and Week** — /today (timeboxes, tasks, progress, inline Prev/Next), /week (week summary)
- **Managing Tasks** — /lists, /list {name}, /add {list} {desc} ~{time}, /done {task} (fuzzy match), inline Done buttons
- **Planning Timeboxes** — /plan {day} {start}-{end}, /assign {day} {time} {list}, /archive {day} {time}, inline Archive buttons
- **Command Reference** — table of all commands with descriptions
- **Day Formats** — today, tomorrow, mon-sun, YYYY-MM-DD
- **Running the Bot** — ./meadow serve standalone, Docker auto-starts if token set

### docs/data-format.md

Data format reference:

- **Task Lists** — markdown with frontmatter (name), one task per line (Description ~duration), duration formats (~24m, ~1h30m, ~2h), # comments, example file
- **Daily Timeboxes** — markdown with date frontmatter, ## sections with start-end times, fields (tasklist, status, tag, note, completed_tasks), statuses (unassigned, active, archived), tag "reserved" for time blockers, examples (regular, reserved, archived with completed tasks)
- **Global Notes** — DATA/notes.md, plain markdown, no frontmatter
- **Directory Layout** — full tree including notes.md, active/tasklists/, active/timeboxes/, archive/{YYYY}/{WW}/ with completed.md, timeboxes.md, and tasklists/ subdirectory
- **Conventions** — British English, DD Mon YYYY dates, Sunday-start weeks, slug generation (lowercase, spaces and / to hyphens)

## Screenshots

### Mock Data

Create temporary mock data in a dedicated directory (not the user's real data):

**Task lists:**
- "Engineering 04/2026" — tasks: "Fix login timeout bug ~30m", "Write API rate limiter ~1h", "Review pull requests ~45m", "Update error handling docs ~20m"
- "Marketing 04/2026" — tasks: "Review Q1 report ~24m", "Update landing page copy ~45m", "Draft newsletter ~30m", "Prepare campaign brief ~1h"

**Timeboxes for Wednesday 01 Apr 2026:**
- 09:00-11:00 assigned to engineering-04-2026 (active, one task marked done)
- 12:00-13:00 reserved with note "Lunch"
- 14:00-16:00 assigned to marketing-04-2026 (active)

**Additional days for week view (week 14, Sun 29 Mar – Sat 04 Apr 2026):**
- Monday 30 Mar: 09:00-12:00 engineering-04-2026 (archived), 13:00-15:00 marketing-04-2026 (active)
- Tuesday 31 Mar: 10:00-11:30 engineering-04-2026 (active)
- Wednesday 01 Apr: as above (the primary day view screenshot day)
- Thursday 02 Apr: 09:00-10:00 reserved "Team standup", 10:00-12:00 engineering-04-2026 (active)
- Friday 03 Apr: 14:00-16:00 marketing-04-2026 (active)

### Capture Method

1. Install VHS: `go install github.com/charmbracelet/vhs@latest`
2. Create mock data files in a temp directory
3. Write VHS tape files for each screenshot
4. Capture three PNGs:
   - `docs/screenshots/day-view.png` — Day view, Execute mode
   - `docs/screenshots/week-view.png` — Week view, Execute mode
   - `docs/screenshots/task-list-menu.png` — Slash menu overlay

### Placement

In README.md Screenshots section, stacked vertically:

```markdown
## Screenshots

![Day view — Execute mode](docs/screenshots/day-view.png)
*Day view: timeboxes with sequenced tasks, breaks, and a reserved time block*

![Week view](docs/screenshots/week-view.png)
*Week view: overview of all timeboxes across the week*

![Task list menu](docs/screenshots/task-list-menu.png)
*Task list menu: browse, filter, archive, and manage lists*
```

## What Changes

| File | Action |
|------|--------|
| README.md | Rewrite — thin front page with screenshots, features, quick start, doc links |
| docs/tui.md | New — full TUI documentation with all current features |
| docs/telegram.md | New — Telegram bot setup and usage |
| docs/data-format.md | New — file formats, directory layout, conventions |
| docs/screenshots/*.png | New — three VHS-captured screenshots with mock data |

## Content Migration

Current README content that moves to docs (not dropped):

| Current README Section | Moves To |
|------------------------|----------|
| Deployment (Docker Compose YAML, local build) | docs/tui.md gets a brief "Running" note; Docker detail stays in Quick Start but trimmed |
| Telegram Bot Setup (BotFather, chat ID) | docs/telegram.md — Setup section |
| Using the TUI (all subsections) | docs/tui.md |
| Using the Telegram Bot (all subsections) | docs/telegram.md |
| Data Format (all subsections) | docs/data-format.md |
| Environment Variables | Stays in README (quick reference) |

The detailed Docker Compose YAML example from the current README moves into docs/telegram.md (since its main purpose is showing env var configuration for the bot). The README Quick Start section uses a minimal 3-step Docker flow without the full YAML.

## What Doesn't Change

- CLAUDE.md (development guide, separate concern)
- All source code
- Existing design specs in docs/superpowers/specs/
