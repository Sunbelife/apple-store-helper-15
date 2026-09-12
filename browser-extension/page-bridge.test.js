import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";

const bridgeSource = await readFile(new URL("./page-bridge.js", import.meta.url), "utf8");

function createPage({ cookie = "", fetchStatuses = [200], runShield } = {}) {
  const listeners = new Map();
  const responses = [];
  const fetchCalls = [];
  let fetchIndex = 0;

  const window = {
    location: {
      origin: "https://www.apple.com",
      hostname: "www.apple.com",
      pathname: "/hk/shop/product/test"
    },
    addEventListener(type, listener) {
      const entries = listeners.get(type) || [];
      entries.push(listener);
      listeners.set(type, entries);
    },
    removeEventListener(type, listener) {
      const entries = listeners.get(type) || [];
      listeners.set(type, entries.filter((entry) => entry !== listener));
    },
    dispatchEvent(event) {
      for (const listener of [...(listeners.get(event.type) || [])]) {
        listener(event);
      }
    },
    postMessage(message) {
      responses.push(message);
    },
    async fetch(url, options) {
      fetchCalls.push({ url, options });
      const status = fetchStatuses[Math.min(fetchIndex, fetchStatuses.length - 1)];
      fetchIndex += 1;
      return {
        status,
        async text() {
          return status === 200 ? '{"ok":true}' : "";
        }
      };
    }
  };
  const document = {
    get cookie() {
      return cookie;
    },
    set cookie(value) {
      const pair = value.split(";", 1)[0];
      if (value.includes("Max-Age=0")) {
        cookie = cookie
          .split(";")
          .filter((item) => !item.trim().startsWith("shld_bt_ck="))
          .join(";");
      } else {
        cookie = pair;
      }
    }
  };

  if (runShield) {
    window.__shldRun = () => runShield({ window, document });
  }

  vm.runInNewContext(bridgeSource, {
    URL,
    clearTimeout,
    document,
    Promise,
    setTimeout,
    window
  });

  async function request(id = "request-1") {
    window.dispatchEvent({
      type: "message",
      source: window,
      data: {
        source: "qiangnimei18-page-bridge",
        direction: "request",
        id,
        path: "/shop/fulfillment-messages?store=R001"
      }
    });
    for (let attempt = 0; attempt < 100 && responses.length === 0; attempt += 1) {
      await new Promise((resolve) => setTimeout(resolve, 5));
    }
    return responses.find((response) => response.id === id);
  }

  return { document, fetchCalls, request, responses, window };
}

test("uses an existing Apple Shield cookie without rerunning Shield", async () => {
  let shieldCalls = 0;
  const page = createPage({
    cookie: "shld_bt_ck=present",
    runShield() {
      shieldCalls += 1;
    }
  });

  const response = await page.request();

  assert.equal(response.status, 200);
  assert.equal(page.fetchCalls.length, 1);
  assert.equal(shieldCalls, 0);
});

test("reruns Apple Shield once and retries inventory once after 541", async () => {
  let shieldCalls = 0;
  let staleCookieWasCleared = false;
  const page = createPage({
    cookie: "shld_bt_ck=stale",
    fetchStatuses: [541, 200],
    runShield({ window, document }) {
      shieldCalls += 1;
      staleCookieWasCleared = !document.cookie.includes("shld_bt_ck=");
      document.cookie = "shld_bt_ck=refreshed";
      window.dispatchEvent({ type: "shldDone", detail: { id: "shld-result" } });
    }
  });

  const response = await page.request();

  assert.equal(response.status, 200);
  assert.equal(page.fetchCalls.length, 2);
  assert.equal(shieldCalls, 1);
  assert.equal(staleCookieWasCleared, true);
  assert.equal(response.shield.initial, "shld-cookie");
  assert.equal(response.shield.retry, "shld-result");
  assert.equal(response.shield.retried, true);
});

test("keeps 541 when Apple Shield reports that no cookie was created", async () => {
  let shieldCalls = 0;
  const page = createPage({
    cookie: "shld_bt_ck=stale",
    fetchStatuses: [541],
    runShield({ window }) {
      shieldCalls += 1;
      window.dispatchEvent({ type: "shldDone", detail: { id: "shld-no-ck" } });
    }
  });

  const response = await page.request();

  assert.equal(response.status, 541);
  assert.equal(page.fetchCalls.length, 1);
  assert.equal(shieldCalls, 1);
});
