package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"sms-server/internal/adb"
	"sms-server/internal/bot"
	"sms-server/internal/db"
	"sms-server/internal/worker"
)

func main() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if botToken == "" || chatID == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set")
	}

	pollIntervalSec := 15
	if intervalStr := os.Getenv("POLL_INTERVAL_SECONDS"); intervalStr != "" {
		if val, err := strconv.Atoi(intervalStr); err == nil && val > 0 {
			pollIntervalSec = val
		}
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/sqlite.db"
	}

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	adbClient := adb.NewClient()
	telegramBot := bot.New(botToken, chatID)

	poller := worker.New(adbClient, database, telegramBot, time.Duration(pollIntervalSec)*time.Second)

	log.Println("Starting SMS Android Server...")
	go poller.Start()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
}
