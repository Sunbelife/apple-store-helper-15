package main

import (
	"apple-store-helper/embedded"
	"encoding/json"
	"testing"
)

func TestEmbeddedProductData(t *testing.T) {
	for _, region := range []string{"cn", "hk", "jp", "sg", "us", "uk", "au"} {
		data, ok := embedded.GetProductData(region)
		if !ok {
			t.Fatalf("missing embedded product data for %s", region)
		}

		var productData struct {
			Products map[string][]struct {
				PurchasePath string `json:"PurchasePath"`
			} `json:"products"`
		}
		if err := json.Unmarshal(data, &productData); err != nil {
			t.Fatalf("parse embedded product data for %s: %v", region, err)
		}
		if len(productData.Products) == 0 {
			t.Fatalf("embedded product data for %s has no products", region)
		}
		for _, removed := range []string{"iPhone 17", "iPhone 17e", "iPhone Air"} {
			if len(productData.Products[removed]) != 0 {
				t.Fatalf("embedded product data for %s still contains %s", region, removed)
			}
		}
		if got := len(productData.Products["iPhone Duo"]); got != 8 {
			t.Fatalf("embedded product data for %s has %d iPhone Duo products, want 8", region, got)
		}
		if got := len(productData.Products["iPhone 18 Pro"]); got != 32 {
			t.Fatalf("embedded product data for %s has %d iPhone 18 Pro products, want 32", region, got)
		}
		watchCounts := map[string]int{
			"Apple Watch Series 12": 24,
			"Apple Watch SE 3":      8,
			"Apple Watch Ultra 4":   2,
		}
		for series, want := range watchCounts {
			products := productData.Products[series]
			if got := len(products); got != want {
				t.Fatalf("embedded product data for %s has %d %s products, want %d", region, got, series, want)
			}
			for _, product := range products {
				if product.PurchasePath == "" {
					t.Fatalf("embedded product data for %s has %s without a purchase path", region, series)
				}
			}
		}
	}
}

func TestEmbeddedStoreData(t *testing.T) {
	for _, region := range []string{"cn", "hk", "jp", "us", "uk", "au"} {
		data, ok := embedded.GetStoreData(region)
		if !ok {
			t.Fatalf("missing embedded store data for %s", region)
		}

		var storeData struct {
			Stores []json.RawMessage `json:"stores"`
		}
		if err := json.Unmarshal(data, &storeData); err != nil {
			t.Fatalf("parse embedded store data for %s: %v", region, err)
		}
		if len(storeData.Stores) == 0 {
			t.Fatalf("embedded store data for %s has no stores", region)
		}
	}
}
