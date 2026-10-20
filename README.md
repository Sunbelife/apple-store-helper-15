# 老孙抢你妹 2026

Apple Store 门店取货库存监控。**正式版 v2.0.37 使用 Wails 2 + React + Go。** 旧版 Fyne 使用说明见 [1.x 使用说明](docs/fyne-1.x.md)。最新安装包见 [GitHub Releases](https://github.com/Sunbelife/apple-store-helper-15/releases/latest)。

## 2.0 做了什么

- 使用系统 WebView 渲染 React 界面，Go 负责实际库存查询，未嵌入整套 Chromium。
- 地区、型号、容量、颜色、省市和门店筛选；「全部门店」一键加入，重复项跳过。
- 库存按行显示状态、更新时间和错误原因，带搜索、单项移除和运行日志。
- 查询中和下一轮倒计时可见；暂停可取消正在进行的请求。
- 启动时自动检查 GitHub Releases；也可点击窗口右上角手动检查，下载后校验 SHA-256、Bundle ID、版本号和应用签名，再原位更新并重启。
- 有货时自动打开对应地区的苹果购物袋，弹窗、持续提示音，可选 Bark 手机推送，然后暂停监控。提醒弹窗另提供商品页入口。
- 配置保存在系统应用配置目录，升级时保留当前监控和提醒配置。
- 使用老孙的店当前图标、灰白底和橙色主题，底部展示九款店铺商品，土豆片、土豆子 2 号机和科技织物壳优先展示。
- 商品使用后台生成的专属分销渠道链接，微信扫码进入对应商品的小程序页面；Mac 浏览器无法直接打开小程序时，可复制链接或手机扫码。
- 保留公开库存上报。受限时显示「触发风控，冷却中，稍后再尝试」和自动重试倒计时。

程序只发现库存并提醒，不下单、不锁定库存、不代付。有货提醒不能保证最终购买成功。电脑需要保持运行、联网；休眠后无法持续监控。

开始监控前，请先在默认浏览器的苹果官网把目标型号、容量和颜色加入购物袋。发现有货后进入同一浏览器购物袋，手动选择提醒中的取货门店并结算。程序不会自动加购或选择门店；购物袋为空时可使用提醒中的商品页入口。

## 使用说明

每次启动先显示加载提示，从苹果官网增量更新产品和门店信息后进入主界面。同编号条目更新、新编号追加，不删除已有条目；更新失败时继续使用本地目录。手动「更新目录」入口保留，更新不会修改已有监控目标。遇到无法使用、型号缺失或需要版本更新，可点击 App 顶部「问题反馈」，前往独立问题反馈页，按选项和输入框提交问题，并在同一浏览器的「我的反馈」查看回复；建议说明版本、Mac 芯片、macOS 版本、问题截图和发生时间，不要附带 Bark Key 或登录资料。监控列表底部可导出最近运行日志，上传前先检查反馈页的脱敏预览。

App 右上角「使用说明」可随时查看操作步骤。[老孙公众号原文](https://mp.weixin.qq.com/s/PCVWj6xmK13lBM_BbBokiA)作为阅读入口保留；以下说明对应当前版本实际行为和浏览器实测。

### 1. 先准备购物袋

在系统默认浏览器打开对应地区的苹果官网，选好目标产品的型号、容量、颜色和购买数量，加入购物袋，提前确认账号能正常登录。后续在同一浏览器、同一用户配置的普通窗口中结账。程序不读取你的浏览器登录信息，也不会帮你自动加购。

### 2. 添加门店并开始监控

在 App 左侧选择地区、型号、容量、颜色、省市和零售店，点击「添加到监控列表」。「全部城市」「全部门店」可批量加入当前筛选范围的门店，重复目标会跳过。核对列表里的产品与购物袋一致，再点击「开始监控」。修改配置时先暂停。

保持电脑运行、联网并避免休眠。可以在「关注的门店」后的闹钟按钮中测试提示音、开启系统通知，并按需配置 Bark。当前版本尚未接入微信绑定和微信到货通知。

### 3. 看懂库存状态

| 状态 | 含义与操作 |
| --- | --- |
| ⏳ 等待／🔎 查询中 | 尚在等待或正在查询，不代表无货。 |
| ⚪️ 暂无库存 | 该次检查明确无货；结合第一列「更新时间」判断结果是否新鲜。 |
| 🟢 有货 | 已发现门店库存，会提醒并暂停监控；以官网结账时的库存为准。 |
| 🧊 冷却中 | 该门店触发风控后单独冷却，其他门店继续监控。连续受限按 1、2、4、5 分钟冷却，成功后恢复正常频率。 |
| ⚠️ 查询失败／未返回库存 | 没有取得有效结果，查看该行原因和运行日志，不能当作无货。 |

不要通过频繁重启催促查询。单个门店失败时，其他门店可能保留上次结果，必须结合更新时间查看。

### 4. 有货后如何下单

1. 留意弹窗中的产品和门店。App 会响铃、打开对应地区的苹果购物袋，并暂停监控。
2. 在浏览器点击「结账」，按苹果提示登录或选择游客结账。
3. 在「你希望如何收到订单商品？」页面选择「我要取货」。购物袋链接不会自动切换取货方式。
4. 选择提醒中的取货门店与可用时段，按官网要求填写取货资料，核对型号、颜色、容量、数量和金额后自行付款。
5. 若购物袋为空，检查浏览器和用户配置，或使用提醒弹窗里的商品页入口重新选购。官网已无货时可关闭提醒并停止响铃，再手动开始监控；关闭弹窗不会自动恢复监控。

我们实测：在同一浏览器先选好「我要取货」，重开当前结账地址能保留选项；但在没有原购物袋会话的另一浏览器打开同一地址会报错，且会话会过期。因此当前仍采用购物袋入口，不依赖直达结账链接。提醒不锁定库存，也不保证最终买到。

## 本地运行

macOS：Go **1.25+**、Node.js **22+** 和 Xcode Command Line Tools。当前依赖 Wails **2.16.0**。

2.0.15 通用安装包包含 Intel 和 Apple Silicon，最低系统版本为 macOS 12。修正了旧包意外继承本机 SDK 的 macOS 27 最低运行版本问题。Intel 与旧系统仍需实机验证。

## GitHub OTA 更新

应用通过 `https://api.github.com/repos/Sunbelife/apple-store-helper-15/releases/latest` 检查正式 Release。Release 标签使用 `vMAJOR.MINOR.PATCH`，并且必须同时上传以下两个同名资产：

- `laosun-qiangnimei-2026-vMAJOR.MINOR.PATCH-macos-universal.zip`
- `laosun-qiangnimei-2026-vMAJOR.MINOR.PATCH-macos-universal.zip.sha256`

版本号由 `desktop/version.go` 统一提供，打包脚本会同步写入应用的 `Info.plist` 和资产名称。应用只接受 GitHub HTTPS 下载地址；下载后先校验 SHA-256，再校验 Bundle ID、版本号和 `codesign`。安装时先保留旧 App，替换或签名验证失败会自动回滚。App 位于受保护的“应用程序”目录时，macOS 会请求管理员授权。当前包仍采用本地签名、未经 Apple 公证。

2.0.15 是 OTA 引导版本：已有的 2.0.14 及更早版本尚无更新器，需要手动安装 2.0.15 一次；从之后的 2.0.16 起即可在 App 内完成更新。

后续打包必须遵守 [macOS 打包要求](workdoc/macOS打包要求.md)，交付前核对两种架构的 Mach-O 最低版本与签名。

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

使用 `/shop/retail/pickup-message`。程序先建立匿名 Apple 商店会话，在内存中保留商店发放的 Cookie 并复用连接；遇到 541 / 403 后，下一次退避重试会重新建立会话。不读取浏览器资料、Apple ID 或登录 Cookie，也不把会话 Cookie 写入配置或日志。中国大陆优先按门店邮编查询附近门店，一次响应覆盖的监控项直接复用，避免重复请求。剩余门店请求至少间隔 3 秒；每轮结束后随机等待 20～60 秒。

苹果可能间歇性返回 541 错误页、403、429 或 503。受限门店按 1、2、4、5 分钟单独退避，其他门店每轮随机等待 20～60 秒；门店成功返回后恢复正常频率。单个门店失败不会覆盖或暂停其他门店，需结合每行更新时间判断新鲜度。响应缺失门店、SKU 或有效状态时显示「未返回库存」，不能当作「无货」。

苹果可以随时改变接口行为。桌面版支持多个地区，但不保证所有地区持续可查询；本次按市场和产品系列抽查真实门店与当地 SKU，详见验证报告。

## 固定商品推荐

商品快照和公开分销链接位于 `desktop/frontend/src/shop-products.json`，图片随应用打包，不在运行时读取店铺管理 API。快照于 2026-10-19 补充土豆片、土豆子 2 号机和科技织物壳，共九款商品，价格和库存以小程序为准。渠道为 `qiangnimei_2026_<商品ID>`；管理员凭据不打入应用。品牌图标来自店铺公开 layout 配置，构建图标位于 `desktop/branding/store-logo.png`。

## 配置与数据

macOS 配置位于 `~/Library/Application Support/Sunbelife/Qiangnimei2/settings.json`，用文件权限 0600 保存，可能包含 Bark 凭据。不要分享或提交该文件。刷新后的产品目录保存在同目录的 `catalogs` 下；首次运行回退到内置目录。

v2.0.37 起移除旧版配置导入入口。每次启动都会清除旧查询状态，恢复为等待；库存状态不会被当作实时数据永久保存。

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


### 库存统计

App 顶部「库存统计」向所有用户展示博客已收到的库存检测汇总，支持北京时间日期、地区、型号、颜色、容量筛选，当日累计／最近一小时、小时分布、机型／颜色／容量汇总及城市排行；各城市显示对应机型、门店、记录数和最近时间，点击城市可筛选。每 5 分钟刷新，也可手动刷新。未检测到的城市、未上报的设备均不在统计范围内；检测记录数不等于库存数量或放货批次，历史记录不代表实时有货。

公开只读接口为 `/iphone-stock/api/stats`；原始事件列表 `/iphone-stock/api/events` 仍需管理员登录。统计接口不返回逐条事件或管理员凭据。


### 官网下载与 Windows 测试版

介绍及双平台下载：https://sunbelife.com/projects/qiangnimei/ 。Windows x64 免安装包通过 `python3 scripts/package_wails_windows.py --output dist` 生成，包含原图标、GUI manifest、PE版本信息、使用说明及SHA-256；运行需要Windows 10/11与WebView2。Windows更新按钮打开官网下载页，不使用macOS安装更新流程。首版未经Windows实机验证，发布为测试版。GitHub Actions已增加Windows打包产物。

## 全球市场与产品（v2.0.34）

当前内置 26 个 Apple 官方在线商店及直营店市场：中国大陆、中国香港、中国台湾、日本、韩国、新加坡、马来西亚、泰国、印度、美国、加拿大、墨西哥、巴西、英国、德国、法国、意大利、西班牙、瑞士、奥地利、比利时、荷兰、瑞典、土耳其、阿联酋、澳大利亚。

其中 25 个市场开放取货监控入口；巴西官方商品页明确 `pickupEnabled: false`，仅浏览目录并禁用添加监控。

“全球”指覆盖官方商店可用的直营店取货查询，不包含经销商库存，也不代表所有国家都有 Apple 直营店。澳门入口未验证可用，暂不列入。瑞士使用 `/ch-fr`、比利时使用 `/be-fr` 的当地商店，购物袋与商品链接保持市场一致。

除现有 iPhone 和 Apple Watch，新增 MacBook Air、MacBook Pro、Mac mini、iMac 和 Mac Studio。Mac 只收录官网提供独立零件号的标准配置，区分芯片、尺寸、内存、硬盘和 CPU/GPU 核数；不把 CTO 定制容器当作库存 SKU。当地未销售或没有可验证标准配置的产品不跨区补齐。中国大陆目录没有 Apple TV 4K。Apple TV 已在 v2.0.35 中移除，暂不提供监控。

目录按 SKU 和门店 ID 增量合并，保留已有监控与本地数据。已验证的固定 Mac SKU 配置可复用；新 SKU 才需请求商品详情。

### 界面语言

默认跟随系统 WebView 的首选语言：中文系统显示中文，其他语言显示英文。在「设置 → 语言」可切换「跟随系统／简体中文／English」，选择会保存在本机。界面、提醒配置、使用说明、统计与产品颜色支持英文；苹果提供的当地城市、门店专名和原始诊断信息保留来源写法。语言切换不修改监控标识、SKU 或购物链接。

### English quick start

1. Add the exact configuration to your bag on your regional Apple Store using your default browser.
2. Choose a region, product and stores, then select **Add to monitors** and **Start monitoring**.
3. Keep the computer awake and online. Errors and cooldown do not mean sold out.
4. When stock is found, monitoring pauses and the regional shopping bag opens. Select pickup and complete checkout yourself. Alerts do not reserve stock.
5. Use **Settings → Language** to follow the system language or choose Chinese/English.

Tests sample real retail stores and regional SKUs; they do not guarantee every configuration or continuous API availability. See `workdoc/global-regions-live.json` and `workdoc/global-products-pickup-live.json` for the exact observed results.

## v2.0.34 更新

- 扩展地区目录和标准配置 Mac 产品，新增中英文界面及系统语言跟随。
- 推送配置支持显示／隐藏、查看已保存地址及独立测试推送。
- 优化库存统计、地区筛选和使用说明，接入问题反馈与日志导出。
- 移除辅助下单实验，保持默认浏览器购物袋流程。
- macOS 通用包支持 Intel 与 Apple Silicon，最低 macOS 12；Windows x64 包仍未实机验证。
- 541 风控、暂未开放取货、未返回目标门店库存均不等同于无货。Apple TV 已修复预约取货日期识别，香港已实测；其他市场及新增市场的博客统计上报尚未全部联调。

## v2.0.35 更新

暂时移除 Apple TV 产品选择、内置目录及自动目录抓取。已有本地缓存中的 Apple TV 也不会重新出现在产品列表。Mac、iPhone 和 Apple Watch 保留。

## v2.0.36 匿名使用统计

「库存统计」页面增加匿名统计说明与「同意参与匿名统计」按钮，没有设置开关。用户主动参与后才生成随机安装标识，上报启动、监控页／统计页访问、成功开始监控及成功库存上报请求，并附 App 版本与系统类型。支持「退出并删除我的统计」；删除失败时停止采集并允许重试。

UV 为所选北京时间日期内发生统计事件的参与安装数，不是全部用户人数。PV 只统计监控页与库存统计页打开次数，不包含自动刷新或筛选。监控 UV 和贡献 UV 分别按成功开始监控、成功库存上报的安装标识去重。成功上报请求次数不等于库存记录数。统计不回填历史用户，也不保证网络丢失时完整计数。

服务端仅存随机标识的摘要，不保存 IP、硬件序列号、Bark 配置或日志；使用事件保留约 30 天。公开接口仅返回汇总。详见 [统计实现及验证](workdoc/匿名使用统计.md)。

## v2.0.37 更新

移除旧版 1.x 配置导入入口和导入接口，设置面板保留语言选项与购买提示。现有监控、提醒配置和匿名统计参与状态不变。
