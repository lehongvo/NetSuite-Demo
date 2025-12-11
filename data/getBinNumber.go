package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type config struct {
	BaseURL        string
	Account        string
	APIVersion     string
	ConsumerKey    string
	ConsumerSecret string
	Token          string
	TokenSecret    string
}

type suiteQLResponse struct {
	Items []map[string]interface{} `json:"items"`
}

type binRecord struct {
	LocationId           string `json:"locationId"`
	LocationName         string `json:"locationName"`
	BinId                string `json:"binId"`
	BinNumber            string `json:"binNumber"`
	OnHand               string `json:"onHand"`
	Available            string `json:"available"`
	LocationActive       string `json:"locationActive"`
	PreferredPerLocation string `json:"preferredPerLocation"`
	WmsPreferred         string `json:"wmsPreferred"`
	WmsSequence          string `json:"wmsSequence"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	itemID := strings.TrimSpace(os.Getenv("NS_ITEM_ID"))
	itemSKU := strings.TrimSpace(os.Getenv("NS_ITEM_SKU"))
	query := buildQuery(itemID, itemSKU)
	records, err := fetchBins(cfg, query)
	if err != nil {
		log.Fatalf("fetch error: %v", err)
	}

	if err := writeOutput("/Users/vincent/NetSuite-Demo/data/index.json", records); err != nil {
		log.Fatalf("write error: %v", err)
	}

	log.Printf("saved %d bin records to data/index.json", len(records))
}

func loadConfig() (config, error) {
	cfg := config{
		BaseURL:        strings.TrimSuffix(os.Getenv("NS_URL"), "/"),
		Account:        os.Getenv("NS_ACCOUNT"),
		APIVersion:     os.Getenv("NS_API_VERSION"),
		ConsumerKey:    os.Getenv("NS_TOKEN"),
		ConsumerSecret: os.Getenv("NS_TOKEN_SECRET"),
		Token:          os.Getenv("NS_AUTH_TOKEN"),
		TokenSecret:    os.Getenv("NS_AUTH_TOKEN_SECRET"),
	}

	missing := make([]string, 0)
	if cfg.BaseURL == "" {
		missing = append(missing, "NS_URL")
	}
	if cfg.Account == "" {
		missing = append(missing, "NS_ACCOUNT")
	}
	if cfg.APIVersion == "" {
		missing = append(missing, "NS_API_VERSION")
	}
	if cfg.ConsumerKey == "" {
		missing = append(missing, "NS_TOKEN")
	}
	if cfg.ConsumerSecret == "" {
		missing = append(missing, "NS_TOKEN_SECRET")
	}
	if cfg.Token == "" {
		missing = append(missing, "NS_AUTH_TOKEN")
	}
	if cfg.TokenSecret == "" {
		missing = append(missing, "NS_AUTH_TOKEN_SECRET")
	}

	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing env: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func fetchBins(cfg config, suiteQL string) ([]binRecord, error) {
	// SuiteQL REST endpoint uses fixed v1 per NetSuite docs.
	endpoint := fmt.Sprintf("%s/services/rest/query/v1/suiteql", cfg.BaseURL)

	bodyData := map[string]string{"q": suiteQL}
	bodyBytes, err := json.Marshal(bodyData)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	authHeader, err := buildOAuth1Header(cfg, http.MethodPost, endpoint)
	if err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	// NetSuite requires Prefer: transient for SuiteQL REST.
	req.Header.Set("Prefer", "transient")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("netsuite status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed suiteQLResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	records := make([]binRecord, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		get := func(key string) (string, bool) {
			v, ok := item[key]
			if !ok {
				return "", false
			}
			return fmt.Sprint(v), true
		}

		binId, ok1 := get("binid")
		binNumber, ok2 := get("binnumber")
		locId, ok3 := get("locationid")
		locName, ok4 := get("locationname")
		onHand, _ := get("onhand")
		available, _ := get("available")
		locInactive, _ := get("locationinactive")
		preferred, _ := get("preferredperlocation")
		wmsPref, _ := get("wmspreferred")
		wmsSeq, _ := get("wmssequence")
		if !(ok1 && ok2 && ok3 && ok4) {
			return nil, errors.New("unexpected response shape from SuiteQL (missing core columns)")
		}

		rec := binRecord{
			BinId:                binId,
			BinNumber:            binNumber,
			LocationId:           locId,
			LocationName:         locName,
			OnHand:               onHand,
			Available:            available,
			LocationActive:       invertBoolString(locInactive),
			PreferredPerLocation: preferred,
			WmsPreferred:         wmsPref,
			WmsSequence:          wmsSeq,
		}
		records = append(records, rec)
	}

	return records, nil
}

func buildOAuth1Header(cfg config, method, url string) (string, error) {
	now := time.Now()
	nonce := randomString(16)
	timestamp := strconv.FormatInt(now.Unix(), 10)

	params := map[string]string{
		"oauth_consumer_key":     cfg.ConsumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA256",
		"oauth_timestamp":        timestamp,
		"oauth_token":            cfg.Token,
		"oauth_version":          "1.0",
	}

	paramString := normalizeParams(params)
	baseString := strings.ToUpper(method) + "&" + percentEncode(url) + "&" + percentEncode(paramString)
	signingKey := percentEncode(cfg.ConsumerSecret) + "&" + percentEncode(cfg.TokenSecret)

	mac := hmac.New(sha256.New, []byte(signingKey))
	if _, err := mac.Write([]byte(baseString)); err != nil {
		return "", fmt.Errorf("sign base string: %w", err)
	}

	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	header := fmt.Sprintf(`OAuth realm="%s", oauth_consumer_key="%s", oauth_token="%s", oauth_signature_method="HMAC-SHA256", oauth_timestamp="%s", oauth_nonce="%s", oauth_version="1.0", oauth_signature="%s"`,
		cfg.Account,
		percentEncode(cfg.ConsumerKey),
		percentEncode(cfg.Token),
		percentEncode(timestamp),
		percentEncode(nonce),
		percentEncode(signature),
	)

	return header, nil
}

func normalizeParams(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, percentEncode(k)+"="+percentEncode(params[k]))
	}
	return strings.Join(parts, "&")
}

func percentEncode(val string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"!", "%21",
		"\"", "%22",
		"#", "%23",
		"$", "%24",
		"&", "%26",
		"'", "%27",
		"(", "%28",
		")", "%29",
		"*", "%2A",
		"+", "%2B",
		",", "%2C",
		"/", "%2F",
		":", "%3A",
		";", "%3B",
		"<", "%3C",
		"=", "%3D",
		">", "%3E",
		"?", "%3F",
		"@", "%40",
		"[", "%5B",
		"\\", "%5C",
		"]", "%5D",
		"^", "%5E",
		"`", "%60",
		"{", "%7B",
		"|", "%7C",
		"}", "%7D",
		"~", "%7E",
	)
	return replacer.Replace(val)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func invertBoolString(inactive string) string {
	val := strings.ToLower(strings.TrimSpace(inactive))
	if val == "t" || val == "true" || val == "1" {
		return "false"
	}
	if val == "f" || val == "false" || val == "0" {
		return "true"
	}
	return inactive
}

func writeOutput(path string, data interface{}) error {
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// buildQuery optionally filters by itemId (internalid) or itemSKU (itemid) so onHand/available align to item-level UI.
// Precedence: itemID (exact internal id), otherwise itemSKU.
func buildQuery(itemID, itemSKU string) string {
	ibqJoin := "LEFT JOIN ItemBinQuantity ibq ON ibq.bin = b.id"
	filter := "NVL(b.isInactive, 'F') = 'F'"

	if itemID != "" {
		ibqJoin = fmt.Sprintf("LEFT JOIN ItemBinQuantity ibq ON ibq.bin = b.id AND ibq.item = '%s'", itemID)
		filter = fmt.Sprintf("%s AND ibq.item = '%s'", filter, itemID)
	} else if itemSKU != "" {
		// Resolve SKU -> item internalid via Item table, then join ItemBinQuantity
		ibqJoin = fmt.Sprintf(`LEFT JOIN Item i ON i.itemid = '%s'
LEFT JOIN ItemBinQuantity ibq ON ibq.bin = b.id AND ibq.item = i.id`, itemSKU)
		filter = fmt.Sprintf("%s AND i.itemid = '%s'", filter, itemSKU)
	}

	return fmt.Sprintf(`
SELECT
  b.id                           AS binId,
  b.binNumber                    AS binNumber,
  b.location                     AS locationId,
  l.name                         AS locationName,
  TO_CHAR(NVL(SUM(ibq.onhand),0))       AS onHand,
  TO_CHAR(NVL(SUM(ibq.onhandavail),0))  AS available,
  l.isInactive                   AS locationInactive,
  ''                             AS preferredPerLocation,
  ''                             AS wmsPreferred,
  ''                             AS wmsSequence
FROM Bin b
LEFT JOIN Location l ON b.location = l.id
%s
WHERE %s
GROUP BY b.id, b.binNumber, b.location, l.name, l.isInactive
ORDER BY l.name, b.binNumber
`, ibqJoin, filter)
}
