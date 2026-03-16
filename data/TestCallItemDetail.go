package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	cpb "git02.smartosc.com/production/pbtypes/core"

	"git02.smartosc.com/production/dragonfly/netsuite"
	gns "git02.smartosc.com/production/platform-connector/go-netsuite"
)

// TestCallItemDetail là tool nhỏ để call NetSuite ItemService.List
// và in/log + dump ra JSON thông tin item + pricing matrix để debug.
//
// LƯU Ý: File này dùng hard-code credential theo yêu cầu, chỉ chạy local,
// không nên commit/push ra repo public.

type priceDump struct {
	Quantity   float64 `json:"quantity"`
	Value string  `json:"value"`
}

type priceLevelDump struct {
	LevelName string      `json:"levelName"`
	Currency  string      `json:"currency"`
	Discount  string      `json:"discount"`
	Prices    []priceDump `json:"prices"`
}

type itemDump struct {
	InternalID   string           `json:"internalId"`
	ItemID       string           `json:"itemId"`
	DisplayName  string           `json:"displayName"`
	UpcCode      string           `json:"upcCode"`
	Cost         string           `json:"cost"`
	LastPurchase string           `json:"lastPurchase"`
	Pricing      []priceLevelDump `json:"pricing"`
}

func main() {
	client := buildHardcodedClient()

	nsClient := netsuite.NewClient(client)
	// Enable verbose SOAP logging when NS_DEBUG=1
	if strings.EqualFold(strings.TrimSpace(os.Getenv("NS_DEBUG")), "1") {
		nsClient.SetIsDebugging(true)
	}

	searchValue := strings.TrimSpace(os.Getenv("NS_TEST_UPC"))
	if searchValue == "" {
		searchValue = strings.TrimSpace(os.Getenv("NS_TEST_SEARCH"))
	}
	if searchValue == "" {
		// Fallback demo UPC used in ConnectPOS test products
		searchValue = "9999841234"
	}

	log.Printf("Calling ItemService.List with UPC search = %q\n", searchValue)

	// Search by UPC (see ProductSearchFieldUPC in item.go)
	search := []string{searchValue}
	fields := []string{gns.ProductSearchFieldUPC}

	resp, err := nsClient.ItemService.List(
		1,   // page
		10,  // limit
		"",  // searchID
		search,
		fields,
		"",  // subsidiaryId
		nil, // configs
		"",  // lastUpdatedDateFrom
		"",  // lastUpdatedDateTo
	)
	if err != nil {
		log.Fatalf("ItemService.List error: %T %v", err, err)
	}

	if len(resp.Records) == 0 {
		log.Println("no items found for given search")
		return
	}

	var output []itemDump

	for idx, item := range resp.Records {
		fmt.Printf("\n==== Item #%d ====\n", idx+1)
		fmt.Printf("Internal ID : %s\n", item.InternalId)
		fmt.Printf("Item ID     : %s\n", item.ItemId)
		fmt.Printf("Display Name: %s\n", item.DisplayName)
		fmt.Printf("UPC Code    : %s\n", item.UpcCode)
		fmt.Printf("Cost        : %s\n", item.Cost.String())
		fmt.Printf("LastPurchase: %s\n", item.LastPurchasePrice.String())

		itemOut := itemDump{
			InternalID:   item.InternalId,
			ItemID:       item.ItemId,
			DisplayName:  item.DisplayName,
			UpcCode:      item.UpcCode,
			Cost:         item.Cost.String(),
			LastPurchase: item.LastPurchasePrice.String(),
			Pricing:      make([]priceLevelDump, 0),
		}

		// Dump pricing matrix in a compact form
		if len(item.PricingMatrix) == 0 {
			fmt.Println("Pricing     : <no pricing matrix>")
			output = append(output, itemOut)
			continue
		}

		fmt.Println("Pricing matrix:")
		for _, pm := range item.PricingMatrix {
			levelName := pm.PriceLevel.Name
			currency := pm.Currency.Name
			discount := pm.Discount.String()

			fmt.Printf("  Price Level: %s (currency=%s, discount=%s)\n", levelName, currency, discount)
			if len(pm.Prices) == 0 {
				fmt.Println("    <no prices>")
				itemOut.Pricing = append(itemOut.Pricing, priceLevelDump{
					LevelName: levelName,
					Currency:  currency,
					Discount:  discount,
					Prices:    nil,
				})
				continue
			}
			pl := priceLevelDump{
				LevelName: levelName,
				Currency:  currency,
				Discount:  discount,
				Prices:    make([]priceDump, 0, len(pm.Prices)),
			}
			for _, pr := range pm.Prices {
				fmt.Printf("    Quantity=%g  Value=%s\n", pr.Quantity.Float64(), pr.Value.String())
				pl.Prices = append(pl.Prices, priceDump{
					Quantity:   pr.Quantity.Float64(),
					Value: pr.Value.String(),
				})
			}
			itemOut.Pricing = append(itemOut.Pricing, pl)
		}
		output = append(output, itemOut)
	}

	// Ghi ra file JSON
	outWrapper := map[string]interface{}{
		"items": output,
	}
	buf, err := json.MarshalIndent(outWrapper, "", "  ")
	if err != nil {
		log.Fatalf("encode json error: %v", err)
	}

	fileName := "item_details.json"
	if err := os.WriteFile(fileName, buf, 0o644); err != nil {
		log.Fatalf("write %s error: %v", fileName, err)
	}

	log.Printf("Dumped %d items to %s\n", len(output), fileName)
}

// buildHardcodedClient tạo core.Client với credential NetSuite được hard-code.
// Mapping:
//   Token / TokenSecret       -> Consumer Key / Consumer Secret
//   AuthToken / AuthTokenSecret -> Token Id / Token Secret
func buildHardcodedClient() *cpb.Client {
	return &cpb.Client{
		Url:       "https://6555930.suitetalk.api.netsuite.com",
		AccountId: "6555930",
		// Để trống để go-netsuite dùng defaultSoapApiVersion ("2021_1")
		ApiVersion: "",

		// OAuth consumer key / secret
		Token:       "55c5b7f0aae6dea9aa97433a1108aedf5996f852ec96287cdf31e9c495dca7af",
		TokenSecret: "d99dd72ad15d4586b1415a69dc7181522f9bcff037833e5601fc38a9d096b306",

		// OAuth token id / secret
		AuthToken:       "dbd4b08ba1152e01af9b5ec2f0d9ede672d3ee107b871d29fc896ed05b23f9aa",
		AuthTokenSecret: "27c691d7260855f7ad1f50c51027a97e43416c61661c2827cb032b81c044df7e",
	}
}

