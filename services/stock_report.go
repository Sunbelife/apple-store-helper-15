package services

import (
	"apple-store-helper/model"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const stockReportEndpoint = "https://sunbelife.com/api/iphone-stock/events"

// StockObservation contains only public store and product facts, never client settings or identity.
type StockObservation struct {
	Area        string `json:"area"`
	Province    string `json:"province"`
	City        string `json:"city"`
	StoreNumber string `json:"store_number"`
	StoreName   string `json:"store_name"`
	Model       string `json:"model"`
	Capacity    string `json:"capacity"`
	Color       string `json:"color"`
	SKU         string `json:"sku"`
	DetectedAt  string `json:"detected_at"`
}

var productTitleParts = regexp.MustCompile(`^(.+?)\s+(\d+(?:\.\d+)?(?:GB|TB|mm))\s+(.+)$`)

func newStockObservation(area string, store model.Store, product model.Product, detectedAt time.Time) StockObservation {
	observation := StockObservation{
		Area: area, Province: store.Province, City: store.City,
		StoreNumber: store.StoreNumber, StoreName: store.CityStoreName,
		SKU: product.Code, DetectedAt: detectedAt.UTC().Format(time.RFC3339),
	}
	if parts := productTitleParts.FindStringSubmatch(product.Title); len(parts) == 4 {
		observation.Model, observation.Capacity, observation.Color = parts[1], parts[2], parts[3]
	}
	if observation.City == "" {
		observation.City = strings.SplitN(store.CityStoreName, "-", 2)[0]
	}
	return observation
}

func sendStockObservation(ctx context.Context, client *http.Client, endpoint string, observation StockObservation) error {
	if observation.Model == "" || observation.Capacity == "" || observation.Color == "" || observation.SKU == "" || observation.StoreNumber == "" {
		return fmt.Errorf("incomplete inventory observation")
	}
	body, err := json.Marshal(observation)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "AppleStockReporter")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("inventory endpoint status %d", response.StatusCode)
	}
	return nil
}

// Called asynchronously after the stock alert is shown; reporting cannot delay the alert.
func reportStockObservation(observation StockObservation) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	for attempt := 0; attempt < 2; attempt++ {
		if sendStockObservation(ctx, client, stockReportEndpoint, observation) == nil {
			return true
		}
		if attempt == 0 {
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return false
			}
		}
	}
	return false
}
