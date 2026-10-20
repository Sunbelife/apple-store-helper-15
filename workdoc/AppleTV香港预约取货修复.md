# Apple TV 香港预约取货修复

2026-10-20 16:15 +08:00，香港 Apple ifc mall（R428），Apple TV 4K MLFF4ZP/A。

真实查询返回 HTTP 200，pickupDisplay=default、storePickEligible=true、storeSelectionEnabled=true，storePickupQuote 为 Nov 13 at Apple ifc mall。原解析只识别 available/unavailable/ineligible，导致显示未返回库存。

修复后保留预约日期并显示「可预约取货」及苹果原始纯文本日期。与当天现货区分，不触发响铃、打开购物袋或现货上报，监控继续。缺少日期、资格或门店可选证据时不推断可预约。现货与预约混合响应中的现货仍保留。

真实接口复测已返回结构化预约结果。回归覆盖响应解析、混合现货、证据缺失和桌面监控不误触发提醒；Go 与 race 检查通过。没有下单或接受协议。
