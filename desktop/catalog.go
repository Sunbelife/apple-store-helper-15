package main

import (
	"apple-store-helper/model"
	"apple-store-helper/services"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Merge by stable Apple identifiers: missing remote entries never remove cached choices.
func mergeStores(old, fresh []model.Store) []model.Store {
	out := append([]model.Store{}, old...)
	positions := map[string]int{}
	for i, s := range out {
		positions[s.StoreNumber] = i
	}
	for _, s := range fresh {
		if s.StoreNumber == "" {
			continue
		}
		if i, ok := positions[s.StoreNumber]; ok {
			if s.CityStoreName != "" {
				out[i].CityStoreName = s.CityStoreName
			}
			if s.Province != "" {
				out[i].Province = s.Province
			}
			if s.City != "" {
				out[i].City = s.City
			}
			if s.District != "" {
				out[i].District = s.District
			}
		} else {
			positions[s.StoreNumber] = len(out)
			out = append(out, s)
		}
	}
	return out
}
func mergeProducts(old, fresh []model.ProductInfo) []model.ProductInfo {
	out := append([]model.ProductInfo{}, old...)
	positions := map[string]int{}
	for i, p := range out {
		positions[p.Code] = i
	}
	for _, p := range fresh {
		if p.Code == "" {
			continue
		}
		p.Color = services.NormalizeProductColor(p.Color)
		if i, ok := positions[p.Code]; ok {
			out[i] = p
		} else {
			positions[p.Code] = len(out)
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Model+out[i].Capacity+out[i].Color < out[j].Model+out[j].Capacity+out[j].Color
	})
	return out
}

var nextDataScript = regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

func parseStoreDirectory(body []byte, locale string) ([]model.Store, error) {
	match := nextDataScript.FindSubmatch(body)
	if len(match) < 2 {
		return nil, fmt.Errorf("门店目录格式暂不支持")
	}
	type directoryStore struct {
		ID      string
		Name    string
		Address struct{ City, PostalCode, StateName string }
	}
	var data struct {
		Props struct {
			PageProps struct {
				StoreList []struct {
					Locale string
					State  []struct{ Store []directoryStore }
					States []struct {
						directoryStore
						Store []directoryStore
					}
				}
			}
		}
	}

	if err := json.Unmarshal(match[1], &data); err != nil {
		return nil, err
	}
	var stores []model.Store
	for _, region := range data.Props.PageProps.StoreList {
		if region.Locale != locale && !strings.HasSuffix(region.Locale, "_"+strings.Split(locale, "_")[len(strings.Split(locale, "_"))-1]) {
			continue
		}
		var entries []directoryStore
		if len(region.State) == 0 {
			for _, entry := range region.States {
				if len(entry.Store) > 0 {
					entries = append(entries, entry.Store...)
				} else {
					entries = append(entries, entry.directoryStore)
				}
			}
		}
		for _, state := range region.State {
			entries = append(entries, state.Store...)
		}
		for _, s := range entries {
			if s.ID == "" || strings.TrimSpace(s.Name) == "" {
				continue
			}
			stores = append(stores, model.Store{StoreNumber: s.ID, CityStoreName: strings.TrimSpace(s.Address.City) + "-" + strings.TrimSpace(s.Name), Province: s.Address.StateName, City: s.Address.City, District: s.Address.PostalCode})
		}
	}
	if len(stores) == 0 {
		return nil, fmt.Errorf("官网未返回该地区门店，已保留本地数据")
	}
	return stores, nil
}
func fetchStoreDirectory(code string) ([]model.Store, error) {
	area, err := areaFor(code)
	if err != nil {
		return nil, err
	}
	locale := area.Locale
	// The official retail directory includes every region, independent of stock APIs.
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://www.apple.com.cn/retail/storelist/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("门店目录 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return parseStoreDirectory(body, locale)
}
func (a *Desktop) refreshCatalog(code string) (Catalog, error) {
	cached, err := a.GetCatalog(code)
	if err != nil {
		return Catalog{}, err
	}
	var products *services.ProductData
	var stores []model.Store
	var productErr, storeErr error
	done := make(chan struct{})
	go func() { stores, storeErr = fetchStoreDirectory(code); close(done) }()
	products, productErr = services.FetchProductData(code)
	<-done
	if productErr == nil {
		var fresh []model.ProductInfo
		for _, ps := range products.Products {
			fresh = append(fresh, ps...)
		}
		cached.Products = mergeProducts(cached.Products, fresh)
	}
	if storeErr == nil {
		cached.Stores = mergeStores(cached.Stores, stores)
	}
	// Persist the merged result outside the signed application bundle.
	if a.settingsPath != "" {
		dir := filepath.Join(filepath.Dir(a.settingsPath), "catalogs")
		if err := os.MkdirAll(dir, 0700); err != nil {
			return Catalog{}, err
		}
		data := services.ProductData{AreaCode: code, UpdateTime: time.Now().Format(time.RFC3339), Products: map[string][]model.ProductInfo{}}
		for _, p := range cached.Products {
			data.Products[p.Model] = append(data.Products[p.Model], p)
		}
		for suffix, value := range map[string]interface{}{".json": data, "-stores.json": cached.Stores} {
			b, err := json.Marshal(value)
			if err != nil {
				return Catalog{}, err
			}
			if err := os.WriteFile(filepath.Join(dir, code+suffix), b, 0600); err != nil {
				return Catalog{}, err
			}
		}
	}
	a.catalogMu.Lock()
	a.catalog[code] = cached
	a.catalogMu.Unlock()
	var failures []error
	if productErr != nil {
		failures = append(failures, fmt.Errorf("产品：%w", productErr))
	}
	if storeErr != nil {
		failures = append(failures, fmt.Errorf("门店：%w", storeErr))
	}
	return cached, errors.Join(failures...)
}
