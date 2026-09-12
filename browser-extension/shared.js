export const AREAS = [
  { code: "cn", title: "中国大陆" },
  { code: "hk", title: "香港" },
  { code: "jp", title: "日本" },
  { code: "sg", title: "新加坡" },
  { code: "us", title: "美国" },
  { code: "uk", title: "英国" },
  { code: "au", title: "澳大利亚" }
];

export const CHECK_INTERVAL_MINUTES = 0.5;

const WATCH_PATHS = {
  watchs12: "/shop/buy-watch/apple-watch",
  watchse3: "/shop/buy-watch/apple-watch-se",
  watchultra4: "/shop/buy-watch/apple-watch-ultra"
};

export function appleBaseURL(areaCode) {
  if (areaCode === "cn") {
    return "https://www.apple.com.cn";
  }
  if (areaCode === "us") {
    return "https://www.apple.com";
  }
  if (["hk", "jp", "sg", "uk", "au"].includes(areaCode)) {
    return "https://www.apple.com/" + areaCode;
  }
  throw new Error("不支持的地区: " + areaCode);
}

export function productPurchaseURL(task) {
  const baseURL = appleBaseURL(task.areaCode);
  const defaultWatchPath = WATCH_PATHS[task.product.Type];
  if (defaultWatchPath) {
    const purchasePath = String(task.product.PurchasePath || defaultWatchPath).trim();
    if (purchasePath !== defaultWatchPath && !purchasePath.startsWith(defaultWatchPath + "/")) {
      throw new Error("手表选配地址无效");
    }
    return baseURL + purchasePath;
  }

  const code = String(task.product.Code || "").replace(/^\/+|\/+$/g, "");
  if (!code) {
    throw new Error("产品 SKU 为空");
  }
  return baseURL + "/shop/product/" + code;
}

export function buildFulfillmentPath(tasks) {
  if (!Array.isArray(tasks) || tasks.length === 0) {
    throw new Error("监控任务为空");
  }

  const first = tasks[0];
  for (const task of tasks) {
    if (task.areaCode !== first.areaCode || task.store.StoreNumber !== first.store.StoreNumber) {
      throw new Error("一次请求只能查询同地区、同门店的任务");
    }
  }

  const params = new URLSearchParams({ fae: "true", pl: "true" });
  tasks.forEach((task, index) => {
    params.set("mts." + index, "regular");
    params.set("parts." + index, task.product.Code);
  });

  if (first.areaCode === "jp") {
    params.set("location", first.store.District || first.store.City || first.store.CityStoreName);
    params.set("cppart", "UNLOCKED_JP");
  } else {
    params.set("store", first.store.StoreNumber);
  }

  const localePrefix = ["hk", "jp", "sg", "uk", "au"].includes(first.areaCode)
    ? "/" + first.areaCode
    : "";
  return localePrefix + "/shop/fulfillment-messages?" + params.toString();
}

function findAvailability(stores, storeNumber, productCode) {
  const store = stores.find((candidate) => String(candidate.storeNumber) === String(storeNumber));
  const availability = store?.partsAvailability?.[productCode];
  if (!availability) {
    return null;
  }
  const pickupDisplay = availability.pickupDisplay || availability.messageTypes?.regular?.pickupDisplay || "";
  return { known: pickupDisplay !== "", available: pickupDisplay === "available", pickupDisplay };
}

export function parseStockResponse(payload, storeNumber, productCode) {
  const rootStores = payload?.body?.stores;
  if (Array.isArray(rootStores) && rootStores.length > 0) {
    return findAvailability(rootStores, storeNumber, productCode) ||
      { known: false, available: false, pickupDisplay: "" };
  }

  const pickupMessage = payload?.body?.content?.pickupMessage;
  const stores = pickupMessage?.stores;
  if (Array.isArray(stores) && stores.length > 0) {
    return findAvailability(stores, storeNumber, productCode) ||
      { known: false, available: false, pickupDisplay: "" };
  }

  const eligibility = pickupMessage?.pickupEligibility?.[productCode];
  if (eligibility) {
    const pickupDisplay = eligibility.messageTypes?.regular?.pickupDisplay ||
      eligibility.pickupDisplay ||
      eligibility.messageTypes?.compact?.pickupDisplay || "";
    return { known: pickupDisplay !== "", available: pickupDisplay === "available", pickupDisplay };
  }

  return { known: false, available: false, pickupDisplay: "" };
}

export function unique(values) {
  return [...new Set(values.filter(Boolean))];
}

export function groupProductsByModel(productGroups) {
  const models = {};
  for (const product of Object.values(productGroups || {}).flat()) {
    if (product?.Model) {
      (models[product.Model] ||= []).push(product);
    }
  }
  return models;
}
