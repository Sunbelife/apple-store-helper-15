package embedded

import (
	"embed"
	"strings"
)

//go:embed data/product/*.json data/store/*.json
var additionalRegions embed.FS

func init() {
	for _, kind := range []string{"product", "store"} {
		entries, _ := additionalRegions.ReadDir("data/" + kind)
		for _, entry := range entries {
			name := entry.Name()
			prefix := "store_"
			target := StoreDataMap
			if kind == "product" {
				prefix = "product_data_"
				target = ProductDataMap
			}
			code := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".json")
			target[code], _ = additionalRegions.ReadFile("data/" + kind + "/" + name)
		}
	}
}
