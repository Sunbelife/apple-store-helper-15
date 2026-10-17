package services

import (
	"apple-store-helper/model"
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// QueryRetailPickup shares the desktop endpoint and parser without touching Fyne UI.
func QueryRetailPickup(ctx context.Context, area model.Area, store model.Store, products []model.Product) (map[string]bool, error) {
	query := "pl=true"
	for i, p := range products {
		query += fmt.Sprintf("&mts.%d=regular&parts.%d=%s", i, i, url.QueryEscape(p.Code))
	}
	query += "&" + pickupLocationQuery(area.ShortCode, store.StoreNumber, store.District)
	if area.ShortCode == "jp" {
		query += "&cppart=UNLOCKED_JP"
	}
	link, err := pickupMessagesURL(area.ShortCode, query)
	if err != nil {
		return nil, err
	}
	service := listenService{Area: area}
	return service.getSkuByLinkContext(ctx, link)
}

func StockQueryErrorStatus(err error) string {
	if errors.Is(err, errAppleStockLimited) {
		return StatusLimited
	}
	if errors.Is(err, errAppleVerificationRequired) {
		return StatusVerify
	}
	return StatusError
}
func StockRetryBackoff(previous time.Duration, limited bool) time.Duration {
	return nextStockRetryBackoff(previous, limited)
}
func NormalizeProductColor(color string) string { return translateColor(color) }
func ReportPublicStock(area string, store model.Store, product model.Product, detectedAt time.Time) {
	reportStockObservation(newStockObservation(area, store, product, detectedAt))
}
