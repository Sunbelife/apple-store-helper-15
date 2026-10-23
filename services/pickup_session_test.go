package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"
)

type sessionRoundTrip func(*http.Request) (*http.Response, error)

func (f sessionRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func sessionResponse(r *http.Request, status int, headers http.Header) *http.Response {
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader("{}")), Request: r}
}
func TestPickupSessionKeepsAnonymousCookiesAndReusesWarmup(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	visits, queries := 0, 0
	s := &pickupSession{client: &http.Client{Jar: jar, Transport: sessionRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/shop" {
			visits++
			return sessionResponse(r, 200, http.Header{"Set-Cookie": []string{"anonymous=test-session; Secure; Path=/"}}), nil
		}
		queries++
		cookie, err := r.Cookie("anonymous")
		if err != nil || cookie.Value != "test-session" {
			t.Error("anonymous session cookie not reused")
		}
		if r.Header.Get("Referer") != "https://www.apple.com.cn/shop" {
			t.Error("missing shop referer")
		}
		return sessionResponse(r, 200, http.Header{}), nil
	})}}
	for i := 0; i < 2; i++ {
		r, err := s.do(context.Background(), "cn", "https://www.apple.com.cn/shop/retail/pickup-message?store=R484")
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
	}
	if visits != 1 || queries != 2 {
		t.Fatalf("shop=%d pickup=%d, want one shared shop session", visits, queries)
	}
}
func TestPickupSessionIsolatesOneRejectionAndEscalatesTwo(t *testing.T) {
	visits, queries := 0, 0
	s := &pickupSession{client: &http.Client{Transport: sessionRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/shop" {
			visits++
			return sessionResponse(r, 200, http.Header{}), nil
		}
		queries++
		status := 541
		if queries >= 3 {
			status = 200
		}
		return sessionResponse(r, status, http.Header{}), nil
	})}}
	base := "https://www.apple.com.cn/shop/retail/pickup-message?store="
	r, e := s.do(context.Background(), "cn", base+"A")
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if s.warmedAt.IsZero() || !s.retryAt.IsZero() {
		t.Fatal("one rejection must not invalidate the entire region")
	}
	if _, e = s.do(context.Background(), "cn", base+"A"); !errors.Is(e, errAppleStockLimited) || queries != 1 || visits != 1 {
		t.Fatal("same query must cool without additional HTTP")
	}
	r, e = s.do(context.Background(), "cn", base+"B")
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if !time.Now().Before(s.retryAt) || !s.warmedAt.IsZero() {
		t.Fatal("two distinct failures must cool and renew the session")
	}
	if _, e = s.do(context.Background(), "cn", base+"C"); !errors.Is(e, errAppleStockLimited) || queries != 2 {
		t.Fatal("region cooldown did not block network")
	}
	s.retryAt = time.Now().Add(-time.Second)
	s.rejected = nil
	r, e = s.do(context.Background(), "cn", base+"C")
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if visits != 2 || queries != 3 {
		t.Fatalf("expected session recovery after cooldown, visits=%d queries=%d", visits, queries)
	}
}
func TestPickupWarmupRestrictionPreservesBackoffClassification(t *testing.T) {
	for _, status := range []int{403, 429, 503, 541} {
		s := &pickupSession{client: &http.Client{Transport: sessionRoundTrip(func(r *http.Request) (*http.Response, error) { return sessionResponse(r, status, http.Header{}), nil })}}
		_, err := s.do(context.Background(), "cn", "https://www.apple.com.cn/shop/retail/pickup-message")
		if !errors.Is(err, errAppleStockLimited) {
			t.Fatalf("HTTP %d lost limited status: %v", status, err)
		}
	}
}
func TestAppleSessionNeverAppliesToOtherHosts(t *testing.T) {
	for _, u := range []string{"https://www.apple.com.cn.evil.example/shop/retail/pickup-message", "http://www.apple.com.cn/shop/retail/pickup-message", "https://example.com/shop/retail/pickup-message"} {
		if isApplePickupURL("cn", u) {
			t.Fatal("foreign URL accepted")
		}
	}
	if !isApplePickupURL("cn", "https://www.apple.com.cn/shop/retail/pickup-message?store=R484") {
		t.Fatal("official endpoint rejected")
	}
}

func TestPickupSessionHonorsRetryAfter(t *testing.T) {
	s := &pickupSession{}
	before := time.Now()
	s.coolDown(429, http.Header{"Retry-After": []string{"120"}})
	if s.retryAt.Before(before.Add(120*time.Second)) || s.retryAt.After(before.Add(121*time.Second)) {
		t.Fatalf("retry-after ignored: %v", s.retryAt)
	}
}
