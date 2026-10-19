package main

import (
	"fmt"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const feedbackProject = "prj_7e2b5a58b457e65d11f6fd01"

func feedbackURL() string {
	u, _ := url.Parse("https://sunbelife.com/support/feedback")
	q := u.Query()
	q.Set("project", feedbackProject)
	q.Set("app_version", AppVersion)
	q.Set("platform", runtime.GOOS+"-"+runtime.GOARCH)
	q.Set("source", "qiangnimei-desktop-test")
	u.RawQuery = q.Encode()
	return u.String()
}

var feedbackSecrets = regexp.MustCompile(`(?i)((?:api[_-]?key|token|password|authorization|bark(?:[_-]?url)?)\s*[:=]\s*)[^\s,;]+`)
var feedbackBearer = regexp.MustCompile(`(?i)Bearer\s+[^\s,;]+`)
var feedbackBark = regexp.MustCompile(`https?://(?:api\.day\.app|bark[^/\s]*)/[^\s]+`)
var feedbackUserPath = regexp.MustCompile(`(?i)(/Users/|/home/|[A-Z]:\\Users\\)[^/\\\s]+`)

func feedbackLog(state State) string {
	var b strings.Builder
	fmt.Fprintf(&b, "老孙抢你妹 · 诊断日志\nversion=%s\nplatform=%s-%s\nexported_at=%s\nphase=%s\nrunning=%t\nmonitor_count=%d\n\n", AppVersion, runtime.GOOS, runtime.GOARCH, time.Now().Format(time.RFC3339), state.Phase, state.Running, len(state.Items))
	for _, entry := range state.Logs {
		fmt.Fprintf(&b, "%s %s\n", entry.Time, entry.Message)
	}
	text := feedbackBearer.ReplaceAllString(b.String(), "Bearer [REDACTED]")
	text = feedbackSecrets.ReplaceAllString(text, "${1}[REDACTED]")
	text = feedbackBark.ReplaceAllString(text, "[BARK URL REDACTED]")
	return feedbackUserPath.ReplaceAllString(text, "${1}[USER]")
}

// Export uses an explicit save dialog. Nothing is uploaded by the desktop app.
func (a *Desktop) ExportFeedbackLog() (bool, error) {
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title: "导出反馈日志", DefaultFilename: "qiangnimei-" + time.Now().Format("20060102-150405") + ".log",
		Filters: []wruntime.FileFilter{{DisplayName: "运行日志", Pattern: "*.log"}},
	})
	if err != nil || path == "" {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return false, err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return false, err
	}
	_, err = f.WriteString(feedbackLog(a.GetState()))
	closeErr := f.Close()
	if err != nil {
		return false, err
	}
	return closeErr == nil, closeErr
}
