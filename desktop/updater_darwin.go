package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const updateAppName = "老孙抢你妹 2026.app"

func prepareAndLaunchUpdate(root, archive, version string) error {
	extracted := filepath.Join(root, "extracted")
	if err := os.MkdirAll(extracted, 0700); err != nil {
		return fmt.Errorf("创建更新解压目录失败：%w", err)
	}
	if output, err := exec.Command("/usr/bin/ditto", "-x", "-k", archive, extracted).CombinedOutput(); err != nil {
		return fmt.Errorf("解压更新包失败：%s", strings.TrimSpace(string(output)))
	}
	source := filepath.Join(extracted, updateAppName)
	info, err := os.Lstat(source)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("更新包中没有有效的 %s", updateAppName)
	}
	plist := filepath.Join(source, "Contents", "Info.plist")
	bundleID, err := plistValue(plist, "CFBundleIdentifier")
	if err != nil || bundleID != "com.sunbelife.qiangnimei2" {
		return fmt.Errorf("更新包 Bundle ID 校验失败")
	}
	bundleVersion, err := plistValue(plist, "CFBundleShortVersionString")
	if err != nil || bundleVersion != version {
		return fmt.Errorf("更新包版本校验失败：期望 %s，实际 %s", version, bundleVersion)
	}
	if output, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", source).CombinedOutput(); err != nil {
		return fmt.Errorf("更新包签名校验失败：%s", strings.TrimSpace(string(output)))
	}
	target, err := currentApplicationPath()
	if err != nil {
		return err
	}
	if strings.HasPrefix(target, "/Volumes/") || strings.Contains(target, "/AppTranslocation/") {
		return fmt.Errorf("请先把 App 拖入“应用程序”文件夹并重新打开，再使用 OTA 更新")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("无法定位更新日志目录：%w", err)
	}
	logDir := filepath.Join(cache, "Sunbelife", "Qiangnimei2")
	if err = os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("创建更新日志目录失败：%w", err)
	}
	logPath := filepath.Join(logDir, "update.log")
	backup := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".backup-"+version)
	replaceScript := filepath.Join(root, "replace.sh")
	launcherScript := filepath.Join(root, "launch-update.sh")
	replace := fmt.Sprintf(`#!/bin/sh
set -u
SOURCE=%s
TARGET=%s
BACKUP=%s
LOG=%s
PID=%d
exec >>"$LOG" 2>&1
echo "$(date '+%%Y-%%m-%%d %%H:%%M:%%S') installing %s"
i=0
while kill -0 "$PID" 2>/dev/null && [ "$i" -lt 200 ]; do
  sleep 0.2
  i=$((i + 1))
done
if kill -0 "$PID" 2>/dev/null; then
  echo "application did not exit"
  exit 1
fi
if [ ! -e "$TARGET" ] && [ -e "$BACKUP" ]; then
  /bin/mv "$BACKUP" "$TARGET" || exit 1
fi
if [ -e "$BACKUP" ]; then
  /bin/rm -rf "$BACKUP" || exit 1
fi
/bin/mv "$TARGET" "$BACKUP" || exit 1
if ! /usr/bin/ditto "$SOURCE" "$TARGET"; then
  /bin/rm -rf "$TARGET"
  /bin/mv "$BACKUP" "$TARGET"
  exit 1
fi
/usr/bin/xattr -cr "$TARGET" || true
if ! /usr/bin/codesign --verify --deep --strict "$TARGET"; then
  /bin/rm -rf "$TARGET"
  /bin/mv "$BACKUP" "$TARGET"
  exit 1
fi
/bin/rm -rf "$BACKUP"
echo "$(date '+%%Y-%%m-%%d %%H:%%M:%%S') installed %s"
`, shellQuote(source), shellQuote(target), shellQuote(backup), shellQuote(logPath), os.Getpid(), version, version)
	if err = os.WriteFile(replaceScript, []byte(replace), 0700); err != nil {
		return fmt.Errorf("创建更新安装脚本失败：%w", err)
	}
	adminCommand := "/bin/sh " + shellQuote(replaceScript)
	launcher := fmt.Sprintf(`#!/bin/sh
set -u
ROOT=%s
TARGET=%s
SCRIPT=%s
LOG=%s
exec >>"$LOG" 2>&1
if /bin/sh "$SCRIPT"; then
  result=0
elif /usr/bin/osascript -e %s; then
  result=0
else
  result=1
fi
if [ -e "$TARGET" ]; then
  /usr/bin/open "$TARGET"
fi
if [ "$result" -eq 0 ]; then
  /bin/rm -rf "$ROOT"
fi
exit "$result"
`, shellQuote(root), shellQuote(target), shellQuote(replaceScript), shellQuote(logPath), shellQuote("do shell script "+strconv.Quote(adminCommand)+" with administrator privileges"))
	if err = os.WriteFile(launcherScript, []byte(launcher), 0700); err != nil {
		return fmt.Errorf("创建更新启动脚本失败：%w", err)
	}
	command := exec.Command("/bin/sh", launcherScript)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = command.Start(); err != nil {
		return fmt.Errorf("启动更新安装程序失败：%w", err)
	}
	return command.Process.Release()
}

func plistValue(path, key string) (string, error) {
	output, err := exec.Command("/usr/bin/plutil", "-extract", key, "raw", "-o", "-", path).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取 Info.plist 失败：%s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func currentApplicationPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法定位当前应用：%w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("无法解析当前应用路径：%w", err)
	}
	for current := filepath.Dir(executable); current != filepath.Dir(current); current = filepath.Dir(current) {
		if filepath.Ext(current) == ".app" {
			return current, nil
		}
	}
	return "", fmt.Errorf("OTA 更新只能从已安装的 .app 中运行")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
