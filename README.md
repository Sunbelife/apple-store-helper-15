# 老孙抢你妹 2026

Apple Store 门店取货库存监控。**2.0 使用 Wails 2 + React + Go，目前在独立分支本地测试。** 稳定的 Fyne 版保留在 `master`，当前版本 1.8.8，见 [1.x 使用说明](docs/fyne-1.x.md)。

## 2.0 做了什么

- 使用系统 WebView 渲染 React 界面，Go 负责实际库存查询，未嵌入整套 Chromium。
- 地区、型号、容量、颜色、省市和门店筛选；「全部门店」一键加入，重复项跳过。
- 库存按行显示状态、更新时间和错误原因，带搜索、单项移除和运行日志。
- 查询中和下一轮倒计时可见；暂停可取消正在进行的请求。
- 有货时自动打开所选 SKU 的官网购买页，弹窗、持续提示音，可选 Bark 手机推送，然后暂停监控。
- 配置保存在系统应用配置目录，支持通过设置页导入 1.x 的 `user_settings.json`。
- 使用老孙的店当前图标、灰白底和橙色主题，底部固定展示六件最近在售商品。
- 商品使用后台生成的专属分销渠道链接，微信扫码进入对应商品的小程序页面；Mac 浏览器无法直接打开小程序时，可复制链接或手机扫码。
- 保留公开库存上报。受限时显示「触发风控，冷却中，稍后再尝试」和自动重试倒计时。

程序只发现库存并提醒，不下单、不锁定库存、不代付。有货提醒不能保证最终购买成功。电脑需要保持运行、联网；休眠后无法持续监控。

## 本地运行

macOS：Go **1.25+**、Node.js **22+** 和 Xcode Command Line Tools。当前依赖 Wails **2.16.0**。

```sh
cd desktop/frontend
npm ci
npm run build
cd ..
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 dev
```

推荐在仓库根目录生成通用 ZIP（临时目录签名）：

```sh
python3 scripts/package_wails_macos.py --output dist
```

也可用 Wails CLI 构建本机或通用架构应用：

```sh
cd desktop
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build -platform darwin/universal
```

输出在 `desktop/build/bin`。本地应用使用临时签名，尚未经过 Apple 公证。本分支不代表已发布的 Release。

```sh
# 在仓库根目录，先生成 frontend/dist 再测试
npm --prefix desktop/frontend ci
npm --prefix desktop/frontend run build
go test ./...
go test -race ./desktop
# 真实生产监控循环的 11 分钟稳定性检查（不缩短查询间隔）
QNM_LIVE_SOAK=1 QNM_SOAK_REPORT=/tmp/qiangnimei-soak.jsonl go test ./desktop -run TestLiveDesktopSoak -v -count=1 -timeout 13m
# 可选的真实苹果接口检查，会发送一次公网请求
APPLE_STOCK_LIVE=1 go test ./services -run TestLivePostalPickupGuangdong -v -count=1
```

## 查询与状态

使用 `/shop/retail/pickup-message`。程序先建立匿名 Apple 商店会话，在内存中保留商店发放的 Cookie 并复用连接；遇到 541 / 403 后，下一次退避重试会重新建立会话。不读取浏览器资料、Apple ID 或登录 Cookie，也不把会话 Cookie 写入配置或日志。中国大陆优先按门店邮编查询附近门店，一次响应覆盖的监控项直接复用，避免重复请求。剩余门店请求至少间隔 3 秒；一轮结束后等 30 秒。

苹果可能间歇性返回 541 错误页、403、429 或 503。受限时立即结束当前轮次，按 1、2、4、5 分钟退避，成功后恢复正常频率。单个门店失败不会覆盖其他门店的有效结果，需结合每行更新时间判断新鲜度。响应缺失门店、SKU 或有效状态时显示「未返回库存」，不能当作「无货」。

苹果可以随时改变接口行为。桌面版支持多个地区，但不保证所有地区持续可查询；本次主要实测中国大陆门店。

## 固定商品推荐

商品快照和公开分销链接位于 `desktop/frontend/src/shop-products.json`，图片随应用打包，不在运行时读取店铺管理 API。快照取自 2026-10-17 最近六件在售商品，价格和库存以小程序为准。渠道为 `qiangnimei_2026_<商品ID>`；管理员凭据不打入应用。品牌图标来自店铺公开 layout 配置，构建图标位于 `desktop/branding/store-logo.png`。

## 配置与数据

macOS 配置位于 `~/Library/Application Support/Sunbelife/Qiangnimei2/settings.json`，用文件权限 0600 保存，可能包含 Bark 凭据。不要分享或提交该文件。刷新后的产品目录保存在同目录的 `catalogs` 下；首次运行回退到内置目录。

旧版配置可在设置页选择「导入旧版配置」。导入不会修改旧版文件。每次启动都会清除旧查询状态，恢复为等待；库存状态不会被当作实时数据永久保存。

有货弹窗显示后，应用异步向 `https://sunbelife.com/api/iphone-stock/events` 提交公开库存事实：地区、省市、门店、型号、容量、颜色、SKU、发现时间。不发送用户身份、设备标识、Bark 地址或监控列表。管理员历史列表位于 `https://sunbelife.com/iphone-stock`；历史报告不代表实时库存。

验证结果和当前限制见 [本地验证记录](docs/verification.md)。微信小程序可行性见 [方案说明](docs/wechat-feasibility.md)。

## 代码结构

- `desktop/app.go`：线程安全的监控状态、配置与 Wails 方法。
- `desktop/frontend`：React + Vite 界面。
- `services/desktop_api.go`：与 Fyne 共用的库存、颜色和报告接口。
- `services/listen.go`：旧版监控和共享库存解析。
- `embedded/data`：内置产品、门店目录。
- `main.go`：保留的 Fyne 入口；新界面不依赖其窗口。

当前迁移仍复用 `services` 包，因此 Go 依赖中暂时保留 Fyne；2.0 实际窗口由 Wails 创建。

## 老孙的店

如果帮你抢到了心仪的 iPhone，欢迎来[老孙的店](https://store.sunbelife.com)逛逛，买点喜欢的东西支持我们。
