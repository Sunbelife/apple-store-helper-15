package services

import (
	"apple-store-helper/model"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStockObservationOnlyIncludesPublicInventory(t *testing.T) {
	store := model.Store{StoreNumber: "R001", CityStoreName: "深圳-益田假日广场", Province: "广东", City: "深圳"}
	product := model.Product{Title: "iPhone Duo 512GB 星光白色", Code: "MK2P4CH/A"}
	observation := newStockObservation("cn", store, product, time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC))
	if observation.Model != "iPhone Duo" || observation.Capacity != "512GB" || observation.Color != "星光白色" {
		t.Fatalf("wrong product: %+v", observation)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("wrong request")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if len(payload) != 10 {
			t.Errorf("unexpected fields: %v", payload)
		}
		for _, key := range []string{"area", "province", "city", "store_number", "store_name", "model", "capacity", "color", "sku", "detected_at"} {
			if _, ok := payload[key]; !ok {
				t.Errorf("missing %s", key)
			}
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	if err := sendStockObservation(context.Background(), server.Client(), server.URL, observation); err != nil {
		t.Fatal(err)
	}
}

func TestStockReportRejectsIncompleteProductAndReturnsServerFailure(t *testing.T) {
	if sendStockObservation(context.Background(), http.DefaultClient, "invalid", StockObservation{}) == nil {
		t.Fatal("expected incomplete observation to fail")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	observation := newStockObservation("cn", model.Store{StoreNumber: "R001", CityStoreName: "上海-浦东", City: "上海"}, model.Product{Title: "iPhone 18 Pro Max 2TB 银色", Code: "TESTCH/A"}, time.Now())
	if sendStockObservation(context.Background(), server.Client(), server.URL, observation) == nil {
		t.Fatal("expected server failure")
	}
}
