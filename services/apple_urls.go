package services

import (
	"apple-store-helper/model"
	"fmt"
	"net/url"
	"strings"
)

func appleStoreBaseURL(areaCode string) (string, error) {
	switch areaCode {
	case "cn":
		return "https://www.apple.com.cn", nil
	case "ch":
		return "https://www.apple.com/ch-fr", nil
	case "be":
		return "https://www.apple.com/be-fr", nil
	case "us":
		return "https://www.apple.com", nil

	default:
		for _, area := range model.Areas {
			if area.ShortCode == areaCode {
				return "https://www.apple.com/" + areaCode, nil
			}
		}
		return "", fmt.Errorf("unsupported area code: %s", areaCode)
	}
}

func pickupMessagesURL(areaCode string, query string) (string, error) {
	baseURL, err := appleStoreBaseURL(areaCode)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/shop/retail/pickup-message?%s", baseURL, query), nil
}

// ShoppingBagURL opens the user's existing browser bag; it does not add items.
func ShoppingBagURL(areaCode string) (string, error) {
	baseURL, err := appleStoreBaseURL(areaCode)
	if err != nil {
		return "", err
	}
	return baseURL + "/shop/bag", nil
}

// ProductPurchaseURL returns the locale-specific Apple purchase page.
// iPhone uses Apple's SKU resolver; Watch uses its complete configuration path.
func ProductPurchaseURL(areaCode string, productType string, partNumber string, purchasePath string) (string, error) {
	baseURL, err := appleStoreBaseURL(areaCode)
	if err != nil {
		return "", err
	}

	switch productType {
	case "iphoneduo", "iphone18promax", "iphone18pro",
		"iphone16promax", "iphone16pro", "iphone16plus", "iphone16",
		"watchs12", "watchse3", "watchultra4",
		"appletv4k", "macbookair", "macbookpro", "macmini", "imac", "macstudio":
	default:
		return "", fmt.Errorf("unsupported product type: %s", productType)
	}

	watchPaths := map[string]string{
		"watchs12":    "/shop/buy-watch/apple-watch",
		"watchse3":    "/shop/buy-watch/apple-watch-se",
		"watchultra4": "/shop/buy-watch/apple-watch-ultra",
	}
	if watchBasePath, ok := watchPaths[productType]; ok {
		purchasePath = strings.TrimSpace(purchasePath)
		if purchasePath == "" {
			purchasePath = watchBasePath
		}
		if purchasePath != watchBasePath && !strings.HasPrefix(purchasePath, watchBasePath+"/") {
			return "", fmt.Errorf("invalid purchase path for product type %s", productType)
		}
		return baseURL + purchasePath, nil
	}

	partNumber = strings.Trim(strings.TrimSpace(partNumber), "/")
	if partNumber == "" {
		return "", fmt.Errorf("part number is empty")
	}

	return fmt.Sprintf("%s/shop/product/%s", baseURL, partNumber), nil
}

// pickupLocationQuery follows the website's postal-code search in mainland China.
// Some valid retail store IDs return HTTP 541 when sent as store=.
func pickupLocationQuery(areaCode, storeNumber, postalCode string) string {
	postalCode = strings.TrimSpace(postalCode)
	storeNumber = strings.TrimSpace(storeNumber)
	// Apple's catalog uses the city-wide 610000 for Chengdu, but its pickup
	// endpoint rejects it. 610021 returns both R502 and R580 (verified 2026-10-23).
	// Apply at request time so saved monitors and refreshed catalogs also benefit.
	if areaCode == "cn" && postalCode == "610000" && (storeNumber == "R502" || storeNumber == "R580") {
		postalCode = "610021"
	}
	validPostal := len(postalCode) == 6
	for _, c := range postalCode {
		if c < '0' || c > '9' {
			validPostal = false
		}
	}
	if (areaCode == "jp" && postalCode != "") || (areaCode == "cn" && validPostal) {
		return "location=" + url.QueryEscape(postalCode)
	}
	return "store=" + url.QueryEscape(storeNumber)
}
