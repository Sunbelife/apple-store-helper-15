# 抢你妹18

Apple Store 零售店取货库存监控工具。选择产品和门店后自动查询，发现有货时响铃、弹窗并打开苹果官网购买页；支持可选的 Bark 推送。程序不下单、不占库存、不代付，提醒不保证最终能买到。

## 当前版本

桌面源码版本 **1.8.6**，使用 Go + Fyne 2.8.2。macOS 打包支持 Apple 芯片和 Intel，最低 macOS 11。源码也可用于其他桌面平台，但本仓库当前主要验证 macOS。

支持中国大陆、香港、日本、新加坡、美国、英国、澳大利亚。产品选项来自各地区产品目录，包括 iPhone Duo、iPhone 18 Pro 系列及 Apple Watch；具体可选型号、颜色以程序加载的目录为准。不同地区库存接口可能受限，不能保证所有地区持续可用。

## 下载与运行

已发布安装包见 [GitHub Releases](https://github.com/Sunbelife/apple-store-helper-15/releases)。源码版本与已发布安装包可能不同；1.8.6 当前为本地测试版，尚未发布 Release。

源码运行需 Go 1.22+、C 编译工具链和 Fyne 的平台依赖。macOS 安装 Xcode Command Line Tools 后：

```sh
git clone https://github.com/Sunbelife/apple-store-helper-15.git
cd apple-store-helper-15
go run .
```

测试、打包：

```sh
go test ./...
python3 scripts/package_macos.py --output dist
```

打包脚本需要 macOS、Xcode 工具链和 Python 3，生成通用架构 ZIP 及 SHA-256。应用使用本地临时签名，未经过 Apple 公证。

## 使用

1. 选择地区、型号、容量或尺寸、颜色或款式。
2. 选择省份、城市和门店。中国大陆可选「全部城市」「全部门店」，点击「添加到监控列表」一次加入当前筛选的门店；重复项跳过。
3. 如需手机通知，将 Bark 应用提供的推送地址填入「Bark 推送」。地址属于私人推送凭据，请勿公开。
4. 点击「开始监控」。每家门店查完立即刷新，有货时立即进入提醒流程；有货后暂停监控，由用户在官网完成购买。
5. 点击「暂停监控」停止后续查询。关闭窗口或让电脑休眠会影响持续监控，需保持程序运行和网络连接。

## 状态与查询频率

| 状态 | 含义 |
| --- | --- |
| 等待 | 尚未查询该监控项 |
| 查询中 | 正在请求门店库存 |
| 有货 / 无货 | 接口明确返回的门店取货状态 |
| 未返回库存 | 响应中没有该门店或 SKU 的有效库存字段 |
| 库存接口受限 | 接口不可用或被限制，不能判断库存 |
| 查询失败 | 请求或解析失败，不能判断库存 |
| 需官网验证 | 接口确实返回验证页面；外部浏览器会话不能自动共享给桌面应用 |

同一门店的 SKU 合并查询，门店逐个请求，间隔至少 3 秒；每轮结束后等 30 秒。受限后等 60 秒重试。间隔目前固定，没有界面设置。有货时立即提醒，不等其他门店查完。

当前门店取货接口为 `/shop/retail/pickup-message`。旧 `/shop/fulfillment-messages` 的 HTTP 541 错误页不能作为库存结果；接口失败或字段缺失均不推断为无货。库存可能在点击通知前变化。

## 数据与配置

产品和门店目录内置在 `embedded/data`；「更新数据」可刷新产品目录。桌面监控配置保存于可执行文件旁的 `user_settings.json`，升级前可备份。该文件可能含 Bark 地址，请勿提交到 Git。

有货提醒弹窗出现后，应用异步向 `https://sunbelife.com/api/iphone-stock/events` 发送公开库存事实：地区、省市、门店、型号、容量、颜色、SKU、发现时间。不发送用户身份、设备标识、Bark 地址或监控列表；上报失败不影响提醒。

管理员历史列表位于 `https://sunbelife.com/iphone-stock`，使用密码登录。保留最近 180 天、最多 100000 条记录，同一分钟同一地区、门店、SKU 去重。历史记录不代表实时库存。服务端源码见 `server/stock-log`。

## 目录

- `main.go`、`view`、`theme`：Fyne 桌面界面。
- `services`：库存请求、目录加载、提醒与配置。
- `model`、`embedded`：数据类型与内置目录。
- `scripts/package_macos.py`：macOS 通用包构建。
- `browser-extension`：早期浏览器扩展实验，未随本次桌面接口修复验证，不作为推荐备用方案。

## 老孙的店

如果抢你妹帮你抢到了心仪的 iPhone，欢迎来[老孙的店](https://store.sunbelife.com)逛逛，买点喜欢的东西支持我们。

问题反馈请到 [Issues](https://github.com/Sunbelife/apple-store-helper-15/issues)，附程序版本、地区和错误状态；请先遮盖 Bark 地址等凭据。
