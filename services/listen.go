package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/golang-module/carbon"
	"github.com/tidwall/gjson"

	"apple-store-helper/model"
	"apple-store-helper/theme"
	"apple-store-helper/view"
)

const (
	StatusOutStock = "无货"
	StatusInStock  = "有货"
	StatusWait     = "等待"
	StatusQuerying = "查询中"
	StatusUnknown  = "未返回库存"
	StatusVerify   = "需官网验证"
	StatusLimited  = "库存接口受限"
	StatusError    = "查询失败"

	Pause   = "暂停"
	Running = "监听中"

	verificationRetryInterval = 15 * time.Second
)

var errAppleVerificationRequired = errors.New("Apple Store requires browser verification")
var errAppleStockLimited = errors.New("Apple 库存接口暂不可用或受限，打开商品页不能解除该状态")

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var Listen = listenService{
	items:  map[string]ListenItem{},
	Status: binding.NewString(),
	Area:   model.Areas[0],
	Logs:   widget.NewLabel(""),
}

type listenService struct {
	items           map[string]ListenItem
	Status          binding.String
	Area            model.Area
	Logs            *widget.Label
	BarkNotifyUrl   string
	requestCount    int64 // 请求计数器
	verifyNotified  bool
	limitedNotified bool
	lookupLimited   bool
	retryBackoff    time.Duration
}

type ListenItem struct {
	Store   model.Store
	Product model.Product
	Status  string
	Time    carbon.DateTime
}

func (s *listenService) Add(areaTitle string, storeTitle string, productTitle string) {

	store := Store.GetStore(areaTitle, storeTitle)
	product := Product.GetProduct(areaTitle, productTitle)

	uniqKey := store.StoreNumber + "." + product.Code

	if s.items[uniqKey].Store.StoreNumber == "" {
		s.items[uniqKey] = ListenItem{
			Store:   store,
			Product: product,
			Status:  StatusWait,
		}
	}

	s.UpdateLogStr()
}

func (s *listenService) AddWithProductInfo(areaTitle string, storeTitle string, productTitle string, productCode string, productType string, purchasePath string) {
	store := Store.GetStore(areaTitle, storeTitle)

	product := model.Product{
		Title:        productTitle,
		Code:         productCode,
		Type:         productType,
		PurchasePath: purchasePath,
	}

	uniqKey := store.StoreNumber + "." + product.Code

	if s.items[uniqKey].Store.StoreNumber == "" {
		s.items[uniqKey] = ListenItem{
			Store:   store,
			Product: product,
			Status:  StatusWait,
		}
	}

	s.UpdateLogStr()
}

func (s *listenService) AddWithStoreInfo(store model.Store, productTitle string, productCode string, productType string, purchasePath string) {
	// 验证门店信息是否有效
	if store.StoreNumber == "" {
		log.Printf("Error: Invalid store information - StoreNumber is empty")
		return
	}

	product := model.Product{
		Title:        productTitle,
		Code:         productCode,
		Type:         productType,
		PurchasePath: purchasePath,
	}

	uniqKey := store.StoreNumber + "." + product.Code

	log.Printf("Adding item with key: %s (Store: %s, Product: %s, Code: %s)",
		uniqKey, store.CityStoreName, productTitle, productCode)

	if s.items[uniqKey].Store.StoreNumber == "" {
		s.items[uniqKey] = ListenItem{
			Store:   store,
			Product: product,
			Status:  StatusWait,
		}
	}

	s.UpdateLogStr()
}

func (s *listenService) SetBarkUrl(barkUrl string) {
	s.BarkNotifyUrl = barkUrl
}

func (s *listenService) SetListenItems(items map[string]ListenItem) {
	s.items = items
	// 保存的是监控配置；历史查询状态不能代表本次启动的会话。
	for key, item := range s.items {
		item.Status = StatusWait
		s.items[key] = item
	}
	s.UpdateLogStr()
}

func (s *listenService) GetListenItems() map[string]ListenItem {
	return s.items
}

func (s *listenService) Clean() {
	s.items = map[string]ListenItem{}
	s.UpdateLogStr()
}

func (s *listenService) UpdateLogStr() {
	var str string

	for _, item := range s.items {

		str += fmt.Sprintf(
			"[%s] %s %s %s %s",
			item.Status,
			item.Time,
			item.Store.CityStoreName,
			item.Product.Title,
			"\n",
		)
	}

	s.Logs.SetText(str)
}

func (s *listenService) UpdateStatus(uniqKey string, status string) {
	item, exists := s.items[uniqKey]
	if !exists {
		log.Printf("WARNING: UpdateStatus called with non-existent key: %s", uniqKey)
		return
	}
	oldStatus := item.Status
	item.Time = carbon.DateTime{Carbon: carbon.Now(carbon.Shanghai)}
	item.Status = status
	s.items[uniqKey] = item
	log.Printf("Updated status for %s: %s -> %s (Store: %s, Product: %s)",
		uniqKey, oldStatus, status, item.Store.CityStoreName, item.Product.Title)
}

func (s *listenService) Run() {
	s.Status.Set(Pause)

	go func() {
		for {
			if stats, ok := s.Status.Get(); ok == nil && stats == Running && len(s.items) > 0 {
				skus, err := s.groupByStore()
				if err != nil {
					log.Printf("Stock lookup failed: %v", err)
					if errors.Is(err, errAppleStockLimited) {
						for key := range s.items {
							s.UpdateStatus(key, StatusLimited)
						}
						if !s.limitedNotified {
							s.limitedNotified = true
							fyne.Do(func() {
								dialog.ShowInformation("库存接口受限", "Apple 库存接口返回了不可用页面。程序没有获得有效库存，不能据此判断有货或无货。无需在商品页反复验证；程序将在 60 秒后重试。", view.Window)
							})
						}
						fyne.Do(s.UpdateLogStr)
						s.waitForRetry(time.Minute)
						continue
					}
					if errors.Is(err, errAppleVerificationRequired) {
						var purchaseURL string
						for key, item := range s.items {
							s.UpdateStatus(key, StatusVerify)
							if purchaseURL == "" {
								var urlErr error
								purchaseURL, urlErr = ProductPurchaseURL(s.Area.ShortCode, item.Product.Type, item.Product.Code, item.Product.PurchasePath)
								if urlErr != nil {
									log.Printf("Failed to build verification URL: %v", urlErr)
									purchaseURL = ""
								}
							}
						}

						if !s.verifyNotified && purchaseURL != "" {
							s.verifyNotified = true
							s.openBrowser(purchaseURL)
							msg := "Apple 库存接口返回验证页面。已打开所选商品页，但外部浏览器的验证会话无法自动共享给桌面版；程序每 15 秒重试，只有实际查到有效库存后，列表才会恢复。"
							fyne.Do(func() {
								dialog.ShowInformation("需官网验证", msg, view.Window)
							})
							go s.SendPushNotificationByBark("需官网验证", msg, purchaseURL)
						}
						fyne.Do(s.UpdateLogStr)
						log.Printf("Apple verification is still required; retrying in %s", verificationRetryInterval)
						s.waitForRetry(verificationRetryInterval)
						continue
					} else {
						for key := range s.items {
							s.UpdateStatus(key, StatusError)
						}
						fyne.Do(func() {
							dialog.ShowError(fmt.Errorf("库存查询失败：%v", err), view.Window)
						})
					}
					s.Status.Set(Pause)
					fyne.Do(s.UpdateLogStr)
					continue
				}
				s.limitedNotified = false
				if s.verifyNotified {
					log.Println("Apple verification cleared; automatic monitoring resumed")
					s.verifyNotified = false
				}

				// 首先检查是否有任何店铺有货（用于location查询）
				availableStores := make(map[string][]string) // productCode -> []storeNumbers
				for skuKey, available := range skus {
					if available {
						parts := strings.Split(skuKey, ".")
						if len(parts) == 2 {
							storeNumber := parts[0]
							productCode := parts[1]
							// 检查是否是我们监控的产品
							for _, item := range s.items {
								if item.Product.Code == productCode {
									availableStores[productCode] = append(availableStores[productCode], storeNumber)
									break
								}
							}
						}
					}
				}

				// 创建店铺编号到名称的映射
				storeNameMap := make(map[string]string)
				for _, stores := range model.ChinaStores {
					for _, store := range stores {
						storeNameMap[store.StoreNumber] = store.CityStoreName
					}
				}

				// 打印有货店铺信息
				for productCode, stores := range availableStores {
					var storeNames []string
					for _, storeNum := range stores {
						if name, ok := storeNameMap[storeNum]; ok {
							storeNames = append(storeNames, name)
						} else {
							storeNames = append(storeNames, storeNum)
						}
					}
					log.Printf("Product %s available at %d stores: %v", productCode, len(stores), storeNames)
				}

				for key, item := range s.items {
					// 检查指定店铺是否有库存
					hasStock := false
					storeKey := fmt.Sprintf("%s.%s", item.Store.StoreNumber, item.Product.Code)
					if _, exists := skus[storeKey]; !exists {
						continue
					}

					if available, exists := skus[storeKey]; exists && available {
						hasStock = true
						log.Printf("Specific store query: Store %s (%s) has stock for product %s",
							item.Store.StoreNumber, item.Store.CityStoreName, item.Product.Code)
					}

					log.Printf("Checking item: key=%s, store=%s (%s), product=%s, hasStock=%v",
						key, item.Store.CityStoreName, item.Store.StoreNumber, item.Product.Title, hasStock)

					if hasStock {
						// 有货（指定店铺有库存）
						s.UpdateStatus(key, StatusInStock)
						s.Status.Set(Pause)

						// 构建提醒消息
						detectedAt := time.Now()
						observation := newStockObservation(s.Area.ShortCode, item.Store, item.Product, detectedAt)
						msg := fmt.Sprintf("%s %s 有货", item.Store.CityStoreName, item.Product.Title)

						// 打开已选好型号、容量和颜色的官网商品页。
						purchaseURL, err := ProductPurchaseURL(s.Area.ShortCode, item.Product.Type, item.Product.Code, item.Product.PurchasePath)
						if err != nil {
							log.Printf("Failed to build purchase URL: %v", err)
							continue
						}
						s.openBrowser(purchaseURL)
						fyne.Do(func() {
							dialog.ShowInformation("有货提醒", msg, view.Window)
							go reportStockObservation(observation)
							view.App.SendNotification(&fyne.Notification{
								Title:   "有货提醒",
								Content: msg,
							})
						})
						go s.AlertMp3()
						go s.SendPushNotificationByBark("有货提醒", msg, purchaseURL)
						break
					} else {
						s.UpdateStatus(key, StatusOutStock)
					}
				}

				fyne.Do(s.UpdateLogStr)
				retryAfter := 30 * time.Second
				s.retryBackoff = nextStockRetryBackoff(s.retryBackoff, s.lookupLimited)
				if s.retryBackoff > 0 {
					retryAfter = s.retryBackoff
				}
				log.Printf("Next stock round in %s", retryAfter)
				s.waitForRetry(retryAfter)
			} else if ok == nil && stats != Running {
				s.verifyNotified = false
				s.limitedNotified = false
			}

			time.Sleep(time.Millisecond * 500)
		}
	}()
}

func (s *listenService) groupByStore() (skus map[string]bool, err error) {
	skus = map[string]bool{}
	s.lookupLimited = false

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("stock lookup failed: %v", r)
		}
	}()

	// 增加请求计数
	s.requestCount++

	// 每10次请求后添加额外延迟，模拟正常用户行为
	if s.requestCount%10 == 0 {
		time.Sleep(time.Duration(2000+rand.Intn(3000)) * time.Millisecond)
		log.Printf("Request count: %d, adding extra delay", s.requestCount)
	}

	group := map[string][]ListenItem{}
	reqs := map[string]string{}

	for _, item := range s.items {
		group[item.Store.StoreNumber] = append(group[item.Store.StoreNumber], item)
	}

	for storeNumber, items := range group {

		// 检查是否为中国大陆、香港、日本、新加坡、美国、英国、澳大利亚
		if s.Area.ShortCode == "cn" || s.Area.ShortCode == "hk" || s.Area.ShortCode == "jp" || s.Area.ShortCode == "sg" || s.Area.ShortCode == "us" || s.Area.ShortCode == "uk" || s.Area.ShortCode == "au" {
			// 使用特定店铺查询API格式
			var params []string

			// 使用当前 pickup-message 的必要参数。
			params = append(params, "pl=true")

			// 3. 所有产品的mts.i=regular和parts.i=SKU（索引递增）
			for i, item := range items {
				params = append(params, fmt.Sprintf("mts.%d=regular", i))
				params = append(params, fmt.Sprintf("parts.%d=%s", i, url.QueryEscape(item.Product.Code)))
			}

			// 4. 添加查询参数
			if len(items) > 0 {
				store := items[0].Store

				params = append(params, pickupLocationQuery(s.Area.ShortCode, store.StoreNumber, store.District))
			}

			// 5. 日本特殊参数cppart放在最后
			if s.Area.ShortCode == "jp" {
				params = append(params, "cppart=UNLOCKED_JP")
			}

			// 构建完整URL
			queryStr := strings.Join(params, "&")
			link, err := pickupMessagesURL(s.Area.ShortCode, queryStr)
			if err != nil {
				return skus, err
			}
			reqs[storeNumber] = link
			log.Printf("Store %s URL: %s", storeNumber, link)
		}
	}

	count := len(reqs)
	if count < 1 {
		return skus, nil
	}

	for storeNumber, link := range reqs {
		if storeResultsComplete(group[storeNumber], skus) {
			continue
		}
		if state, err := s.Status.Get(); err == nil && state != Running {
			break
		}
		for key, item := range s.items {
			if item.Store.StoreNumber == storeNumber {
				s.UpdateStatus(key, StatusQuerying)
			}
		}
		fyne.Do(s.UpdateLogStr)
		result, err := s.getSkuByLink(link)
		if err != nil {
			status := StatusError
			if errors.Is(err, errAppleStockLimited) {
				status = StatusLimited
			}
			if errors.Is(err, errAppleVerificationRequired) {
				status = StatusVerify
			}
			log.Printf("Store %s lookup failed: %v", storeNumber, err)
			for key, item := range s.items {
				if item.Store.StoreNumber == storeNumber {
					s.UpdateStatus(key, status)
				}
			}
			fyne.Do(s.UpdateLogStr)
			if status == StatusLimited || status == StatusVerify {
				s.lookupLimited = true
				return skus, nil
			}
			s.waitForRetry(3 * time.Second)
			continue
		}
		for key, available := range result {
			skus[key] = available
		}
		foundStock := false
		for key, item := range s.items {
			available, exists := result[key]
			if !exists && item.Store.StoreNumber != storeNumber {
				continue
			}
			status := StatusUnknown
			if exists {
				status = StatusOutStock
				if available {
					status = StatusInStock
					foundStock = true
				}
			}
			s.UpdateStatus(key, status)
		}
		fyne.Do(s.UpdateLogStr)
		// 发现目标库存后立即交给提醒流程，不等待其他门店。
		if foundStock {
			return skus, nil
		}
		if count > 1 {
			s.waitForRetry(3 * time.Second)
		}
		if state, err := s.Status.Get(); err == nil && state != Running {
			break
		}
	}

	log.Printf("Total SKUs collected: %d", len(skus))
	for k, v := range skus {
		if v {
			log.Printf("Available SKU: %s", k)
		}
	}

	return skus, nil
}

func (s *listenService) getSkuByLink(skUrl string) (map[string]bool, error) {
	return s.getSkuByLinkContext(context.Background(), skUrl)
}

func (s *listenService) getSkuByLinkContext(ctx context.Context, skUrl string) (map[string]bool, error) {
	skus := map[string]bool{}

	// 添加随机延迟（100-500ms）
	delay := time.Duration(100+rand.Intn(400)) * time.Millisecond
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return skus, ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, skUrl, nil)
	if err != nil {
		return skus, err
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", getAcceptLanguage(s.Area.ShortCode))
	req.Header.Set("User-Agent", "AppleStoreHelper/1.8 (+https://github.com/Sunbelife/apple-store-helper-15)")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	var resp *http.Response
	if isApplePickupURL(s.Area.ShortCode, skUrl) {
		resp, err = sessionForArea(s.Area.ShortCode).do(ctx, s.Area.ShortCode, skUrl)
	} else {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err = client.Do(req)
	}
	if err != nil {
		return skus, err
	}
	defer resp.Body.Close()

	log.Println(resp.Status, skUrl)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return skus, err
	}
	if err := classifyStockHTTPResponse(resp.StatusCode, body); err != nil {
		return skus, fmt.Errorf("HTTP %d: %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		return skus, fmt.Errorf("stock endpoint returned HTTP %d", resp.StatusCode)
	}

	return parseStockResponse(body, skUrl)
}

func classifyStockHTTPResponse(status int, body []byte) error {
	text := strings.ToLower(string(body))
	if status == 541 {
		if strings.Contains(text, "page not found") || strings.Contains(text, "page you’re looking for") || strings.Contains(text, "page you're looking for") {
			return errAppleStockLimited
		}
		return errAppleVerificationRequired
	}
	if status == http.StatusForbidden || status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		return errAppleStockLimited
	}
	return nil
}

func (s *listenService) waitForRetry(delay time.Duration) {
	deadline := time.NewTimer(delay)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			return
		case <-ticker.C:
			if state, err := s.Status.Get(); err != nil || state != Running {
				return
			}
		}
	}
}

func parseStockResponse(body []byte, skUrl string) (map[string]bool, error) {
	skus := map[string]bool{}
	if !gjson.ValidBytes(body) {
		return skus, fmt.Errorf("stock endpoint returned invalid JSON")
	}

	// 解析响应JSON
	// 中国地区的响应格式: body.stores[].partsAvailability
	// 其他地区的响应格式: body.content.pickupMessage.stores 或 body.content.pickupMessage.pickupEligibility
	// 库存判断字段: pickupDisplay ("available"=有货, "ineligible"=无货)

	// 先尝试中国地区格式 (body.stores)
	stores := gjson.GetBytes(body, "body.stores").Array()
	if len(stores) > 0 {
		// 中国地区格式
		for _, store := range stores {
			storeNumber := store.Get("storeNumber").String()
			// 检查每个产品在该店铺的可用性
			partsAvailability := store.Get("partsAvailability").Map()
			for productCode, availability := range partsAvailability {
				uniqKey := fmt.Sprintf("%s.%s", storeNumber, productCode)
				// 检查 pickupDisplay 字段，"available" 表示有货，"ineligible" 表示无货
				pickupDisplay := availability.Get("pickupDisplay").String()
				if pickupDisplay != "available" && pickupDisplay != "unavailable" && pickupDisplay != "ineligible" {
					continue
				}
				isAvailable := (pickupDisplay == "available")

				// pickupDisplay 为 "available" 表示有货
				skus[uniqKey] = isAvailable

				if isAvailable {
					log.Printf("✅ Store %s Product %s: IN STOCK (pickupDisplay=%s)", storeNumber, productCode, pickupDisplay)
				} else {
					log.Printf("❌ Store %s Product %s: OUT OF STOCK (pickupDisplay=%s)", storeNumber, productCode, pickupDisplay)
				}
			}
		}
	} else {
		// 尝试其他地区格式 (body.content.pickupMessage.stores)
		stores = gjson.GetBytes(body, "body.content.pickupMessage.stores").Array()
		if len(stores) > 0 {
			for _, result := range stores {
				storeNumber := result.Get("storeNumber").String()
				// 检查每个产品在该店铺的可用性
				for productCode, availability := range result.Get("partsAvailability").Map() {
					uniqKey := fmt.Sprintf("%s.%s", storeNumber, productCode)
					// 检查 pickupDisplay 字段，"available" 表示有货，"ineligible" 表示无货
					pickupDisplay := availability.Get("pickupDisplay").String()
					if pickupDisplay != "available" && pickupDisplay != "unavailable" && pickupDisplay != "ineligible" {
						continue
					}
					isAvailable := (pickupDisplay == "available")

					// pickupDisplay 为 "available" 表示有货
					skus[uniqKey] = isAvailable

					if isAvailable {
						log.Printf("✅ Store %s Product %s: IN STOCK (pickupDisplay=%s)", storeNumber, productCode, pickupDisplay)
					} else {
						log.Printf("❌ Store %s Product %s: OUT OF STOCK (pickupDisplay=%s)", storeNumber, productCode, pickupDisplay)
					}
				}
			}
		} else {
			// 单店铺查询逻辑（新格式）- 国外地区
			pickupEligibility := gjson.GetBytes(body, "body.content.pickupMessage.pickupEligibility")
			if pickupEligibility.Exists() {
				// 从URL中提取店铺编号
				u, _ := url.Parse(skUrl)
				storeNumber := u.Query().Get("store")

				// 遍历所有产品
				pickupEligibility.ForEach(func(productCode, productInfo gjson.Result) bool {
					uniqKey := fmt.Sprintf("%s.%s", storeNumber, productCode)

					// 检查 pickupDisplay 字段，"available" 表示有货，"ineligible" 表示无货
					// 在这种格式下，pickupDisplay 可能在不同的路径
					pickupDisplay := productInfo.Get("messageTypes.regular.pickupDisplay").String()
					if pickupDisplay == "" {
						// 尝试其他可能的路径
						pickupDisplay = productInfo.Get("pickupDisplay").String()
					}
					if pickupDisplay == "" {
						// 再尝试另一个路径
						pickupDisplay = productInfo.Get("messageTypes.compact.pickupDisplay").String()
					}

					if pickupDisplay != "available" && pickupDisplay != "unavailable" && pickupDisplay != "ineligible" {
						return true
					}
					isAvailable := (pickupDisplay == "available")

					skus[uniqKey] = isAvailable

					if isAvailable {
						log.Printf("✅ Store %s Product %s: IN STOCK (pickupDisplay=%s)", storeNumber, productCode, pickupDisplay)
					} else {
						log.Printf("❌ Store %s Product %s: OUT OF STOCK (pickupDisplay=%s)", storeNumber, productCode, pickupDisplay)
					}

					return true
				})
			} else {
				// 如果没有找到任何已知格式，打印响应预览帮助调试
				return skus, fmt.Errorf("unknown stock response format: %s", body[:min(200, len(body))])
			}
		}
	}

	return skus, nil
}

// 型号对应预约地址
func (s *listenService) model2Url(productType string, partNumber string) string {
	purchaseURL, err := ProductPurchaseURL(s.Area.ShortCode, productType, partNumber, "")
	if err != nil {
		return ""
	}
	return purchaseURL
}

func (s *listenService) openBrowser(link string) {
	parse, err := url.Parse(link)
	if err != nil {
		dialog.ShowError(err, view.Window)
		return
	}

	err = view.App.OpenURL(parse)
	if err != nil {
		dialog.ShowError(err, view.Window)
		return
	}
}

func (s *listenService) AlertMp3() {
	reader := bytes.NewReader(theme.Mp3().Content())
	streamer, _, err := mp3.Decode(io.NopCloser(reader))
	if err != nil {
		panic(err)
	}
	defer streamer.Close()

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))
	<-done
}

func (s *listenService) SendPushNotificationByBark(title string, content string, bagUrl string) {
	if len(s.BarkNotifyUrl) <= 0 {
		log.Println("Bark URL is empty, skipping notification")
		return
	}

	// 对标题和内容进行URL编码
	encodedTitle := url.QueryEscape(title)
	encodedContent := url.QueryEscape(content)
	encodedUrl := url.QueryEscape(bagUrl)

	// 构建正确的Bark API URL格式
	// 格式: https://api.day.app/{device_key}?title={title}&body={body}&url={url}
	apiUrl := fmt.Sprintf("%s?title=%s&body=%s&url=%s",
		strings.TrimRight(s.BarkNotifyUrl, "/"),
		encodedTitle,
		encodedContent,
		encodedUrl)

	log.Printf("Sending Bark notification to: %s", apiUrl)

	response, err := http.Get(apiUrl)
	if err != nil {
		log.Printf("Bark notification error: %v", err)
		return
	}
	defer response.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Failed to read Bark response: %v", err)
	}

	// 检查响应状态
	if response.StatusCode != 200 {
		log.Printf("Bark notification failed with status: %d, response: %s", response.StatusCode, string(body))
		return
	}

	log.Printf("Bark notification sent successfully, response: %s", string(body))
}

// A postal response can include nearby monitored stores. Query only missing SKUs.
func storeResultsComplete(items []ListenItem, results map[string]bool) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if _, exists := results[item.Store.StoreNumber+"."+item.Product.Code]; !exists {
			return false
		}
	}
	return true
}

func nextStockRetryBackoff(previous time.Duration, limited bool) time.Duration {
	if !limited {
		return 0
	}
	if previous < time.Minute {
		return time.Minute
	}
	if previous >= 5*time.Minute/2 {
		return 5 * time.Minute
	}
	return previous * 2
}
