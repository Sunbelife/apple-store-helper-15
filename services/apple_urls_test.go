package services

import "testing"

func TestShoppingBagURL(t *testing.T) {
	for area, want := range map[string]string{
		"cn": "https://www.apple.com.cn/shop/bag",
		"us": "https://www.apple.com/shop/bag",
		"hk": "https://www.apple.com/hk/shop/bag",
		"jp": "https://www.apple.com/jp/shop/bag",
		"sg": "https://www.apple.com/sg/shop/bag",
		"uk": "https://www.apple.com/uk/shop/bag",
		"au": "https://www.apple.com/au/shop/bag",
	} {
		got, err := ShoppingBagURL(area)
		if err != nil || got != want {
			t.Fatalf("%s: got %s, %v; want %s", area, got, err, want)
		}
	}
	if _, err := ShoppingBagURL(""); err == nil {
		t.Fatal("unknown region must not open another region's bag")
	}
}

func TestProductPurchaseURL(t *testing.T) {
	tests := []struct {
		name         string
		areaCode     string
		productType  string
		partNumber   string
		purchasePath string
		want         string
	}{
		{
			name:        "China iPhone Duo",
			areaCode:    "cn",
			productType: "iphoneduo",
			partNumber:  "MK2M4CH/A",
			want:        "https://www.apple.com.cn/shop/product/MK2M4CH/A",
		},
		{
			name:        "China iPhone 18 Pro Max",
			areaCode:    "cn",
			productType: "iphone18promax",
			partNumber:  "MJY64CH/A",
			want:        "https://www.apple.com.cn/shop/product/MJY64CH/A",
		},
		{
			name:         "China Apple Watch Series 12",
			areaCode:     "cn",
			productType:  "watchs12",
			partNumber:   "MJF54CH/B",
			purchasePath: "/shop/buy-watch/apple-watch/42mm-gps-space-gray-aluminium-green-spark-nike-sport-loop",
			want:         "https://www.apple.com.cn/shop/buy-watch/apple-watch/42mm-gps-space-gray-aluminium-green-spark-nike-sport-loop",
		},
		{
			name:        "US Apple Watch Ultra 4",
			areaCode:    "us",
			productType: "watchultra4",
			partNumber:  "TESTLL/A",
			want:        "https://www.apple.com/shop/buy-watch/apple-watch-ultra",
		},
		{
			name:        "Japan Apple Watch SE 3",
			areaCode:    "jp",
			productType: "watchse3",
			partNumber:  "TESTJ/A",
			want:        "https://www.apple.com/jp/shop/buy-watch/apple-watch-se",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProductPurchaseURL(tt.areaCode, tt.productType, tt.partNumber, tt.purchasePath)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProductPurchaseURLRejectsUnknownProduct(t *testing.T) {
	if _, err := ProductPurchaseURL("cn", "unknown", "TESTCH/A", ""); err == nil {
		t.Fatal("expected unknown product type to fail")
	}
}

func TestProductPurchaseURLRejectsMismatchedWatchPath(t *testing.T) {
	if _, err := ProductPurchaseURL("cn", "watchs12", "MJF54CH/B", "/shop/buy-watch/apple-watch-se"); err == nil {
		t.Fatal("expected mismatched watch purchase path to fail")
	}
}

func TestPickupMessagesURL(t *testing.T) {
	tests := map[string]string{
		"cn": "https://www.apple.com.cn/shop/retail/pickup-message?fae=true",
		"hk": "https://www.apple.com/hk/shop/retail/pickup-message?fae=true",
		"jp": "https://www.apple.com/jp/shop/retail/pickup-message?fae=true",
		"sg": "https://www.apple.com/sg/shop/retail/pickup-message?fae=true",
		"us": "https://www.apple.com/shop/retail/pickup-message?fae=true",
		"uk": "https://www.apple.com/uk/shop/retail/pickup-message?fae=true",
		"au": "https://www.apple.com/au/shop/retail/pickup-message?fae=true",
	}
	for region, want := range tests {
		got, err := pickupMessagesURL(region, "fae=true")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s: got %q, want %q", region, got, want)
		}
	}
}

func TestPickupLocationQuery(t *testing.T) {
	for _, tt := range []struct{ area, store, postal, want string }{
		{"cn", "R639", "510623", "location=510623"},
		{"cn", "R761", "518001", "location=518001"},
		{"cn", "R793", "518101", "location=518101"},
		{"cn", "R484", "深圳", "store=R484"},
		{"cn", "R484", "", "store=R484"},
		{"jp", "R079", "104-0061", "location=104-0061"},
		{"us", "R001", "95014", "store=R001"},
	} {
		if got := pickupLocationQuery(tt.area, tt.store, tt.postal); got != tt.want {
			t.Errorf("%+v got %s", tt, got)
		}
	}
}
