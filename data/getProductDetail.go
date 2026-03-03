package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	cpb "git02.smartosc.com/production/pbtypes/core"

	"git02.smartosc.com/production/dragonfly/netsuite"
	"git02.smartosc.com/production/platform-connector/go-netsuite/suiteql"
)

// searchPayload represents the JSON structure for product search.
// Single array containing both internalId (numeric strings) and upcCodes (non-numeric strings).
// {
//   "listInternalIdOrUpcCode": ["41946", "47733", "47735", "47052", "SKUTES0006A"]
// }
type searchPayload struct {
	ListInternalIdOrUpcCode []string `json:"listInternalIdOrUpcCode"`
}

func main() {
	client, err := loadClientFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Read list of internal IDs from data/listInternalId.json (or path from env)
	inputPath := os.Getenv("NS_ITEM_ID_LIST_PATH")
	if strings.TrimSpace(inputPath) == "" {
		inputPath = "listInternalId.json"
	}

	searchItems, err := loadSearchItems(inputPath)
	if err != nil {
		log.Fatalf("load search items error: %v", err)
	}
	if len(searchItems) == 0 {
		log.Fatalf("no search items found in %s", inputPath)
	}

	// Separate internalIds (numeric) and upcCodes (non-numeric)
	internalIds, upcCodes := classifySearchItems(searchItems)

	// Build WHERE clause
	var whereClauses []string
	if len(internalIds) > 0 {
		inClause := strings.Join(internalIds, ",")
		whereClauses = append(whereClauses, fmt.Sprintf("i.id IN (%s)", inClause))
	}
	if len(upcCodes) > 0 {
		// Escape single quotes in UPC codes
		escapedUpcCodes := make([]string, len(upcCodes))
		for i, upc := range upcCodes {
			escapedUpcCodes[i] = "'" + strings.ReplaceAll(upc, "'", "''") + "'"
		}
		upcClause := strings.Join(escapedUpcCodes, ",")
		whereClauses = append(whereClauses, fmt.Sprintf("i.upccode IN (%s)", upcClause))
	}

	if len(whereClauses) == 0 {
		log.Fatalf("no valid search criteria found")
	}

	whereClause := strings.Join(whereClauses, " OR ")

	// Query Item table for basic info
	query := "SELECT " +
		"i.id AS internalId, " +
		"i.itemid AS itemId, " +
		"i.displayname AS displayName, " +
		"i.upccode AS upcCode " +
		"FROM Item i " +
		"WHERE " + whereClause + " " +
		"ORDER BY i.itemid"

	req := &suiteql.QueryRequest{
		Query: query,
		Page:  1,
		Limit: 1000,
	}

	nsClient := netsuite.NewSuiteQLClient(client)
	if strings.EqualFold(strings.TrimSpace(os.Getenv("NS_DEBUG")), "1") {
		if nsClient.BaseClient != nil {
			nsClient.BaseClient.IsDebugging = true
		}
	}

	resp := &suiteql.SuiteQLSuccessResponse{}
	if err := nsClient.Execute(req, resp); err != nil {
		if nsErr, ok := err.(suiteql.NsError); ok {
			log.Fatalf("product fetch error: %s (details=%v)", nsErr.Error(), nsErr.ErrorDetails)
		}
		log.Fatalf("product fetch error: %T %v", err, err)
	}

	outputPath := "inventory_items.json"
	if err := writeOutput(outputPath, resp); err != nil {
		log.Fatalf("write error: %v", err)
	}

	log.Printf("saved %d product rows to %s", resp.Count, outputPath)
}

// loadSearchItems reads listInternalId.json and returns the listInternalIdOrUpcCode slice.
func loadSearchItems(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var payload searchPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	return payload.ListInternalIdOrUpcCode, nil
}

// classifySearchItems separates numeric strings (internalIds) from non-numeric strings (upcCodes).
func classifySearchItems(items []string) (internalIds []string, upcCodes []string) {
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		// Check if item is numeric (internalId) or non-numeric (upcCode)
		isNumeric := true
		for _, r := range item {
			if r < '0' || r > '9' {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			internalIds = append(internalIds, item)
		} else {
			upcCodes = append(upcCodes, item)
		}
	}
	return internalIds, upcCodes
}

// loadClientFromEnv builds a NetSuite client config from environment variables.
func loadClientFromEnv() (*cpb.Client, error) {
	client := &cpb.Client{
		Url:             strings.TrimSuffix(os.Getenv("NS_URL"), "/"),
		AccountId:       os.Getenv("NS_ACCOUNT"),
		ApiVersion:      os.Getenv("NS_API_VERSION"),
		Token:           os.Getenv("NS_TOKEN"),
		TokenSecret:     os.Getenv("NS_TOKEN_SECRET"),
		AuthToken:       os.Getenv("NS_AUTH_TOKEN"),
		AuthTokenSecret: os.Getenv("NS_AUTH_TOKEN_SECRET"),
	}

	missing := make([]string, 0)
	if client.Url == "" {
		missing = append(missing, "NS_URL")
	}
	if client.AccountId == "" {
		missing = append(missing, "NS_ACCOUNT")
	}
	if client.ApiVersion == "" {
		missing = append(missing, "NS_API_VERSION")
	}
	if client.Token == "" {
		missing = append(missing, "NS_TOKEN")
	}
	if client.TokenSecret == "" {
		missing = append(missing, "NS_TOKEN_SECRET")
	}
	if client.AuthToken == "" {
		missing = append(missing, "NS_AUTH_TOKEN")
	}
	if client.AuthTokenSecret == "" {
		missing = append(missing, "NS_AUTH_TOKEN_SECRET")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing env: %s", strings.Join(missing, ", "))
	}

	return client, nil
}

// writeOutput writes the response to a JSON file with pretty-printing.
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

