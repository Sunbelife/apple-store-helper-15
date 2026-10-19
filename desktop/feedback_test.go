package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestFeedbackContextAndRedaction(t *testing.T) {
	u, e := url.Parse(feedbackURL())
	if e != nil {
		t.Fatal(e)
	}
	if u.Scheme != "https" || u.Host != "sunbelife.com" || u.Path != "/support/feedback" || u.Query().Get("project") != feedbackProject || u.Query().Get("app_version") != AppVersion {
		t.Fatal("invalid feedback destination")
	}
	state := State{Phase: "已暂停", Logs: []Entry{{Time: "12:00", Message: `token=secret123 password=hunter api_key=private Bearer abcxyz https://api.day.app/hiddenkey/title /Users/testuser/Library C:\Users\testuser\AppData`}}}
	text := feedbackLog(state)
	for _, secret := range []string{"secret123", "hunter", "private", "abcxyz", "hiddenkey", "testuser"} {
		if strings.Contains(text, secret) {
			t.Errorf("log leaks %q", secret)
		}
	}
	if !strings.Contains(text, "version="+AppVersion) || !strings.Contains(text, "[REDACTED]") {
		t.Fatal("missing diagnostic context")
	}
}
