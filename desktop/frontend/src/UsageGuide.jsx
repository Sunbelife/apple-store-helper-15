import React, { useEffect, useRef } from "react";
import { X, ArrowUpRight } from "lucide-react";

const steps = [
  {
    title: "先准备苹果购物袋",
    text: "在系统默认浏览器打开对应地区的苹果官网，选好型号、容量、颜色和购买数量，加入购物袋。提前确认账号能正常登录。请使用普通窗口，后续也在同一浏览器、同一用户配置中结账。",
  },
  {
    title: "添加你要关注的门店",
    text: "在左侧选择地区、产品、省市和零售店，再点「添加到监控列表」。「全部城市」和「全部门店」可以批量加入当前筛选范围的门店，重复目标会自动跳过。开始前请核对列表里的产品与购物袋一致。",
  },
  {
    title: "开始监控，留意更新时间",
    text: "点击「开始监控」，保持电脑运行、联网并避免休眠。列表第一列是更新时间，第二列是库存状态。等待或查询中不代表无货；「暂无库存」只代表该次检查的结果。需要修改配置时先暂停。",
  },
  {
    title: "触发风控时等待自动重试",
    text: "看到「触发风控，冷却中，稍后再尝试」时，该门店会按 1、2、4、5 分钟单独冷却，其他门店继续监控；成功后恢复正常频率。查询失败、冷却中或未返回库存都不等于无货，不要反复重启来催促查询。",
  },
  {
    title: "有货后，在购物袋里选择取货",
    text: "发现有货后会响铃、弹窗，打开对应地区的苹果购物袋，并暂停监控。在浏览器点击「结账」，按官网提示登录或游客结账，选择「我要取货」，再手动选择提醒中的门店、可用时段并填写取货资料，核对后自行付款。",
  },
  {
    title: "遇到空购物袋或库存变化",
    text: "购物袋为空时，先检查是否用了同一浏览器和用户配置；也可以点击有货弹窗里的商品页入口重新选购。官网无货时可以关闭提醒并停止响铃，再手动开始监控。关闭有货弹窗不会自动恢复监控。",
  },
];

export default function UsageGuide({ onClose, onArticle }) {
  const closeButton = useRef(null);
  const close = useRef(onClose);
  close.current = onClose;
  useEffect(() => {
    const previous = document.activeElement;
    closeButton.current?.focus();
    const keydown = (event) => {
      if (event.key === "Escape") close.current();
      if (event.key === "Tab") {
        const buttons = closeButton.current?.closest("section")?.querySelectorAll("button");
        const first = buttons?.[0], last = buttons?.[buttons.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    };
    document.addEventListener("keydown", keydown);
    return () => {
      document.removeEventListener("keydown", keydown);
      previous?.focus();
    };
  }, []);
  return (
    <div className="overlay" onClick={onClose}>
      <section className="dialog usage-guide" role="dialog" aria-modal="true" aria-labelledby="usage-title" onClick={(event) => event.stopPropagation()}>
        <button ref={closeButton} className="icon-button dialog-close" aria-label="关闭使用说明" onClick={onClose}><X size={18} /></button>
        <div className="eyebrow">开始之前</div>
        <h2 id="usage-title">使用说明</h2>
        <p className="guide-intro">提前加入购物袋 → 电脑监控门店 → 有货后手动取货下单</p>
        <p className="guide-extra">每次打开会自动尝试更新产品目录，失败时仍可使用本地目录；也可手动点击「更新目录」。不能使用或需要更新？点击主界面顶部「问题反馈」，到微博联系老孙。</p>
        <ol className="guide-steps">
          {steps.map((step) => <li key={step.title}><h3>{step.title}</h3><p>{step.text}</p></li>)}
        </ol>
        <div className="guide-note">
          <strong>提醒不等于锁定库存</strong>
          <p>程序不自动加购、选店、下单或付款。库存会随时变化，以苹果结账页为准。我们保留购物袋入口：结账直达链接依赖会过期的浏览器会话，不能保证直接进入取货页。</p>
        </div>
        <p className="guide-extra">可在监控列表顶部的「到货提醒」测试提示音、开启系统通知，或配置 Bark。当前版本尚未接入微信绑定和微信到货通知。</p>
        <button className="text-button" onClick={onArticle}>查看老孙公众号原文 <ArrowUpRight size={13} /></button>
      </section>
    </div>
  );
}
