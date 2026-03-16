package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	restclient "git02.smartosc.com/production/platform-connector/go-rest-netsuite/client"
)

// Input models the payload from data/createOrder/data.json.
// Chúng ta chỉ dùng phần order để build request cho Restlet.
type Input struct {
	StoreID        string `json:"storeId"`
	ShiftID        string `json:"shiftId"`
	RegisterID     string `json:"registerId"`
	OutletID       string `json:"outletId"`
	OutletName     string `json:"outletName"`
	RegisterName   string `json:"registerName"`
	NotifyCustomer bool   `json:"notifyCustomer"`
	Order          Order  `json:"order"`
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
	RawPayload     json.RawMessage `json:"rawPayload,omitempty"` // lưu full payload để trace trên NS
}

type idRef struct {
	InternalID string `json:"internalId"`
}

type cashSaleItem struct {
	ItemID   string  `json:"itemid"`
	Quantity float64 `json:"quantity"`
	Rate     float64 `json:"rate,omitempty"`
	Amount   float64 `json:"amount,omitempty"`
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

// Hard-coded NetSuite OAuth credentials for Toor CORP demo store.
// Mapping:
// - consumerKey / consumerSecret   -> Integration (Consumer) Key / Secret
// - accessToken / accessTokenSecret -> Token Id / Token Secret
const (
	nsAccountID        = "6555930"
	nsConsumerKey      = "55c5b7f0aae6dea9aa97433a1108aedf5996f852ec96287cdf31e9c495dca7af"
	nsConsumerSecret   = "d99dd72ad15d4586b1415a69dc7181522f9bcff037833e5601fc38a9d096b306"
	nsAccessToken      = "dbd4b08ba1152e01af9b5ec2f0d9ede672d3ee107b871d29fc896ed05b23f9aa"
	nsAccessTokenSecret = "27c691d7260855f7ad1f50c51027a97e43416c61661c2827cb032b81c044df7e"
)

func main() {
	// Load .env (lấy key OAuth & config NS)
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
	if err := postRestlet(client, body, &resp); err != nil {
		log.Fatalf("restlet post failed: %v", err)
	}

	log.Printf("Restlet status=%s internalid=%v message=%v", resp.Status, resp.InternalID, resp.Message)

	// Log full response as JSON
	respJSON, _ := json.MarshalIndent(resp, "", "  ")
	log.Printf("Full response: %s", string(respJSON))

	log.Printf("Using tranId=%s (done)", req.TranId)

	// Check taxitem on the created Cash Sale
	if resp.InternalID != nil {
		var internalID int64
		switch v := resp.InternalID.(type) {
		case float64:
			internalID = int64(v)
		case int64:
			internalID = v
		}
		if internalID > 0 {
			checkTaxItem(client, internalID)
		}
	}
}

// checkTaxItem calls NetSuite REST Records API to verify taxitem on the Cash Sale.
func checkTaxItem(c *restclient.RestletClient, internalID int64) {
	url := fmt.Sprintf(
		"https://%s.suitetalk.api.netsuite.com/services/rest/record/v1/cashsale/%d?fields=taxitem,taxtotal,taxrate",
		nsAccountID, internalID,
	)
	log.Printf("Checking Cash Sale taxitem: GET %s", url)

	httpResp, err := c.GetClient().Get(url)
	if err != nil {
		log.Printf("checkTaxItem GET failed: %v", err)
		return
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("checkTaxItem parse failed: %v — raw: %s", err, string(body))
		return
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	log.Printf("Cash Sale #%d tax fields:\n%s", internalID, string(pretty))

	// Check taxitem name
	if taxitem, ok := result["taxitem"].(map[string]any); ok {
		name := taxitem["refName"]
		id := taxitem["id"]
		if name == "CA_EL CAJON" || name == "A_EL CAJON" {
			log.Printf("✅ TAX ITEM CORRECT: %v (id=%v)", name, id)
		} else {
			log.Printf("❌ TAX ITEM WRONG: %v (id=%v) — expected CA_EL CAJON", name, id)
		}
	} else {
		log.Printf("taxitem field: %v", result["taxitem"])
	}
}

// loadOrder đọc toàn bộ data.json, map sang struct Input rồi lấy Order.
func loadOrder(path string) (Order, json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Order{}, nil, fmt.Errorf("read payload file: %w", err)
	}

	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		return Order{}, nil, fmt.Errorf("decode payload: %w", err)
	}

	if input.Order.Customer.ID == "" {
		return Order{}, nil, errors.New("order.customer.id is empty")
	}

	return input.Order, raw, nil
}

func buildCashSaleRequest(order Order, rawPayload json.RawMessage) cashSaleRequest {
	// NetSuite Restlet script mong đợi format dd/mm/yyyy
	transDate := time.Now().Format("02/01/2006")
	memo := strings.TrimSpace(order.Note)
	if memo == "" {
		memo = "POS Order"
	}

	// Lấy taxCode từ order nếu có, không thì để NetSuite dùng default trên item
	taxCode := ""
	if len(order.TaxLines) > 0 && order.TaxLines[0].ExternalID != "" {
		taxCode = order.TaxLines[0].ExternalID
	}

	items := make([]cashSaleItem, 0, len(order.Items))
	for _, it := range order.Items {
		// Dùng giá trước thuế cho rate (Price / TotalPrice),
		// và dùng giá đã gồm thuế cho amount/grossAmt để NetSuite tự tính thuế 1 lần.
		rate := pickNumber(it.Price, it.TotalPrice)
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

// pickNumber chọn số đầu tiên parse được > 0 từ danh sách string.
func pickNumber(candidates ...string) float64 {
	for _, c := range candidates {
		if v, err := strconv.ParseFloat(c, 64); err == nil && v > 0 {
			return v
		}
	}
	return 0
}

func newRestletClient() (*restclient.RestletClient, error) {
	scriptID := envOrDefault("NS_RESTLET_SCRIPT_ID", "1792")
	deployID := envOrDefault("NS_RESTLET_DEPLOY_ID", "2")

	c := restclient.NewRestletClient(
		nsAccountID,
		nsConsumerKey,
		nsConsumerSecret,
		nsAccessToken,
		nsAccessTokenSecret,
		scriptID,
	).WithDeploy(deployID)

	log.Printf("Using scriptID=%s deployID=%s accountID=%s", scriptID, deployID, nsAccountID)
	return c, nil
}

func postRestlet(c *restclient.RestletClient, body, resource any) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	log.Printf("POST %s", c.GetUrl())

	resp, err := c.GetClient().Post(c.GetUrl(), "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	if resource != nil {
		return json.Unmarshal(respBytes, resource)
	}
	return nil
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
		".env",       // Current directory
		"../.env",    // Parent directory (project root)
		"../../.env", // Two levels up
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
		envPath := filepath.Join(wd, ".env")
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("Loaded .env from %s", envPath)
			return nil
		}
	}

	return fmt.Errorf("could not find .env file")
}