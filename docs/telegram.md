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
