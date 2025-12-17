package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"git02.smartosc.com/production/dragonfly/inventory_item"
	cpb "git02.smartosc.com/production/pbtypes/core"
	pb "git02.smartosc.com/production/pbtypes/platform"
)

func main() {
	client, err := loadClientFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Parse command line args or env vars for request parameters
	productId := strings.TrimSpace(os.Getenv("NS_PRODUCT_ID"))
	sku := strings.TrimSpace(os.Getenv("NS_ITEM_SKU"))
	locationIds := strings.TrimSpace(os.Getenv("NS_LOCATION_IDS"))
	page := uint32(1)
	limit := uint32(1000)

	// Build request
	req := &pb.ListInventoryItemRequest{
		Client:      client,
		ProductId:   productId,
		Sku:         sku,
		LocationIds: locationIds,
		Page:        page,
		Limit:       limit,
	}

	// Force itemIds to the requested item 44246
	itemIds := []string{"44246"}

	service := inventory_item.NewService()
	resp, err := service.GetInventoryItemListWithBinNumbers(context.Background(), client, req, itemIds)
	if err != nil {
		log.Fatalf("fetch error: %v", err)
	}

	outputPath := "inventory_items.json"
	if err := writeOutput(outputPath, resp); err != nil {
		log.Fatalf("write error: %v", err)
	}

	log.Printf("saved %d inventory items with bin numbers to %s", len(resp), outputPath)
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
