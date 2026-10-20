package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"time"
)

var usageIDPattern = regexp.MustCompile(`^[0-9a-f-]{36}$`)

// Only called after the frontend finds a previously accepted consent or the user clicks Accept.
func (a *Desktop) SetUsageIdentity(id string) error {
	a.usageMu.Lock()
	defer a.usageMu.Unlock()
	if !usageIDPattern.MatchString(id) {
		return fmt.Errorf("invalid analytics identifier")
	}
	a.mu.Lock()
	a.usageIdentity = id
	a.mu.Unlock()
	return nil
}
func (a *Desktop) TrackUsage(event string) error {
	a.usageMu.Lock()
	defer a.usageMu.Unlock()
	a.mu.Lock()
	id := a.usageIdentity
	a.mu.Unlock()
	if id == "" {
		return nil
	}
	switch event {
	case "app_open", "page_monitor", "page_stats", "monitor_start", "stock_report":
	default:
		return fmt.Errorf("invalid analytics event")
	}
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return e
	}
	s := hex.EncodeToString(b)
	eid := s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
	return usageRequest("events", map[string]string{"installation": id, "event_id": eid, "event": event, "version": AppVersion, "platform": runtime.GOOS})
}
func usageRequest(path string, payload map[string]string) error {
	raw, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Post("https://sunbelife.com/api/iphone-stock/usage/"+path, "application/json", bytes.NewReader(raw))
	if e != nil {
		return fmt.Errorf("统计服务暂不可用")
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("统计服务暂不可用")
	}
	return nil
}
func (a *Desktop) WithdrawUsage(id string) error {
	a.usageMu.Lock()
	defer a.usageMu.Unlock()
	if !usageIDPattern.MatchString(id) {
		return fmt.Errorf("invalid analytics identifier")
	}
	a.mu.Lock()
	a.usageIdentity = ""
	a.mu.Unlock()
	return usageRequest("erase", map[string]string{"installation": id})
}
