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
