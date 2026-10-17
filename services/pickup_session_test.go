package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
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
func TestPickupSessionRefreshesAfterRejection(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	visits, queries := 0, 0
	s := &pickupSession{client: &http.Client{Jar: jar, Transport: sessionRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/shop" {
			visits++
			return sessionResponse(r, 200, http.Header{}), nil
		}
		queries++
		code := 200
		if queries == 1 {
			code = 541
		}
		return sessionResponse(r, code, http.Header{}), nil
	})}}
	r, err := s.do(context.Background(), "cn", "https://www.apple.com.cn/shop/retail/pickup-message")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if !s.warmedAt.IsZero() {
		t.Fatal("rejected session must be renewed next round")
	}
	r, err = s.do(context.Background(), "cn", "https://www.apple.com.cn/shop/retail/pickup-message")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if visits != 2 {
		t.Fatalf("warmup visits=%d, want 2", visits)
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
