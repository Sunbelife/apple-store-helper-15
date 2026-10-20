package main

import (
	"apple-store-helper/model"
	"apple-store-helper/services"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type regionCheck struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Stores      int            `json:"stores"`
	Products    int            `json:"products"`
	Series      map[string]int `json:"series"`
	Store       string         `json:"store"`
	SKU         string         `json:"sku"`
	Stock       string         `json:"stock"`
	StockError  string         `json:"stock_error,omitempty"`
	BagHTTP     int            `json:"bag_http"`
	ProductHTTP int            `json:"product_http"`
	Error       string         `json:"error,omitempty"`
	CheckedAt   string         `json:"checked_at"`
}

// Opt-in: fetch public regional catalogs and make one stock request per region.
// Saves genuine regional seed data, never substitutes another market's SKU.
func TestExpandedRegionsLive(t *testing.T) {
	if os.Getenv("QNM_EXPAND_LIVE") != "1" {
		t.Skip("opt-in public Apple checks")
	}
	root := filepath.Clean("..")
	out := filepath.Join(root, "workdoc", "global-regions-live.json")
	client := &http.Client{Timeout: 20 * time.Second}
	r, e := client.Get("https://www.apple.com/retail/storelist/")
	if e != nil {
		t.Fatal(e)
	}
	directory, e := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	r.Body.Close()
	if e != nil || r.StatusCode != 200 {
		t.Fatal("directory unavailable", e)
	}
	codes := map[string]bool{}
	for _, a := range model.Areas {
		codes[a.ShortCode] = true
	}
	checks := make(chan regionCheck, len(codes))
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, a := range model.Areas {
		if !codes[a.ShortCode] {
			continue
		}
		wg.Add(1)
		go func(area model.Area) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			c := regionCheck{Code: area.ShortCode, Name: area.Title, Series: map[string]int{}, CheckedAt: time.Now().Format(time.RFC3339)}
			defer func() { checks <- c }()
			stores, e := parseStoreDirectory(directory, area.Locale)
			if e != nil {
				c.Error = e.Error()
				return
			}
			c.Stores = len(stores)
			data, e := services.FetchProductData(area.ShortCode)
			if e != nil {
				c.Error = e.Error()
				return
			}
			// Refresh seeds incrementally, preserving verified entries on transient failures.
			if raw, err := os.ReadFile(filepath.Join(root, "data", "product", "product_data_"+area.ShortCode+".json")); err == nil {
				var cached services.ProductData
				if json.Unmarshal(raw, &cached) == nil {
					for family, ps := range cached.Products {
						data.Products[family] = mergeProducts(ps, data.Products[family])
					}
				}
			}
			for name, ps := range data.Products {
				c.Series[name] = len(ps)
				c.Products += len(ps)
			}
			for _, path := range []string{filepath.Join(root, "data", "product"), filepath.Join(root, "embedded", "data", "product")} {
				os.MkdirAll(path, 0755)
				b, _ := json.MarshalIndent(data, "", "  ")
				if e := os.WriteFile(filepath.Join(path, "product_data_"+area.ShortCode+".json"), b, 0644); e != nil {
					c.Error = e.Error()
					return
				}
			}
			sd := services.StoreData{AreaCode: area.ShortCode, UpdateTime: time.Now().Format(time.RFC3339), Stores: stores}
			b, _ := json.MarshalIndent(sd, "", "  ")
			for _, path := range []string{filepath.Join(root, "data", "store"), filepath.Join(root, "embedded", "data", "store")} {
				os.MkdirAll(path, 0755)
				if e := os.WriteFile(filepath.Join(path, "store_"+area.ShortCode+".json"), b, 0644); e != nil {
					c.Error = e.Error()
					return
				}
			}
			ps := data.Products["iPhone Duo"]
			if len(ps) == 0 {
				c.Error = "iPhone Duo catalog missing"
				return
			}
			p := ps[0]
			c.SKU = p.Code
			c.Store = stores[0].StoreNumber
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			stock, e := services.QueryRetailPickup(ctx, area, stores[0], []model.Product{{Type: p.Type, Code: p.Code}})
			cancel()
			if e != nil {
				c.Stock = services.StockQueryErrorStatus(e)
				c.StockError = e.Error()
			} else if value, ok := stock[c.Store+"."+p.Code]; ok {
				if value {
					c.Stock = "有货"
				} else {
					c.Stock = "无货"
				}
			} else {
				c.Stock = "未知：响应没有目标门店"
			}
			bag, _ := services.ShoppingBagURL(area.ShortCode)
			link, _ := services.ProductPurchaseURL(area.ShortCode, p.Type, p.Code, p.PurchasePath)
			for i, u := range []string{bag, link} {
				r, e := client.Get(u)
				if e != nil {
					continue
				}
				io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
				r.Body.Close()
				if !strings.HasPrefix(r.Request.URL.String(), strings.TrimSuffix(bag, "/shop/bag")+"/shop/") {
					continue
				}
				if i == 0 {
					c.BagHTTP = r.StatusCode
				} else {
					c.ProductHTTP = r.StatusCode
				}
			}
		}(a)
	}
	wg.Wait()
	close(checks)
	var all []regionCheck
	for c := range checks {
		all = append(all, c)
		t.Logf("REGION %s stores=%d products=%d stock=%s bag=%d product=%d error=%s", c.Code, c.Stores, c.Products, c.Stock, c.BagHTTP, c.ProductHTTP, c.Error)
	}
	b, _ := json.MarshalIndent(all, "", "  ")
	if e := os.WriteFile(out, b, 0644); e != nil {
		t.Fatal(e)
	}
	for _, c := range all {
		if c.Error != "" || c.Stores == 0 || c.Products == 0 {
			t.Errorf("region %s incomplete: %s", c.Code, c.Error)
		}
	}
}

func TestExpandedRegionsInventoryLive(t *testing.T) {
	if os.Getenv("QNM_EXPAND_STOCK_LIVE") != "1" {
		t.Skip("opt-in stock-only follow-up")
	}
	path := filepath.Join("..", "workdoc", "expanded-regions-live.json")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var checks []regionCheck
	if e = json.Unmarshal(raw, &checks); e != nil {
		t.Fatal(e)
	}
	for i := range checks {
		c := &checks[i]
		area, e := areaFor(c.Code)
		if e != nil {
			t.Fatal(e)
		}
		app := NewDesktop()
		catalog, e := app.GetCatalog(c.Code)
		if e != nil {
			t.Fatal(e)
		}
		var store model.Store
		for _, s := range catalog.Stores {
			if s.StoreNumber == c.Store {
				store = s
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		stock, e := services.QueryRetailPickup(ctx, area, store, []model.Product{{Code: c.SKU, Type: "iphoneduo"}})
		cancel()
		c.StockError = ""
		if e != nil {
			c.Stock = services.StockQueryErrorStatus(e)
			c.StockError = e.Error()
		} else if v, ok := stock[c.Store+"."+c.SKU]; ok {
			if v {
				c.Stock = "有货"
			} else {
				c.Stock = "无货"
			}
		} else {
			c.Stock = "未知：未返回目标 SKU 的明确库存"
		}
		c.CheckedAt = time.Now().Format(time.RFC3339)
		t.Logf("REGION %s stock=%s", c.Code, c.Stock)
		if c.Stock != services.StatusOutStock && c.Stock != services.StatusInStock && c.Stock != services.StatusNotOpen {
			t.Errorf("unconfirmed inventory for %s: %s", c.Code, c.Stock)
		}
	}
	raw, _ = json.MarshalIndent(checks, "", "  ")
	if e = os.WriteFile(path, raw, 0644); e != nil {
		t.Fatal(e)
	}
}

// One standard SKU per newly supported product family and market, not every SKU.
func TestGlobalProductPickupLive(t *testing.T) {
	if os.Getenv("QNM_PRODUCT_PICKUP_LIVE") != "1" {
		t.Skip("opt-in public Apple pickup checks")
	}
	type result struct {
		Area      string `json:"area"`
		Family    string `json:"family"`
		SKU       string `json:"sku"`
		Store     string `json:"store"`
		Status    string `json:"status"`
		Detail    string `json:"detail,omitempty"`
		CheckedAt string `json:"checked_at"`
	}
	families := []string{"Apple TV 4K", "MacBook Air", "MacBook Pro", "Mac mini", "iMac", "Mac Studio"}
	var all []result
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	app := NewDesktop()
	// Load catalogs sequentially; the legacy store cache is not concurrent-safe.
	for _, a := range model.Areas {
		catalog, e := app.GetCatalog(a.ShortCode)
		if e != nil {
			t.Error(e)
			continue
		}
		data, e := services.LoadProductData(a.ShortCode)
		if e != nil {
			t.Error(e)
			continue
		}
		wg.Add(1)
		go func(area model.Area, catalog Catalog, data *services.ProductData) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			for _, family := range families {
				r := result{Area: area.ShortCode, Family: family, CheckedAt: time.Now().Format(time.RFC3339)}
				ps := data.Products[family]
				if len(ps) == 0 {
					r.Status = "当地目录无可验证标准配置"
				} else {
					p := ps[0]
					r.SKU = p.Code
					r.Store = catalog.Stores[0].StoreNumber
					ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
					values, e := services.QueryRetailPickup(ctx, area, catalog.Stores[0], []model.Product{{Code: p.Code, Type: p.Type}})
					cancel()
					if e != nil {
						r.Status = services.StockQueryErrorStatus(e)
						r.Detail = e.Error()
					} else if in, ok := values[r.Store+"."+p.Code]; ok {
						if in {
							r.Status = "有货"
						} else {
							r.Status = "无货"
						}
					} else {
						r.Status = "未返回目标门店库存"
					}
				}
				mu.Lock()
				all = append(all, r)
				mu.Unlock()
				t.Logf("PICKUP %s %s %s %s", r.Area, r.Family, r.SKU, r.Status)
			}
		}(a, catalog, data)
	}
	wg.Wait()
	b, _ := json.MarshalIndent(all, "", "  ")
	if e := os.WriteFile("../workdoc/global-products-pickup-live.json", b, 0644); e != nil {
		t.Fatal(e)
	}
}
