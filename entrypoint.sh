#!/bin/sh
# Start Telegram bot daemon (if token is set)
if [ -n "$TELEGRAM_BOT_TOKEN" ]; then
  /app/meadow serve &
fi

# Start ttyd serving the TUI
exec ttyd --port 34135 --writable \
  -t fontSize=${FONT_SIZE:-16} \
  -t 'theme={"background":"#FAF5FF","foreground":"#3B0764","cursor":"#7C3AED","selectionBackground":"#E9D5FF"}' \
  -t 'titleFixed=Meadow BubbleTea' \
  /app/meadow tui
