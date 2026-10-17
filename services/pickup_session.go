package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"
)

// Apple gives anonymous shop visitors routing/session cookies. Keep those cookies
// in memory and reuse the same transport instead of issuing stateless requests.
// No browser profile, Apple ID, or authenticated cookies are read.
type pickupSession struct {
	mu       sync.Mutex
	client   *http.Client
	warmedAt time.Time
	referer  string
}

var pickupSessions = struct {
	sync.Mutex
	byArea map[string]*pickupSession
}{byArea: map[string]*pickupSession{}}

const pickupUserAgent = "AppleStoreHelper/2.0 (+https://github.com/Sunbelife/apple-store-helper-15)"

func sessionForArea(area string) *pickupSession {
	pickupSessions.Lock()
	defer pickupSessions.Unlock()
	if s := pickupSessions.byArea[area]; s != nil {
		return s
	}
	jar, _ := cookiejar.New(nil)
	s := &pickupSession{client: &http.Client{Timeout: 15 * time.Second, Jar: jar}}
	pickupSessions.byArea[area] = s
	return s
}

func (s *pickupSession) do(ctx context.Context, area, link string) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	base, err := appleStoreBaseURL(area)
	if err != nil {
		return nil, err
	}
	if s.warmedAt.IsZero() || time.Since(s.warmedAt) > 20*time.Minute {
		// A normal storefront visit creates the anonymous session used by pickup.
		storefront := base + "/shop"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, storefront, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", pickupUserAgent)
		req.Header.Set("Accept-Language", getAcceptLanguage(area))
		response, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("建立 Apple 商店会话失败：%w", err)
		}
		_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if response.StatusCode != http.StatusOK {
			cause := classifyStockHTTPResponse(response.StatusCode, nil)
			if response.StatusCode == 541 {
				cause = errAppleStockLimited
			}
			if cause != nil {
				return nil, fmt.Errorf("建立 Apple 商店会话 HTTP %d：%w", response.StatusCode, cause)
			}
			return nil, fmt.Errorf("建立 Apple 商店会话：HTTP %d", response.StatusCode)
		}
		s.referer = response.Request.URL.String()
		s.warmedAt = time.Now()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", pickupUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", getAcceptLanguage(area))
	req.Header.Set("Referer", s.referer)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	response, err := s.client.Do(req)
	if err == nil && (response.StatusCode == 541 || response.StatusCode == http.StatusForbidden) {
		s.warmedAt = time.Time{}
	}
	return response, err
}

func isApplePickupURL(area, link string) bool {
	base, err := appleStoreBaseURL(area)
	if err != nil {
		return false
	}
	expected, _ := url.Parse(base)
	actual, err := url.Parse(link)
	return err == nil && actual.Scheme == "https" && actual.Host == expected.Host && actual.Path == expected.Path+"/shop/retail/pickup-message"
}
