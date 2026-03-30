package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mhwang-1/meadow-bubbletea/internal/service"
)

// Bot wraps the Telegram Bot API with authorisation and wizard-based routing.
type Bot struct {
	api            *tgbotapi.BotAPI
	svc            *service.Service
	allowedChatIDs map[int64]bool
	sessions       *sessionStore
}

// NewBot creates a new Bot, initialises the API client, and populates the
// allowed chat ID set.
func NewBot(token string, chatIDs []int64, svc *service.Service) (*Bot, error) {
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
		svc:            svc,
		allowedChatIDs: allowed,
		sessions:       newSessionStore(),
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
			chatID := update.CallbackQuery.Message.Chat.ID
			if !b.allowedChatIDs[chatID] {
				continue
			}
			routeCallback(b, update.CallbackQuery)
			continue
		}

		// Handle regular messages.
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		if !b.allowedChatIDs[chatID] {
			continue
		}

		if update.Message.IsCommand() {
			// Only /start is supported.
			if update.Message.Command() == "start" {
				b.handleStartCommand(chatID)
			}
			continue
		}

		// Non-command text: route to wizard text input handler.
		b.handleTextInput(chatID, update.Message.Text)
	}

	return nil
}

// Stop gracefully stops the bot's long-polling loop.
func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}

// handleStartCommand sends the main view for today.
func (b *Bot) handleStartCommand(chatID int64) {
	if err := sendMainView(b, chatID, today(), 0); err != nil {
		log.Printf("Error sending main view: %v", err)
	}
}

// handleTextInput handles non-command text messages by checking the wizard
// session for the chat.
func (b *Bot) handleTextInput(chatID int64, text string) {
	ws := b.sessions.get(chatID)
	if ws == nil {
		// No active wizard session; send the main view.
		if err := sendMainView(b, chatID, today(), 0); err != nil {
			log.Printf("Error sending main view: %v", err)
		}
		return
	}

	handleWizardTextInput(b, chatID, ws, text)
}
