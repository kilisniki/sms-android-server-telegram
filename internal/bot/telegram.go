package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Bot struct {
	Token  string
	ChatID string
}

func New(token, chatID string) *Bot {
	return &Bot{
		Token:  token,
		ChatID: chatID,
	}
}

type telegramRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (b *Bot) SendMessage(htmlText string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.Token)

	reqBody := telegramRequest{
		ChatID:    b.ChatID,
		Text:      htmlText,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status: %d", resp.StatusCode)
	}

	return nil
}
