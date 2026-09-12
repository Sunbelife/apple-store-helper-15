import { AREAS, groupProductsByModel, unique } from "./shared.js";

const elements = {
  area: document.querySelector("#area"),
  province: document.querySelector("#province"),
  store: document.querySelector("#store"),
  model: document.querySelector("#model"),
  capacity: document.querySelector("#capacity"),
  color: document.querySelector("#color"),
  bark: document.querySelector("#bark"),
  add: document.querySelector("#add"),
  start: document.querySelector("#start"),
  stop: document.querySelector("#stop"),
  check: document.querySelector("#check"),
  tasks: document.querySelector("#tasks"),
  emptyTasks: document.querySelector("#empty-tasks"),
  monitorBadge: document.querySelector("#monitor-badge"),
  lastCheck: document.querySelector("#last-check"),
  logs: document.querySelector("#logs"),
  formMessage: document.querySelector("#form-message")
};

let productsByModel = {};
let stores = [];
let tasks = [];
let monitorState = null;
let monitoring = false;

await initialize();

async function initialize() {
  fillSelect(elements.area, AREAS.map((area) => ({ value: area.code, label: area.title })));
  const saved = await chrome.storage.local.get([
    "tasks",
    "barkUrl",
    "monitorState",
    "monitoring",
    "uiSelection"
  ]);
  tasks = Array.isArray(saved.tasks) ? saved.tasks : [];
  monitorState = saved.monitorState || null;
  monitoring = Boolean(saved.monitoring);
  elements.bark.value = saved.barkUrl || "";
  elements.area.value = saved.uiSelection?.areaCode || "cn";
  bindEvents();
  await loadAreaData(saved.uiSelection || {});
  render();
}

function bindEvents() {
  elements.area.addEventListener("change", async () => {
    await loadAreaData({});
    await saveSelection();
  });
  elements.province.addEventListener("change", () => {
    populateStores();
    saveSelection();
  });
  elements.store.addEventListener("change", saveSelection);
  elements.model.addEventListener("change", () => {
    populateCapacities();
    saveSelection();
  });
  elements.capacity.addEventListener("change", () => {
    populateColors();
    saveSelection();
  });
  elements.color.addEventListener("change", saveSelection);
  elements.bark.addEventListener("change", async () => {
    await chrome.storage.local.set({ barkUrl: elements.bark.value.trim() });
  });
  elements.add.addEventListener("click", addTask);
  elements.start.addEventListener("click", startMonitoring);
  elements.stop.addEventListener("click", stopMonitoring);
  elements.check.addEventListener("click", checkNow);

  chrome.storage.onChanged.addListener((changes, areaName) => {
    if (areaName !== "local") {
      return;
    }
    if (changes.tasks) {
      tasks = Array.isArray(changes.tasks.newValue) ? changes.tasks.newValue : [];
    }
    if (changes.monitorState) {
      monitorState = changes.monitorState.newValue || null;
    }
    if (changes.monitoring) {
      monitoring = Boolean(changes.monitoring.newValue);
    }
    render();
  });
}

async function loadAreaData(selection) {
  const areaCode = elements.area.value;
  setFormMessage("正在载入产品和门店…", false);
  const productURL = chrome.runtime.getURL("data/products/product_data_" + areaCode + ".json");
  const storeURL = chrome.runtime.getURL("data/stores/store_" + areaCode + ".json");
  const [productResponse, storeResponse] = await Promise.all([fetch(productURL), fetch(storeURL)]);
  if (!productResponse.ok || !storeResponse.ok) {
    throw new Error("当前地区的内置数据载入失败");
  }
  const productData = await productResponse.json();
  const storeData = await storeResponse.json();
  productsByModel = groupProductsByModel(productData.products);
  stores = Array.isArray(storeData.stores) ? storeData.stores : [];

  populateProvinces(selection.province);
  populateModels(selection.model);
  if (selection.storeNumber) {
    elements.store.value = selection.storeNumber;
  }
  if (selection.capacity) {
    elements.capacity.value = selection.capacity;
    populateColors(selection.color);
  }
  setFormMessage("", false);
}

function populateProvinces(preferredValue) {
  const provinces = unique(stores.map((store) => store.Province || store.City || "全部"))
    .sort((a, b) => a.localeCompare(b, "zh-CN"));
  fillSelect(elements.province, provinces.map((value) => ({ value, label: value })));
  if (preferredValue && provinces.includes(preferredValue)) {
    elements.province.value = preferredValue;
  }
  populateStores();
}

function populateStores() {
  const province = elements.province.value;
  const filtered = stores
    .filter((store) => (store.Province || store.City || "全部") === province)
    .sort((a, b) => a.CityStoreName.localeCompare(b.CityStoreName, "zh-CN"));
  fillSelect(elements.store, filtered.map((store) => ({
    value: store.StoreNumber,
    label: store.CityStoreName
  })));
}

function populateModels(preferredValue) {
  const models = Object.keys(productsByModel);
  fillSelect(elements.model, models.map((value) => ({ value, label: value })));
  if (preferredValue && models.includes(preferredValue)) {
    elements.model.value = preferredValue;
  }
  populateCapacities();
}

function populateCapacities() {
  const products = productsByModel[elements.model.value] || [];
  const capacities = unique(products.map((product) => product.Capacity));
  fillSelect(elements.capacity, capacities.map((value) => ({ value, label: value })));
  populateColors();
}

function populateColors(preferredValue) {
  const products = productsByModel[elements.model.value] || [];
  const colors = unique(products
    .filter((product) => product.Capacity === elements.capacity.value)
    .map((product) => product.Color));
  fillSelect(elements.color, colors.map((value) => ({ value, label: value })));
  if (preferredValue && colors.includes(preferredValue)) {
    elements.color.value = preferredValue;
  }
}

function fillSelect(select, options) {
  select.replaceChildren();
  for (const optionData of options) {
    const option = document.createElement("option");
    option.value = optionData.value;
    option.textContent = optionData.label;
    select.append(option);
  }
  select.disabled = options.length === 0;
}

async function saveSelection() {
  await chrome.storage.local.set({
    uiSelection: {
      areaCode: elements.area.value,
      province: elements.province.value,
      storeNumber: elements.store.value,
      model: elements.model.value,
      capacity: elements.capacity.value,
      color: elements.color.value
    }
  });
}

async function addTask() {
  const area = AREAS.find((candidate) => candidate.code === elements.area.value);
  const store = stores.find((candidate) => candidate.StoreNumber === elements.store.value);
  const product = (productsByModel[elements.model.value] || []).find((candidate) =>
    candidate.Capacity === elements.capacity.value && candidate.Color === elements.color.value
  );
  if (!area || !store || !product) {
    setFormMessage("请完整选择地区、门店和商品。", true);
    return;
  }

  const duplicate = tasks.some((task) =>
    task.areaCode === area.code &&
    task.store.StoreNumber === store.StoreNumber &&
    task.product.Code === product.Code
  );
  if (duplicate) {
    setFormMessage("这个门店和商品已经在监控列表中。", true);
    return;
  }

  const task = {
    id: crypto.randomUUID(),
    areaCode: area.code,
    areaTitle: area.title,
    store: structuredClone(store),
    product: structuredClone(product)
  };
  tasks = [...tasks, task];
  await chrome.storage.local.set({ tasks });
  await saveSelection();
  setFormMessage("已添加到监控列表。", false);
  render();
}

async function startMonitoring() {
  if (tasks.length === 0) {
    setFormMessage("请先添加至少一个监控任务。", true);
    return;
  }
  setFormMessage("正在打开 Apple 官网监控标签页…", false);
  const result = await chrome.runtime.sendMessage({
    type: "start-monitor",
    tasks,
    barkUrl: elements.bark.value.trim()
  });
  if (!result?.ok) {
    setFormMessage(result?.error || "启动监控失败", true);
    return;
  }
  setFormMessage("监控已启动。", false);
}

async function stopMonitoring() {
  const result = await chrome.runtime.sendMessage({ type: "stop-monitor" });
  if (!result?.ok) {
    setFormMessage(result?.error || "暂停失败", true);
  }
}

async function checkNow() {
  if (!monitoring) {
    setFormMessage("请先开始监控。", true);
    return;
  }
  setFormMessage("正在检查库存…", false);
  const result = await chrome.runtime.sendMessage({ type: "check-now" });
  if (!result?.ok) {
    setFormMessage(result?.error || "检查失败", true);
    return;
  }
  setFormMessage("", false);
}

async function removeTask(taskId) {
  tasks = tasks.filter((task) => task.id !== taskId);
  await chrome.storage.local.set({ tasks });
  render();
}

async function openProduct(task) {
  const result = await chrome.runtime.sendMessage({ type: "open-product", task });
  if (!result?.ok) {
    setFormMessage(result?.error || "无法打开商品页", true);
  }
}

function render() {
  renderTasks();
  renderMonitorState();
  renderLogs();
}

function renderTasks() {
  elements.tasks.replaceChildren();
  elements.emptyTasks.hidden = tasks.length > 0;
  for (const task of tasks) {
    const item = document.createElement("article");
    item.className = "task";

    const content = document.createElement("div");
    const title = document.createElement("h3");
    title.textContent = task.product.Model + " · " + task.product.Capacity;
    const meta = document.createElement("p");
    meta.textContent = task.areaTitle + " · " + task.store.CityStoreName + " · " + task.product.Color;
    const itemState = monitorState?.items?.[task.id];
    const status = document.createElement("span");
    status.className = "task-status " + statusClass(itemState?.status);
    status.textContent = itemState?.status || "等待";
    if (itemState?.detail) {
      status.title = itemState.detail;
    }
    content.append(title, meta, status);

    const actions = document.createElement("div");
    actions.className = "task-actions";
    const openButton = document.createElement("button");
    openButton.type = "button";
    openButton.textContent = "官网";
    openButton.addEventListener("click", () => openProduct(task));
    const removeButton = document.createElement("button");
    removeButton.type = "button";
    removeButton.textContent = "移除";
    removeButton.addEventListener("click", () => removeTask(task.id));
    actions.append(openButton, removeButton);

    item.append(content, actions);
    elements.tasks.append(item);
  }
}

function renderMonitorState() {
  elements.monitorBadge.className = "status-badge";
  if (!monitoring) {
    elements.monitorBadge.classList.add("idle");
    elements.monitorBadge.textContent = "已暂停";
  } else if (monitorState?.checking) {
    elements.monitorBadge.classList.add("checking");
    elements.monitorBadge.textContent = "检查中";
  } else {
    elements.monitorBadge.classList.add("running");
    elements.monitorBadge.textContent = "监控中";
  }
  elements.start.disabled = tasks.length === 0;
  elements.stop.disabled = !monitoring;
  elements.check.disabled = !monitoring;
  elements.lastCheck.textContent = monitorState?.lastCheck
    ? "上次检查 " + formatTime(monitorState.lastCheck)
    : "尚未检查";
}

function renderLogs() {
  elements.logs.replaceChildren();
  const log = Array.isArray(monitorState?.log) ? [...monitorState.log].reverse() : [];
  if (log.length === 0) {
    const empty = document.createElement("p");
    empty.textContent = "启动监控后，这里会显示查询状态。";
    elements.logs.append(empty);
    return;
  }
  for (const entry of log) {
    const row = document.createElement("p");
    row.textContent = "[" + entry.time + "] " + entry.message;
    elements.logs.append(row);
  }
}

function statusClass(status) {
  if (status === "有货") {
    return "in-stock";
  }
  if (status === "需官网验证") {
    return "verify";
  }
  if (status === "查询异常") {
    return "error";
  }
  return "";
}

function setFormMessage(message, isError) {
  elements.formMessage.textContent = message;
  elements.formMessage.style.color = isError ? "var(--red)" : "var(--green)";
}

function formatTime(value) {
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}
