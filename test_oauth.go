package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	// Load from .env or use provided values
	consumerKey := getEnv("NS_CONSUMER_KEY", "6cd361314fff88eec57bcb3c614cc756dbe04a0e89d2a3ab66e6c64c3dc82737")
	consumerSecret := getEnv("NS_CONSUMER_SECRET", "8978447bead27499f6ee2d6a465c2f9acee6c359fa1623001b62bf504f70d803")
	accessToken := getEnv("NS_ACCESS_TOKEN", "dbc891d00323d50d09ff761b5d91249332beb521fcb7c7fc83376f60682210ad")
	accessTokenSecret := getEnv("NS_ACCESS_TOKEN_SECRET", "437be927859404d5dc9eb861776e0d87e3a791a13255d0d298fcaeac63a51ff8")
	accountID := getEnv("NS_ACCOUNT", "9342705")

	// URL and method
	baseURL := fmt.Sprintf("https://%s.suitetalk.api.netsuite.com/services/rest/query/v1/suiteql", accountID)
	method := "POST"

	// Query parameters
	queryParams := url.Values{}
	queryParams.Set("limit", "1000")
	queryParams.Set("offset", "100000")
	
	fullURL := baseURL + "?" + queryParams.Encode()

	// Request body - original query
	body := `{
    "q": "SELECT il.*, i.itemid, l.name as locationname, TO_CHAR( il.lastquantityavailablechange, 'YYYY-MM-DD HH24:MI:SS TZH:TZM') AS updatedat FROM item i, inventoryitemlocations il, location l WHERE i.isinactive = 'F' AND i.id = il.item AND il.location = l.id "
}`

	// Generate OAuth parameters
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce()

	// Parse URL to get host and path
	u, _ := url.Parse(fullURL)
	baseURLForSigning := u.Scheme + "://" + u.Host + u.Path

	// OAuth parameters
	oauthParams := map[string]string{
		"oauth_consumer_key":     consumerKey,
		"oauth_token":            accessToken,
		"oauth_signature_method": "HMAC-SHA256",
		"oauth_timestamp":        timestamp,
		"oauth_nonce":            nonce,
		"oauth_version":          "1.0",
	}

	// Create signature base string
	signatureBaseString := createSignatureBaseString(method, baseURLForSigning, queryParams, oauthParams, body)
	
	// Create signing key
	signingKey := url.QueryEscape(consumerSecret) + "&" + url.QueryEscape(accessTokenSecret)
	
	// Generate signature
	signature := generateSignature(signatureBaseString, signingKey)
	oauthParams["oauth_signature"] = signature

	// Build Authorization header
	authHeader := buildAuthHeader(accountID, oauthParams)

	// Make HTTP request
	req, err := http.NewRequest(method, fullURL, bytes.NewBufferString(body))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	req.Header.Set("Prefer", "transient")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	// Print results
	fmt.Printf("Status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Printf("Response:\n%s\n", string(respBody))
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return strings.TrimSpace(val)
	}
	return defaultValue
}

func generateNonce() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func createSignatureBaseString(method, baseURL string, queryParams url.Values, oauthParams map[string]string, body string) string {
	// Combine all parameters
	allParams := make(map[string]string)
	
	// Add query parameters
	for k, v := range queryParams {
		if len(v) > 0 {
			allParams[k] = v[0]
		}
	}
	
	// Add OAuth parameters
	for k, v := range oauthParams {
		if k != "oauth_signature" {
			allParams[k] = v
		}
	}
	
	// Sort and encode parameters
	keys := make([]string, 0, len(allParams))
	for k := range allParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	paramPairs := make([]string, 0, len(keys))
	for _, k := range keys {
		paramPairs = append(paramPairs, url.QueryEscape(k)+"="+url.QueryEscape(allParams[k]))
	}
	
	parameterString := strings.Join(paramPairs, "&")
	
	// Create base string: METHOD&URL&PARAMETERS
	return method + "&" + url.QueryEscape(baseURL) + "&" + url.QueryEscape(parameterString)
}

func generateSignature(baseString, signingKey string) string {
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(baseString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return url.QueryEscape(signature)
}

func buildAuthHeader(realm string, oauthParams map[string]string) string {
	parts := []string{fmt.Sprintf(`realm="%s"`, realm)}
	
	keys := []string{"oauth_consumer_key", "oauth_token", "oauth_signature_method", 
		"oauth_timestamp", "oauth_nonce", "oauth_version", "oauth_signature"}
	
	for _, k := range keys {
		if v, ok := oauthParams[k]; ok {
			parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
		}
	}
	
	return "OAuth " + strings.Join(parts, ",")
}
