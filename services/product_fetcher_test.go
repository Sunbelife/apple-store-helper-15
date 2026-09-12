package services

import "testing"

func TestParseMetricsProductsSupportsCurrentAppleMarkup(t *testing.T) {
	html := `<html><script type="application/json" id="metrics">
{
  "data": {
    "products": [
      {"sku":"MJT74","partNumber":"MJT74CH/A","name":"iPhone 18 Pro 256GB Burgundy"}
    ]
  }
}
</script></html>`

	products, err := parseMetricsProducts(html, "iphone18pro")
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 {
		t.Fatalf("got %d products, want 1", len(products))
	}
	got := products[0]
	if got.Model != "iPhone 18 Pro" || got.Capacity != "256GB" || got.Color != "勃艮第酒红色" || got.Code != "MJT74CH/A" || got.Type != "iphone18pro" {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestParseMetricsProductsRequiresMetrics(t *testing.T) {
	if _, err := parseMetricsProducts("<html></html>", "iphone18pro"); err == nil {
		t.Fatal("expected missing metrics to fail")
	}
}

func TestParseMetricsProductsSplits18ProTypes(t *testing.T) {
	html := `<html><script type="application/json" id="metrics">
{
  "data": {
    "products": [
      {"sku":"MJT74","partNumber":"MJT74CH/A","name":"iPhone 18 Pro 256GB Black"},
      {"sku":"MJY64","partNumber":"MJY64CH/A","name":"iPhone 18 Pro Max 256GB Black"}
    ]
  }
}
</script></html>`

	products, err := parseMetricsProducts(html, "iphone18pro")
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 2 {
		t.Fatalf("got %d products, want 2", len(products))
	}
	if products[0].Type != "iphone18pro" || products[1].Type != "iphone18promax" {
		t.Fatalf("unexpected product types: %q, %q", products[0].Type, products[1].Type)
	}
}

func TestParseMetricsProductsSupportsDuo(t *testing.T) {
	html := `<html><script type="application/json" id="metrics">
{"data":{"products":[{"sku":"MK2M4","partNumber":"MK2M4CH/A","name":"iPhone Duo 256GB Star White"}]}}
</script></html>`

	products, err := parseMetricsProducts(html, "iphoneduo")
	if err != nil {
		t.Fatal(err)
	}
	got := products[0]
	if got.Model != "iPhone Duo" || got.Capacity != "256GB" || got.Color != "星光白色" || got.Type != "iphoneduo" {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestTranslateCurrentAppleColors(t *testing.T) {
	tests := map[string]string{
		"Burgundy":   "勃艮第酒红色",
		"Glacier":    "冰川蓝色",
		"Night Sky":  "夜空色",
		"Star White": "星光白色",
	}
	for input, want := range tests {
		if got := translateColor(input); got != want {
			t.Fatalf("%s: got %q, want %q", input, got, want)
		}
	}
}

func TestParseWatchProducts(t *testing.T) {
	html := `<script>
window.PRODUCT_SELECTION_BOOTSTRAP = {
  productSelectionData: {
    "watchProductSelectionDataNoJS":[{
      "url":"https://www.apple.com.cn/shop/buy-watch/apple-watch/42mm-gps-space-gray-aluminium-green-spark-nike-sport-loop"
    }],
    "products":[
    {"part":"MJF54CH/B","dimensions":{
      "watch_cases-dimensionCaseSize":"42mm",
      "watch_cases-dimensionCaseMaterial":"aluminum",
      "watch_cases-dimensionColor":"space_gray",
      "watch_cases-dimensionConnection":"gps"
    }}
  ]},
  anotherProperty: true
};
</script>`

	products, err := parseWatchProducts(html, "Apple Watch Series 12", "watchs12")
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 {
		t.Fatalf("got %d products, want 1", len(products))
	}
	got := products[0]
	if got.Model != "Apple Watch Series 12" || got.Capacity != "42mm" || got.Color != "铝金属 · 深空灰色 · GPS" || got.Code != "MJF54CH/B" || got.Type != "watchs12" || got.PurchasePath != "/shop/buy-watch/apple-watch/42mm-gps-space-gray-aluminium-green-spark-nike-sport-loop" {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestParseWatchProductsAddsUltraFixedAttributes(t *testing.T) {
	html := `productSelectionData: {"products":[
  {"part":"MJCW4CH/B","dimensions":{
    "watch_cases-dimensionCaseSize":"49mm",
    "watch_cases-dimensionColor":"natural"
  }}
]}`

	products, err := parseWatchProducts(html, "Apple Watch Ultra 4", "watchultra4")
	if err != nil {
		t.Fatal(err)
	}
	got := products[0]
	if got.Color != "钛金属 · 原色 · GPS + 蜂窝网络" || got.Type != "watchultra4" {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestParseWatchProductsRequiresSelectionData(t *testing.T) {
	if _, err := parseWatchProducts("<html></html>", "Apple Watch SE 3", "watchse3"); err == nil {
		t.Fatal("expected missing product selection data to fail")
	}
}

func TestParseWatchProductsMatchesLocalizedCaseBeforeBandColor(t *testing.T) {
	html := `productSelectionData: {
  "displayValues": {
    "watch_cases-dimensionCaseSize": {"40mm":{"header":"40mm"}},
    "watch_cases-dimensionCaseMaterial": {"aluminum":{}},
    "watch_cases-dimensionColor": {
      "midnight":{"text":"ミッドナイト"},
      "starlight":{"text":"スターライト"}
    }
  },
  "watchProductSelectionDataNoJS": [
    {
      "url":"https://www.apple.com/jp/shop/buy-watch/apple-watch-se/40mm-gpsmodel-starlight-aluminium-midnight-band-se",
      "text":"Apple Watch SE 3 (GPSモデル) - 40mmスターライトアルミニウムケースとミッドナイトバンド"
    },
    {
      "url":"https://www.apple.com/jp/shop/buy-watch/apple-watch-se/40mm-gpsmodel-midnight-aluminium-starlight-band-se",
      "text":"Apple Watch SE 3 (GPSモデル) - 40mmミッドナイトアルミニウムケースとスターライトバンド"
    }
  ],
  "products": [{
    "part":"MEHX4J/A",
    "dimensions": {
      "watch_cases-dimensionCaseSize":"40mm",
      "watch_cases-dimensionCaseMaterial":"aluminum",
      "watch_cases-dimensionColor":"midnight",
      "watch_cases-dimensionConnection":"gps"
    }
  }]
}`

	products, err := parseWatchProducts(html, "Apple Watch SE 3", "watchse3")
	if err != nil {
		t.Fatal(err)
	}
	want := "/shop/buy-watch/apple-watch-se/40mm-gpsmodel-midnight-aluminium-starlight-band-se"
	if got := products[0].PurchasePath; got != want {
		t.Fatalf("got purchase path %q, want %q", got, want)
	}
}
