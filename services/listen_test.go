package services

import (
	"apple-store-helper/model"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
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

func TestStoreResultsRequireAllExplicitSKUs(t *testing.T) {
	items := []ListenItem{
		{Store: model.Store{StoreNumber: "R639"}, Product: model.Product{Code: "MK2P4CH/A"}},
		{Store: model.Store{StoreNumber: "R639"}, Product: model.Product{Code: "OTHERCH/A"}},
	}
	results := map[string]bool{"R639.MK2P4CH/A": false}
	if storeResultsComplete(items, results) {
		t.Fatal("missing SKU must still be queried")
	}
	results["R639.OTHERCH/A"] = true
	if !storeResultsComplete(items, results) {
		t.Fatal("both true and false are explicit results and may be reused")
	}
}

func TestLivePostalPickupGuangdong(t *testing.T) {
	if os.Getenv("APPLE_STOCK_LIVE") != "1" {
		t.Skip("opt-in public Apple API check")
	}
	link, err := pickupMessagesURL("cn", "pl=true&mts.0=regular&parts.0=MK2P4CH%2FA&"+pickupLocationQuery("cn", "R639", "510623"))
	if err != nil {
		t.Fatal(err)
	}
	service := listenService{Area: modelAreaForTest("cn")}
	stock, err := service.getSkuByLink(link)
	if err != nil {
		t.Fatal(err)
	}
	for _, store := range []string{"R639", "R577", "R793", "R484", "R761"} {
		available, exists := stock[store+".MK2P4CH/A"]
		if !exists {
			t.Errorf("missing monitored store %s", store)
		}
		t.Logf("%s available=%v", store, available)
	}
}

func TestStockBackoff(t *testing.T) {
	delay := time.Duration(0)
	for _, want := range []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute} {
		delay = nextStockRetryBackoff(delay, true)
		if delay != want {
			t.Fatalf("got %s want %s", delay, want)
		}
	}
	if nextStockRetryBackoff(delay, false) != 0 {
		t.Fatal("successful round must clear backoff")
	}
}

func TestNotYetOpenPickupIsDistinctFromNoStockAndRiskControl(t *testing.T) {
	body := []byte(`{"body":{"stores":[{"storeNumber":"R369","partsAvailability":{"REGIONAL/A":{"pickupDisplay":"default","messageTypes":{"regular":{"storeSelectionEnabled":false}}}}}]}}`)
	stock, err := parseStockResponse(body, "")
	if !errors.Is(err, errPickupNotOpen) || StockQueryErrorStatus(err) != StatusNotOpen || len(stock) != 0 {
		t.Fatalf("not-open state lost: %v %v", stock, err)
	}
	// Unknown/default without an explicit disabled selector must stay unknown.
	stock, err = parseStockResponse([]byte(`{"body":{"stores":[{"storeNumber":"R369","partsAvailability":{"REGIONAL/A":{"pickupDisplay":"default"}}}]}}`), "")
	if err != nil || len(stock) != 0 {
		t.Fatal("unknown pickup must not be classified without evidence")
	}
	// An available SKU is never hidden by another product awaiting its release.
	stock, err = parseStockResponse([]byte(`{"body":{"stores":[{"storeNumber":"R369","partsAvailability":{"REGIONAL/A":{"pickupDisplay":"default","messageTypes":{"regular":{"storeSelectionEnabled":false}}},"OTHER/A":{"pickupDisplay":"available"}}}]}}`), "")
	if err != nil || !stock["R369.OTHER/A"] {
		t.Fatal("partial available stock lost")
	}
}

func TestHongKongAppleTVScheduledPickup(t *testing.T) {
	body := []byte(`{"body":{"stores":[{"storeNumber":"R428","partsAvailability":{"MLFF4ZP/A":{"storePickEligible":true,"pickupDisplay":"default","pickupSearchQuote":"Available Nov 13","messageTypes":{"regular":{"storeSelectionEnabled":true,"storePickupQuote":"Nov 13 at Apple ifc mall"}}},"OTHER/A":{"pickupDisplay":"available"}}}]}}`)
	stock, err := parseStockResponse(body, "")
	var scheduled *PickupSchedule
	if !errors.As(err, &scheduled) || scheduled.Quotes["R428.MLFF4ZP/A"] != "Nov 13 at Apple ifc mall" {
		t.Fatalf("lost pickup date: %v", err)
	}
	if _, exists := stock["R428.MLFF4ZP/A"]; exists {
		t.Fatal("scheduled pickup was classified as current inventory")
	}
	if !stock["R428.OTHER/A"] {
		t.Fatal("mixed current stock lost")
	}
	for _, b := range []string{
		`{"body":{"stores":[{"storeNumber":"R428","partsAvailability":{"MLFF4ZP/A":{"pickupDisplay":"default","storePickEligible":true,"messageTypes":{"regular":{"storeSelectionEnabled":true}}}}}]}}`,
		`{"body":{"stores":[{"storeNumber":"R428","partsAvailability":{"MLFF4ZP/A":{"pickupDisplay":"default","pickupSearchQuote":"Available Nov 13","storePickEligible":false,"messageTypes":{"regular":{"storeSelectionEnabled":true}}}}}]}}`,
	} {
		_, e := parseStockResponse([]byte(b), "")
		if errors.As(e, &scheduled) {
			t.Fatal("incomplete evidence treated as scheduled pickup")
		}
	}
}
