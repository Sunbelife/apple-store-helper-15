# macOS 打包要求

2026-10-18 用户确认，后续版本必须遵守。

## 兼容范围

- 默认交付 Universal 通用包，同时包含 Intel `x86_64` 和 Apple Silicon `arm64`。不能仅在 Apple Silicon 上运行成功就视为 Intel 验证通过。
- 最低支持 macOS 12.0，`Info.plist` 的 `LSMinimumSystemVersion` 和两种架构的 Mach-O 最低系统版本必须一致为 12.0。
- 用户的 Intel Mac 为 macOS Tahoe 26.7.1，必须在兼容范围内。
- 编译必须显式设置 `MACOSX_DEPLOYMENT_TARGET=12.0`、`CGO_CFLAGS=-mmacosx-version-min=12.0` 和 `CGO_LDFLAGS=-mmacosx-version-min=12.0`，不能继承本机 SDK 的默认最低版本。

## 标准流程

在项目根目录运行：

```sh
python3 scripts/package_wails_macos.py --arch universal --output dist
```

脚本负责构建前端、分别编译两种架构、合并通用程序、检查 Mach-O 最低版本、生成图标、签名验证，以及生成 zip 和 SHA-256 校验文件。其最低版本检查必须保留，失败时禁止交付。

交付前解压最终 zip，针对解压后的应用再次核对：

```sh
lipo -archs '老孙抢你妹 2026.app/Contents/MacOS/qiangnimei2'
vtool -show-build '老孙抢你妹 2026.app/Contents/MacOS/qiangnimei2'
plutil -extract LSMinimumSystemVersion raw '老孙抢你妹 2026.app/Contents/Info.plist'
codesign --verify --deep --strict '老孙抢你妹 2026.app'
```

必须看到 `x86_64 arm64`（顺序不限），每种架构的 `minos` 均为 `12.0`，plist 最低版本为 `12.0`，且签名验证通过。`sdk` 可以是构建机器的新版本，不能与 `minos` 混淆。

同步更新界面版本、Wails 配置版本、打包脚本版本与递增的 `CFBundleVersion`。保持应用名称「老孙抢你妹 2026」、原店铺应用图标、Bundle ID `com.sunbelife.qiangnimei2` 及现有配置路径，避免升级丢失监控配置。

将最终通用 zip 放到下载文件夹供用户使用，并区分“完成架构／最低版本／签名检查”和“Intel 实机验证”。没有实际运行，不得宣称 Intel 已实测。

## 已发生的问题

2.0.9 及之前的包虽然包含两种架构，plist 标注 macOS 11，但实际 Mach-O 的 `minos` 为 27.0，导致 macOS 26.7.1 提示不能使用此版本应用。仅修改 plist 无法修复。

2.0.10 已显式设置部署目标，最终解压包的两种架构均验证为 `minos 12.0`，签名验证通过。用户尚未反馈 Intel 实机运行结果。
