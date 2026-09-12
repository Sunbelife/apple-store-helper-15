package services

import (
	"apple-store-helper/model"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseStockResponse(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		stockURL string
		key      string
		want     bool
	}{
		{
			name: "store list response",
			body: `{"body":{"content":{"pickupMessage":{"stores":[{"storeNumber":"R359","partsAvailability":{"MHU64CH/A":{"pickupDisplay":"available"}}}]}}}}`,
			key:  "R359.MHU64CH/A",
			want: true,
		},
		{
			name: "China root store response",
			body: `{"body":{"stores":[{"storeNumber":"R390","partsAvailability":{"MHU64CH/A":{"pickupDisplay":"ineligible"}}}]}}`,
			key:  "R390.MHU64CH/A",
			want: false,
		},
		{
			name:     "single store response",
			body:     `{"body":{"content":{"pickupMessage":{"pickupEligibility":{"MHU64CH/A":{"messageTypes":{"regular":{"pickupDisplay":"available"}}}}}}}}`,
			stockURL: "https://www.apple.com.cn/shop/fulfillment-messages?store=R401",
			key:      "R401.MHU64CH/A",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStockResponse([]byte(tt.body), tt.stockURL)
			if err != nil {
				t.Fatal(err)
			}
			if got[tt.key] != tt.want {
				t.Fatalf("got %v for %s, want %v", got[tt.key], tt.key, tt.want)
			}
		})
	}
}

func TestParseStockResponseRejectsHTML(t *testing.T) {
	if _, err := parseStockResponse([]byte("<html>blocked</html>"), ""); err == nil {
		t.Fatal("expected HTML response to fail")
	}
}

func TestStockLookupDetectsAppleVerification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(541)
	}))
	defer server.Close()

	service := listenService{Area: modelAreaForTest("cn")}
	_, err := service.getSkuByLink(server.URL)
	if !errors.Is(err, errAppleVerificationRequired) {
		t.Fatalf("got %v, want Apple verification error", err)
	}
}

func modelAreaForTest(shortCode string) model.Area {
	for _, area := range model.Areas {
		if area.ShortCode == shortCode {
			return area
		}
	}
	return model.Area{}
}
