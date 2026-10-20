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
	for _, registered := range model.Areas {
		if registered.ShortCode == area.ShortCode && registered.PickupUnsupported {
			return nil, errPickupUnsupported
		}
	}
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

var errPickupUnsupported = errors.New("该地区 Apple 官网未开放门店取货查询")

var errPickupNotOpen = errors.New("Apple 尚未开放该产品的门店取货，请稍后查看；不是无货或风控")

func StockQueryErrorStatus(err error) string {
	if errors.Is(err, errPickupUnsupported) {
		return "暂不支持取货"
	}
	if errors.Is(err, errPickupNotOpen) {
		return StatusNotOpen
	}
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

// PickupSchedule carries explicit future pickup quotes alongside any current stock.
// Scheduled pickup is never treated as available-now inventory.
type PickupSchedule struct{ Quotes map[string]string }

func (p *PickupSchedule) Error() string { return "可预约取货，非当天现货" }
