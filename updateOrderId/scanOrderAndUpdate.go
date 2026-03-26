package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	gns "git02.smartosc.com/production/platform-connector/go-netsuite"
)

type OrderInfo struct {
	CorrectOrder  string `json:"correctOrder"`
	WrongOrder    string `json:"wrongOrder"`
	InternalID    string `json:"internalID"`
	Status        string `json:"status,omitempty"` // "matched", "wrong", "error", "updated", "skipped_same"
	CurrentTranID string `json:"currentTranId,omitempty"`
	IsSynced      bool   `json:"isSynced"`
}

func retryWithBackoff(maxRetries int, operation func() error) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err
		errMsg := err.Error()
		isRetryable := strings.Contains(errMsg, "concurrent request limit exceeded") ||
			strings.Contains(errMsg, "Request blocked") ||
			strings.Contains(errMsg, "connection reset") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "503") ||
			strings.Contains(errMsg, "ECONNREFUSED") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "temporary failure") ||
			strings.Contains(errMsg, "SSLHandshake")
		if isRetryable {
			waitTime := time.Duration(2<<uint(i)) * time.Second
			fmt.Printf("     Retryable error, retry %d/%d after %v: %s\n", i+1, maxRetries, waitTime, errMsg)
			time.Sleep(waitTime)
			continue
		}
		return err
	}
	return lastErr
}

func main() {
	if err := loadEnv("../.env"); err != nil {
		log.Printf("Warning: Failed to load .env: %v", err)
	}

	fmt.Println("=== Scan & Update Order TranId on NetSuite ===")
	fmt.Println()

	jsonFile, err := os.ReadFile("orderInfo.json")
	if err != nil {
		log.Fatalf("Failed to read orderInfo.json: %v", err)
	}

	var orders []OrderInfo
	if err := json.Unmarshal(jsonFile, &orders); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	fmt.Printf("Found %d orders to check\n\n", len(orders))

	client, err := newNetsuiteSOAPClient()
	if err != nil {
		log.Fatalf("Failed to create SOAP client: %v", err)
	}

	matchedCount := 0
	updatedCount := 0
	errorCount := 0
	skippedSameCount := 0
	startTime := time.Now()

	for i := range orders {
		order := &orders[i]

		fmt.Printf("[%d/%d] InternalID: %s | correct: %s | wrong: %s\n",
			i+1, len(orders), order.InternalID, order.CorrectOrder, order.WrongOrder)

		// Skip already processed
		if order.Status == "updated" || order.Status == "matched" || order.Status == "skipped_same" {
			fmt.Printf("  Skipped (already %s)\n\n", order.Status)
			if order.Status == "matched" {
				matchedCount++
			} else if order.Status == "skipped_same" {
				skippedSameCount++
			} else {
				updatedCount++
			}
			continue
		}

		// Skip if correctOrder == wrongOrder (already correct, no API call needed)
		if order.CorrectOrder == order.WrongOrder {
			fmt.Printf("  Skipped (correctOrder == wrongOrder: %s)\n\n", order.CorrectOrder)
			order.Status = "skipped_same"
			order.IsSynced = true
			skippedSameCount++
			saveJSON(orders)
			continue
		}

		if order.InternalID == "" {
			fmt.Println("  Skipped (no internalID)")
			order.Status = "error"
			order.CurrentTranID = ""
			order.IsSynced = false
			errorCount++
			saveJSON(orders)
			fmt.Println()
			continue
		}

		var cashSale *gns.SalesTransaction
		err = retryWithBackoff(1000, func() error {
			var getErr error
			cashSale, getErr = client.SalesTransactionService.Get(order.InternalID, "cashSale")
			return getErr
		})
		if err != nil {
			fmt.Printf("  Failed to get order: %v\n", err)
			order.Status = "error"
			order.IsSynced = false
			errorCount++
			saveJSON(orders)
			fmt.Println()
			if i < len(orders)-1 {
				time.Sleep(0 * time.Second)
			}
			continue
		}

		currentTranId := cashSale.TranId
		order.CurrentTranID = currentTranId

		if currentTranId == order.CorrectOrder {
			fmt.Printf("  MATCHED - current tranId = %s (already correct)\n", currentTranId)
			order.Status = "matched"
			order.IsSynced = true
			matchedCount++
		} else if currentTranId == order.WrongOrder {
			fmt.Printf("  WRONG - current tranId = %s, updating to %s...\n", currentTranId, order.CorrectOrder)

			// Step 1: Undeposit
			fmt.Println("    Step 1: Undepositing...")
			undepositReq := &gns.CashSaleRequest{
				XsiType:    gns.EntityCashSaleRequest,
				InternalId: order.InternalID,
				Status:     gns.CashSaleStatusLabelNotDeposited,
				Account:    &gns.RecordRef{InternalId: "1"},
			}
			err = retryWithBackoff(1000, func() error {
				return client.CashSaleService.Update(undepositReq)
			})
			if err != nil {
				fmt.Printf("    Failed to undeposit: %v\n", err)
				order.Status = "error_undeposit"
				order.IsSynced = false
				errorCount++
				saveJSON(orders)
				fmt.Println()
				continue
			}
			fmt.Println("    Undeposited OK")
			time.Sleep(0 * time.Millisecond)

			// Step 2: Update tranId
			fmt.Println("    Step 2: Updating tranId...")
			updateReq := &gns.CashSaleRequest{
				XsiType:    gns.EntityCashSaleRequest,
				InternalId: order.InternalID,
				TranId:     order.CorrectOrder,
				Account:    &gns.RecordRef{InternalId: "1"},
			}
			err = retryWithBackoff(1000, func() error {
				return client.CashSaleService.Update(updateReq)
			})
			if err != nil {
				fmt.Printf("    Failed to update tranId: %v\n", err)
				// Re-deposit even if update fails
				fmt.Println("    Re-depositing (rollback)...")
				redepReq := &gns.CashSaleRequest{
					XsiType:    gns.EntityCashSaleRequest,
					InternalId: order.InternalID,
					Status:     gns.CashSaleStatusLabelDeposited,
					Account:    &gns.RecordRef{InternalId: "1"},
				}
				_ = retryWithBackoff(1000, func() error {
					return client.CashSaleService.Update(redepReq)
				})
				order.Status = "error_update"
				order.IsSynced = false
				errorCount++
				saveJSON(orders)
				fmt.Println()
				continue
			}
			fmt.Printf("    TranId updated: %s -> %s\n", order.WrongOrder, order.CorrectOrder)
			time.Sleep(0 * time.Millisecond)

			// Step 3: Re-deposit
			fmt.Println("    Step 3: Re-depositing...")
			redepReq := &gns.CashSaleRequest{
				XsiType:    gns.EntityCashSaleRequest,
				InternalId: order.InternalID,
				Status:     gns.CashSaleStatusLabelDeposited,
				Account:    &gns.RecordRef{InternalId: "1"},
			}
			err = retryWithBackoff(1000, func() error {
				return client.CashSaleService.Update(redepReq)
			})
			if err != nil {
				fmt.Printf("    CRITICAL: Failed to re-deposit: %v\n", err)
				fmt.Println("    Order is LEFT as Not Deposited! Must fix manually.")
				order.Status = "error_redeposit"
				order.IsSynced = false
				errorCount++
				// Save immediately so we don't lose this critical state
				saveJSON(orders)
				fmt.Println()
				continue
			}
			fmt.Println("    Re-deposited OK")

			fmt.Printf("  DONE: %s → %s ✓\n", order.WrongOrder, order.CorrectOrder)
			order.Status = "updated"
			order.IsSynced = true
			order.CurrentTranID = order.CorrectOrder
			updatedCount++
		} else {
			fmt.Printf("  UNEXPECTED - current tranId = %s (neither correct nor wrong)\n", currentTranId)
			order.Status = "unexpected"
			order.IsSynced = false
			errorCount++
		}

		// Save after every order (status + isSynced updated)
		saveJSON(orders)

		fmt.Println()

		if i < len(orders)-1 {
			time.Sleep(0 * time.Millisecond)
		}
	}

	// Save final results
	saveJSON(orders)
	fmt.Println("Saved results to orderInfo.json")

	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("Summary:")
	fmt.Printf("  Matched (already correct): %d\n", matchedCount)
	fmt.Printf("  Updated (fixed):           %d\n", updatedCount)
	fmt.Printf("  Skipped (same order):      %d\n", skippedSameCount)
	fmt.Printf("  Errors/Unexpected:         %d\n", errorCount)
	fmt.Printf("  Total:                     %d\n", len(orders))
	fmt.Printf("  Elapsed:                   %v\n", time.Since(startTime).Round(time.Second))
}

func saveJSON(orders []OrderInfo) {
	data, err := json.MarshalIndent(orders, "", "    ")
	if err != nil {
		log.Printf("Failed to marshal JSON: %v", err)
		return
	}
	if err := os.WriteFile("orderInfo.json", data, 0644); err != nil {
		log.Printf("Failed to save JSON: %v", err)
	}
}

func newNetsuiteSOAPClient() (*gns.Client, error) {
	consumerKey := strings.TrimSpace(os.Getenv("NS_CONSUMER_KEY"))
	if consumerKey == "" {
		return nil, errors.New("NS_CONSUMER_KEY is required")
	}
	consumerSecret := strings.TrimSpace(os.Getenv("NS_CONSUMER_SECRET"))
	if consumerSecret == "" {
		return nil, errors.New("NS_CONSUMER_SECRET is required")
	}
	accessToken := strings.TrimSpace(os.Getenv("NS_ACCESS_TOKEN"))
	if accessToken == "" {
		return nil, errors.New("NS_ACCESS_TOKEN is required")
	}
	accessTokenSecret := strings.TrimSpace(os.Getenv("NS_ACCESS_TOKEN_SECRET"))
	if accessTokenSecret == "" {
		return nil, errors.New("NS_ACCESS_TOKEN_SECRET is required")
	}
	accountID := strings.TrimSpace(os.Getenv("NS_ACCOUNT"))
	if accountID == "" {
		return nil, errors.New("NS_ACCOUNT is required")
	}

	url := envOrDefault("NS_URL", "https://webservices.netsuite.com/services/NetSuitePort_2023_1")
	apiVersion := envOrDefault("NS_API_VERSION", "2023_1")

	return gns.NewSoapClient(
		url,
		accountID,
		consumerKey,
		consumerSecret,
		accessToken,
		accessTokenSecret,
		apiVersion,
		0,
	), nil
}

func loadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		os.Setenv(key, value)
	}
	return scanner.Err()
}

func envOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}
