package services

import (
	"apple-store-helper/model"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestStockLookupDoesNotMistake541NotFoundForVerification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(541)
		_, _ = w.Write([]byte(`<html><title>Page Not Found - Apple</title><h1>The page you're looking for can't be found.</h1></html>`))
	}))
	defer server.Close()
	service := listenService{Area: modelAreaForTest("cn")}
	_, err := service.getSkuByLink(server.URL)
	if !errors.Is(err, errAppleStockLimited) {
		t.Fatalf("got %v, want stock endpoint unavailable", err)
	}
	if errors.Is(err, errAppleVerificationRequired) {
		t.Fatal("not-found response must not ask user to verify")
	}
}

func TestStockHTTPResponseClassification(t *testing.T) {
	for _, status := range []int{403, 429, 503} {
		if !errors.Is(classifyStockHTTPResponse(status, nil), errAppleStockLimited) {
			t.Fatalf("status %d must remain unknown stock, not verification", status)
		}
	}
	if classifyStockHTTPResponse(200, []byte(`{"body":{}}`)) != nil {
		t.Fatal("valid HTTP response should proceed to stock parser")
	}
	if !errors.Is(classifyStockHTTPResponse(541, []byte(`<html>browser verification required</html>`)), errAppleVerificationRequired) {
		t.Fatal("actual verification response misclassified")
	}
}

func TestNewPickupResponseUnavailableAndMissingState(t *testing.T) {
	body := []byte(`{"body":{"stores":[{"storeNumber":"R572","partsAvailability":{"MK2P4CH/A":{"pickupDisplay":"unavailable"},"OTHERCH/A":{"pickupDisplay":"unknown"},"EMPTYCH/A":{}}}]}}`)
	stock, err := parseStockResponse(body, "")
	if err != nil {
		t.Fatal(err)
	}
	if available, found := stock["R572.MK2P4CH/A"]; !found || available {
		t.Fatalf("explicit unavailable lost: %v", stock)
	}
	for _, key := range []string{"R572.OTHERCH/A", "R572.EMPTYCH/A", "R572.MISSINGCH/A"} {
		if _, found := stock[key]; found {
			t.Fatalf("unknown result falsely becomes no stock: %s", key)
		}
	}
}

func TestLiveRetailPickup(t *testing.T) {
	if os.Getenv("APPLE_STOCK_LIVE") != "1" {
		t.Skip("opt-in public Apple API check")
	}
	url, err := pickupMessagesURL("cn", "pl=true&mts.0=regular&parts.0=MK2P4CH%2FA&store=R572")
	if err != nil {
		t.Fatal(err)
	}
	service := listenService{Area: modelAreaForTest("cn")}
	stock, err := service.getSkuByLink(url)
	if err != nil {
		t.Fatal(err)
	}
	available, found := stock["R572.MK2P4CH/A"]
	if !found {
		t.Fatal("target store/SKU missing from live response")
	}
	t.Logf("Real Apple retail pickup response: R572 MK2P4CH/A available=%v", available)
}
