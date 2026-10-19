import React, { useState, useEffect, useRef } from "react";
import { createRoot } from "react-dom/client";
import {
  Plus,
  Play,
  Pause,
  Settings2,
  RefreshCw,
  X,
  ChevronDown,
  Search,
  MapPin,
  Bell,
  ArrowUpRight,
  Volume2,
  Download,
} from "lucide-react";
import "./style.css";
import ShopRail from "./ShopRail.jsx";
import profileAvatar from "./assets/weibo-avatar.jpg";
import UsageGuide from "./UsageGuide.jsx";
const COOLDOWN_STATUS = "触发风控，冷却中，稍后再尝试";
const call = (n, ...a) =>
  window.go?.main?.Desktop?.[n](...a) ??
  Promise.reject(Error("请在桌面应用中运行"));
const unique = (a) =>
  [...new Set(a)].filter(Boolean).sort((a, b) => a.localeCompare(b, "zh-CN"));
function Select({ label, value, onChange, children, disabled }) {
  return (
    <label className="field">
      <span>{label}</span>
      <span className="select">
        <select
          disabled={disabled}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        >
          {children}
        </select>
        <ChevronDown size={14} />
      </span>
    </label>
  );
}
function App() {
  const [s, setS] = useState({
      items: [],
      logs: [],
      phase: "正在载入",
      running: false,
      nextAt: 0,
    }),
    [c, setC] = useState({ areas: [], stores: [], products: [] });
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
    [catalogRefreshing, setCatalogRefreshing] = useState(false),
    [logs, setLogs] = useState(false),
    [toast, setToast] = useState(null),
    [search, setSearch] = useState(""),
    [bark, setBark] = useState(""),
    [appInfo, setAppInfo] = useState({ version: "" }),
    [update, setUpdate] = useState(null),
    [showUpdate, setShowUpdate] = useState(false),
    [checkingUpdate, setCheckingUpdate] = useState(false),
    [installingUpdate, setInstallingUpdate] = useState(false),
    [now, setNow] = useState(Date.now());
  const audio = useRef(null),
    request = useRef(0),
    refreshedAreas = useRef(new Set()),
    selection = useRef(null);
  selection.current = { model, cap, sku };
  const locked = busy || s.running;
  const message = (text, error = false) => setToast({ text, error });
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
    call("GetAppInfo").then(setAppInfo).catch(() => {});
    checkUpdate(false);
  }, []);
  useEffect(() => {
    let id = ++request.current;
    setLoading(true);
    setCatalogRefreshing(false);
    call("GetCatalog", area)
      .then((c) => {
        if (id !== request.current) return;
        apply(c);
        setLoading(false);
        if (refreshedAreas.current.has(area)) return;
        refreshedAreas.current.add(area);
        setCatalogRefreshing(true);
        // Use cached choices immediately; refreshing never edits monitoring targets.
        call("RefreshCatalog", area)
          .then((fresh) => {
            if (id !== request.current) return;
            const old = selection.current;
            const selected = fresh.products.find((p) => p.Code === old.sku)
              || fresh.products.find((p) => p.Model === old.model && p.Capacity === old.cap)
              || fresh.products.find((p) => p.Model === old.model)
              || fresh.products[0];
            setC(fresh);
            setModel(selected?.Model || "");
            setCap(selected?.Capacity || "");
            setSku(selected?.Code || "");
            message("产品目录已自动更新");
          })
          .catch(() => {
            if (id === request.current) message("目录自动更新未成功，继续使用本地目录，可稍后手动更新");
          })
          .finally(() => {
            if (id === request.current) setCatalogRefreshing(false);
          });
      })
      .catch((e) => {
        if (id === request.current) message(String(e), true);
      })
      .finally(() => {
        if (id === request.current) setLoading(false);
      });
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
    cities = unique(
      c.stores
        .filter((x) => !province || x.Province === province)
        .map((x) => x.City),
    ),
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
  const shown = s.items.filter((t) =>
    (t.store.CityStoreName + " " + t.product.Title)
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
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
      label={label}
      value={value}
      onChange={setter}
      disabled={locked || loading}
    >
      {empty && <option value="">{empty}</option>}
      {options.map((o) =>
        typeof o === "string" ? (
          <option key={o}>{o}</option>
        ) : (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ),
      )}
    </Select>
  );
  return (
    <>
      <div className="titlebar">
        <span aria-hidden="true" />
        <button
          className={"title-update " + (update?.available ? "available" : "")}
          disabled={checkingUpdate}
          onClick={() => update?.available ? setShowUpdate(true) : checkUpdate(true)}
        >
          {checkingUpdate
            ? "正在检查更新…"
            : update?.available
              ? `v${update.latestVersion} 可更新`
              : `${appInfo.version ? `v${appInfo.version} · ` : ""}检查更新`}
        </button>
      </div>
      <header className="app-header">
        <div className="brand">
          <span className="brand-mark">
            <img src={profileAvatar} alt="Sunbelife 微博头像" />
          </span>
          <div>
            <h1>
              老孙抢你妹 <span className="brand-year">2026</span>
            </h1>
            <p>Apple Store 门店取货监控</p>
          </div>
        </div>
        <div className="header-actions">
        <button className="button secondary feedback-button" onClick={() => action(() => call("OpenFeedback"))} title="不能使用或需要更新？到微博联系 Sunbelife">问题反馈 <ArrowUpRight size={14} /></button>
        <button className="text-button guide-trigger" onClick={() => setShowGuide(true)}>使用说明</button>
        <button
          className="icon-button"
          aria-label="配置与迁移" title="配置与迁移"
          onClick={() => setSettings(true)}
        >
          <Settings2 size={20} />
        </button>
        </div>
      </header>
      <main className="workspace">
        <aside className="configuration">
          <div className="section-heading">
            <h2>添加监控</h2>
            <button
              className="text-button catalog-refresh"
              aria-label="更新产品目录"
              title="从苹果官网更新型号、容量和颜色目录，不刷新库存"
              disabled={locked || loading || catalogRefreshing}
              onClick={() =>
                action(async () => {
                  setLoading(true);
                  try {
                    apply(await call("RefreshCatalog", area));
                    message("产品目录已更新");
                  } finally {
                    setLoading(false);
                  }
                })
              }
            >
              <RefreshCw size={13} className={loading || catalogRefreshing ? "spin" : ""} />
              {catalogRefreshing ? "更新中" : "更新目录"}
            </button>
          </div>
          {dropdown(
            "地区",
            area,
            setArea,
            c.areas.length
              ? c.areas.map((x) => ({ value: x.ShortCode, label: x.Title }))
              : [{ value: "cn", label: "中国大陆" }],
          )}
          <div className="form-divider">
            <span>产品</span>
          </div>
          {dropdown("型号", model, changeModel, models)}
          <div className="field-pair">
          {dropdown(
            model.includes("Watch") ? "尺寸" : "容量",
            cap,
            changeCap,
            caps,
          )}
          {dropdown(
            model.includes("Watch") ? "款式" : "颜色",
            sku,
            setSku,
            products.map((p) => ({ value: p.Code, label: p.Color })),
          )}
          </div>
          <div className="form-divider">
            <span>门店</span>
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
              cities,
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
            `全部门店（${stores.length} 家）`,
          )}
          <button
            className="button primary add"
            disabled={locked || loading || !sku || !stores.length}
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
            添加到监控列表
          </button>
          <p className="form-note">
            同一产品、同一门店会自动去重。
            <br />
            {s.running
              ? "暂停后可以修改监控配置。"
              : "「全部门店」可一次加入当前省市门店。"}
          </p>
        </aside>
        <section className="monitor">
          <div className="monitor-reminders" aria-label="到货提醒配置">
            <div className="reminder-toolbar">
              <span className="reminder-title"><Bell size={14} />到货提醒</span>
              <button className="text-button" onClick={beep}><Volume2 size={13} />测试提示音</button>
              <button className="text-button" disabled={busy} onClick={() => action(async () => message((await call("EnableSystemNotifications")) ? "已允许系统通知" : "系统通知未获允许，可在系统设置中开启"))}>允许系统通知</button>
            </div>
            <div className="reminder-bark">
              <label htmlFor="monitor-bark">Bark 手机推送 <span>{s.barkConfigured ? "已配置" : "未配置"}</span></label>
              <input id="monitor-bark" type="password" autoComplete="off" aria-label="Bark 推送地址" placeholder={s.barkConfigured ? "输入新地址可替换现有配置" : "粘贴 Bark 的 HTTPS 推送地址"} value={bark} onChange={(event) => setBark(event.target.value)} />
              <button className="text-button" disabled={busy || !bark.trim()} onClick={() => action(async () => { await call("SaveBark", bark); setBark(""); message("推送地址已保存"); })}>保存</button>
              {s.barkConfigured && <button className="text-button" disabled={busy} onClick={() => action(() => call("SaveBark", ""))}>停用</button>}
            </div>
          </div>
          <div className="monitor-heading">
            <div>
              <div className="eyebrow">库存监控</div>
              <h2>
                关注的门店<span>{s.items.length}</span>
              </h2>
            </div>
            <button
              className={"button " + (s.running ? "secondary" : "primary")}
              disabled={busy || (!s.running && !s.items.length)}
              onClick={() => {
                readyAudio();
                action(() => call(s.running ? "Pause" : "Start"));
              }}
            >
              {s.running ? <Pause size={16} /> : <Play size={16} />}{" "}
              {s.running ? "暂停监控" : "开始监控"}
            </button>
          </div>
          <div className="monitor-summary">
            <div>
              <i className={"live-dot " + (s.running ? "active" : "")} />
              <strong>{s.phase}</strong>
              {s.running && s.nextAt > 0 && (
                <span>
                  {Math.max(0, Math.ceil((s.nextAt - now) / 1000))} 秒后自动重试
                </span>
              )}
            </div>
            <div className="summary-counts">
              <span>
                有货 <b>{available}</b>
              </span>
              <span className={abnormal ? "warn" : ""}>
                异常 <b>{abnormal}</b>
              </span>
            </div>
          </div>
          {abnormal > 0 && (
            <div className="inline-warning">
              {s.items.some(
                (t) =>
                  t.status === COOLDOWN_STATUS || t.status === "库存接口受限",
              )
                ? "触发风控的门店会单独冷却；其他门店继续监控，不会被一起暂停。"
                : "部分查询未获得有效库存，请查看该行原因。"}
              <span className="warning-detail">
                冷却门店会在冷却期结束后自动重试；未查到不代表无货。
              </span>
            </div>
          )}
          <div className="table-tools">
            <label className="search">
              <Search size={15} />
              <input
                aria-label="搜索门店或产品"
                placeholder="搜索门店或产品"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </label>
            <button
              className="text-button"
              disabled={locked || !s.items.length}
              onClick={() => action(() => call("ClearTargets"))}
            >
              清空列表
            </button>
          </div>
          {!s.items.length ? (
            <div className="empty">
              <span className="empty-icon">
                <MapPin size={30} strokeWidth={1.3} />
              </span>
              <h3>从一家门店开始</h3>
              <p>
                在左侧选择心仪的产品和门店。
                <br />
                有货时，我们会提醒你并打开苹果购物袋。
              </p>
            </div>
          ) : (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>更新时间</th>
                    <th>库存状态</th>
                    <th>门店 / 产品</th>
                    <th>
                      <span className="sr-only">操作</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {shown.map((t) => (
                    <tr key={t.id}>
                      <td className="timestamp">
                        {t.checkedAt
                          ? new Date(t.checkedAt).toLocaleTimeString("zh-CN", { hour12: false })
                          : "—"}
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
                            }[t.status] || "error")
                          }
                        >
                          <span className="stock-emoji" aria-hidden="true">
                            {({ 有货: "🟢", 无货: "⚪️", 等待: "⏳", 查询中: "🔎" }[t.status]) || (t.status === COOLDOWN_STATUS ? "🧊" : "⚠️")}
                          </span>
                          <span className="stock-copy">
                            <span className="stock-label">{t.status === COOLDOWN_STATUS ? "冷却中" : t.status === "无货" ? "暂无库存" : t.status}</span>
                          </span>
                        </span>
                        {t.detail && (
                          <details className="error-detail">
                            <summary>查看原因</summary>
                            <p>{t.detail}</p>
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
                          {t.store.CityStoreName}
                          <ArrowUpRight size={12} />
                        </button>
                        <div className="product-name">{t.product.Title}</div>

                      </td>
                      <td>
                        <button
                          className="icon-button small"
                          disabled={locked}
                          aria-label={"移除 " + t.store.CityStoreName}
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
              {!shown.length && <p className="no-match">没有匹配的监控项</p>}
            </div>
          )}
          <div className="monitor-footer">
            <button className="text-button" onClick={() => setLogs(!logs)}>
              {logs ? "收起" : "查看"}运行日志
              <ChevronDown size={13} />
            </button>
            <span>库存随时变化，以苹果结算页为准</span>
          </div>
          {logs && (
            <div className="logs" role="log">
              {s.logs.length ? (
                s.logs
                  .slice()
                  .reverse()
                  .map((l, i) => (
                    <p key={i}>
                      <time>{l.time}</time>
                      {l.message}
                    </p>
                  ))
              ) : (
                <p>开始监控后显示查询记录。</p>
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
          {toast.text}
          <button aria-label="关闭提示" onClick={() => setToast(null)}>
            <X size={14} />
          </button>
        </div>
      )}
      {showGuide && <UsageGuide onClose={() => setShowGuide(false)} onArticle={() => action(() => call("OpenUsageArticle"))} />}
      {showUpdate && update?.available && (
        <div className="overlay" onClick={() => !installingUpdate && setShowUpdate(false)}>
          <section
            className="dialog update-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="update-title"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              className="icon-button dialog-close"
              aria-label="稍后更新"
              disabled={installingUpdate}
              onClick={() => setShowUpdate(false)}
            >
              <X size={18} />
            </button>
            <span className="dialog-icon update-icon">
              <Download size={24} />
            </span>
            <div className="eyebrow">GitHub OTA 更新</div>
            <h2 id="update-title">发现 v{update.latestVersion}</h2>
            <p>当前版本 v{update.currentVersion}，更新包约 {Math.max(1, Math.ceil(update.downloadSize / 1024 / 1024))} MB。</p>
            <div className="update-notes">
              {update.notes || update.name || "包含功能改进和问题修复。"}
            </div>
            <button
              className="button primary update-install"
              disabled={installingUpdate}
              onClick={installUpdate}
            >
              <Download size={16} />
              {installingUpdate ? "正在下载、校验并安装…" : "下载、安装并重启"}
            </button>
            <p className="update-security">安装前会校验 GitHub Release 的 SHA-256、Bundle ID、版本号和应用签名；需要写入“应用程序”时，macOS 会请求管理员授权。</p>
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
              aria-label="关闭设置"
              onClick={() => setSettings(false)}
            >
              <X size={18} />
            </button>
            <span className="dialog-icon">
              <Bell size={24} />
            </span>
            <h2 id="settings-title">配置与迁移</h2>
            <p>请先在默认浏览器的苹果官网把目标型号、容量和颜色加入购物袋。有货时自动打开该地区购物袋并持续响铃，监控随后暂停；取货门店需手动选择。</p>
            <div className="import">
              <h3>从 1.x 迁移</h3>
              <p>
                选择旧应用中的 user_settings.json，导入门店、产品和 Bark 配置。
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
                导入旧版配置
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
            aria-label="有货提醒"
          >
            <span className="dialog-icon found">
              <Bell size={26} />
            </span>
            <div className="eyebrow">发现门店库存</div>
            <h2>{s.alert.store.CityStoreName}</h2>
            <p>{s.alert.product.Title}</p>
            <p>
              已打开苹果购物袋，监控已暂停。
              <br />
              请在购物袋选择提醒中的取货门店，再核对并下单。
            </p>
            <button
              className="button primary"
              onClick={() => action(() => call("OpenBag", s.alert.id))}
            >
              前往购物袋
              <ArrowUpRight size={16} />
            </button>
            <button
              className="text-button"
              onClick={() => action(() => call("OpenPurchase", s.alert.id))}
            >
              购物袋没有目标手机？查看商品页
            </button>
            <button
              className="button secondary stock-shop-button"
              onClick={() => action(() => call("OpenShop"))}
            >
              来老孙的店买个配件
              <ArrowUpRight size={16} />
            </button>
            <button
              className="text-button"
              onClick={() => action(() => call("DismissAlert"))}
            >
              关闭提醒并停止响铃
            </button>
          </section>
        </div>
      )}
    </>
  );
}
createRoot(document.getElementById("root")).render(<App />);
