package main

import (
	"apple-store-helper/model"
	"testing"
)

func TestCatalogIncrementalMerge(t *testing.T) {
	stores := mergeStores([]model.Store{{StoreNumber: "R001", City: "Old", District: "123"}, {StoreNumber: "R002"}}, []model.Store{{StoreNumber: "R001", City: "New"}, {StoreNumber: "R003"}})
	if len(stores) != 3 || stores[0].City != "New" || stores[0].District != "123" || stores[1].StoreNumber != "R002" {
		t.Fatalf("lost cached store data: %+v", stores)
	}
	products := mergeProducts([]model.ProductInfo{{Code: "a", Model: "Old"}, {Code: "b"}}, []model.ProductInfo{{Code: "a", Model: "New"}, {Code: "c"}})
	found := map[string]string{}
	for _, p := range products {
		found[p.Code] = p.Model
	}
	if len(products) != 3 || found["a"] != "New" {
		t.Fatalf("unexpected products: %+v", products)
	}
}
func TestStoreDirectorySelectsRegion(t *testing.T) {
	body := []byte(`<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"storeList":[{"locale":"zh_CN","state":[{"store":[{"id":"R639","name":"珠江新城 ","address":{"city":"广州","stateName":"广东","postalCode":"510623"}}]}]},{"locale":"en_US","state":[{"store":[{"id":"R001","name":"Test"}]}]}]}}}</script>`)
	stores, err := parseStoreDirectory(body, "zh_CN")
	if err != nil || len(stores) != 1 || stores[0].CityStoreName != "广州-珠江新城" || stores[0].District != "510623" {
		t.Fatalf("bad directory: %+v %v", stores, err)
	}
	if _, err := parseStoreDirectory(body, "en_JP"); err == nil {
		t.Fatal("empty region accepted")
	}
	if _, err := parseStoreDirectory([]byte("blocked"), "zh_CN"); err == nil {
		t.Fatal("invalid response accepted")
	}
}

func TestStoreDirectoryWithoutStates(t *testing.T) {
	body := []byte(`<script id="__NEXT_DATA__">{"props":{"pageProps":{"storeList":[{"locale":"en_SG","states":[{"id":"R669","name":"Orchard Road","address":{"city":"Singapore","postalCode":"238857"}}]}]}}}</script>`)
	stores, err := parseStoreDirectory(body, "en_SG")
	if err != nil || len(stores) != 1 || stores[0].StoreNumber != "R669" {
		t.Fatalf("bad city region: %+v %v", stores, err)
	}
}
