package services

import (
	"apple-store-helper/model"
	"context"
	"encoding/json"
	"github.com/tidwall/gjson"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestLivePickupLocationAudit(t *testing.T) {
	if os.Getenv("APPLE_LOCATION_AUDIT") != "1" {
		t.Skip("opt-in public pickup location audit")
	}
	type row struct {
		Area     string      `json:"area"`
		Store    model.Store `json:"store"`
		Location string      `json:"location_query"`
		HTTP     int         `json:"http"`
		Reason   string      `json:"reason"`
		Stores   []string    `json:"returned_stores"`
	}
	var results []row
	for _, area := range model.Areas {
		if area.PickupUnsupported {
			continue
		}
		var st struct {
			Stores []model.Store `json:"stores"`
		}
		raw, err := os.ReadFile(filepath.Join("..", "embedded", "data", "store", "store_"+area.ShortCode+".json"))
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal(raw, &st)
		var pr struct {
			Products map[string][]model.Product `json:"products"`
		}
		raw, err = os.ReadFile(filepath.Join("..", "embedded", "data", "product", "product_data_"+area.ShortCode+".json"))
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal(raw, &pr)
		var keys []string
		for k := range pr.Products {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		code := ""
		for _, k := range keys {
			if len(pr.Products[k]) > 0 {
				code = pr.Products[k][0].Code
				if len(k) >= 6 && k[:6] == "iPhone" {
					break
				}
			}
		}
		if code == "" {
			continue
		}
		seen := map[string]bool{}
		for _, store := range st.Stores {
			loc := pickupLocationQuery(area.ShortCode, store.StoreNumber, store.District)
			if seen[loc] {
				continue
			}
			seen[loc] = true
			if area.ShortCode != "cn" && area.ShortCode != "jp" && len(seen) > 1 {
				break
			}
			query := "pl=true&mts.0=regular&parts.0=" + code + "&" + loc
			if area.ShortCode == "jp" {
				query += "&cppart=UNLOCKED_JP"
			}
			link, _ := pickupMessagesURL(area.ShortCode, query)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			resp, err := sessionForArea(area.ShortCode).do(ctx, area.ShortCode, link)
			r := row{Area: area.ShortCode, Store: store, Location: loc}
			if err != nil {
				r.Reason = err.Error()
			} else {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				r.HTTP = resp.StatusCode
				r.Reason = gjson.GetBytes(body, "body.errorMessage").String()
				for _, p := range []string{"body.stores", "body.content.pickupMessage.stores"} {
					for _, x := range gjson.GetBytes(body, p).Array() {
						r.Stores = append(r.Stores, x.Get("storeNumber").String())
					}
				}
				if r.Reason == "" {
					_, err = parseStockResponse(body, link)
					if err != nil {
						r.Reason = err.Error()
					}
				}
			}
			cancel()
			results = append(results, r)
			data, _ := json.MarshalIndent(results, "", "  ")
			os.WriteFile("../workdoc/pickup-location-audit-20261023.json", data, 0644)
			t.Logf("%s %s %s HTTP=%d stores=%d reason=%s", r.Area, store.StoreNumber, loc, r.HTTP, len(r.Stores), r.Reason)
			if r.HTTP != 200 {
				t.Log("audit incomplete: stopped globally after upstream rejection; do not count untested stores as passed")
				return
			}
			time.Sleep(1200 * time.Millisecond)
		}
	}
}
