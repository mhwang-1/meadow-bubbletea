# Telegram Bot

## Setup

1. Message [@BotFather](https://t.me/BotFather) on Telegram and create a new bot with `/newbot`. Copy the token it gives you.

2. Find your chat ID. The easiest way is to message [@userinfobot](https://t.me/userinfobot) -- it replies with your ID.

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

## Getting started

Send `/start` to the bot. This is the only slash command -- everything else is driven by inline buttons. The bot replies with the main day view for today, and all further interaction happens by tapping the buttons beneath each message.

If you send any text while no wizard flow is active, the bot also shows the main view.

## Main view

The main view shows today's timeboxes with their scheduled and completed tasks. Each timebox displays a time range, the assigned list name (if any), and a progress indicator (completed minutes / total minutes). Tasks are shown with `[done]` or `[ ]` markers, and breaks are labelled as `Break`.

Reserved timeboxes show "Reserved" with an optional note. Unassigned timeboxes show "(unassigned)". Archived timeboxes show `[archived]` with their completed tasks.

The heading includes the day label, the date in DD Mon YYYY format, and the week number.

Buttons below the main view:

- **Done** -- start the mark-done wizard
- **Archive** -- start the archive wizard
- **<< / >>** -- navigate to the previous or next day (labels show "Yesterday", "Today", "Tomorrow", or the weekday and date)
- **Lists** -- open the task lists view
- **New Timebox** -- start the new timebox wizard
- **Notes** -- open the notes view

## Marking tasks done

Press **Done** on the main view. The bot shows active timeboxes that have pending (non-completed, non-break) tasks. If only one timebox qualifies, it is selected automatically.

After selecting a timebox, the bot shows its pending tasks as buttons. If only one task is pending, it is selected automatically. Tapping a task marks it as done and returns to the main view.

Completed tasks can be undone from the main view via an undo callback, which unmarks the task and refreshes the view.

## Archiving timeboxes

Press **Archive** on the main view. The bot shows active, assigned, non-reserved timeboxes. If only one qualifies, it is selected automatically.

If all tasks in the timebox are completed, the timebox is archived immediately. If tasks are still pending, the bot asks for confirmation ("N task(s) still pending. Archive anyway?") with Yes/No buttons. Choosing Yes force-archives the timebox; choosing No returns to the main view.

Archived timeboxes can be unarchived via an unarchive callback, which restores them to active status.

## Creating timeboxes

Press **New Timebox** on the main view. The wizard proceeds through three steps:

1. **Select day** -- buttons for Today, Tomorrow, and the next five weekdays.
2. **Select start time** -- 30-minute intervals from 08:00 to 17:30, plus a "Custom..." button. Custom prompts you to type a time in HH:MM format.
3. **Select end time** -- offsets of 30, 60, 90, 120, 150, and 180 minutes from the start time, plus a "Custom..." button for a typed HH:MM entry.

After the timebox is created, the bot shows post-creation options:

- **Assign List** -- shows active task lists to assign to the new timebox.
- **Mark Reserved** -- prompts you to type an optional note (send `.` to skip), then marks the timebox as reserved.
- **Leave Unassigned** -- returns to the main view without assigning.

## Task lists

Press **Lists** on the main view. The lists view has three tabs, shown as a row of buttons at the top:

### Active tab

Shows all active task lists with their task count and total duration. Each list is a button. Tapping a list opens its detail view, which shows numbered tasks and summary stats. The detail view has these action buttons:

- **Assign** -- assign the list to an unassigned timebox today. Shows available timebox time slots as buttons.
- **Edit** -- enter edit mode, which shows each task with a "Remove" button beside it, plus an "Add Task" button. Adding a task prompts you to type the task in `description ~duration` format (e.g. `Review report ~24m`).
- **Archive List** -- archives the task list.
- **Delete List** -- deletes the task list (only succeeds if no completed tasks exist in any archive week).
- **Back to Lists** -- returns to the lists overview.

### History tab

Shows completed tasks for a given week, grouped by list slug. Each task shows its description, duration, and completion date. Use the **<< / >>** buttons to browse to earlier or later weeks.

### Archived tab

Shows archived task lists for a given week, with task count and total duration per list. Use the **<< / >>** buttons to browse weeks.

## Notes

Press **Notes** on the main view. The bot displays the contents of the global notes file.

- **Edit** -- prompts you to send the new notes content as a message. This replaces the existing notes entirely.
- **Cancel** -- returns to the main view.

## Running the bot

**Standalone:**

```bash
export TELEGRAM_BOT_TOKEN=your-token
export TELEGRAM_CHAT_ID=your-chat-id
./meadow serve
```

**Docker:** The bot starts automatically alongside the TUI if `TELEGRAM_BOT_TOKEN` is set in the environment. See the [Quick Start](../README.md#quick-start) in the README.
