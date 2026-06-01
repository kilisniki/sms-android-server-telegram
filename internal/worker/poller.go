package worker

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"sms-server/internal/adb"
	"sms-server/internal/bot"
	"sms-server/internal/db"
)

type Poller struct {
	adbClient *adb.Client
	db        *db.DB
	bot       *bot.Bot
	interval  time.Duration
	simNames  map[string]string
}

func New(adbClient *adb.Client, database *db.DB, telegramBot *bot.Bot, interval time.Duration, simNames map[string]string) *Poller {
	return &Poller{
		adbClient: adbClient,
		db:        database,
		bot:       telegramBot,
		interval:  interval,
		simNames:  simNames,
	}
}

func parseDate(timestampMs string) string {
	if timestampMs == "" {
		return ""
	}
	ts, err := strconv.ParseInt(timestampMs, 10, 64)
	if err != nil {
		return timestampMs
	}
	t := time.UnixMilli(ts)
	return t.Format("02.01.2006 15:04:05")
}

func (p *Poller) getSimName(subID string) string {
	if name, ok := p.simNames[subID]; ok && name != "" {
		return name
	}
	// Show slot number if no mapping found
	return "SIM " + subID
}

func (p *Poller) Start() {
	stateCh := make(chan bool)
	go p.adbClient.TrackDevices(stateCh)

	deviceOnline := false
	ticker := time.NewTicker(p.interval)

	for {
		select {
		case online := <-stateCh:
			if online != deviceOnline {
				deviceOnline = online
				if deviceOnline {
					log.Println("Device connected")
					p.bot.SendMessage("🔋 <b>Устройство подключено</b>")
					// Give ADB time to fully stabilize the USB connection
					time.Sleep(3 * time.Second)
					p.poll()
				} else {
					log.Println("Device disconnected")
					p.bot.SendMessage("🔌 <b>Устройство отключено</b>")
				}
			}
		case <-ticker.C:
			if deviceOnline {
				p.poll()
			}
		}
	}
}

func (p *Poller) poll() {
	p.pollSMS()
	p.pollCalls()
}

func (p *Poller) pollSMS() {
	messages, err := p.adbClient.QuerySMS()
	if err != nil {
		log.Printf("Error querying SMS: %v", err)
		return
	}

	for _, msg := range messages {
		if msg.ID == "" {
			continue
		}

		processed, err := p.db.IsSMSProcessed(msg.ID)
		if err != nil {
			log.Printf("DB Error: %v", err)
			continue
		}

		if !processed {
			simInfo := ""
			if msg.SubID != "" {
				simInfo = fmt.Sprintf("\n<b>SIM:</b> %s", p.getSimName(msg.SubID))
			}

			dateInfo := ""
			if parsedDate := parseDate(msg.Date); parsedDate != "" {
				dateInfo = fmt.Sprintf("\n<b>Время:</b> %s", parsedDate)
			}

			// Escape HTML special characters to avoid Telegram API errors
			body := strings.ReplaceAll(msg.Body, "&", "&amp;")
			body = strings.ReplaceAll(body, "<", "&lt;")
			body = strings.ReplaceAll(body, ">", "&gt;")

			text := fmt.Sprintf("✉️ <b>Новое SMS!</b>\n<b>От:</b> %s%s%s\n\n%s", msg.Address, simInfo, dateInfo, body)

			if err := p.bot.SendMessage(text); err != nil {
				log.Printf("Failed to send SMS to telegram: %v", err)
				continue // Do not mark as processed if sending failed
			}

			p.db.MarkSMSProcessed(msg.ID)
			log.Printf("Processed SMS %s from %s", msg.ID, msg.Address)
		}
	}
}

func (p *Poller) pollCalls() {
	calls, err := p.adbClient.QueryCalls()
	if err != nil {
		log.Printf("Error querying Calls: %v", err)
		return
	}

	for _, call := range calls {
		if call.ID == "" {
			continue
		}

		// Only process incoming (1) and missed (3) calls
		if call.Type != "1" && call.Type != "3" {
			continue
		}

		processed, err := p.db.IsCallProcessed(call.ID)
		if err != nil {
			log.Printf("DB Error: %v", err)
			continue
		}

		if !processed {
			simInfo := ""
			if call.SubID != "" {
				simInfo = fmt.Sprintf("\n<b>SIM:</b> %s", p.getSimName(call.SubID))
			}

			dateInfo := ""
			if parsedDate := parseDate(call.Date); parsedDate != "" {
				dateInfo = fmt.Sprintf("\n<b>Время:</b> %s", parsedDate)
			}

			var text string
			if call.Type == "3" {
				text = fmt.Sprintf("❌ <b>Пропущенный звонок</b>\n<b>От:</b> %s%s%s", call.Number, simInfo, dateInfo)
			} else {
				text = fmt.Sprintf("📞 <b>Входящий звонок</b>\n<b>От:</b> %s\n<b>Длительность:</b> %s сек%s%s", call.Number, call.Duration, simInfo, dateInfo)
			}

			if err := p.bot.SendMessage(text); err != nil {
				log.Printf("Failed to send Call to telegram: %v", err)
				continue
			}

			p.db.MarkCallProcessed(call.ID)
			log.Printf("Processed Call %s from %s (Type: %s)", call.ID, call.Number, call.Type)
		}
	}
}

