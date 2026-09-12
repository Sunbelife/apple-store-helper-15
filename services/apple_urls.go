package services

import (
	"fmt"
	"strings"
)

func appleStoreBaseURL(areaCode string) (string, error) {
	switch areaCode {
	case "cn":
		return "https://www.apple.com.cn", nil
	case "us":
		return "https://www.apple.com", nil
	case "hk", "jp", "sg", "uk", "au":
		return "https://www.apple.com/" + areaCode, nil
	default:
		return "", fmt.Errorf("unsupported area code: %s", areaCode)
	}
}

func fulfillmentMessagesURL(areaCode string, query string) (string, error) {
	baseURL, err := appleStoreBaseURL(areaCode)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/shop/fulfillment-messages?%s", baseURL, query), nil
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
		"watchs12", "watchse3", "watchultra4":
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
