//go:build bin_number
// +build bin_number

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

// Simple test runner to fetch bin numbers with the provided SuiteQL query.
func main() {
	client, err := loadClientFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Query provided in the request, including Bin.SequenceNumber as wmssequence and ordering by it
	query := "SELECT ibq.item AS itemId, b.id AS binId, b.binNumber AS binNumber, b.location AS locationId, l.name AS locationName, " +
		"TO_CHAR(NVL(SUM(ibq.onhand),0)) AS onHand, " +
		"TO_CHAR(NVL(SUM(ibq.onhandavail),0)) AS available, " +
		"l.isInactive AS locationInactive, " +
		"'' AS preferredPerLocation, " +
		"'' AS wmsPreferred, " +
		"b.sequenceNumber AS wmssequence " +
		"FROM Bin b " +
		"LEFT JOIN Location l ON b.location = l.id " +
		"LEFT JOIN ItemBinQuantity ibq ON ibq.bin = b.id " +
		"WHERE NVL(b.isInactive, 'F') = 'F' AND ibq.item IN ('44246') " +
		"GROUP BY ibq.item, b.id, b.binNumber, b.location, l.name, l.isInactive, b.sequenceNumber " +
		"HAVING SUM(ibq.onhand) > 0 " +
		"ORDER BY ibq.item, b.id DESC, l.name, b.binNumber"

	req := &suiteql.QueryRequest{
		Query: query,
		Page:  1,
		Limit: 1000,
	}

	suiteQLClient := netsuite.NewSuiteQLClient(client)
	binResp, err := suiteQLClient.BinNumber.List(req)
	if err != nil {
		log.Fatalf("bin fetch error: %v", err)
	}

	if err := writeOutput("bin_numbers.json", binResp); err != nil {
		log.Fatalf("write error: %v", err)
	}

	log.Printf("saved %d bin rows to bin_numbers.json", len(binResp.Items))
}

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
