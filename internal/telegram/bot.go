package telegram

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hwang/meadow-bubbletea/internal/store"
)

// Bot wraps the Telegram Bot API with authorisation and command routing.
type Bot struct {
	api            *tgbotapi.BotAPI
	store          *store.Store
	allowedChatIDs map[int64]bool
}

// NewBot creates a new Bot, initialises the API client, and populates the
// allowed chat ID set.
func NewBot(token string, chatIDs []int64, s *store.Store) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	allowed := make(map[int64]bool, len(chatIDs))
	for _, id := range chatIDs {
		allowed[id] = true
	}

	return &Bot{
		api:            api,
		store:          s,
		allowedChatIDs: allowed,
	}, nil
}

// Run starts the long-polling loop. It blocks until an error occurs.
func (b *Bot) Run() error {
	log.Printf("Authorised as @%s", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		// Handle callback queries (inline button presses).
		if update.CallbackQuery != nil {
			if !b.allowedChatIDs[update.CallbackQuery.Message.Chat.ID] {
				continue
			}
			handleCallback(b, update.CallbackQuery)
			continue
		}

		// Handle regular messages.
		if update.Message == nil || !update.Message.IsCommand() {
			continue
		}

		if !b.allowedChatIDs[update.Message.Chat.ID] {
			continue
		}

		b.routeCommand(update)
	}

	return nil
}

// routeCommand dispatches an incoming command to the appropriate handler.
func (b *Bot) routeCommand(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	cmd := update.Message.Command()
	args := strings.TrimSpace(update.Message.CommandArguments())

	var msg tgbotapi.MessageConfig
	var err error

	switch cmd {
	case "today":
		msg, err = b.handleToday(chatID, args)
	case "week":
		msg, err = b.handleWeek(chatID)
	case "lists":
		msg, err = b.handleLists(chatID)
	case "list":
		msg, err = b.handleList(chatID, args)
	case "done":
		msg, err = b.handleDone(chatID, args)
	case "add":
		msg, err = b.handleAdd(chatID, args)
	case "plan":
		msg, err = b.handlePlan(chatID, args)
	case "assign":
		msg, err = b.handleAssign(chatID, args)
	case "archive":
		msg, err = b.handleArchive(chatID, args)
	case "help":
		msg, err = b.handleHelp(chatID)
	default:
		msg = tgbotapi.NewMessage(chatID, "Unknown command. Try /help")
	}

	if err != nil {
		log.Printf("Error handling /%s: %v", cmd, err)
		msg = tgbotapi.NewMessage(chatID, "Error: "+err.Error())
	}

	msg.ParseMode = "HTML"
	if _, sendErr := b.api.Send(msg); sendErr != nil {
		log.Printf("Error sending message: %v", sendErr)
	}
}
