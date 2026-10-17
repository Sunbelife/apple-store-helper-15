# 公开库存日志服务

仅保存公开 Apple 门店、产品和检测时间，不保存客户端身份、设备标识、Bark URL、监控列表、IP 或请求日志。库存事件不等于当前实时库存；客户端提交不构成官方库存证明。

- POST `/api/iphone-stock/events`：仅接受固定十个字段。请求上限 4KB，同一分钟相同地区、门店和 SKU 去重。
- GET `/iphone-stock`：密码登录页，登录后显示日志、搜索和分页。没有博客前台入口。
- GET `/iphone-stock/api/events`：需要有效管理员会话。
- POST `/iphone-stock/login`、`/iphone-stock/logout`：登录、退出。会话使用 Secure、HttpOnly、SameSite=Strict Cookie，12 小时过期。
- 保留最近 180 天、最多 100000 条记录。

运行：Python 3 标准库，`python3 server.py`，监听 `127.0.0.1:18774`。通过 HTTPS 反向代理开放上述路径，数据库默认 `/var/lib/iphone-stock/events.sqlite3`（`STOCK_DB` 可覆盖），配置默认 `/etc/iphone-stock-auth.json`（`STOCK_AUTH_CONFIG` 可覆盖）。配置包含 base64 编码的 `salt`、PBKDF2-SHA256 `password_hash`、`cookie_secret` 和 `pbkdf2_rounds`，不可提交到 Git。生产部署复用账单网关的密码校验值，使用独立会话密钥。

测试：`python3 -m unittest discover -s server/stock-log -p 'test_*.py'`。
