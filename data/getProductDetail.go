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

// listInternalIdPayload represents the JSON structure in data/listInternalId.json.
// {
//   "listInternalId": ["41946", "47733", "47735"]
// }
type listInternalIdPayload struct {
	ListInternalId []string `json:"listInternalId"`
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

	itemIds, err := loadItemIds(inputPath)
	if err != nil {
		log.Fatalf("load item ids error: %v", err)
	}
	if len(itemIds) == 0 {
		log.Fatalf("no item internal IDs found in %s", inputPath)
	}

	// Build IN clause for internal IDs.
	// Example: WHERE i.id IN (41946, 47733, 47735)
	inClause := strings.Join(itemIds, ",")

	query := "SELECT " +
		"i.id AS internalId, " +
		"i.itemid AS itemId, " +
		"i.displayname AS displayName " +
		"FROM Item i " +
		"WHERE i.id IN (" + inClause + ") " +
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

// loadItemIds reads listInternalId.json and returns the listInternalId slice.
func loadItemIds(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var payload listInternalIdPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	return payload.ListInternalId, nil
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

