package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFeaturedProductsKeepMiniProgramPromotionLinks(t *testing.T) {
	var products []shopProduct
	if err := json.Unmarshal(shopProductsJSON, &products); err != nil {
		t.Fatal(err)
	}
	if len(products) != 9 {
		t.Fatalf("got %d featured products, want nine", len(products))
	}
	seen := map[string]bool{}
	for _, p := range products {
		if seen[p.ID] {
			t.Fatal("duplicate featured product")
		}
		seen[p.ID] = true
		link, err := shopProductURL(p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if link != p.URL || !strings.HasPrefix(p.Source, "qiangnimei_2026_") {
			t.Fatalf("lost product promotion attribution: %+v", p)
		}
	}
	if _, err := shopProductURL("https://example.com"); err == nil {
		t.Fatal("arbitrary URLs must not be opened")
	}
}
