package main

import (
	"apple-store-helper/services"
	"strings"
	"testing"
)

func TestExpandedRegionsHaveOfflineCatalogAndLocalPurchaseURLs(t *testing.T) {
	app := NewDesktop()
	for _, code := range []string{"tw", "kr", "my", "th", "ca", "de", "fr", "it", "es"} {
		t.Run(code, func(t *testing.T) {
			catalog, e := app.GetCatalog(code)
			if e != nil {
				t.Fatal(e)
			}
			if len(catalog.Stores) == 0 || len(catalog.Products) == 0 {
				t.Fatal("empty offline catalog")
			}
			ids := map[string]bool{}
			for _, store := range catalog.Stores {
				if store.StoreNumber == "" || ids[store.StoreNumber] {
					t.Fatal("missing or duplicated store ID")
				}
				ids[store.StoreNumber] = true
			}
			for _, p := range catalog.Products {
				u, e := services.ProductPurchaseURL(code, p.Type, p.Code, p.PurchasePath)
				if e != nil || !strings.HasPrefix(u, "https://www.apple.com/"+code+"/shop/") {
					t.Fatalf("bad local product URL: %s %v", u, e)
				}
			}
			u, e := services.ShoppingBagURL(code)
			if e != nil || u != "https://www.apple.com/"+code+"/shop/bag" {
				t.Fatal("wrong shopping bag")
			}
		})
	}
}
func TestRetailDirectoryNestedStatesAndLocaleAlias(t *testing.T) {
	body := []byte(`<script id="__NEXT_DATA__">{"props":{"pageProps":{"storeList":[{"locale":"en_HK","state":[{"store":[{"id":"R001","name":"Central","address":{"city":"Hong Kong"}}]}],"states":[{"name":"Hong Kong","store":[{"id":"R001","name":"Central"}]}]},{"locale":"fr_FR","states":[{"name":"Paris","store":[{"id":"R002","name":"Opera","address":{"city":"Paris"}}]}]}]}}}</script>`)
	hk, e := parseStoreDirectory(body, "zh_HK")
	if e != nil || len(hk) != 1 || hk[0].StoreNumber != "R001" {
		t.Fatalf("alias or duplicate directory: %+v %v", hk, e)
	}
	fr, e := parseStoreDirectory(body, "fr_FR")
	if e != nil || len(fr) != 1 || fr[0].StoreNumber != "R002" {
		t.Fatalf("nested directory: %+v %v", fr, e)
	}
}

func TestGlobalStorefrontURLsStayInMarket(t *testing.T) {
	for _, test := range []struct{ code, base string }{{"ch", "https://www.apple.com/ch-fr"}, {"be", "https://www.apple.com/be-fr"}, {"in", "https://www.apple.com/in"}, {"ae", "https://www.apple.com/ae"}} {
		bag, e := services.ShoppingBagURL(test.code)
		if e != nil || bag != test.base+"/shop/bag" {
			t.Fatalf("%s: %s %v", test.code, bag, e)
		}
		for _, kind := range []string{"appletv4k", "macbookair", "macbookpro", "macmini", "imac", "macstudio"} {
			link, e := services.ProductPurchaseURL(test.code, kind, "MTEST/A", "")
			if e != nil || link != test.base+"/shop/product/MTEST/A" {
				t.Fatalf("%s/%s: %s %v", test.code, kind, link, e)
			}
		}
	}
}
