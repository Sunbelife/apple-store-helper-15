package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"apple-store-helper/services"
)

var defaultRegions = []string{"cn", "hk", "jp", "sg", "us", "uk", "au"}

func main() {
	regions := defaultRegions
	if len(os.Args) > 1 {
		regions = os.Args[1:]
	}

	for _, region := range regions {
		data, err := services.FetchProductData(region)
		if err != nil {
			fatalf("fetch %s: %v", region, err)
		}

		encoded, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			fatalf("encode %s: %v", region, err)
		}
		encoded = append(encoded, '\n')

		for _, directory := range []string{"data/product", "embedded/data/product"} {
			path := filepath.Join(directory, fmt.Sprintf("product_data_%s.json", region))
			if err := os.WriteFile(path, encoded, 0644); err != nil {
				fatalf("write %s: %v", path, err)
			}
		}

		fmt.Printf("refreshed %s: %d series\n", region, len(data.Products))
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
