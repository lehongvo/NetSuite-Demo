package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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

	// Request body
	body := `{
    "q": "SELECT il.*, i.itemid, l.name as locationname, TO_CHAR( il.lastquantityavailablechange, 'YYYY-MM-DD HH24:MI:SS TZH:TZM') AS updatedat FROM item i, inventoryitemlocations il, location l WHERE i.isinactive = 'F' AND i.id = il.item AND il.location = l.id "
}`

	// Generate OAuth parameters
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce()

	fmt.Println("=== OAuth Signature Calculation ===\n")
	fmt.Printf("1. Consumer Key: %s\n", consumerKey)
	fmt.Printf("2. Consumer Secret: %s\n", consumerSecret)
	fmt.Printf("3. Access Token: %s\n", accessToken)
	fmt.Printf("4. Access Token Secret: %s\n", accessTokenSecret)
	fmt.Printf("5. Timestamp: %s\n", timestamp)
	fmt.Printf("6. Nonce: %s\n\n", nonce)

	// Parse URL to get host and path
	u, _ := url.Parse(fullURL)
	baseURLForSigning := u.Scheme + "://" + u.Host + u.Path

	fmt.Printf("7. Base URL for signing: %s\n", baseURLForSigning)
	fmt.Printf("8. Method: %s\n\n", method)

	// OAuth parameters
	oauthParams := map[string]string{
		"oauth_consumer_key":     consumerKey,
		"oauth_token":            accessToken,
		"oauth_signature_method": "HMAC-SHA256",
		"oauth_timestamp":        timestamp,
		"oauth_nonce":            nonce,
		"oauth_version":          "1.0",
	}

	fmt.Println("9. OAuth Parameters:")
	for k, v := range oauthParams {
		fmt.Printf("   - %s = %s\n", k, v)
	}
	fmt.Println()

	// Create signature base string
	signatureBaseString := createSignatureBaseString(method, baseURLForSigning, queryParams, oauthParams, body)
	
	fmt.Println("10. Signature Base String (before encoding):")
	fmt.Printf("    %s\n\n", signatureBaseString)
	
	fmt.Println("11. Signature Base String (URL encoded):")
	fmt.Printf("    %s\n\n", url.QueryEscape(signatureBaseString))
	
	// Create signing key
	signingKey := url.QueryEscape(consumerSecret) + "&" + url.QueryEscape(accessTokenSecret)
	
	fmt.Println("12. Signing Key:")
	fmt.Printf("    Consumer Secret (URL encoded) + '&' + Access Token Secret (URL encoded)\n")
	fmt.Printf("    = %s\n", signingKey)
	fmt.Println()

	// Generate signature
	signature := generateSignature(signatureBaseString, signingKey)
	
	fmt.Println("13. HMAC-SHA256 Signature (Base64):")
	// Show raw signature before URL encoding
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(signatureBaseString))
	rawSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	fmt.Printf("    Raw (Base64): %s\n", rawSignature)
	fmt.Printf("    URL Encoded: %s\n", signature)
	fmt.Println()

	fmt.Println("=== Final OAuth Signature ===")
	fmt.Printf("%s\n", signature)
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

