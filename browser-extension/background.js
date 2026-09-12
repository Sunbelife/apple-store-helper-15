import {
  CHECK_INTERVAL_MINUTES,
  appleBaseURL,
  buildFulfillmentPath,
  parseStockResponse,
  productPurchaseURL
} from "./shared.js";

const ALARM_NAME = "qiangnimei18-stock-check";
const DASHBOARD_URL = chrome.runtime.getURL("app.html");
let currentCheck = null;

chrome.runtime.onInstalled.addListener((details) => {
  if (details.reason === "install") {
    chrome.tabs.create({ url: DASHBOARD_URL });
  }
});

chrome.action.onClicked.addListener(() => {
  openDashboard();
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === ALARM_NAME) {
    runStockCheck();
  }
});

chrome.notifications.onClicked.addListener(async (notificationId) => {
  if (!notificationId.startsWith("stock:")) {
    return;
  }
  const taskId = notificationId.slice("stock:".length);
  const { tasks = [] } = await chrome.storage.local.get("tasks");
  const task = tasks.find((candidate) => candidate.id === taskId);
  if (task) {
    await openProduct(task);
  }
  chrome.notifications.clear(notificationId);
});

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  handleMessage(message)
    .then((result) => sendResponse(result))
    .catch((error) => sendResponse({
      ok: false,
      error: error instanceof Error ? error.message : String(error)
    }));
  return true;
});

async function handleMessage(message) {
  switch (message?.type) {
    case "start-monitor": {
      const tasks = Array.isArray(message.tasks) ? message.tasks : [];
      if (tasks.length === 0) {
        throw new Error("请先添加至少一个监控任务");
      }
      const monitorState = createInitialState(tasks);
      await chrome.storage.local.set({
        tasks,
        barkUrl: String(message.barkUrl || "").trim(),
        monitoring: true,
        monitorState
      });
      chrome.alarms.create(ALARM_NAME, { periodInMinutes: CHECK_INTERVAL_MINUTES });
      await openMonitorTabs(tasks);
      await runStockCheck();
      return { ok: true };
    }
    case "stop-monitor":
      await chrome.storage.local.set({ monitoring: false });
      await chrome.alarms.clear(ALARM_NAME);
      await patchMonitorState((state) => {
        state.running = false;
        addLog(state, "监控已暂停");
      });
      return { ok: true };
    case "check-now":
      await runStockCheck();
      return { ok: true };
    case "open-product":
      await openProduct(message.task);
      return { ok: true };
    case "open-dashboard":
      await openDashboard();
      return { ok: true };
    default:
      return { ok: false, error: "未知操作" };
  }
}

function createInitialState(tasks) {
  const items = {};
  for (const task of tasks) {
    items[task.id] = taskState(task, "等待", "");
  }
  return {
    running: true,
    checking: false,
    lastCheck: "",
    items,
    log: [{ time: timeText(), message: "监控已启动；库存检查在 Apple 官网标签页内执行" }]
  };
}

function taskState(task, status, detail) {
  return {
    status,
    detail,
    checkedAt: new Date().toISOString(),
    label: task.store.CityStoreName + " · " + task.product.Model + " " +
      task.product.Capacity + " " + task.product.Color
  };
}

function groupTasks(tasks) {
  const groups = new Map();
  for (const task of tasks) {
    const key = task.areaCode + "::" + task.store.StoreNumber;
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key).push(task);
  }
  return [...groups.values()];
}

async function runStockCheck() {
  if (currentCheck) {
    return currentCheck;
  }
  currentCheck = performStockCheck().finally(() => {
    currentCheck = null;
  });
  return currentCheck;
}

async function performStockCheck() {
  const saved = await chrome.storage.local.get(["monitoring", "tasks", "monitorState"]);
  if (!saved.monitoring || !Array.isArray(saved.tasks) || saved.tasks.length === 0) {
    return;
  }

  const state = saved.monitorState || createInitialState(saved.tasks);
  state.running = true;
  state.checking = true;
  await chrome.storage.local.set({ monitorState: state });

  for (const tasks of groupTasks(saved.tasks)) {
    try {
      const first = tasks[0];
      const tab = await ensureAppleTab(first, false);
      await waitForTabReady(tab.id);
      const result = await sendStockRequest(tab.id, buildFulfillmentPath(tasks));

      if (result.status === 541) {
        for (const task of tasks) {
          state.items[task.id] = taskState(task, "需官网验证", "请在已打开的 Apple 标签页完成验证");
        }
        addLog(state, first.areaTitle + " · " + first.store.CityStoreName + " 需要官网验证");
        continue;
      }
      if (result.status !== 200) {
        throw new Error(result.error || "Apple 库存接口返回 HTTP " + result.status);
      }

      let payload;
      try {
        payload = JSON.parse(result.body);
      } catch {
        throw new Error("Apple 库存接口未返回 JSON");
      }

      for (const task of tasks) {
        const availability = parseStockResponse(
          payload,
          task.store.StoreNumber,
          task.product.Code
        );
        if (!availability.known) {
          state.items[task.id] = taskState(task, "查询异常", "响应中没有该门店或 SKU");
          continue;
        }

        const previousStatus = state.items[task.id]?.status;
        const status = availability.available ? "有货" : "无货";
        state.items[task.id] = taskState(task, status, availability.pickupDisplay);
        if (availability.available && previousStatus !== "有货") {
          await notifyInStock(task);
          addLog(state, task.store.CityStoreName + " · " + task.product.Model + " 有货");
        }
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      for (const task of tasks) {
        state.items[task.id] = taskState(task, "查询异常", message);
      }
      addLog(state, tasks[0].store.CityStoreName + " 查询失败：" + message);
    }
  }

  state.checking = false;
  state.lastCheck = new Date().toISOString();
  await chrome.storage.local.set({ monitorState: state });
}

async function openMonitorTabs(tasks) {
  const firstByArea = new Map();
  for (const task of tasks) {
    if (!firstByArea.has(task.areaCode)) {
      firstByArea.set(task.areaCode, task);
    }
  }
  let makeActive = true;
  for (const task of firstByArea.values()) {
    await ensureAppleTab(task, makeActive);
    makeActive = false;
  }
}

async function ensureAppleTab(task, active) {
  const saved = await chrome.storage.local.get("monitorTabs");
  const monitorTabs = saved.monitorTabs || {};
  let tab = null;
  const tabId = monitorTabs[task.areaCode];

  if (Number.isInteger(tabId)) {
    try {
      const candidate = await chrome.tabs.get(tabId);
      if (isMatchingAreaURL(candidate.pendingUrl || candidate.url, task.areaCode)) {
        tab = candidate;
      }
    } catch {
      tab = null;
    }
  }

  if (!tab) {
    tab = await chrome.tabs.create({
      url: productPurchaseURL(task),
      active
    });
    monitorTabs[task.areaCode] = tab.id;
    await chrome.storage.local.set({ monitorTabs });
  } else if (active) {
    tab = await chrome.tabs.update(tab.id, { active: true });
  }
  return tab;
}

function isMatchingAreaURL(value, areaCode) {
  if (!value) {
    return false;
  }
  try {
    const url = new URL(value);
    const base = new URL(appleBaseURL(areaCode));
    if (url.origin !== base.origin) {
      return false;
    }
    if (areaCode === "us") {
      return !/^\/(hk|jp|sg|uk|au)(\/|$)/.test(url.pathname);
    }
    if (areaCode === "cn") {
      return true;
    }
    return url.pathname === "/" + areaCode || url.pathname.startsWith("/" + areaCode + "/");
  } catch {
    return false;
  }
}

async function waitForTabReady(tabId) {
  const tab = await chrome.tabs.get(tabId);
  if (tab.status === "complete") {
    return;
  }

  await new Promise((resolve) => {
    const timer = setTimeout(() => {
      chrome.tabs.onUpdated.removeListener(onUpdated);
      resolve();
    }, 30000);
    function onUpdated(updatedTabId, changeInfo) {
      if (updatedTabId === tabId && changeInfo.status === "complete") {
        clearTimeout(timer);
        chrome.tabs.onUpdated.removeListener(onUpdated);
        resolve();
      }
    }
    chrome.tabs.onUpdated.addListener(onUpdated);
  });
}

async function sendStockRequest(tabId, path) {
  try {
    await chrome.tabs.sendMessage(tabId, { type: "ping" });
  } catch {
    await chrome.scripting.executeScript({
      target: { tabId },
      files: ["page-bridge.js"],
      world: "MAIN"
    });
    await chrome.scripting.executeScript({
      target: { tabId },
      files: ["content.js"]
    });
  }
  return chrome.tabs.sendMessage(tabId, { type: "stock-check", path });
}

async function openProduct(task) {
  if (!task) {
    throw new Error("产品信息为空");
  }
  const tab = await ensureAppleTab(task, true);
  const targetURL = productPurchaseURL(task);
  if (tab.url !== targetURL) {
    await chrome.tabs.update(tab.id, { url: targetURL, active: true });
  }
}

async function notifyInStock(task) {
  const title = task.store.CityStoreName + " 有货";
  const message = task.product.Model + " " + task.product.Capacity + " " + task.product.Color;
  await chrome.notifications.create("stock:" + task.id, {
    type: "basic",
    iconUrl: "icons/icon128.png",
    title,
    message,
    priority: 2,
    requireInteraction: true
  });

  const { barkUrl = "" } = await chrome.storage.local.get("barkUrl");
  if (barkUrl) {
    await sendBark(barkUrl, title, message, productPurchaseURL(task));
  }
}

async function sendBark(rawURL, title, body, link) {
  try {
    const barkURL = new URL(rawURL);
    if (barkURL.protocol !== "https:" || barkURL.hostname !== "api.day.app") {
      throw new Error("Bark 地址必须使用 https://api.day.app/");
    }
    barkURL.searchParams.set("title", title);
    barkURL.searchParams.set("body", body);
    barkURL.searchParams.set("url", link);
    await fetch(barkURL.href);
  } catch (error) {
    await patchMonitorState((state) => {
      addLog(state, "Bark 推送失败：" + (error instanceof Error ? error.message : String(error)));
    });
  }
}

async function openDashboard() {
  const tabs = await chrome.tabs.query({});
  const existing = tabs.find((tab) => tab.url === DASHBOARD_URL);
  if (existing) {
    await chrome.tabs.update(existing.id, { active: true });
    if (existing.windowId) {
      await chrome.windows.update(existing.windowId, { focused: true });
    }
    return;
  }
  await chrome.tabs.create({ url: DASHBOARD_URL });
}

async function patchMonitorState(mutator) {
  const { monitorState = { running: false, checking: false, items: {}, log: [] } } =
    await chrome.storage.local.get("monitorState");
  mutator(monitorState);
  await chrome.storage.local.set({ monitorState });
}

function addLog(state, message) {
  state.log = Array.isArray(state.log) ? state.log : [];
  const last = state.log[state.log.length - 1];
  if (last?.message === message) {
    last.time = timeText();
    return;
  }
  state.log.push({ time: timeText(), message });
  state.log = state.log.slice(-80);
}

function timeText() {
  return new Date().toLocaleString("zh-CN", { hour12: false });
}
