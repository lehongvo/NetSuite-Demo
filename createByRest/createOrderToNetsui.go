package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	rest "git02.smartosc.com/production/platform-connector/go-rest-netsuite"
)

// Input models the example payload stored in data.json.
type Input struct {
	Payload struct {
		Order Order `json:"order"`
	} `json:"payload"`
}

type Order struct {
	Customer   Customer  `json:"customer"`
	Items      []Item    `json:"items"`
	LocationID string    `json:"locationId"`
	TaxLines   []TaxLine `json:"taxLines"`
	Note       string    `json:"note"`
}

type Customer struct {
	ID string `json:"id"`
}

type TaxLine struct {
	ExternalID string `json:"externalId"`
}

type Item struct {
	ProductID   string  `json:"productId"`
	Quantity    float64 `json:"quantity"`
	Price       string  `json:"price"`
	PriceIncTax string  `json:"priceIncTax"`
	TotalPrice  string  `json:"totalPrice"`
	TotalTax    string  `json:"totalTax"`
}

// Restlet payload structures expected by the NetSuite script.
type cashSaleRequest struct {
	InternalID     string          `json:"internalid,omitempty"`
	Entity         idRef           `json:"entity"`
	TransDate      string          `json:"transDate"`
	TranId         string          `json:"tranId,omitempty"`
	Memo           string          `json:"memo,omitempty"`
	Location       idRef           `json:"location"`
	DiscountItemID string          `json:"discountitemid,omitempty"`
	AccountID      string          `json:"accountid,omitempty"`
	Items          []cashSaleItem  `json:"items"`
	RawPayload     json.RawMessage `json:"rawPayload,omitempty"` // stored on NS side for traceability
}

type idRef struct {
	InternalID string `json:"internalId"`
}

type cashSaleItem struct {
	ItemID   string  `json:"itemid"`
	Quantity float64 `json:"quantity"`
	Rate     float64 `json:"rate"`
	Amount   float64 `json:"amount"`
	GrossAmt float64 `json:"grossAmt,omitempty"`
	TaxCode  string  `json:"taxCode,omitempty"`
}

type restletBody struct {
	Type string          `json:"type"`
	Data cashSaleRequest `json:"data"`
}

type restletResponse struct {
	Status     string `json:"status"`
	Message    any    `json:"message"`
	InternalID any    `json:"internalid"`
}

func main() {
	// Load .env file from project root
	if err := loadEnvFile(); err != nil {
		log.Printf("Warning: Failed to load .env file: %v", err)
	}

	inputPath := envOrDefault("INPUT_PATH", "data.json")

	order, rawPayload, err := loadOrder(inputPath)
	if err != nil {
		log.Fatalf("load order payload: %v", err)
	}

	req := buildCashSaleRequest(order, rawPayload)

	client, err := newRestletClient()
	if err != nil {
		log.Fatalf("build restlet client: %v", err)
	}

	body := restletBody{
		Type: "createCashSale",
		Data: req,
	}

	log.Printf("Using tranId=%s", req.TranId)

	if buf, err := json.MarshalIndent(body, "", "  "); err == nil {
		log.Printf("Restlet request:\n%s", string(buf))
	}

	var resp restletResponse
	if err := client.Post(body, &resp); err != nil {
		log.Fatalf("restlet post failed: %v", err)
	}

	log.Printf("Restlet status=%s internalid=%v message=%v", resp.Status, resp.InternalID, resp.Message)
	log.Printf("Using tranId=%s (done)", req.TranId)
}

func loadOrder(path string) (Order, json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Order{}, nil, fmt.Errorf("read payload file: %w", err)
	}

	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		return Order{}, nil, fmt.Errorf("decode payload: %w", err)
	}

	if input.Payload.Order.Customer.ID == "" {
		return Order{}, nil, errors.New("payload.customer.id is empty")
	}

	return input.Payload.Order, raw, nil
}

func buildCashSaleRequest(order Order, rawPayload json.RawMessage) cashSaleRequest {
	transDate := time.Now().Format("02/01/2006") // NetSuite expects dd/mm/yyyy
	memo := strings.TrimSpace(order.Note)
	if memo == "" {
		memo = "POS Order"
	}

	taxCode := ""
	if len(order.TaxLines) > 0 {
		taxCode = order.TaxLines[0].ExternalID
	}

	items := make([]cashSaleItem, 0, len(order.Items))
	for _, it := range order.Items {
		rate := pickNumber(it.PriceIncTax, it.Price, it.TotalPrice)
		amount := pickNumber(it.PriceIncTax, it.TotalPrice, it.Price)
		items = append(items, cashSaleItem{
			ItemID:   it.ProductID,
			Quantity: it.Quantity,
			Rate:     rate,
			Amount:   amount,
			GrossAmt: amount,
			TaxCode:  taxCode,
		})
	}

	accountID := strings.TrimSpace(os.Getenv("NS_ACCOUNT_ID"))
	if !isDigits(accountID) {
		log.Printf("NS_ACCOUNT_ID is not a numeric internal ID, ignoring: %q", accountID)
		accountID = ""
	} else {
		log.Printf("Using NS_ACCOUNT_ID=%s", accountID)
	}

	tranID := fmt.Sprintf("POS-%d", time.Now().Unix())
	log.Printf("Generated tranId=%s", tranID)

	return cashSaleRequest{
		Entity:         idRef{InternalID: order.Customer.ID},
		TransDate:      envOrDefault("NS_TRAN_DATE", transDate),
		TranId:         tranID,
		Memo:           memo,
		Location:       idRef{InternalID: order.LocationID},
		DiscountItemID: strings.TrimSpace(os.Getenv("NS_DISCOUNT_ITEM_ID")),
		AccountID:      accountID,
		Items:          items,
		RawPayload:     rawPayload,
	}
}

func pickNumber(candidates ...string) float64 {
	for _, c := range candidates {
		if v, err := strconv.ParseFloat(c, 64); err == nil && v > 0 {
			return v
		}
	}
	return 0
}

func newRestletClient() (*rest.RestletClient, error) {
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
		return nil, errors.New("NS_ACCOUNT is required for restlet calls")
	}
	scriptID := envOrDefault("NS_RESTLET_SCRIPT_ID", "3266")

	return rest.NewRestletClient(
		consumerKey,
		consumerSecret,
		accessToken,
		accessTokenSecret,
		accountID,
		scriptID,
	), nil
}

func envOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// loadEnvFile loads .env file from project root (parent directory)
func loadEnvFile() error {
	// Try to find .env file in current directory or parent directory
	envPaths := []string{
		".env",           // Current directory
		"../.env",        // Parent directory (project root)
		"../../.env",     // Two levels up
	}
	
	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			if err := godotenv.Load(envPath); err == nil {
				log.Printf("Loaded .env from %s", envPath)
				return nil
			}
		}
	}
	
	// Try absolute path based on current working directory
	wd, err := os.Getwd()
	if err == nil {
		// If we're in createByRest, go up one level
		if filepath.Base(wd) == "createByRest" {
			envPath := filepath.Join(filepath.Dir(wd), ".env")
			if err := godotenv.Load(envPath); err == nil {
				log.Printf("Loaded .env from %s", envPath)
				return nil
			}
		} else {
			// Try current directory
			envPath := filepath.Join(wd, ".env")
			if err := godotenv.Load(envPath); err == nil {
				log.Printf("Loaded .env from %s", envPath)
				return nil
			}
		}
	}
	
	return fmt.Errorf("could not find .env file")
}
