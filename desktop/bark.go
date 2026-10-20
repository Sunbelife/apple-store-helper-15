package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Read the secret only when the user opens reminder settings, never in polling state.
func (a *Desktop) GetBark() string { a.mu.Lock(); defer a.mu.Unlock(); return a.bark }

// Test the text currently in the input without changing the saved configuration.
func (a *Desktop) TestBark(value string) error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	title, body := "老孙抢你妹 · 测试推送", "这是一条测试通知。收到后说明当前 Bark 地址可以接收推送。"
	a.mu.Lock()
	english := a.uiLanguage == "en"
	a.mu.Unlock()
	if english {
		title = "Sunbelife · Test notification"
		body = "This is a test notification. If you received it, your Bark address is working."
	}
	return sendBarkRequest(ctx, barkClient(), strings.TrimSpace(value), title, body, "")
}
func barkClient() *http.Client {
	return &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func sendBarkRequest(ctx context.Context, client *http.Client, endpoint, title, body, purchase string) error {
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("请输入 Bark 提供的 HTTPS 推送地址")
	}
	q := u.Query()
	q.Set("title", title)
	q.Set("body", body)
	if purchase != "" {
		q.Set("url", purchase)
	} else {
		q.Del("url")
	}
	u.RawQuery = q.Encode()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return fmt.Errorf("推送地址格式不正确")
	}
	resp, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("测试推送连接失败或超时，请检查网络和 Bark 地址")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bark 服务返回 HTTP %d，请检查地址和设备 Key", resp.StatusCode)
	}
	var result struct {
		Code int `json:"code"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result) != nil || result.Code != 200 {
		return fmt.Errorf("Bark 未确认推送成功，请检查设备 Key 和服务地址")
	}
	return nil
}
