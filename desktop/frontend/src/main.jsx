import { t as tr, locale, useLanguage } from "./i18n.js";
import React, { useState, useEffect, useRef } from "react";
import { createRoot } from "react-dom/client";
import {
  Plus,
  Play,
  Pause,
  Settings2,
  AlarmClock,
  RefreshCw,
  X,
  ChevronDown,
  MapPin,
  Bell,
  ArrowUpRight,
  Volume2,
  Download,
  Eye,
  EyeOff,
} from "lucide-react";
import "./style.css";
import ShopRail from "./ShopRail.jsx";
import profileAvatar from "./assets/weibo-avatar.jpg";
import UsageGuide from "./UsageGuide.jsx";
import StockStatistics from "./StockStatistics.jsx";
const COOLDOWN_STATUS = "触发风控，冷却中，稍后再尝试";
const call = (n, ...a) =>
  window.go?.main?.Desktop?.[n](...a) ??
  Promise.reject(Error("请在桌面应用中运行"));
const unique = (a) =>
  [...new Set(a)].filter(Boolean).sort((a, b) => a.localeCompare(b, "zh-CN"));
function Select({ label, value, onChange, children, disabled, title }) {
  return (
    <label className="field">
      <span>{tr(label)}</span>
      <span className="select">
        <select
          title={title}
          disabled={disabled}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        >
          {tr(children)}
        </select>
        <ChevronDown size={14} />
      </span>
    </label>
  );
}
function App() {
  const [languageMode, setLanguageMode, language] = useLanguage();
  useEffect(() => {
    call("SetUILanguage", language).catch(() => {});
  }, [language]);
  const [s, setS] = useState({
      items: [],
      logs: [],
      phase: "正在载入",
      running: false,
      nextAt: 0,
    }),
    [c, setC] = useState({
      areas: [],
      stores: [],
      products: [],
    });
  const [area, setArea] = useState("cn"),
    [province, setProvince] = useState(""),
    [city, setCity] = useState(""),
    [store, setStore] = useState(""),
    [model, setModel] = useState(""),
    [cap, setCap] = useState(""),
    [sku, setSku] = useState("");
  const [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false),
    [settings, setSettings] = useState(false),
    [showGuide, setShowGuide] = useState(false),
    [showStatistics, setShowStatistics] = useState(false),
    [reminderOpen, setReminderOpen] = useState(false),
    [systemNotifications, setSystemNotifications] = useState(false),
    [catalogRefreshing, setCatalogRefreshing] = useState(false),
    [logs, setLogs] = useState(false),
    [toast, setToast] = useState(null),
    [bark, setBark] = useState(""),
    [showBark, setShowBark] = useState(true),
    [barkLoading, setBarkLoading] = useState(false),
    [barkTesting, setBarkTesting] = useState(false),
    [barkResult, setBarkResult] = useState(null),
    [appInfo, setAppInfo] = useState({
      version: "",
    }),
    [update, setUpdate] = useState(null),
    [showUpdate, setShowUpdate] = useState(false),
    [checkingUpdate, setCheckingUpdate] = useState(false),
    [installingUpdate, setInstallingUpdate] = useState(false),
    [now, setNow] = useState(Date.now());
  const audio = useRef(null),
    request = useRef(0),
    refreshedAreas = useRef(new Set());
  const locked = busy || s.running;
  const pickupUnsupported = c.areas.find(
    (x) => x.ShortCode === area,
  )?.PickupUnsupported;
  const message = (text, error = false) =>
    setToast({
      text: tr(text),
      error,
    });
  async function sync() {
    setS(await call("GetState"));
  }
  async function action(f) {
    setBusy(true);
    try {
      await f();
      await sync();
    } catch (e) {
      message(String(e), true);
    } finally {
      setBusy(false);
    }
  }
  async function checkUpdate(notify = true) {
    if (checkingUpdate) return;
    setCheckingUpdate(true);
    try {
      const info = await call("CheckForUpdates");
      setUpdate(info);
      if (info.available) {
        setShowUpdate(true);
      } else if (notify) {
        message(`当前 v${info.currentVersion} 已是最新版本`);
      }
    } catch (e) {
      if (notify) message(String(e), true);
    } finally {
      setCheckingUpdate(false);
    }
  }
  async function installUpdate() {
    setInstallingUpdate(true);
    try {
      await call("InstallUpdate");
    } catch (e) {
      setInstallingUpdate(false);
      message(String(e), true);
    }
  }
  function apply(c) {
    setC(c);
    const m = c.products.some((p) => p.Model === "iPhone Duo")
      ? "iPhone Duo"
      : c.products[0]?.Model || "";
    const v = c.products.find((p) => p.Model === m);
    setModel(m);
    setCap(v?.Capacity || "");
    setSku(v?.Code || "");
    setProvince(
      area === "cn" && c.stores.some((x) => x.Province === "广东")
        ? "广东"
        : "",
    );
    setCity("");
    setStore("");
  }
  useEffect(() => {
    if (!reminderOpen) {
      setBark("");
      setBarkResult(null);
      return;
    }
    let cancelled = false;
    setBarkLoading(true);
    setBarkResult(null);
    call("GetBark")
      .then((value) => {
        if (!cancelled) {
          setBark(value);
          setShowBark(!value);
        }
      })
      .catch(() => {
        if (!cancelled) message("读取已保存的推送地址失败", true);
      })
      .finally(() => {
        if (!cancelled) setBarkLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [reminderOpen]);
  useEffect(() => {
    call("GetSystemNotificationStatus")
      .then(setSystemNotifications)
      .catch(() => setSystemNotifications(false));
  }, [reminderOpen]);
  useEffect(() => {
    if (!reminderOpen) return;
    const onKey = (event) => {
      if (event.key === "Escape") setReminderOpen(false);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [reminderOpen]);
  useEffect(() => {
    let closed = false,
      pending = false;
    const poll = async () => {
      if (pending) return;
      pending = true;
      try {
        const state = await call("GetState");
        if (!closed) setS(state);
      } catch (e) {
        if (!closed) message(String(e), true);
      } finally {
        pending = false;
      }
    };
    poll();
    const t = setInterval(poll, 800),
      clock = setInterval(() => setNow(Date.now()), 1000);
    return () => {
      closed = true;
      clearInterval(t);
      clearInterval(clock);
    };
  }, []);
  useEffect(() => {
    call("GetAppInfo")
      .then(setAppInfo)
      .catch(() => {});
    checkUpdate(false);
  }, []);
  useEffect(() => {
    let id = ++request.current;
    setLoading(true);
    setCatalogRefreshing(false);
    (async () => {
      try {
        const cached = await call("GetCatalog", area);
        if (id !== request.current) return;
        apply(cached);
        if (refreshedAreas.current.has(area)) return;
        setCatalogRefreshing(true);
        try {
          const fresh = await call("RefreshCatalog", area);
          if (id !== request.current) return;
          refreshedAreas.current.add(area);
          setC(fresh);
        } catch (e) {
          if (id === request.current) {
            const merged = await call("GetCatalog", area);
            if (id === request.current) {
              setC(merged);
              message(`更新未全部完成，已保留本地目录：${e}`, true);
            }
          }
        }
      } catch (e) {
        if (id === request.current) message(String(e), true);
      } finally {
        if (id === request.current) {
          setCatalogRefreshing(false);
          setLoading(false);
        }
      }
    })();
  }, [area]);
  useEffect(() => {
    if (!toast) return;
    let t = setTimeout(() => setToast(null), 6000);
    return () => clearTimeout(t);
  }, [toast]);
  function readyAudio() {
    if (!audio.current)
      audio.current = new (window.AudioContext || window.webkitAudioContext)();
    audio.current.resume().catch(() => {});
  }
  function beep() {
    try {
      readyAudio();
      const a = audio.current;
      [0, 0.22, 0.44].forEach((d) => {
        const o = a.createOscillator(),
          g = a.createGain();
        o.frequency.value = 880;
        g.gain.setValueAtTime(0, a.currentTime + d);
        g.gain.linearRampToValueAtTime(0.1, a.currentTime + d + 0.02);
        g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + d + 0.18);
        o.connect(g);
        g.connect(a.destination);
        o.start(a.currentTime + d);
        o.stop(a.currentTime + d + 0.2);
      });
    } catch (e) {
      message("提示音不可用：" + e, true);
    }
  }
  const alertKey = s.alert ? s.alert.id + s.alert.checkedAt : "";
  useEffect(() => {
    if (!alertKey) return;
    beep();
    call("AlertShown", s.alert.id).catch((e) => message(String(e), true));
    const t = setInterval(beep, 5000);
    return () => clearInterval(t);
  }, [alertKey]);
  const models = unique(c.products.map((p) => p.Model)),
    caps = unique(
      c.products.filter((p) => p.Model === model).map((p) => p.Capacity),
    ),
    products = c.products.filter(
      (p) => p.Model === model && p.Capacity === cap,
    );
  const provinces = unique(c.stores.map((x) => x.Province)),
    stores = c.stores.filter(
      (x) =>
        (!province || x.Province === province) && (!city || x.City === city),
    );
  const abnormal = s.items.filter((t) =>
      [
        COOLDOWN_STATUS,
        "库存接口受限",
        "需官网验证",
        "查询失败",
        "未返回库存",
      ].includes(t.status),
    ).length,
    available = s.items.filter((t) => t.status === "有货").length;
  const shown = s.items;
  function changeModel(m) {
    setModel(m);
    const p = c.products.find((p) => p.Model === m);
    setCap(p?.Capacity || "");
    setSku(p?.Code || "");
  }
  function changeCap(v) {
    setCap(v);
    setSku(
      c.products.find((p) => p.Model === model && p.Capacity === v)?.Code || "",
    );
  }
  const dropdown = (label, value, setter, options, empty) => (
    <Select
      label={tr(label)}
      title={tr(
        options.find((o) =>
          typeof o === "string" ? o === value : o.value === value,
        )?.label || value,
      )}
      value={value}
      onChange={setter}
      disabled={locked || loading}
    >
      {empty && <option value="">{tr(empty)}</option>}
      {options.map((o) =>
        typeof o === "string" ? (
          <option key={o} value={o}>
            {tr(o)}
          </option>
        ) : (
          <option key={o.value} value={o.value}>
            {tr(o.label)}
          </option>
        ),
      )}
    </Select>
  );
  return (
    <>
      {loading && (
        <div className="catalog-loading" role="status" aria-live="polite">
          <RefreshCw size={24} className="spin" />
          <strong>{tr("正在更新产品和门店信息…")}</strong>
          <span>
            {tr("增量合并，保留已有目录与监控；网络异常时使用本地数据")}
          </span>
        </div>
      )}
      <div className="titlebar">
        <span aria-hidden="true" />
        <button
          className={"title-update " + (update?.available ? "available" : "")}
          disabled={checkingUpdate}
          onClick={() =>
            appInfo.platform === "windows"
              ? call("OpenDownloads")
              : update?.available
                ? setShowUpdate(true)
                : checkUpdate(true)
          }
        >
          {tr(
            appInfo.platform === "windows"
              ? `v${appInfo.version} · 下载更新`
              : checkingUpdate
                ? "正在检查更新…"
                : update?.available
                  ? `v${update.latestVersion} 可更新`
                  : `${appInfo.version ? `v${appInfo.version} · ` : ""}检查更新`,
          )}
        </button>
      </div>
      <header className="app-header">
        <div className="brand">
          <span className="brand-mark">
            <img src={profileAvatar} alt={tr("Sunbelife 微博头像")} />
          </span>
          <div>
            <h1>
              {tr("老孙抢你妹")} <span className="brand-year">2026</span>
            </h1>
            <p>{tr("Apple Store 门店取货监控")}</p>
          </div>
        </div>
        <div className="header-actions">
          <button
            className="button secondary feedback-button"
            onClick={() => action(() => call("OpenFeedback"))}
            title={tr("提交问题、附加日志，并查看老孙的回复")}
          >
            {tr("问题反馈")}
            <ArrowUpRight size={14} />
          </button>
          <button
            className="text-button guide-trigger"
            onClick={() => setShowStatistics(true)}
          >
            {tr("库存统计")}
          </button>
          <button
            className="text-button guide-trigger"
            onClick={() => setShowGuide(true)}
          >
            {tr("使用说明")}
          </button>
          <button
            className="icon-button"
            aria-label={tr("配置与迁移")}
            title={tr("配置与迁移")}
            onClick={() => setSettings(true)}
          >
            <Settings2 size={20} />
          </button>
        </div>
      </header>
      <main className="workspace">
        <aside
          className={
            "configuration" +
            (/Mac|iMac/.test(model) ? " computer-configuration" : "")
          }
        >
          <div className="configuration-fields">
            <div className="section-heading">
              <h2>{tr("添加监控")}</h2>
              <button
                className="text-button catalog-refresh"
                aria-label={tr("更新产品和门店信息")}
                title={tr("从苹果官网增量更新产品和门店信息，保留已有监控")}
                disabled={locked || loading || catalogRefreshing}
                onClick={() =>
                  action(async () => {
                    setLoading(true);
                    try {
                      setC(await call("RefreshCatalog", area));
                      message("产品和门店信息已增量更新");
                    } catch (e) {
                      setC(await call("GetCatalog", area));
                      throw e;
                    } finally {
                      setLoading(false);
                    }
                  })
                }
              >
                <RefreshCw
                  size={13}
                  className={loading || catalogRefreshing ? "spin" : ""}
                />
                {tr(catalogRefreshing ? "更新中" : "更新目录")}
              </button>
            </div>
            <div
              className={
                "field-pair" +
                (/Mac|iMac/.test(model) ? " computer-fields" : "")
              }
            >
              {dropdown(
                "地区",
                area,
                setArea,
                c.areas.length
                  ? c.areas.map((x) => ({
                      value: x.ShortCode,
                      label:
                        tr(x.Title) +
                        (x.PickupUnsupported ? tr(" · 暂不支持取货") : ""),
                    }))
                  : [
                      {
                        value: "cn",
                        label: "中国大陆",
                      },
                    ],
              )}
              {dropdown("型号", model, changeModel, models)}
            </div>
            <div
              className={
                "field-pair" +
                (/Mac|iMac/.test(model) ? " computer-fields" : "")
              }
            >
              {dropdown(
                model.includes("Apple TV")
                  ? "SKU"
                  : model.includes("Watch")
                    ? "尺寸"
                    : /Mac|iMac/.test(model)
                      ? "配置"
                      : "容量",
                cap,
                changeCap,
                caps.map((value) => ({
                  value,
                  label: /Mac|iMac/.test(model)
                    ? value.replace(/(\d+) CPU \/ (\d+) GPU/, "$1C/$2G")
                    : value,
                })),
              )}
              {dropdown(
                model.includes("Apple TV")
                  ? "版本"
                  : model.includes("Watch") || /Mac mini|Mac Studio/.test(model)
                    ? "款式"
                    : "颜色",
                sku,
                setSku,
                products.map((p) => ({
                  value: p.Code,
                  label: p.Color,
                })),
              )}
            </div>
            <div className="field-pair">
              {dropdown(
                "省份 / 州",
                province,
                (v) => {
                  setProvince(v);
                  setCity("");
                  setStore("");
                },
                provinces,
                "全部省份",
              )}
              {dropdown(
                "城市",
                city,
                (v) => {
                  setCity(v);
                  setStore("");
                },
                unique(
                  c.stores
                    .filter((x) => !province || x.Province === province)
                    .map((x) => x.City),
                ),
                "全部城市",
              )}
            </div>
            {dropdown(
              "零售店",
              store,
              setStore,
              stores.map((x) => ({
                value: x.StoreNumber,
                label: x.CityStoreName,
              })),
              `全部门店 · ${stores.length}`,
            )}
          </div>
          {model.includes("Apple TV") && !pickupUnsupported && (
            <p className="market-note">
              {tr("Apple TV 支持预约取货状态；预约日期不代表当天现货。")}
            </p>
          )}
          {pickupUnsupported && (
            <p className="market-note">
              {tr("该地区官网暂未提供门店取货查询，仅可浏览产品目录。")}
            </p>
          )}
          <div className="configuration-actions">
            <button
              className="button secondary add add-neutral"
              disabled={
                locked || loading || pickupUnsupported || !sku || !stores.length
              }
              onClick={() =>
                action(async () => {
                  const n = await call(
                    "AddTargets",
                    area,
                    store ? [store] : stores.map((x) => x.StoreNumber),
                    sku,
                  );
                  message(
                    n ? `已添加 ${n} 项监控` : "监控项已存在，无需重复添加",
                  );
                })
              }
            >
              <Plus size={17} />
              {tr("添加到监控列表")}
            </button>
            <button
              className={
                "button monitor-toggle monitor-toggle-sidebar " +
                (s.running ? "secondary" : "primary")
              }
              disabled={busy || (!s.running && !s.items.length)}
              onClick={() => {
                readyAudio();
                action(() => call(s.running ? "Pause" : "Start"));
              }}
            >
              {tr(s.running ? <Pause size={16} /> : <Play size={16} />)}
              <span className="monitor-toggle-copy">
                <strong>{tr(s.running ? "暂停监控" : "开始监控")}</strong>
                {s.running && (
                  <small title={tr(s.phase)}>
                    {tr(s.phase)}
                    {tr(
                      s.running && s.nextAt > 0
                        ? ` · ${Math.max(0, Math.ceil((s.nextAt - now) / 1000))} 秒后重试`
                        : "",
                    )}
                  </small>
                )}
              </span>
            </button>
          </div>
        </aside>
        <section className="monitor">
          <div className="monitor-heading">
            <div>
              <div className="monitor-title-row">
                <h2>{tr("关注的门店")}</h2>
                <button
                  className="alarm-trigger"
                  aria-label={tr("到货提醒配置")}
                  title={tr("到货提醒配置")}
                  onClick={() => setReminderOpen(true)}
                >
                  <AlarmClock size={17} strokeWidth={1.6} />
                </button>
                <span className="monitor-count">{tr(s.items.length)}</span>
              </div>
            </div>
            <div className="monitor-controls">
              <div
                className="monitor-counts"
                aria-label={`${tr("有货")} ${available}, ${tr("异常")} ${abnormal}`}
              >
                <span>
                  {tr("有货")}
                  <b>{tr(available)}</b>
                </span>
                <span className={abnormal ? "warn" : ""}>
                  {tr("异常")}
                  <b>{tr(abnormal)}</b>
                </span>
              </div>
              <button
                className="text-button"
                disabled={locked || !s.items.length}
                onClick={() => action(() => call("ClearTargets"))}
              >
                {tr("清空列表")}
              </button>
            </div>
          </div>
          {abnormal > 0 && (
            <div className="inline-warning">
              {tr(
                s.items.some(
                  (t) =>
                    t.status === COOLDOWN_STATUS || t.status === "库存接口受限",
                )
                  ? "触发风控的门店会单独冷却；其他门店继续监控，不会被一起暂停。"
                  : "部分查询未获得有效库存，请查看该行原因。",
              )}
              <span className="warning-detail">
                {tr("冷却门店会在冷却期结束后自动重试；未查到不代表无货。")}
              </span>
            </div>
          )}
          {tr(
            !s.items.length ? (
              <div className="empty">
                <span className="empty-icon">
                  <MapPin size={30} strokeWidth={1.3} />
                </span>
                <h3>{tr("从一家门店开始")}</h3>
                <p>
                  {tr("在左侧选择心仪的产品和门店。")}

                  <br />
                  {tr("有货时，我们会提醒你并打开苹果购物袋。")}
                </p>
              </div>
            ) : (
              <div className="table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th>{tr("更新时间")}</th>
                      <th>{tr("库存状态")}</th>
                      <th>{tr("门店 / 产品")}</th>
                      <th>
                        <span className="sr-only">{tr("操作")}</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {shown.map((t) => (
                      <tr key={t.id}>
                        <td className="timestamp">
                          {tr(
                            t.checkedAt
                              ? new Date(t.checkedAt).toLocaleTimeString(
                                  locale(),
                                  {
                                    hour12: false,
                                  },
                                )
                              : "—",
                          )}
                        </td>
                        <td>
                          <span
                            className={
                              "stock " +
                              ({
                                有货: "in",
                                无货: "out",
                                等待: "waiting",
                                查询中: "querying",
                                可预约取货: "waiting",
                              }[t.status] || "error")
                            }
                          >
                            <span className="stock-emoji" aria-hidden="true">
                              {tr(
                                {
                                  有货: "🟢",
                                  无货: "⚪️",
                                  等待: "⏳",
                                  查询中: "🔎",
                                  暂未开放取货: "🗓️",
                                  可预约取货: "🗓️",
                                }[t.status] ||
                                  (t.status === COOLDOWN_STATUS ? "🧊" : "⚠️"),
                              )}
                            </span>
                            <span className="stock-copy">
                              <span className="stock-label">
                                {tr(
                                  t.status === COOLDOWN_STATUS
                                    ? "冷却中"
                                    : t.status === "无货"
                                      ? "暂无库存"
                                      : t.status,
                                )}
                              </span>
                            </span>
                          </span>
                          {t.status === "可预约取货" && t.detail && <span className="pickup-quote">{t.detail}</span>}
                          {t.detail && t.status !== "可预约取货" && (
                            <details className="error-detail">
                              <summary>{tr("查看原因")}</summary>
                              <p>{tr(t.detail)}</p>
                            </details>
                          )}
                        </td>
                        <td>
                          <button
                            className="store-link"
                            onClick={() =>
                              action(() => call("OpenPurchase", t.id))
                            }
                          >
                            {tr(t.store.CityStoreName)}
                            <ArrowUpRight size={12} />
                          </button>
                          <div className="product-name">
                            {tr(t.product.Title)}
                          </div>
                        </td>
                        <td>
                          <button
                            className="icon-button small"
                            disabled={locked}
                            aria-label={tr("移除 ") + t.store.CityStoreName}
                            onClick={() =>
                              action(() => call("RemoveTarget", t.id))
                            }
                          >
                            <X size={14} />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {!shown.length && (
                  <p className="no-match">{tr("没有匹配的监控项")}</p>
                )}
              </div>
            ),
          )}
          <div className="monitor-footer">
            <button className="text-button" onClick={() => setLogs(!logs)}>
              {tr(logs ? "收起" : "查看")}
              {tr("运行日志")}
              <ChevronDown size={13} />
            </button>
            <button
              className="text-button"
              onClick={() =>
                action(async () => {
                  if (await call("ExportFeedbackLog"))
                    message("日志已导出，请在问题反馈页检查后上传");
                })
              }
              title={tr("导出最近运行日志，上传前请检查内容")}
            >
              <Download size={13} />
              {tr("导出反馈日志")}
            </button>
            <span>{tr("库存随时变化，以苹果结算页为准")}</span>
          </div>
          {logs && (
            <div className="logs" role="log">
              {tr(
                s.logs.length ? (
                  s.logs
                    .slice()
                    .reverse()
                    .map((l, i) => (
                      <p key={i}>
                        <time>{tr(l.time)}</time>
                        {tr(l.message)}
                      </p>
                    ))
                ) : (
                  <p>{tr("开始监控后显示查询记录。")}</p>
                ),
              )}
            </div>
          )}
        </section>
      </main>
      <ShopRail
        onCopy={(id) =>
          call("CopyShopProductLink", id)
            .then(() => message("小程序购买链接已复制"))
            .catch((e) => message(String(e), true))
        }
        onOpen={(id) =>
          call("OpenShopProduct", id).catch((e) => message(String(e), true))
        }
      />

      {toast && (
        <div
          className={"toast " + (toast.error ? "toast-error" : "")}
          role={toast.error ? "alert" : "status"}
        >
          {tr(toast.text)}
          <button aria-label={tr("关闭提示")} onClick={() => setToast(null)}>
            <X size={14} />
          </button>
        </div>
      )}
      {reminderOpen && (
        <div className="overlay" onClick={() => setReminderOpen(false)}>
          <section
            className="dialog reminder-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="reminder-heading"
            onClick={(event) => event.stopPropagation()}
          >
            <button
              autoFocus
              className="icon-button dialog-close"
              aria-label={tr("关闭提醒配置")}
              onClick={() => setReminderOpen(false)}
            >
              <X size={18} />
            </button>
            <h2 id="reminder-heading">{tr("到货提醒")}</h2>
            <p>{tr("发现有货后自动响铃并打开苹果购物袋，监控随后暂停。")}</p>
            <div className="monitor-reminders" aria-label={tr("到货提醒配置")}>
              <div className="reminder-toolbar">
                <span className="reminder-title">
                  <Bell size={14} />
                  {tr("到货提醒")}
                </span>
                <button className="text-button" onClick={beep}>
                  <Volume2 size={13} />
                  {tr("测试提示音")}
                </button>
                <button
                  className="text-button"
                  disabled={busy}
                  onClick={() =>
                    action(async () =>
                      message(
                        (await call("EnableSystemNotifications").then(
                          (allowed) => {
                            setSystemNotifications(allowed);
                            return allowed;
                          },
                        ))
                          ? "已允许系统通知"
                          : "系统通知未获允许，可在系统设置中开启",
                      ),
                    )
                  }
                >
                  {tr("允许系统通知")}
                </button>
              </div>
              <div className="reminder-bark">
                <label htmlFor="monitor-bark">
                  {tr("Bark 手机推送")}
                  <span>{tr(s.barkConfigured ? "已配置" : "未配置")}</span>
                </label>
                <div className="bark-address-field">
                  <input
                    id="monitor-bark"
                    type={showBark ? "text" : "password"}
                    autoComplete="off"
                    spellCheck={false}
                    autoCapitalize="none"
                    aria-label={tr("Bark 推送地址")}
                    disabled={barkLoading || barkTesting}
                    placeholder={tr(
                      barkLoading
                        ? "正在读取已保存地址…"
                        : "粘贴 Bark 的 HTTPS 推送地址",
                    )}
                    value={bark}
                    onChange={(event) => {
                      setBark(event.target.value);
                      setBarkResult(null);
                    }}
                  />
                  <button
                    type="button"
                    className="icon-button"
                    aria-label={tr(showBark ? "隐藏推送地址" : "显示推送地址")}
                    title={tr(showBark ? "隐藏推送地址" : "显示推送地址")}
                    onClick={() => setShowBark(!showBark)}
                  >
                    {tr(showBark ? <EyeOff size={16} /> : <Eye size={16} />)}
                  </button>
                </div>
                <div className="bark-actions">
                  <button
                    className="text-button"
                    disabled={
                      busy || barkLoading || barkTesting || !bark.trim()
                    }
                    onClick={() =>
                      action(async () => {
                        await call("SaveBark", bark);
                        setBark(bark.trim());
                        setBarkResult(null);
                        message("推送地址已保存，可点击眼睛查看");
                      })
                    }
                  >
                    {tr("保存地址")}
                  </button>
                  <button
                    className="text-button"
                    disabled={
                      busy || barkLoading || barkTesting || !bark.trim()
                    }
                    onClick={async () => {
                      setBarkTesting(true);
                      setBarkResult(null);
                      try {
                        await call("TestBark", bark);
                        setBarkResult({
                          ok: true,
                          text: "Bark 服务已接受测试推送，请在手机确认收到。测试不会自动保存地址。",
                        });
                      } catch (e) {
                        setBarkResult({
                          ok: false,
                          text: String(e),
                        });
                      } finally {
                        setBarkTesting(false);
                      }
                    }}
                  >
                    {tr(barkTesting ? "正在发送…" : "测试推送")}
                  </button>
                  {s.barkConfigured && (
                    <button
                      className="text-button"
                      disabled={busy || barkLoading || barkTesting}
                      onClick={() =>
                        action(async () => {
                          await call("SaveBark", "");
                          setBark("");
                          setShowBark(true);
                          setBarkResult(null);
                        })
                      }
                    >
                      {tr("停用")}
                    </button>
                  )}
                </div>
                {barkResult && (
                  <p
                    className={
                      "bark-test-result " +
                      (barkResult.ok ? "success" : "failure")
                    }
                    role={barkResult.ok ? "status" : "alert"}
                  >
                    {tr(barkResult.text)}
                  </p>
                )}
              </div>
            </div>
            <p className="reminder-status-text">
              {tr(
                s.barkConfigured
                  ? "Bark 手机推送已配置。"
                  : "Bark 手机推送未配置。",
              )}
              {tr(
                systemNotifications
                  ? "系统通知已允许。"
                  : "系统通知未允许或暂不可用。",
              )}
              {tr("请另行点击「开始监控」开始监控。")}
            </p>
          </section>
        </div>
      )}
      {showStatistics && (
        <StockStatistics
          onClose={() => setShowStatistics(false)}
          load={(filters) => call("GetStockStatistics", filters)}
        />
      )}
      {showGuide && (
        <UsageGuide
          onClose={() => setShowGuide(false)}
          onArticle={() => action(() => call("OpenUsageArticle"))}
        />
      )}
      {showUpdate && update?.available && (
        <div
          className="overlay"
          onClick={() => !installingUpdate && setShowUpdate(false)}
        >
          <section
            className="dialog update-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="update-title"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              className="icon-button dialog-close"
              aria-label={tr("稍后更新")}
              disabled={installingUpdate}
              onClick={() => setShowUpdate(false)}
            >
              <X size={18} />
            </button>
            <span className="dialog-icon update-icon">
              <Download size={24} />
            </span>
            <div className="eyebrow">{tr("GitHub OTA 更新")}</div>
            <h2 id="update-title">
              {tr("发现 v")}
              {tr(update.latestVersion)}
            </h2>
            <p>
              {tr("当前版本 v")}
              {tr(update.currentVersion)}
              {tr("，更新包约")}
              {tr(
                Math.max(1, Math.ceil(update.downloadSize / 1024 / 1024)),
              )}{" "}
              MB。
            </p>
            <div className="update-notes">
              {tr(update.notes || update.name || "包含功能改进和问题修复。")}
            </div>
            <button
              className="button primary update-install"
              disabled={installingUpdate}
              onClick={installUpdate}
            >
              <Download size={16} />
              {tr(
                installingUpdate ? "正在下载、校验并安装…" : "下载、安装并重启",
              )}
            </button>
            <p className="update-security">
              {tr(
                "安装前会校验 GitHub Release 的 SHA-256、Bundle ID、版本号和应用签名；需要写入“应用程序”时，macOS 会请求管理员授权。",
              )}
            </p>
          </section>
        </div>
      )}
      {settings && (
        <div className="overlay" onClick={() => setSettings(false)}>
          <section
            className="dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="settings-title"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              className="icon-button dialog-close"
              aria-label={tr("关闭设置")}
              onClick={() => setSettings(false)}
            >
              <X size={18} />
            </button>
            <span className="dialog-icon">
              <Bell size={24} />
            </span>
            <h2 id="settings-title">{tr("配置与迁移")}</h2>
            <label className="field language-field">
              <span>{tr("语言")}</span>
              <select
                value={languageMode}
                onChange={(event) => setLanguageMode(event.target.value)}
              >
                <option value="auto">{tr("跟随系统")}</option>
                <option value="zh">简体中文</option>
                <option value="en">English</option>
              </select>
            </label>
            <p>
              {tr(
                "请先在默认浏览器的苹果官网把目标型号、容量和颜色加入购物袋。有货时自动打开该地区购物袋并持续响铃，监控随后暂停；取货门店需手动选择。",
              )}
            </p>
            <div className="import">
              <h3>{tr("从 1.x 迁移")}</h3>
              <p>
                {tr(
                  "选择旧应用中的 user_settings.json，导入门店、产品和 Bark 配置。",
                )}
              </p>
              <button
                className="button secondary"
                disabled={locked}
                onClick={() =>
                  action(async () =>
                    message(`已导入 ${await call("ImportLegacy")} 项监控`),
                  )
                }
              >
                {tr("导入旧版配置")}
              </button>
            </div>
          </section>
        </div>
      )}
      {s.alert && (
        <div className="overlay">
          <section
            className="dialog stock-dialog"
            role="alertdialog"
            aria-modal="true"
            aria-label={tr("有货提醒")}
          >
            <span className="dialog-icon found">
              <Bell size={26} />
            </span>
            <div className="eyebrow">{tr("发现门店库存")}</div>
            <h2>{tr(s.alert.store.CityStoreName)}</h2>
            <p>{tr(s.alert.product.Title)}</p>
            <p>
              {tr("已打开苹果购物袋，监控已暂停。")}

              <br />
              {tr("请在购物袋选择提醒中的取货门店，再核对并下单。")}
            </p>
            <button
              className="button primary"
              onClick={() => action(() => call("OpenBag", s.alert.id))}
            >
              {tr("前往购物袋")}

              <ArrowUpRight size={16} />
            </button>
            <button
              className="text-button"
              onClick={() => action(() => call("OpenPurchase", s.alert.id))}
            >
              {tr("购物袋没有目标手机？查看商品页")}
            </button>
            <button
              className="button secondary stock-shop-button"
              onClick={() => action(() => call("OpenShop"))}
            >
              {tr("来老孙的店买个配件")}

              <ArrowUpRight size={16} />
            </button>
            <button
              className="text-button"
              onClick={() => action(() => call("DismissAlert"))}
            >
              {tr("关闭提醒并停止响铃")}
            </button>
          </section>
        </div>
      )}
    </>
  );
}
createRoot(document.getElementById("root")).render(<App />);
