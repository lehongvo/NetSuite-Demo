package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Config - Telegram config, sẽ được user điền
const (
	TelegramBotToken = "REMOVED_TOKEN"
	TelegramChatID   = "6012401431"
	CheckInterval    = 1 * time.Minute
	OrderInfoPath    = "../orderInfo.json"
)

type OrderInfo struct {
	CorrectOrder  string `json:"correctOrder"`
	WrongOrder    string `json:"wrongOrder"`
	InternalID    string `json:"internalID"`
	Status        string `json:"status,omitempty"`
	CurrentTranID string `json:"currentTranId,omitempty"`
	IsSynced      bool   `json:"isSynced"`
}

type Stats struct {
	Total       int
	Updated     int
	Matched     int
	SkippedSame int
	Error       int
	Unexpected  int
	Wrong       int // chưa xử lý
}

func readStats() (*Stats, error) {
	data, err := os.ReadFile(OrderInfoPath)
	if err != nil {
		return nil, err
	}

	var orders []OrderInfo
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}

	stats := &Stats{Total: len(orders)}
	for _, o := range orders {
		switch o.Status {
		case "updated", "updated_unverified":
			stats.Updated++
		case "matched":
			stats.Matched++
		case "skipped_same":
			stats.SkippedSame++
		case "error", "error_undeposit", "error_update", "error_redeposit":
			stats.Error++
		case "unexpected":
			stats.Unexpected++
		default:
			stats.Wrong++ // "wrong" or empty = chưa xử lý
		}
	}
	return stats, nil
}

func sendTelegram(message string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", TelegramBotToken)
	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id":    {TelegramChatID},
		"text":       {message},
		"parse_mode": {"Markdown"},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned %d", resp.StatusCode)
	}
	return nil
}

func buildMessage(stats *Stats, prevUpdated int) string {
	done := stats.Updated + stats.Matched + stats.SkippedSame
	remaining := stats.Total - done - stats.Error - stats.Unexpected
	newUpdated := stats.Updated - prevUpdated
	percent := float64(done) * 100 / float64(stats.Total)

	msg := fmt.Sprintf("📊 *Marina Order Update Progress*\n\n"+
		"✅ Updated: *%d* (+%d new)\n"+
		"✅ Matched: %d\n"+
		"⏭ Skipped (same): %d\n"+
		"❌ Errors: %d\n"+
		"⚠️ Unexpected: %d\n"+
		"⏳ Remaining: *%d*\n\n"+
		"📈 Progress: *%.1f%%* (%d/%d)\n"+
		"🕐 Time: %s",
		stats.Updated, newUpdated,
		stats.Matched,
		stats.SkippedSame,
		stats.Error,
		stats.Unexpected,
		remaining,
		percent, done, stats.Total,
		time.Now().Format("15:04:05 02/01/2006"),
	)

	if remaining == 0 {
		msg += "\n\n🎉 *DONE! All orders processed.*"
	}

	return msg
}

func main() {
	fmt.Println("=== Telegram Notifier for Order Update ===")
	fmt.Printf("Checking %s every %v\n", OrderInfoPath, CheckInterval)
	fmt.Printf("Telegram Bot: %s\n", TelegramBotToken[:10]+"...")
	fmt.Printf("Chat ID: %s\n\n", TelegramChatID)

	prevUpdated := 0

	// First check immediately
	stats, err := readStats()
	if err != nil {
		log.Printf("Error reading stats: %v", err)
	} else {
		msg := buildMessage(stats, prevUpdated)
		fmt.Println(msg)
		if err := sendTelegram(msg); err != nil {
			log.Printf("Failed to send Telegram: %v", err)
		} else {
			fmt.Println("Sent to Telegram ✓")
		}
		prevUpdated = stats.Updated
	}

	// Then check every 5 minutes
	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		stats, err := readStats()
		if err != nil {
			log.Printf("Error reading stats: %v", err)
			continue
		}

		msg := buildMessage(stats, prevUpdated)
		fmt.Println(msg)

		if err := sendTelegram(msg); err != nil {
			log.Printf("Failed to send Telegram: %v", err)
		} else {
			fmt.Println("Sent to Telegram ✓")
		}

		prevUpdated = stats.Updated

		// Stop if all done
		done := stats.Updated + stats.Matched + stats.SkippedSame
		remaining := stats.Total - done - stats.Error - stats.Unexpected
		if remaining == 0 {
			fmt.Println("All orders processed. Exiting.")
			break
		}
	}
}
