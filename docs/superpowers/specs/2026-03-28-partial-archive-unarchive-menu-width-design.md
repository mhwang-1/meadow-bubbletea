# Partial Archive, Unarchive, and Menu Width

**Date:** 28 Mar 2026

## Context

Three related improvements to the timebox workflow:

1. **Partial archive** — Currently `canArchiveTimebox()` blocks archiving when undone tasks remain. Users want to archive incomplete timeboxes, letting undone tasks flow to the next timebox for the same list.
2. **Unarchive** — No way to revert an archived timebox. Users need a quick undo.
3. **Menu width** — The task list menu (slash menu) is 70 chars wide; the stats line (`Total: xxhxxm | …`) needs 74 chars and overflows to a second line.

A fourth change (shortcut bar reordering) is included to accommodate the new `U unarchive` key and improve readability.

## 1. Partial Archive

### Behaviour

- When archiving a timebox with **all tasks done**: archive immediately (unchanged).
- When archiving with **undone tasks**: show confirmation dialog — `"N task(s) undone — archive anyway? (y/n)"`.
- On confirm: archive proceeds as normal. Undone tasks stay in the task list file and `SequenceTasks` picks them up for the next timebox.
- On cancel: no-op.

### Changes

**`internal/model/root.go`**
- Add `confirmMessage string` field to `RootModel` (after `confirmAction` at line 58).

**`internal/model/timebox.go`**
- **`canArchiveTimebox()`** (line 374): change return signature from `(bool, string)` to `(bool, int, string)`. The `int` is the count of pending tasks. When pending tasks exist, return `(true, N, "")` instead of `(false, 0, "pending tasks remain")`. Other invalid states (`already archived`, `reserved`, `unassigned`, `task list unavailable`) still return `(false, 0, reason)`.
- **`handleTimeboxArchive()`** (line 295): after calling `canArchiveTimebox`, check `pendingCount`. If >0, set `m.showConfirm = true`, `m.confirmMessage`, and `m.confirmAction = m.executeTimeboxArchive`. If 0, call `m.executeTimeboxArchive()` directly.
- **New `executeTimeboxArchive()`**: extract lines 319–371 (the four archive steps) into this method. Re-reads daily timeboxes from disk (same pattern as `executeTimeboxDelete()`).
- **`renderConfirmLine()`** (line 701): return `m.confirmMessage + " (y/n)"` when `confirmMessage` is set; fall back to `"Are you sure? (y/n)"`.
- **`clearConfirm()`** (line 691): also reset `m.confirmMessage = ""`.

## 2. Unarchive Timebox

### Behaviour

- **Key:** `U` (Shift-U) in day view, both Execute and Plan modes.
- **Action:** reactivate an archived timebox; remove the timebox record from `archive/{YYYY}/{WW}/timeboxes.md`.
- Completed tasks stay completed (in `tb.CompletedTasks` and `completed.md`).
- No confirmation dialog — the operation is safe and reversible.

### Changes

**`internal/model/root.go`**
- Add `case "U":` in the key switch (after the `"a"` case at line 239). Guard: `m.view == ViewDay && m.maxTimeboxCount() > 0`. Call `m.handleTimeboxUnarchive()`.

**`internal/model/timebox.go`**
- **New `handleTimeboxUnarchive()`**: read daily timeboxes, validate selected timebox is archived, set `tb.Status = domain.StatusActive`, write daily timeboxes, then call `removeArchivedTimeboxRecord()`.
- **New `removeArchivedTimeboxRecord(year, week int, tb domain.Timebox)`**: read archived timeboxes, find and remove the matching record (match on date + start hour/minute + end hour/minute + task list slug), write back. Best-effort — silently no-ops if record is missing.

## 3. Menu Width

### Change

**`internal/model/tasklist_menu.go`** line 516: change `menuWidth := 70` to `menuWidth := 80`.

Inner width becomes 76 chars, fitting the max stats line of 74 chars (`Total: xxhxxm | Scheduled: xxhxxm | Unscheduled: xxhxxm | Done: xxhxxm`) with 2 chars margin.

## 4. Shortcut Bar Reordering

### Changes

**`internal/model/root.go`** — `shortcutBarText()` (line 517):

Grouped logically: navigation → primary actions → lifecycle → overlays → mode/view → quit. Labels shortened to fit.

- **Execute mode (day):** `↑↓ navigate · x done · u undo · a archive · U unarchive · / lists · - notes · t today · Tab plan · Shift+Tab week · q quit`
- **Plan mode (day):** `↑↓ navigate · n new · Enter edit · r reserve · / assign · d delete · a archive · U unarchive · - notes · t today · Tab execute · Shift+Tab week · q quit`
- **Week view:** unchanged.

## Files Modified

| File | Changes |
|------|---------|
| `internal/model/root.go` | `confirmMessage` field, `U` key binding, shortcut bar text |
| `internal/model/timebox.go` | `canArchiveTimebox` signature, `handleTimeboxArchive` refactor, `executeTimeboxArchive`, `handleTimeboxUnarchive`, `removeArchivedTimeboxRecord`, `renderConfirmLine`, `clearConfirm` |
| `internal/model/tasklist_menu.go` | `menuWidth` constant |

## Verification

1. `go build ./cmd/meadow` — compiles cleanly.
2. `go test ./...` — all existing tests pass.
3. Manual TUI testing:
   - Create a timebox with a task list, mark some tasks done, leave others undone. Press `a` — confirm dialog appears with correct count. Press `y` — timebox archives, undone tasks appear in the next timebox for the same list.
   - On the archived timebox, press `U` — timebox reactivates, tasks re-sequence.
   - Open `/` menu — stats line fits within the menu border, no line wrapping.
   - Check shortcut bar text in Execute and Plan modes — new ordering and `U unarchive` visible.
