package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mhwang-1/meadow-bubbletea/internal/api"
	"github.com/mhwang-1/meadow-bubbletea/internal/model"
	"github.com/mhwang-1/meadow-bubbletea/internal/service"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
	"github.com/mhwang-1/meadow-bubbletea/internal/telegram"
)

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: meadow <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  tui     Start the terminal UI")
	fmt.Fprintln(os.Stderr, "  serve   Start the Telegram bot server")
}

func main() {
	cmd := "tui"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "tui":
		dataDir := os.Getenv("DATA")
		if dataDir == "" {
			dataDir = "./data"
		}
		s := store.NewStore(dataDir)
		m := model.NewRootModel(s)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	case "serve":
		token := os.Getenv("TELEGRAM_BOT_TOKEN")
		if token == "" {
			fmt.Fprintln(os.Stderr, "TELEGRAM_BOT_TOKEN is required")
			os.Exit(1)
		}

		chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
		if chatIDStr == "" {
			fmt.Fprintln(os.Stderr, "TELEGRAM_CHAT_ID is required")
			os.Exit(1)
		}

		var chatIDs []int64
		for _, part := range strings.Split(chatIDStr, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid chat ID %q: %v\n", part, err)
				os.Exit(1)
			}
			chatIDs = append(chatIDs, id)
		}

		dataDir := os.Getenv("DATA")
		if dataDir == "" {
			dataDir = "./data"
		}
		s := store.NewStore(dataDir)
		svc := service.New(s)

		// Listen for termination signals for graceful shutdown.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Optionally start the REST API server if API_TOKEN is set.
		var apiServer *api.Server
		apiToken := os.Getenv("API_TOKEN")
		if apiToken != "" {
			apiPort := 34136
			if portStr := os.Getenv("API_PORT"); portStr != "" {
				p, err := strconv.Atoi(portStr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Invalid API_PORT %q: %v\n", portStr, err)
					os.Exit(1)
				}
				apiPort = p
			}

			apiServer = api.New(svc, apiToken, apiPort)
			go func() {
				if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
					log.Printf("API server error: %v", err)
				}
			}()
		}

		bot, err := telegram.NewBot(token, chatIDs, svc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating bot: %v\n", err)
			os.Exit(1)
		}

		// Run the bot in a goroutine so the main goroutine can wait for signals.
		botDone := make(chan error, 1)
		go func() {
			botDone <- bot.Run()
		}()

		fmt.Println("Starting Telegram bot...")

		// Wait for a termination signal or bot exit.
		select {
		case sig := <-sigCh:
			log.Printf("Received %v, shutting down...", sig)
		case err := <-botDone:
			if err != nil {
				log.Printf("Bot error: %v", err)
			}
		}

		// Graceful shutdown: stop bot and API server.
		bot.Stop()

		if apiServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := apiServer.Shutdown(ctx); err != nil {
				log.Printf("API server shutdown error: %v", err)
			}
		}

		log.Println("Shutdown complete.")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}
