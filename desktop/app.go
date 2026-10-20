package main

import (
	"apple-store-helper/model"
	"apple-store-helper/services"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Target struct {
	ID        string        `json:"id"`
	Area      model.Area    `json:"area"`
	Store     model.Store   `json:"store"`
	Product   model.Product `json:"product"`
	Status    string        `json:"status"`
	CheckedAt string        `json:"checkedAt"`
	Detail    string        `json:"detail"`
}
type Entry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}
type State struct {
	Items          []Target `json:"items"`
	Running        bool     `json:"running"`
	Phase          string   `json:"phase"`
	NextAt         int64    `json:"nextAt"`
	BarkConfigured bool     `json:"barkConfigured"`
	Logs           []Entry  `json:"logs"`
	Alert          *Target  `json:"alert"`
}
type Catalog struct {
	Areas    []model.Area        `json:"areas"`
	Stores   []model.Store       `json:"stores"`
	Products []model.ProductInfo `json:"products"`
}
type savedSettings struct {
	Items []Target `json:"items"`
	Bark  string   `json:"bark"`
}
type Desktop struct {
	onStock          func(context.Context, uint64, Target)
	query            func(context.Context, model.Area, model.Store, []model.Product) (map[string]bool, error)
	queryErrorStatus func(error) string
	updater          *githubUpdater
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	generation       uint64
	state            State
	bark             string
	usageMu          sync.Mutex
	usageIdentity    string
	uiLanguage       string
	settingsPath     string
	reported         string
	catalogMu        sync.Mutex
	catalog          map[string]Catalog
}

func NewDesktop() *Desktop {
	return &Desktop{
		query:            services.QueryRetailPickup,
		queryErrorStatus: services.StockQueryErrorStatus,
		updater:          newGitHubUpdater(),
		state:            State{Items: []Target{}, Logs: []Entry{}, Phase: "已暂停"},
		catalog:          map[string]Catalog{},
	}
}
func (a *Desktop) startup(ctx context.Context) {
	a.ctx = ctx
	dir, err := os.UserConfigDir()
	if err != nil {
		a.addLog("无法定位配置目录：" + err.Error())
		return
	}
	a.settingsPath = filepath.Join(dir, "Sunbelife", "Qiangnimei2", "settings.json")
	data, err := os.ReadFile(a.settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		a.addLog("读取配置失败：" + err.Error())
		return
	}
	var saved savedSettings
	if err = json.Unmarshal(data, &saved); err != nil {
		a.addLog("配置文件损坏，未加载：" + err.Error())
		return
	}
	a.bark = saved.Bark
	for i := range saved.Items {
		saved.Items[i].Status = services.StatusWait
		saved.Items[i].CheckedAt = ""
		saved.Items[i].Detail = ""
	}
	if saved.Items != nil {
		a.state.Items = saved.Items
	}
	a.state.BarkConfigured = a.bark != ""
}
func (a *Desktop) shutdown(context.Context) { a.Pause() }
func (a *Desktop) addLog(msg string)        { a.mu.Lock(); defer a.mu.Unlock(); a.logLocked(msg) }
func (a *Desktop) logLocked(msg string) {
	a.state.Logs = append(a.state.Logs, Entry{time.Now().Format("15:04:05"), msg})
	if len(a.state.Logs) > 100 {
		a.state.Logs = a.state.Logs[len(a.state.Logs)-100:]
	}
}
func (a *Desktop) GetState() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.state
	s.Items = append([]Target{}, a.state.Items...)
	s.Logs = append([]Entry{}, a.state.Logs...)
	if s.Alert != nil {
		alert := *s.Alert
		s.Alert = &alert
	}
	return s
}
func areaFor(code string) (model.Area, error) {
	for _, ar := range model.Areas {
		if ar.ShortCode == code {
			return ar, nil
		}
	}
	return model.Area{}, fmt.Errorf("不支持的地区")
}
func (a *Desktop) GetCatalog(code string) (Catalog, error) {
	area, err := areaFor(code)
	if err != nil {
		return Catalog{}, err
	}
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	if c, ok := a.catalog[code]; ok {
		return c, nil
	}
	data, err := services.LoadProductData(code)
	if a.settingsPath != "" {
		if cached, readErr := os.ReadFile(filepath.Join(filepath.Dir(a.settingsPath), "catalogs", code+".json")); readErr == nil {
			var saved services.ProductData
			if json.Unmarshal(cached, &saved) == nil && saved.AreaCode == code && len(saved.Products) > 0 {
				data = &saved
				err = nil
			}
		}
	}
	if err != nil {
		return Catalog{}, err
	}
	c := Catalog{Areas: model.Areas, Stores: services.Store.ByArea(area), Products: []model.ProductInfo{}}
	if a.settingsPath != "" {
		if cached, readErr := os.ReadFile(filepath.Join(filepath.Dir(a.settingsPath), "catalogs", code+"-stores.json")); readErr == nil {
			var saved []model.Store
			if json.Unmarshal(cached, &saved) == nil {
				c.Stores = mergeStores(c.Stores, saved)
			}
		}
	}
	for _, products := range data.Products {
		for _, p := range products {
			if p.Type == "appletv4k" || strings.HasPrefix(p.Model, "Apple TV") {
				continue
			}
			p.Color = services.NormalizeProductColor(p.Color)
			c.Products = append(c.Products, p)
		}
	}
	sort.Slice(c.Products, func(i, j int) bool {
		return c.Products[i].Model+c.Products[i].Capacity+c.Products[i].Color < c.Products[j].Model+c.Products[j].Capacity+c.Products[j].Color
	})
	a.catalog[code] = c
	return c, nil
}
func (a *Desktop) RefreshCatalog(code string) (Catalog, error) {
	a.mu.Lock()
	running := a.state.Running
	a.mu.Unlock()
	if running {
		return Catalog{}, fmt.Errorf("请先暂停监控，再更新目录")
	}
	return a.refreshCatalog(code)
}

func (a *Desktop) saveLocked() error {
	if a.settingsPath == "" {
		return fmt.Errorf("配置目录不可用")
	}
	if err := os.MkdirAll(filepath.Dir(a.settingsPath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(savedSettings{a.state.Items, a.bark}, "", "  ")
	if err != nil {
		return err
	}
	tmp := a.settingsPath + ".tmp"
	if err = os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, a.settingsPath)
}
func targetID(area, store, sku string) string { return area + ":" + store + ":" + sku }
func (a *Desktop) AddTargets(code string, storeIDs []string, sku string) (int, error) {
	c, err := a.GetCatalog(code)
	if err != nil {
		return 0, err
	}
	area, _ := areaFor(code)
	if area.PickupUnsupported {
		return 0, fmt.Errorf("该地区 Apple 官网未开放门店取货查询")
	}
	var product model.Product
	for _, p := range c.Products {
		if p.Code == sku {
			product = model.Product{Title: p.Model + " " + p.Capacity + " " + p.Color, Code: p.Code, Type: p.Type, PurchasePath: p.PurchasePath}
			break
		}
	}
	if product.Code == "" {
		return 0, fmt.Errorf("请先选择产品")
	}
	stores := map[string]model.Store{}
	for _, s := range c.Stores {
		stores[s.StoreNumber] = s
	}
	for _, id := range storeIDs {
		if _, ok := stores[id]; !ok {
			return 0, fmt.Errorf("门店不属于所选地区")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state.Running {
		return 0, fmt.Errorf("请先暂停监控")
	}
	before := append([]Target{}, a.state.Items...)
	exists := map[string]bool{}
	for _, i := range before {
		exists[i.ID] = true
	}
	n := 0
	for _, id := range storeIDs {
		key := targetID(code, id, sku)
		if exists[key] {
			continue
		}
		exists[key] = true
		a.state.Items = append(a.state.Items, Target{ID: key, Area: area, Store: stores[id], Product: product, Status: services.StatusWait})
		n++
	}
	if err = a.saveLocked(); err != nil {
		a.state.Items = before
		return 0, err
	}
	return n, nil
}
func (a *Desktop) RemoveTarget(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state.Running {
		return fmt.Errorf("请先暂停监控")
	}
	before := a.state.Items
	next := []Target{}
	for _, i := range before {
		if i.ID != id {
			next = append(next, i)
		}
	}
	a.state.Items = next
	if err := a.saveLocked(); err != nil {
		a.state.Items = before
		return err
	}
	return nil
}
func (a *Desktop) ClearTargets() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state.Running {
		return fmt.Errorf("请先暂停监控")
	}
	before := a.state.Items
	a.state.Items = []Target{}
	if err := a.saveLocked(); err != nil {
		a.state.Items = before
		return err
	}
	return nil
}
func (a *Desktop) SaveBark(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("请输入 Bark 提供的 HTTPS 推送地址")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	before := a.bark
	a.bark = value
	if err := a.saveLocked(); err != nil {
		a.bark = before
		return err
	}
	a.state.BarkConfigured = value != ""
	return nil
}
func (a *Desktop) ImportLegacy() (int, error) {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "选择旧版 user_settings.json", Filters: []wruntime.FileFilter{{DisplayName: "旧版监控配置", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var saved services.UserSettings
	if err = json.Unmarshal(data, &saved); err != nil {
		return 0, err
	}
	area, err := areaFor("cn")
	if err != nil {
		return 0, err
	}
	for _, ar := range model.Areas {
		if ar.Title == saved.SelectedArea {
			area = ar
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state.Running {
		return 0, fmt.Errorf("请先暂停监控")
	}
	before := a.state.Items
	barkBefore := a.bark
	items := append([]Target{}, before...)
	exists := map[string]bool{}
	for _, i := range items {
		exists[i.ID] = true
	}
	n := 0
	for _, i := range saved.ListenItems {
		if i.Store.StoreNumber == "" || i.Product.Code == "" {
			continue
		}
		id := targetID(area.ShortCode, i.Store.StoreNumber, i.Product.Code)
		if exists[id] {
			continue
		}
		exists[id] = true
		items = append(items, Target{ID: id, Area: area, Store: i.Store, Product: i.Product, Status: services.StatusWait})
		n++
	}
	a.state.Items = items
	a.bark = saved.BarkNotifyUrl
	if err = a.saveLocked(); err != nil {
		a.state.Items = before
		a.bark = barkBefore
		return 0, err
	}
	a.state.BarkConfigured = a.bark != ""
	return n, nil
}
func (a *Desktop) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state.Running {
		return nil
	}
	if len(a.state.Items) == 0 {
		return fmt.Errorf("请先添加要监控的产品和门店")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.generation++
	id := a.generation
	a.state.Running = true
	a.state.Alert = nil
	a.state.NextAt = 0
	a.state.Phase = "正在查询"
	a.logLocked("开始监控")
	go a.run(ctx, id)
	return nil
}
func (a *Desktop) Pause() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
	a.generation++
	a.state.Running = false
	a.state.NextAt = 0
	a.state.Phase = "已暂停"
	for i := range a.state.Items {
		if a.state.Items[i].Status == services.StatusQuerying {
			a.state.Items[i].Status = services.StatusWait
		}
	}
}
func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func (a *Desktop) update(id uint64, key, status, detail string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if id != a.generation || !a.state.Running {
		return false
	}
	for i := range a.state.Items {
		if a.state.Items[i].ID == key {
			a.state.Items[i].Status = status
			a.state.Items[i].Detail = detail
			if status != services.StatusQuerying {
				a.state.Items[i].CheckedAt = time.Now().Format(time.RFC3339)
			}
			return true
		}
	}
	return false
}

type storeCooldown struct {
	retryAt time.Time
	backoff time.Duration
}

const (
	minPollDelay = 20 * time.Second
	maxPollDelay = 60 * time.Second
)

func nextPollDelay() time.Duration {
	spanSeconds := int((maxPollDelay - minPollDelay) / time.Second)
	return pollDelayFor(rand.IntN(spanSeconds + 1))
}

func pollDelayFor(randomSecond int) time.Duration {
	return minPollDelay + time.Duration(randomSecond)*time.Second
}

func monitorGroupKey(t Target) string {
	return t.Area.ShortCode + ":" + t.Store.StoreNumber
}

func (a *Desktop) run(ctx context.Context, id uint64) {
	cooldowns := map[string]storeCooldown{}
	for ctx.Err() == nil {
		a.mu.Lock()
		items := append([]Target{}, a.state.Items...)
		if id != a.generation {
			a.mu.Unlock()
			return
		}
		a.state.NextAt = 0
		a.state.Phase = "正在查询"
		a.mu.Unlock()
		groups := map[string][]Target{}
		keys := []string{}
		for _, t := range items {
			key := monitorGroupKey(t)
			if _, ok := groups[key]; !ok {
				keys = append(keys, key)
			}
			groups[key] = append(groups[key], t)
		}
		sort.Strings(keys)
		known := map[string]bool{}
		requests := 0
		for _, key := range keys {
			if ctx.Err() != nil {
				return
			}
			if cooldown, ok := cooldowns[key]; ok && time.Now().Before(cooldown.retryAt) {
				continue
			}
			group := groups[key]
			products := []model.Product{}
			complete := true
			for _, t := range group {
				products = append(products, t.Product)
				if _, ok := known[t.ID]; !ok {
					complete = false
				}
			}
			if complete {
				continue
			}
			if requests > 0 && !sleepContext(ctx, 3*time.Second) {
				return
			}
			for _, t := range group {
				a.update(id, t.ID, services.StatusQuerying, "")
			}
			result, err := a.query(ctx, group[0].Area, group[0].Store, products)
			requests++
			var scheduled *services.PickupSchedule
			if errors.As(err, &scheduled) {
				err = nil
			}
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				status := a.queryErrorStatus(err)
				detail := err.Error()
				if status == services.StatusLimited || status == services.StatusVerify {
					previous := cooldowns[key].backoff
					delay := services.StockRetryBackoff(previous, true)
					cooldowns[key] = storeCooldown{retryAt: time.Now().Add(delay), backoff: delay}
					detail = fmt.Sprintf("%s；仅该门店冷却 %d 秒，其他门店继续监控", detail, int(delay.Seconds()))
				}
				for _, t := range group {
					a.update(id, t.ID, status, detail)
				}
				a.addLog(group[0].Store.CityStoreName + "：" + detail)
				continue
			}
			// A valid response clears this store's own risk-control backoff, even if
			// Apple omitted one of the requested SKUs from the payload.
			delete(cooldowns, key)
			var found *Target
			for _, t := range items {
				if t.Area.ShortCode != group[0].Area.ShortCode {
					continue
				}
				if scheduled != nil {
					if quote, ok := scheduled.Quotes[t.Store.StoreNumber+"."+t.Product.Code]; ok {
						known[t.ID] = false
						a.update(id, t.ID, "可预约取货", quote)
						continue
					}
				}
				available, exists := result[t.Store.StoreNumber+"."+t.Product.Code]
				if !exists {
					continue
				}
				// Postal-code responses can cover nearby monitored stores. Treat an
				// explicit result as a successful lookup for that store as well.
				delete(cooldowns, monitorGroupKey(t))
				known[t.ID] = available
				status := services.StatusOutStock
				if available {
					status = services.StatusInStock
				}
				a.update(id, t.ID, status, "")
				if available && found == nil {
					copy := t
					found = &copy
				}
			}
			for _, t := range group {
				if _, exists := known[t.ID]; !exists {
					a.update(id, t.ID, services.StatusUnknown, "返回结果未包含所选门店 / SKU")
				}
			}
			if found != nil {
				if a.onStock != nil {
					a.onStock(ctx, id, *found)
				} else {
					a.stockFound(ctx, id, *found)
				}
				return
			}
		}
		if ctx.Err() != nil {
			return
		}
		delay := nextPollDelay()
		now := time.Now()
		cooling := 0
		var earliestRetry time.Time
		for _, key := range keys {
			cooldown, ok := cooldowns[key]
			if !ok || !now.Before(cooldown.retryAt) {
				continue
			}
			cooling++
			if earliestRetry.IsZero() || cooldown.retryAt.Before(earliestRetry) {
				earliestRetry = cooldown.retryAt
			}
		}
		// When every store is cooling, sleep until the first store is eligible.
		// Otherwise healthy stores use a fresh random 20-60 second delay each round.
		if cooling == len(keys) && !earliestRetry.IsZero() {
			delay = time.Until(earliestRetry)
			if delay < 0 {
				delay = 0
			}
		}
		a.mu.Lock()
		if id != a.generation {
			a.mu.Unlock()
			return
		}
		a.state.NextAt = time.Now().Add(delay).UnixMilli()
		a.state.Phase = "等待下一轮"
		if cooling == len(keys) && cooling > 0 {
			a.state.Phase = services.StatusLimited
		} else if cooling > 0 {
			a.state.Phase = "部分门店冷却中，其他门店继续监控"
		}
		if cooling > 0 {
			a.logLocked(fmt.Sprintf("本轮发送 %d 次请求，%d 个门店单独冷却，%d 秒后继续", requests, cooling, int(delay.Seconds())))
		} else {
			a.logLocked(fmt.Sprintf("本轮发送 %d 次请求，%d 秒后重试", requests, int(delay.Seconds())))
		}
		a.mu.Unlock()
		if !sleepContext(ctx, delay) {
			return
		}
	}
}
func (a *Desktop) stockFound(ctx context.Context, id uint64, t Target) {
	purchase, err := services.ShoppingBagURL(t.Area.ShortCode)
	if err != nil {
		a.update(id, t.ID, services.StatusError, err.Error())
		a.addLog(err.Error())
		a.Pause()
		return
	}
	a.mu.Lock()
	if id != a.generation || ctx.Err() != nil {
		a.mu.Unlock()
		return
	}
	t.Status = services.StatusInStock
	t.CheckedAt = time.Now().Format(time.RFC3339)
	a.state.Alert = &t
	a.state.Running = false
	a.state.NextAt = 0
	a.state.Phase = "发现有货，已暂停"
	a.reported = ""
	bark := a.bark
	title := "有货提醒"
	if a.uiLanguage == "en" {
		title = "Stock alert"
	}
	a.logLocked(t.Store.CityStoreName + " " + t.Product.Title + " 有货")
	a.mu.Unlock()
	wruntime.BrowserOpenURL(a.ctx, purchase)
	if allowed, err := wruntime.CheckNotificationAuthorization(a.ctx); err == nil && allowed {
		if err := wruntime.InitializeNotifications(a.ctx); err == nil {
			_ = wruntime.SendNotification(a.ctx, wruntime.NotificationOptions{ID: t.ID, Title: title, Body: t.Store.CityStoreName + " " + t.Product.Title})
		}
	}
	if bark != "" {
		go sendBarkRequest(context.Background(), barkClient(), bark, title, t.Store.CityStoreName+" "+t.Product.Title, purchase)
	}
}

// The frontend calls this only after presenting the real stock alert.
func (a *Desktop) AlertShown(id string) {
	a.mu.Lock()
	alert := a.state.Alert
	if alert == nil || alert.ID != id || a.reported == alert.ID+alert.CheckedAt {
		a.mu.Unlock()
		return
	}
	t := *alert
	a.reported = t.ID + t.CheckedAt
	a.mu.Unlock()
	detected, _ := time.Parse(time.RFC3339, t.CheckedAt)
	go func() {
		if services.ReportPublicStock(t.Area.ShortCode, t.Store, t.Product, detected) {
			_ = a.TrackUsage("stock_report")
		}
	}()
}
func (a *Desktop) DismissAlert() { a.mu.Lock(); defer a.mu.Unlock(); a.state.Alert = nil }
func (a *Desktop) OpenBag(id string) error {
	a.mu.Lock()
	var area string
	for _, target := range a.state.Items {
		if target.ID == id {
			area = target.Area.ShortCode
			break
		}
	}
	a.mu.Unlock()
	u, err := services.ShoppingBagURL(area)
	if err != nil {
		return err
	}
	wruntime.BrowserOpenURL(a.ctx, u)
	return nil
}

func (a *Desktop) OpenPurchase(id string) error {
	a.mu.Lock()
	var target *Target
	for _, i := range a.state.Items {
		if i.ID == id {
			copy := i
			target = &copy
			break
		}
	}
	a.mu.Unlock()
	if target == nil {
		return fmt.Errorf("监控项不存在")
	}
	u, err := services.ProductPurchaseURL(target.Area.ShortCode, target.Product.Type, target.Product.Code, target.Product.PurchasePath)
	if err == nil {
		wruntime.BrowserOpenURL(a.ctx, u)
	}
	return err
}
func (a *Desktop) OpenShop() { wruntime.BrowserOpenURL(a.ctx, "https://store.sunbelife.com") }
func (a *Desktop) OpenUsageArticle() {
	wruntime.BrowserOpenURL(a.ctx, "https://mp.weixin.qq.com/s/PCVWj6xmK13lBM_BbBokiA")
}
func (a *Desktop) OpenFeedback() {
	wruntime.BrowserOpenURL(a.ctx, feedbackURL())
}
func sendBark(endpoint, message, purchase string) {
	_ = sendBarkRequest(context.Background(), barkClient(), endpoint, "有货提醒", message, purchase)
}

// EnableSystemNotifications is invoked only by the user's settings action.
func (a *Desktop) GetSystemNotificationStatus() (bool, error) {
	return wruntime.CheckNotificationAuthorization(a.ctx)
}

func (a *Desktop) EnableSystemNotifications() (bool, error) {
	if err := wruntime.InitializeNotifications(a.ctx); err != nil {
		return false, err
	}
	return wruntime.RequestNotificationAuthorization(a.ctx)
}

// The frontend resolves automatic language from the system WebView preferences.
func (a *Desktop) SetUILanguage(language string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if language == "en" {
		a.uiLanguage = "en"
	} else {
		a.uiLanguage = "zh"
	}
}
