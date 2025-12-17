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

// Simple SuiteQL runner to list account internal IDs.
// Configure optional keyword filter via env NS_ACCOUNT_KEYWORD (matches account name or number).
func main() {
	client, err := loadClientFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	keyword := strings.TrimSpace(os.Getenv("NS_ACCOUNT_KEYWORD"))
	var where string
	if keyword != "" {
		// Using LIKE on both name and accountnumber
		kw := strings.ReplaceAll(keyword, "'", "''")
		where = fmt.Sprintf("AND (LOWER(a.name) LIKE '%%%%%s%%%%' OR LOWER(a.accountnumber) LIKE '%%%%%s%%%%') ", strings.ToLower(kw), strings.ToLower(kw))
	}

	// Keep SuiteQL simple to avoid INVALID_PARAMETER
	query := "SELECT a.id AS internalId, a.acctnumber AS number, a.acctname AS name, a.isInactive AS isInactive " +
		"FROM Account a " +
		"WHERE a.isInactive = 'F' " + where +
		"ORDER BY a.acctnumber, a.acctname"

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

	accountResp := &suiteql.SuiteQLSuccessResponse{}
	err = nsClient.Execute(req, accountResp)
	if err != nil {
		if nsErr, ok := err.(suiteql.NsError); ok {
			log.Fatalf("account fetch error: %s (details=%v)", nsErr.Error(), nsErr.ErrorDetails)
		}
		log.Fatalf("account fetch error: %T %v", err, err)
	}

	if err := writeOutput("accounts.json", accountResp); err != nil {
		log.Fatalf("write error: %v", err)
	}

	// Items is interface{}; try to assert slice length if possible
	count := accountResp.Count
	log.Printf("saved %d account rows to accounts.json", count)
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
