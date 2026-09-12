package services

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/parnurzeal/gorequest"

	"apple-store-helper/embedded"
	"apple-store-helper/model"
)

// ProductData 存储从Apple官网获取的产品数据
type ProductData struct {
	UpdateTime string                         `json:"update_time"`
	AreaCode   string                         `json:"area_code"`
	Products   map[string][]model.ProductInfo `json:"products"`
}

// FetchProductData 从Apple官网获取产品数据
func FetchProductData(areaCode string) (*ProductData, error) {
	productData := &ProductData{
		UpdateTime: time.Now().Format("2006-01-02 15:04:05"),
		AreaCode:   areaCode,
		Products:   make(map[string][]model.ProductInfo),
	}

	baseURL, err := appleStoreBaseURL(areaCode)
	if err != nil {
		return nil, err
	}

	// 构建所有产品系列的URL
	// 只获取当前存在的产品系列
	series := []struct {
		name      string
		url       string
		modelType string
	}{
		{"iPhone Duo", fmt.Sprintf("%s/shop/buy-iphone/iphone-duo", baseURL), "iphoneduo"},
		{"iPhone 18 Pro", fmt.Sprintf("%s/shop/buy-iphone/iphone-18-pro", baseURL), "iphone18pro"},
		{"iPhone 16", fmt.Sprintf("%s/shop/buy-iphone/iphone-16", baseURL), "iphone16"},
	}

	for _, s := range series {
		products, err := fetchSeriesProducts(s.url, s.modelType)
		if err != nil {
			log.Printf("Failed to fetch %s: %v", s.name, err)
			continue
		}
		if len(products) > 0 {
			productData.Products[s.name] = products
		}
	}

	watchSeries := []struct {
		name      string
		url       string
		modelType string
	}{
		{"Apple Watch Series 12", fmt.Sprintf("%s/shop/buy-watch/apple-watch", baseURL), "watchs12"},
		{"Apple Watch SE 3", fmt.Sprintf("%s/shop/buy-watch/apple-watch-se", baseURL), "watchse3"},
		{"Apple Watch Ultra 4", fmt.Sprintf("%s/shop/buy-watch/apple-watch-ultra", baseURL), "watchultra4"},
	}

	for _, s := range watchSeries {
		products, err := fetchWatchProducts(s.url, s.name, s.modelType)
		if err != nil {
			log.Printf("Failed to fetch %s: %v", s.name, err)
			continue
		}
		if len(products) > 0 {
			productData.Products[s.name] = products
		}
	}
	if len(productData.Products) == 0 {
		return nil, fmt.Errorf("no current products found for area %s", areaCode)
	}

	return productData, nil
}

// fetchSeriesProducts 获取特定系列的产品
func fetchSeriesProducts(url string, modelType string) ([]model.ProductInfo, error) {
	body, err := fetchProductPage(url)
	if err != nil {
		return nil, err
	}
	return parseMetricsProducts(body, modelType)
}

func fetchWatchProducts(url string, modelName string, modelType string) ([]model.ProductInfo, error) {
	body, err := fetchProductPage(url)
	if err != nil {
		return nil, err
	}
	return parseWatchProducts(body, modelName, modelType)
}

func fetchProductPage(url string) (string, error) {
	resp, body, errs := gorequest.New().
		Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36").
		Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		Set("Accept-Language", "zh-CN,zh;q=0.9").
		Timeout(time.Second * 10).
		Get(url).
		End()

	if len(errs) > 0 {
		return "", fmt.Errorf("request failed: %v", errs[0])
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code: %d", resp.StatusCode)
	}

	return body, nil
}

func parseMetricsProducts(body string, modelType string) ([]model.ProductInfo, error) {
	// Metrics is JSON embedded in the current Apple Store product page.
	re := regexp.MustCompile(`(?is)<script[^>]*id=["']metrics["'][^>]*>(.*?)</script>`)
	matches := re.FindStringSubmatch(body)
	if len(matches) < 2 {
		return nil, fmt.Errorf("metrics data not found in HTML")
	}

	var metricsData struct {
		Data struct {
			Products []struct {
				PartNumber string `json:"partNumber"`
				Name       string `json:"name"`
				SKU        string `json:"sku"`
			} `json:"products"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(matches[1]), &metricsData); err != nil {
		return nil, fmt.Errorf("failed to parse metrics data: %v", err)
	}

	products := make([]model.ProductInfo, 0, len(metricsData.Data.Products))
	for _, product := range metricsData.Data.Products {
		log.Printf("Found product: SKU=%s, PartNumber=%s, Name=%s", product.SKU, product.PartNumber, product.Name)
		info := parseProductInfo(product.Name, product.PartNumber, modelType)
		if info.Code != "" && info.Model != "" && info.Capacity != "" && info.Color != "" {
			products = append(products, info)
		}
	}
	if len(products) == 0 {
		return nil, fmt.Errorf("no products found in metrics data")
	}

	return products, nil
}

func parseWatchProducts(body string, modelName string, modelType string) ([]model.ProductInfo, error) {
	const marker = "productSelectionData:"
	markerIndex := strings.Index(body, marker)
	if markerIndex < 0 {
		return nil, fmt.Errorf("watch product selection data not found in HTML")
	}

	var selectionData struct {
		DisplayValues                 map[string]map[string]json.RawMessage `json:"displayValues"`
		WatchProductSelectionDataNoJS []struct {
			URL  string `json:"url"`
			Text string `json:"text"`
		} `json:"watchProductSelectionDataNoJS"`
		Products []struct {
			Part       string            `json:"part"`
			Dimensions map[string]string `json:"dimensions"`
		} `json:"products"`
	}
	decoder := json.NewDecoder(strings.NewReader(body[markerIndex+len(marker):]))
	if err := decoder.Decode(&selectionData); err != nil {
		return nil, fmt.Errorf("failed to parse watch product selection data: %v", err)
	}

	products := make([]model.ProductInfo, 0, len(selectionData.Products))
	for _, product := range selectionData.Products {
		size := product.Dimensions["watch_cases-dimensionCaseSize"]
		material := product.Dimensions["watch_cases-dimensionCaseMaterial"]
		color := product.Dimensions["watch_cases-dimensionColor"]
		connection := product.Dimensions["watch_cases-dimensionConnection"]

		// Ultra pages omit fixed attributes from the dimensions object.
		if modelType == "watchultra4" {
			material = "titanium"
			connection = "gpscell"
		}

		if product.Part == "" || size == "" || material == "" || color == "" || connection == "" {
			continue
		}

		products = append(products, model.ProductInfo{
			Model:    modelName,
			Capacity: size,
			Color: strings.Join([]string{
				translateWatchMaterial(material),
				translateWatchColor(color),
				translateWatchConnection(connection),
			}, " · "),
			Code: product.Part,
			Type: modelType,
			PurchasePath: watchPurchasePath(
				selectionData.WatchProductSelectionDataNoJS,
				selectionData.DisplayValues,
				modelType,
				size,
				material,
				color,
				connection,
			),
		})
	}
	if len(products) == 0 {
		return nil, fmt.Errorf("no products found in watch product selection data")
	}

	return products, nil
}

func watchPurchasePath(
	links []struct {
		URL  string `json:"url"`
		Text string `json:"text"`
	},
	displayValues map[string]map[string]json.RawMessage,
	modelType string,
	size string,
	material string,
	color string,
	connection string,
) string {
	basePaths := map[string]string{
		"watchs12":    "/shop/buy-watch/apple-watch",
		"watchse3":    "/shop/buy-watch/apple-watch-se",
		"watchultra4": "/shop/buy-watch/apple-watch-ultra",
	}
	basePath := basePaths[modelType]
	if basePath == "" {
		return ""
	}

	connectionSlug := connection
	if connection == "gpscell" {
		connectionSlug = "cellular"
	}
	colorSlugValues := map[string][]string{
		"darkbronze":  {"dark-bronze"},
		"lightgold":   {"light-gold"},
		"nightblue":   {"night-blue"},
		"pearlwhite":  {"pearl-white"},
		"radiantgold": {"radiant-gold"},
		"space_gray":  {"space-gray", "space-grey"},
	}
	colorSlugs := colorSlugValues[color]
	if len(colorSlugs) == 0 {
		colorSlugs = []string{strings.ReplaceAll(color, "_", "-")}
	}
	sizeSlugs := []string{size, strings.Replace(size, "mm", "-mm", 1)}
	materialSlugs := []string{material}
	if material == "aluminum" {
		materialSlugs = append(materialSlugs, "aluminium")
	}

	for _, link := range links {
		path := localizedWatchPath(link.URL, basePath)
		for _, sizeSlug := range sizeSlugs {
			for _, colorSlug := range colorSlugs {
				for _, materialSlug := range materialSlugs {
					prefix := fmt.Sprintf("%s/%s-%s-%s-%s-", basePath, sizeSlug, connectionSlug, colorSlug, materialSlug)
					if strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix)) {
						return path
					}
				}
			}
		}
	}

	sizeLabel := watchDisplayLabel(displayValues, "watch_cases-dimensionCaseSize", size)
	if sizeLabel == "" {
		sizeLabel = size
	}
	for _, link := range links {
		linkText := cleanHTMLText(link.Text)
		if !strings.Contains(normalizeWatchComparison(linkText), normalizeWatchComparison(sizeLabel)) {
			continue
		}
		if firstWatchDimensionValue(linkText, displayValues, "watch_cases-dimensionColor") != color {
			continue
		}
		if detectedMaterial := firstWatchDimensionValue(linkText, displayValues, "watch_cases-dimensionCaseMaterial"); detectedMaterial != "" && detectedMaterial != material {
			continue
		}
		isCellular := strings.Contains(linkText, "+")
		if (connection == "gpscell") != isCellular {
			continue
		}
		if path := localizedWatchPath(link.URL, basePath); path != "" {
			return path
		}
	}

	return basePath
}

func localizedWatchPath(rawURL string, basePath string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	baseIndex := strings.Index(strings.ToLower(path), strings.ToLower(basePath))
	if baseIndex < 0 {
		return ""
	}
	return path[baseIndex:]
}

func watchDisplayLabel(displayValues map[string]map[string]json.RawMessage, dimension string, value string) string {
	raw := displayValues[dimension][value]
	var display struct {
		Header string `json:"header"`
		Text   string `json:"text"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &display) != nil {
		return ""
	}
	if display.Text != "" {
		return cleanHTMLText(display.Text)
	}
	if dimension == "watch_cases-dimensionCaseMaterial" {
		divPattern := regexp.MustCompile(`(?is)<div[^>]*>(.*?)</div>`)
		if match := divPattern.FindStringSubmatch(display.Header); len(match) > 1 {
			return cleanHTMLText(match[1])
		}
	}
	return cleanHTMLText(display.Header)
}

func firstWatchDimensionValue(text string, displayValues map[string]map[string]json.RawMessage, dimension string) string {
	lowerText := strings.ToLower(text)
	firstValue := ""
	firstIndex := len(lowerText) + 1
	for value := range displayValues[dimension] {
		if value == "variantOrder" {
			continue
		}
		label := watchDisplayLabel(displayValues, dimension, value)
		if label == "" {
			continue
		}
		if index := strings.Index(lowerText, strings.ToLower(label)); index >= 0 && index < firstIndex {
			firstIndex = index
			firstValue = value
		}
	}
	return firstValue
}

func cleanHTMLText(value string) string {
	tagPattern := regexp.MustCompile(`(?s)<[^>]*>`)
	return normalizeSpaces(tagPattern.ReplaceAllString(html.UnescapeString(value), " "))
}

func normalizeWatchComparison(value string) string {
	return strings.Map(func(character rune) rune {
		if character == '-' || character == '‑' || character == '–' || character == '—' || character == ' ' {
			return -1
		}
		return character
	}, strings.ToLower(value))
}

func translateWatchMaterial(material string) string {
	values := map[string]string{
		"aluminum": "铝金属",
		"titanium": "钛金属",
		"ceramic":  "陶瓷",
	}
	if translated, ok := values[material]; ok {
		return translated
	}
	return material
}

func translateWatchColor(color string) string {
	values := map[string]string{
		"black":       "黑色",
		"darkbronze":  "深古铜色",
		"lightgold":   "浅金色",
		"midnight":    "午夜色",
		"natural":     "原色",
		"nightblue":   "夜蓝色",
		"pearlwhite":  "珍珠白色",
		"radiantgold": "炫金色",
		"space_gray":  "深空灰色",
		"starlight":   "星光色",
	}
	if translated, ok := values[color]; ok {
		return translated
	}
	return color
}

func translateWatchConnection(connection string) string {
	values := map[string]string{
		"gps":     "GPS",
		"gpscell": "GPS + 蜂窝网络",
	}
	if translated, ok := values[connection]; ok {
		return translated
	}
	return connection
}

// normalizeSpaces 规范化字符串中的各种空格字符
func normalizeSpaces(s string) string {
	// 替换各种Unicode空格字符为普通空格
	// \u00A0 = 不间断空格 (NBSP)
	// \u2002 = En空格
	// \u2003 = Em空格
	// \u3000 = 全角空格
	s = strings.ReplaceAll(s, "\u00A0", " ")
	s = strings.ReplaceAll(s, "\u2002", " ")
	s = strings.ReplaceAll(s, "\u2003", " ")
	s = strings.ReplaceAll(s, "\u3000", " ")

	// 将多个连续空格替换为单个空格
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")

	// 去除首尾空格
	return strings.TrimSpace(s)
}

// parseProductInfo 解析产品信息
func parseProductInfo(name string, partNumber string, modelType string) model.ProductInfo {
	info := model.ProductInfo{
		Code: partNumber,
		Type: modelType,
	}

	// 如果name为空，直接返回
	if name == "" {
		return info
	}

	// 规范化名称中的空格
	name = normalizeSpaces(name)

	// 解析产品名称
	parts := strings.Split(name, " ")
	if len(parts) >= 3 {
		// 格式: "iPhone 16 Plus 128GB Black" 或 "iPhone 16 128GB Black"
		modelParts := []string{}
		capacityIdx := -1

		for i, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasSuffix(part, "GB") || strings.HasSuffix(part, "TB") {
				capacityIdx = i
				break
			}
			if part != "" {
				modelParts = append(modelParts, part)
			}
		}

		if capacityIdx > 0 && len(modelParts) > 0 {
			info.Model = strings.Join(modelParts, " ")
			info.Type = productTypeForModel(info.Model, modelType)
			info.Capacity = strings.TrimSpace(parts[capacityIdx])
			if capacityIdx+1 < len(parts) {
				var colorParts []string
				for j := capacityIdx + 1; j < len(parts); j++ {
					part := strings.TrimSpace(parts[j])
					if part != "" {
						colorParts = append(colorParts, part)
					}
				}
				if len(colorParts) > 0 {
					info.Color = translateColor(strings.Join(colorParts, " "))
				}
			}
		}
	}

	return info
}

func productTypeForModel(model string, fallback string) string {
	types := map[string]string{
		"iPhone Duo":        "iphoneduo",
		"iPhone 18 Pro":     "iphone18pro",
		"iPhone 18 Pro Max": "iphone18promax",
		"iPhone 16 Pro":     "iphone16pro",
		"iPhone 16 Pro Max": "iphone16promax",
		"iPhone 16 Plus":    "iphone16plus",
	}
	if productType, ok := types[model]; ok {
		return productType
	}
	return fallback
}

// translateColor 翻译颜色名称到官方中文颜色
func translateColor(color string) string {
	// 如果颜色为空，直接返回
	if color == "" {
		return color
	}

	// Apple 官方颜色翻译映射表
	colorMap := map[string]string{
		// iPhone 基础颜色
		"Black":       "黑色",
		"White":       "白色",
		"Pink":        "粉色",
		"Teal":        "深青色",
		"Ultramarine": "群青色",

		// iPhone Pro 系列颜色
		"Black Titanium":   "黑色钛金属",
		"White Titanium":   "白色钛金属",
		"Natural Titanium": "原色钛金属",
		"Desert Titanium":  "沙漠色钛金属",

		// iPhone 18 系列颜色
		"Burgundy":   "勃艮第酒红色",
		"Glacier":    "冰川蓝色",
		"Night Sky":  "夜空色",
		"Star White": "星光白色",

		// Apple Watch 系列颜色
		"Space Black":   "深空黑色",
		"Gold":          "金色",
		"Rose Gold":     "玫瑰金色",
		"Midnight":      "午夜色",
		"Starlight":     "星光色",
		"Blue":          "蓝色",
		"Red":           "红色",
		"Green":         "绿色",
		"Yellow":        "黄色",
		"Orange":        "橙色",
		"Purple":        "紫色",
		"Product Red":   "红色",
		"Forest Green":  "森林绿色",
		"Ocean Blue":    "海洋蓝色",
		"Sunset Orange": "日落橙色",

		// 其他常见颜色
		"Graphite":          "石墨色",
		"Titanium":          "钛金属色",
		"Aluminum":          "铝金属色",
		"Stainless Steel":   "不锈钢色",
		"Ceramic":           "陶瓷色",
		"Leather":           "皮革色",
		"Fabric":            "织物色",
		"Sport Band":        "运动表带",
		"Sport Loop":        "运动表环",
		"Braided Solo Loop": "编织单圈表带",
		"Solo Loop":         "单圈表带",
		"Link Bracelet":     "链式表带",
		"Milanese Loop":     "米兰尼斯表带",
		"Leather Link":      "皮革链式表带",
		"Leather Loop":      "皮革表环",
		"Modern Buckle":     "现代扣式表带",
		"Classic Buckle":    "经典扣式表带",
	}

	if cn, ok := colorMap[color]; ok {
		return cn
	}

	// 如果没有找到翻译，尝试部分匹配
	for english, chinese := range colorMap {
		if strings.Contains(strings.ToLower(color), strings.ToLower(english)) {
			return chinese
		}
	}

	// 如果都没有找到，返回原始颜色
	return color
}

// SaveProductData 保存产品数据到本地文件
func SaveProductData(data *ProductData) error {
	// 获取可执行文件所在目录
	execDir, err := os.Executable()
	if err != nil {
		// 如果获取可执行文件路径失败，使用当前工作目录
		execDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %v", err)
		}
	} else {
		execDir = filepath.Dir(execDir)
	}

	// 在程序目录下创建data/product子目录
	dataDir := filepath.Join(execDir, "data", "product")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	// 使用地区代码作为文件名的一部分
	fileName := fmt.Sprintf("product_data_%s.json", data.AreaCode)
	filePath := filepath.Join(dataDir, fileName)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filePath, jsonData, 0644)
}

// LoadProductData 优先加载运行时更新的数据，再回退到内置数据。
func LoadProductData(areaCode string) (*ProductData, error) {
	fileName := fmt.Sprintf("product_data_%s.json", areaCode)
	var candidates []string
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "data", "product", fileName))
	}
	if workDir, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(workDir, "data", "product", fileName))
	}

	for _, filePath := range candidates {
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		var productData ProductData
		if err := json.Unmarshal(data, &productData); err != nil {
			return nil, fmt.Errorf("failed to parse product data %s: %v", filePath, err)
		}
		log.Printf("Loaded product data for %s from %s", areaCode, filePath)
		return &productData, nil
	}

	if data, exists := embedded.GetProductData(areaCode); exists {
		var productData ProductData
		if err := json.Unmarshal(data, &productData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal embedded product data for %s: %v", areaCode, err)
		}
		log.Printf("Loaded embedded product data for %s", areaCode)
		return &productData, nil
	}

	return nil, fmt.Errorf("product data not found for area %s", areaCode)
}

// UpdateProductDatabase 更新产品数据库
func UpdateProductDatabase(areaCode string) error {
	log.Println("Fetching latest product data from Apple...")

	data, err := FetchProductData(areaCode)
	if err != nil {
		return fmt.Errorf("failed to fetch product data: %v", err)
	}

	if err := SaveProductData(data); err != nil {
		return fmt.Errorf("failed to save product data: %v", err)
	}

	log.Printf("Product data updated successfully. Total series: %d", len(data.Products))

	// 更新Product服务的产品列表
	Product.UpdateFromDynamicData(data)

	return nil
}
