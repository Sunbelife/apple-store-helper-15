const KEY = "qnm-usage-consent-v1";
const STOP = "qnm-usage-stopped-v1";
const api = (name, ...args) => window.go?.main?.Desktop?.[name]?.(...args) ?? Promise.reject(Error("App required"));
export const participating = () => { try { return !!localStorage.getItem(KEY) && !localStorage.getItem(STOP); } catch { return false; } };
export const hasConsentRecord = () => { try { return !!localStorage.getItem(KEY); } catch { return false; } };
let ready;
function initialize() {
  if (!ready) ready = (async () => { const id = localStorage.getItem(KEY); if (id) await api("SetUsageIdentity", id); })().catch(() => {});
  return ready;
}
export async function trackUsage(event) { if (!participating()) return; await initialize(); await api("TrackUsage", event).catch(() => {}); }
export async function acceptUsage() {
  if (localStorage.getItem(STOP)) throw Error("请先重试删除原有统计");
  const id = crypto.randomUUID();
  localStorage.setItem(KEY, id);
  try { await api("SetUsageIdentity", id); } catch(e) {localStorage.removeItem(KEY); throw e;}
  ready = Promise.resolve();
  await trackUsage("app_open"); await trackUsage("page_stats");
}
export async function withdrawUsage() {
 const id = localStorage.getItem(KEY); localStorage.setItem(STOP, "true"); ready = undefined;
 try { await api("WithdrawUsage", id); } catch { throw Error("已停止统计，删除暂未成功，请点击按钮重试。"); }
 localStorage.removeItem(KEY); localStorage.removeItem(STOP);
}
