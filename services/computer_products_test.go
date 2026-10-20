package services

import (
	"apple-store-helper/model"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func TestStandardRetailPartsExcludeCTO(t *testing.T) {
	parts, err := standardRetailParts(`productSelectionData:{"products":[{"type":"CONFIGURABLE","btrOrFdPartNumber":null,"aosContainerPartNumber":"NOT_A_SKU"},{"btrOrFdPartNumber":"MTESTLL/A"},{"partNumber":"MTVLL/A"},{"btrOrFdPartNumber":"MTESTLL/A"}]}`)
	if err != nil || len(parts) != 2 {
		t.Fatalf("parts=%v err=%v", parts, err)
	}
}
func TestComputerConfigurationExactSKU(t *testing.T) {
	body := `<link rel="canonical" href="https://www.apple.com.cn/shop/buy-mac/macbook-air/13-inch-sky-blue-m5-chip-10-core-cpu-8-core-gpu-16gb-memory-512gb-storage"><script id="metrics">{"data":{"products":[{"partNumber":"MTESTCH/A"}]}}</script>`
	p, err := parseComputerProduct(body, "MTESTCH/A", "MacBook Air", "macbookair")
	if err != nil || p.Model != "MacBook Air M5 13″" || !strings.Contains(p.Capacity, "16GB RAM / 512GB SSD") || !strings.Contains(p.Capacity, "8 GPU") {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := parseComputerProduct(body, "MOTHERLL/A", "MacBook Air", "macbookair"); err == nil {
		t.Fatal("accepted another market's SKU")
	}
	if _, err := parseComputerProduct(strings.ReplaceAll(body, "16gb-memory-", ""), "MTESTCH/A", "MacBook Air", "macbookair"); err == nil {
		t.Fatal("accepted missing RAM")
	}
}
func TestComputerLive(t *testing.T) {
	if os.Getenv("QNM_COMPUTER_LIVE") != "1" {
		t.Skip("opt-in live product checks")
	}
	for _, code := range []string{"cn", "us"} {
		base, _ := appleStoreBaseURL(code)
		for _, s := range additionalProductSeries {
			ps, e := fetchComputerProducts(base, s.Path, s.Name, s.Type)
			t.Logf("%s %s: %d %v", code, s.Name, len(ps), e)
			if e != nil && !(code == "cn" && s.Type == "appletv4k") {
				t.Error(e)
			}
		}
	}
}

func TestLocalizedMacConfigurations(t *testing.T) {
	for _, slug := range []string{
		"13-zoll-himmelblau-m5-chip-10‑core-cpu-8‑core-gpu-16-gb-arbeitsspeicher-512gb-speicher",
		"13-pouces-bleu-ciel-puce-m5-cpu-10-cœurs-gpu-8-cœurs-16-go-mémoire-512go-stockage",
		"13インチ-sky-blue-m5-チップ-10コアcpu-8コアgpu-16gb-のメモリ-512gb-のストレージ",
		"13-吋-sky-blue-m5-晶片-10-核心-cpu-8-核心-gpu-16gb-記憶體-512gb-儲存裝置",
		"13‑inch-hemelsblauw-m5-chip-10‑core-cpu-8‑core-gpu-16-gb-geheugen-512gb-opslag",
	} {
		body := `<link rel="canonical" href="https://www.apple.com/shop/buy-mac/macbook-air/` + slug + `"><script id="metrics">{"data":{"products":[{"partNumber":"MTEST/A","name":"13-inch MacBook Air - Sky Blue"}]}}</script>`
		p, e := parseComputerProduct(body, "MTEST/A", "MacBook Air", "macbookair")
		if e != nil || p.Model != "MacBook Air M5 13″" || p.Color != "天蓝色" || !strings.Contains(p.Capacity, "16GB RAM / 512GB SSD") || !strings.Contains(p.Capacity, "10 CPU") || !strings.Contains(p.Capacity, "8 GPU") {
			t.Errorf("%s: %+v %v", slug, p, e)
		}
	}
}

func TestNormalizeMacSeedChipsLive(t *testing.T) {
	if os.Getenv("QNM_NORMALIZE_MAC_LIVE") != "1" {
		t.Skip("opt-in official Mac configuration metadata refresh")
	}
	type update struct {
		code string
		data *ProductData
	}
	results := make(chan update, len(model.Areas))
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, area := range model.Areas {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			raw, e := os.ReadFile("../data/product/product_data_" + code + ".json")
			if e != nil {
				t.Error(e)
				return
			}
			var d ProductData
			if json.Unmarshal(raw, &d) != nil {
				return
			}
			cache := map[string]model.ProductInfo{}
			for _, ps := range d.Products {
				for _, p := range ps {
					cache[p.Code] = p
				}
			}
			base, _ := appleStoreBaseURL(code)
			for _, s := range additionalProductSeries {
				if s.Type == "appletv4k" {
					continue
				}
				needs := false
				for _, p := range d.Products[s.Name] {
					if !regexp.MustCompile(`M\d`).MatchString(p.Model) {
						needs = true
					}
				}
				if !needs {
					continue
				}
				ps, e := fetchComputerProducts(base, s.Path, s.Name, s.Type, cache)
				if e != nil {
					t.Errorf("%s %s: %v", code, s.Name, e)
					continue
				}
				d.Products[s.Name] = ps
			}
			for family, ps := range d.Products {
				for i := range ps {
					if ps[i].Color == "Standard" {
						ps[i].Color = "标准款"
					}
				}
				d.Products[family] = ps
			}
			results <- update{code, &d}
		}(area.ShortCode)
	}
	wg.Wait()
	close(results)
	for r := range results {
		raw, _ := json.MarshalIndent(r.data, "", "  ")
		for _, folder := range []string{"../data/product/", "../embedded/data/product/"} {
			if e := os.WriteFile(folder+"product_data_"+r.code+".json", raw, 0644); e != nil {
				t.Error(e)
			}
		}
	}
}

func TestBrazilPickupIsExplicitlyUnavailable(t *testing.T) {
	area := model.Area{ShortCode: "br"}
	_, e := QueryRetailPickup(context.Background(), area, model.Store{}, nil)
	if e == nil || StockQueryErrorStatus(e) != "暂不支持取货" {
		t.Fatalf("unsupported market must fail before making a request: %v", e)
	}
}

func TestCatalogRejectsCrossMarketRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/de/shop/buy-mac/macbook-air" {
			http.Redirect(w, r, "/shop/buy-mac/macbook-air", http.StatusFound)
			return
		}
		w.Write([]byte("a valid-looking product page"))
	}))
	defer server.Close()
	if _, e := fetchProductPage(server.URL + "/de/shop/buy-mac/macbook-air"); e == nil {
		t.Fatal("accepted a redirect to a different storefront")
	}
}
