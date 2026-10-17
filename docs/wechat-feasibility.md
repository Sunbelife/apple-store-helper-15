# 微信小程序库存提醒方案

## 可实现的流程

用户选择型号、颜色和门店，主动订阅到货通知 → 后端集中轮询公开库存 → 发现明确有货 → 给拥有有效订阅次数的用户发送订阅消息 → 用户点击消息进入我们的小程序 → 用户点击购买按钮 → 跳转 Apple Store 小程序 → 用户核对并付款。

这里的“抢到”只能表示发现有货，不能表示已经下单或锁定库存。

## 必须在接入前验证

1. **后台持续运行**：小程序退到后台后不能作为可靠常驻轮询器；监控放在服务器。同一地区、门店和 SKU 合并订阅、合并查询，受限退避，避免每位用户各请求一次。
2. **订阅消息**：需要当前小程序服务类目允许的消息模板和用户主动授权。一次性订阅不是永久推送权限；不能假定零售提醒能使用长期模板。保存和消费可用订阅次数，失败及重复通知要有处理策略。
3. **通知落地页**：订阅消息的 page 指向我们的小程序页面。不能直接把 Apple Store 小程序路径塞进我们消息的 page 当作跨小程序入口。
4. **跨小程序跳转**：`wx.navigateToMiniProgram` 需要目标 AppID；path 和 query 必须是目标小程序实际支持的路径、参数。Apple 官网 SKU URL 不能自动转换成 Apple Store 小程序商品页。
5. **用户点击**：跳转接口要求用户点击触发。不能收到库存后在后台强制弹开 Apple Store。
6. **Apple Store 路径**：需要实际获取并验证 Apple Store 小程序的 AppID、商品页及门店参数。没有这些信息时只能承诺一个购买入口，不能承诺直接进入某门店某颜色的结算页。
7. **付款**：订单、库存锁定、Apple 登录及付款都由用户在 Apple 的程序中完成。当前没有经过验证的 Apple 下单接口可供我们代抢。
8. **运营接入**：核对我们现有小程序的服务类目、模板与审核要求，并实测 iOS / Android 的跳转和订阅通知。方案说明不代表已经通过审核或验证 Apple 的具体页面。

## 参考

- [wx.navigateToMiniProgram](https://developers.weixin.qq.com/miniprogram/dev/api/navigate/wx.navigateToMiniProgram.html)
- [wx.requestSubscribeMessage](https://developers.weixin.qq.com/miniprogram/dev/api/open-api/subscribe-message/wx.requestSubscribeMessage.html)

这份说明记录可行性和待验证条件，尚未接入现有微信小程序。
