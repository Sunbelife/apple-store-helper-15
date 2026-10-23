package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
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
	retryAt  time.Time
	backoff  time.Duration
	rejected map[string]time.Time
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
	if time.Now().Before(s.retryAt) {
		return nil, &PickupCooldown{Until: s.retryAt, Region: true}
	}
	if until := s.rejected[link]; time.Now().Before(until) {
		return nil, &PickupCooldown{Until: until}
	}
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
				s.coolDown(response.StatusCode, response.Header)
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
	if err == nil {
		if response.StatusCode == 541 {
			if s.rejected == nil {
				s.rejected = map[string]time.Time{}
			}
			now := time.Now()
			for key, until := range s.rejected {
				if !now.Before(until) {
					delete(s.rejected, key)
				}
			}
			s.rejected[link] = now.Add(time.Minute)
			// One endpoint rejection need not affect other stores or products.
			// Two distinct rejected queries indicate a broader session problem.
			if len(s.rejected) >= 2 {
				s.coolDown(541, response.Header)
				s.warmedAt = time.Time{}
			}
		} else if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
			s.coolDown(response.StatusCode, response.Header)
			s.warmedAt = time.Time{}
		} else if response.StatusCode == http.StatusOK {
			s.retryAt = time.Time{}
			s.backoff = 0
			delete(s.rejected, link)
		}
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

// Called with mu held. Honor bounded server Retry-After without immediate retries.
func (s *pickupSession) coolDown(status int, headers http.Header) {
	s.backoff = nextStockRetryBackoff(s.backoff, true)
	delay := s.backoff
	value := headers.Get("Retry-After")
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		if seconds > 900 {
			seconds = 900
		}
		if d := time.Duration(seconds) * time.Second; d > delay {
			delay = d
		}
	} else if until, err := http.ParseTime(value); err == nil {
		if d := time.Until(until); d > delay {
			if d > 15*time.Minute {
				d = 15 * time.Minute
			}
			delay = d
		}
	}
	s.retryAt = time.Now().Add(delay)
}

func PickupRegionCooling(area string) bool {
	s := sessionForArea(area)
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Now().Before(s.retryAt)
}

// PickupCooldown means no network request was sent; callers must not increase backoff.
type PickupCooldown struct {
	Until  time.Time
	Region bool
}

func (e *PickupCooldown) Error() string {
	scope := "该查询"
	if e.Region {
		scope = "同地区 Apple 会话"
	}
	return fmt.Sprintf("%s冷却至 %s，尚未重新请求 Apple", scope, e.Until.Format("15:04:05"))
}
func (e *PickupCooldown) Unwrap() error { return errAppleStockLimited }
