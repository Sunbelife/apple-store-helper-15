package services

import (
	"apple-store-helper/model"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var additionalProductSeries = []struct{ Name, Path, Type string }{
	{"MacBook Air", "/shop/buy-mac/macbook-air", "macbookair"},
	{"MacBook Pro", "/shop/buy-mac/macbook-pro", "macbookpro"},
	{"Mac mini", "/shop/buy-mac/mac-mini", "macmini"},
	{"iMac", "/shop/buy-mac/imac", "imac"},
	{"Mac Studio", "/shop/buy-mac/mac-studio", "macstudio"},
}

// Only concrete retail part numbers are monitorable. CTO containers are not SKUs.
func standardRetailParts(body string) ([]string, error) {
	marker := "productSelectionData:"
	i := strings.Index(body, marker)
	if i < 0 {
		return nil, fmt.Errorf("product selection data not found")
	}
	var data struct {
		Products []struct {
			PartNumber string `json:"partNumber"`
			RetailPart string `json:"btrOrFdPartNumber"`
		} `json:"products"`
	}
	if err := json.NewDecoder(strings.NewReader(body[i+len(marker):])).Decode(&data); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var parts []string
	for _, p := range data.Products {
		part := p.RetailPart
		if part == "" {
			part = p.PartNumber
		}
		if strings.Contains(part, "/") && !seen[part] {
			seen[part] = true
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("no standard retail configurations available")
	}
	sort.Strings(parts)
	return parts, nil
}

var schemaProductScript = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
var macMemory = regexp.MustCompile(`(?:^|-)(\d+(?:gb|tb))-memory(?:-|$)`)
var macStorage = regexp.MustCompile(`(?:^|-)(\d+(?:gb|tb))-storage(?:-|$)`)
var macChip = regexp.MustCompile(`(?:^|-)(m\d+(?:-?pro|-?max|-?ultra)?)(?:-|$)`)
var macScreen = regexp.MustCompile(`(?:^|/)(\d+(?:-\d+)?)(?:-|インチ|인치)`)
var macCores = regexp.MustCompile(`(?:^|-)(\d+)(?:-[^-\d/]+-|[^-\d/]*)(cpu|gpu)`)

func parseComputerProduct(body, part, family, productType string) (model.ProductInfo, error) {
	productURL := ""
	metricName := ""
	verified := false
	for _, match := range schemaProductScript.FindAllStringSubmatch(body, -1) {
		var p struct {
			Type   string `json:"@type"`
			URL    string `json:"url"`
			Offers []struct {
				SKU string `json:"sku"`
			} `json:"offers"`
		}
		if json.Unmarshal([]byte(match[1]), &p) != nil || p.Type != "Product" {
			continue
		}
		for _, offer := range p.Offers {
			if offer.SKU == part {
				verified = true
				productURL = p.URL
			}
		}
	}
	// Mainland pages use Baidu JSON-LD instead of schema.org Product.
	// Match the exact regional SKU in metrics before trusting the canonical URL.
	metrics := regexp.MustCompile(`(?is)<script[^>]*id=["']metrics["'][^>]*>(.*?)</script>`).FindStringSubmatch(body)
	if len(metrics) > 1 {
		var m struct {
			Data struct {
				Products []struct {
					Part string `json:"partNumber"`
					Name string `json:"name"`
				} `json:"products"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(metrics[1]), &m) == nil {
			for _, p := range m.Data.Products {
				if p.Part == part {
					metricName = p.Name
					verified = true
				}
			}
		}
	}
	if productURL == "" {
		canonical := regexp.MustCompile(`(?is)<link[^>]*rel=["']canonical["'][^>]*href=["']([^"']+)["']`).FindStringSubmatch(body)
		if len(canonical) > 1 {
			productURL = canonical[1]
		}
	}
	if productType == "appletv4k" {
		parts, _ := standardRetailParts(body)
		for _, candidate := range parts {
			if candidate == part {
				verified = true
			}
		}
	}
	if verified {
		info := model.ProductInfo{Model: family, Code: part, Type: productType, Color: "标准款", Capacity: part}
		if productType == "appletv4k" {
			return info, nil
		}
		u, err := url.Parse(productURL)
		if err != nil {
			return info, err
		}
		slug, _ := url.PathUnescape(strings.ToLower(u.Path))
		slug = strings.NewReplacer("‑", "-", "–", "-", "−", "-", "—", "-").Replace(slug)
		mem, storage := macMemory.FindStringSubmatch(slug), macStorage.FindStringSubmatch(slug)
		if len(mem) < 2 || len(storage) < 2 {
			// Localized canonical slugs retain the numeric memory/storage units.
			// Require both fields, in Apple's RAM-then-SSD configuration order.
			units := regexp.MustCompile(`(\d+)-?(gb|tb|go|to)`).FindAllStringSubmatch(slug, -1)
			if len(units) == 2 {
				unit := func(u []string) []string {
					suffix := u[2]
					if suffix == "go" {
						suffix = "gb"
					}
					if suffix == "to" {
						suffix = "tb"
					}
					return []string{u[0], u[1] + suffix}
				}
				mem = unit(units[0])
				storage = unit(units[1])
			}
		}
		if len(mem) < 2 || len(storage) < 2 {
			return info, fmt.Errorf("missing memory/storage in official configuration URL for %s", part)
		}
		info.Capacity = strings.ToUpper(mem[1]) + " RAM / " + strings.ToUpper(storage[1]) + " SSD"
		if chip := macChip.FindStringSubmatch(slug); len(chip) > 1 {
			info.Model += " " + strings.ToUpper(strings.ReplaceAll(chip[1], "-", " "))
		}
		if screen := macScreen.FindStringSubmatch(slug); len(screen) > 1 {
			info.Model += " " + strings.ReplaceAll(screen[1], "-", ".") + "″"
		}
		for _, c := range macCores.FindAllStringSubmatch(slug, -1) {
			info.Capacity += " / " + c[1] + " " + strings.ToUpper(c[2])
		}
		for _, color := range []string{"space-black", "space-gray", "sky-blue", "midnight", "starlight", "silver", "blue", "green", "pink", "purple", "orange", "yellow"} {
			if strings.Contains(slug, "-"+color+"-") {
				info.Color = translateColor(strings.Title(strings.ReplaceAll(color, "-", " ")))
				break
			}
		}
		for _, c := range regexp.MustCompile(`(?:^|-)(cpu|gpu)-(\d+)`).FindAllStringSubmatch(slug, -1) {
			info.Capacity += " / " + c[2] + " " + strings.ToUpper(c[1])
		}
		if index := strings.LastIndex(metricName, " - "); index >= 0 {
			info.Color = translateColor(strings.TrimSpace(metricName[index+3:]))
		}
		return info, nil
	}
	return model.ProductInfo{}, fmt.Errorf("official product detail does not match SKU %s", part)
}

func fetchComputerProducts(base, path, family, productType string, cache ...map[string]model.ProductInfo) ([]model.ProductInfo, error) {
	body, err := fetchProductPage(base + path)
	if err != nil {
		return nil, err
	}
	parts, err := standardRetailParts(body)
	if err != nil {
		return nil, err
	}
	var selection struct {
		Products []struct {
			Part       string            `json:"btrOrFdPartNumber"`
			Container  string            `json:"aosContainerPartNumber"`
			Dimensions map[string]string `json:"dimensions"`
		} `json:"products"`
	}
	marker := "productSelectionData:"
	if i := strings.Index(body, marker); i >= 0 {
		_ = json.NewDecoder(strings.NewReader(body[i+len(marker):])).Decode(&selection)
	}
	chips := map[string]string{}
	for _, p := range selection.Products {
		chip := p.Dimensions["processor-dimensionChip"]
		if chip == "" {
			match := regexp.MustCompile(`(?i)_(m\d+(?:pro|max|ultra)?)_`).FindStringSubmatch(p.Container)
			if len(match) > 1 {
				chip = match[1]
			}
		}
		if chip != "" {
			chip = strings.NewReplacer("max", " Max", "pro", " Pro", "ultra", " Ultra").Replace(strings.ToLower(chip))
			chips[p.Part] = strings.ToUpper(chip)
		}
	}
	var products []model.ProductInfo
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	for _, part := range parts {
		if len(cache) > 0 {
			if p, ok := cache[0][part]; ok && p.Type == productType && (productType == "appletv4k" || strings.Contains(p.Capacity, " RAM / ")) {
				if p.Color == "Standard" {
					p.Color = "标准款"
				}
				if chip := chips[part]; chip != "" && !regexp.MustCompile(`M\d`).MatchString(p.Model) {
					p.Model += " " + chip
				}
				mu.Lock()
				products = append(products, p)
				mu.Unlock()
				continue
			}
		}
		wg.Add(1)
		go func(part string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detail := body
			if productType != "appletv4k" {
				var e error
				detail, e = fetchProductPage(base + "/shop/product/" + part)
				if e != nil {
					return
				}
			}
			p, e := parseComputerProduct(detail, part, family, productType)
			if e != nil {
				log.Printf("Computer detail %s: %v", part, e)
				return
			}
			if chip := chips[part]; chip != "" && !regexp.MustCompile(`M\d`).MatchString(p.Model) {
				p.Model += " " + chip
			}
			mu.Lock()
			products = append(products, p)
			mu.Unlock()
		}(part)
	}
	wg.Wait()
	sort.Slice(products, func(i, j int) bool { return products[i].Code < products[j].Code })
	if len(products) == 0 {
		return nil, fmt.Errorf("no verified retail configurations for %s", family)
	}
	return products, nil
}
