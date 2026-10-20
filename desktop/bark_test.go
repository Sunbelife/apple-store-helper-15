package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBarkResponseValidationAndSecretSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{"accepted", 200, `{"code":200,"message":"success"}`, false},
		{"invalid-device", 200, `{"code":400,"message":"secret-device-key"}`, true},
		{"http-failure", 404, `secret-device-key`, true},
		{"not-bark", 200, `<html>OK</html>`, true},
		{"redirect", 302, ``, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("title") != "测试" || r.URL.Query().Get("body") != "通知" {
					t.Error("missing push content")
				}
				if r.URL.Query().Has("url") {
					t.Error("test must not open shopping link")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			err := sendBarkRequest(context.Background(), client, server.URL+"/secret-device-key?url=old", "测试", "通知", "")
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-device-key") {
				t.Fatal("device key leaked")
			}
		})
	}
	a := NewDesktop()
	a.bark = "https://example.invalid/saved-secret"
	if a.GetBark() != a.bark {
		t.Fatal("cannot inspect saved address")
	}
	if a.TestBark("http://invalid/secret-device-key") == nil {
		t.Fatal("insecure address accepted")
	}
	if a.bark != "https://example.invalid/saved-secret" {
		t.Fatal("test changed saved configuration")
	}
}
