(() => {
  if (window.__QIANGNIMEI18_BRIDGE__) {
    return;
  }
  window.__QIANGNIMEI18_BRIDGE__ = true;

  const SOURCE = "qiangnimei18-page-bridge";
  const SHIELD_COOKIE = "shld_bt_ck";
  const SHIELD_TIMEOUT_MS = 20_000;
  let shieldAttempt = null;

  function hasShieldCookie() {
    return document.cookie
      .split(";")
      .some((item) => {
        const entry = item.trim();
        return entry.startsWith(SHIELD_COOKIE + "=") && entry.length > SHIELD_COOKIE.length + 1;
      });
  }

  function clearShieldCookie() {
    const paths = new Set(["/", "/shop"]);
    const area = window.location.pathname.match(/^\/(hk|jp|sg|uk|au)(?:\/|$)/)?.[1];
    if (area) {
      paths.add("/" + area);
      paths.add("/" + area + "/shop");
    }

    const rootDomain = window.location.hostname.replace(/^www\./, "");
    const domains = ["", "; Domain=" + window.location.hostname];
    if (rootDomain !== window.location.hostname) {
      domains.push("; Domain=." + rootDomain);
    }

    for (const path of paths) {
      for (const domain of domains) {
        document.cookie = SHIELD_COOKIE + "=; Max-Age=0; path=" + path + domain + "; Secure";
      }
    }
  }

  function ensureShieldReady(force = false) {
    if (!force && hasShieldCookie()) {
      return Promise.resolve({ ready: true, id: "shld-cookie" });
    }
    if (shieldAttempt) {
      return shieldAttempt;
    }

    shieldAttempt = new Promise((resolve) => {
      let retryTimer = null;
      let settled = false;

      const finish = (result) => {
        if (settled) {
          return;
        }
        settled = true;
        clearTimeout(timeoutTimer);
        clearTimeout(retryTimer);
        window.removeEventListener("shldDone", onShieldDone);
        resolve(result);
      };

      const onShieldDone = (event) => {
        const id = String(event?.detail?.id || "shld-error");
        finish({
          ready: id === "shld-result" && hasShieldCookie(),
          id
        });
      };

      const runOfficialShield = () => {
        if (typeof window.__shldRun !== "function") {
          retryTimer = setTimeout(runOfficialShield, 100);
          return;
        }
        Promise.resolve(window.__shldRun()).catch(() => {
          finish({ ready: false, id: "shld-error" });
        });
      };

      const timeoutTimer = setTimeout(() => {
        finish({
          ready: hasShieldCookie(),
          id: hasShieldCookie() ? "shld-cookie" : "shld-timeout"
        });
      }, SHIELD_TIMEOUT_MS);

      window.addEventListener("shldDone", onShieldDone);
      runOfficialShield();
    }).finally(() => {
      shieldAttempt = null;
    });

    return shieldAttempt;
  }

  async function fetchStock(requestURL) {
    return window.fetch(requestURL.href, {
      method: "GET",
      credentials: "include",
      headers: {
        Accept: "application/json, text/javascript, */*; q=0.01",
        "X-Requested-With": "XMLHttpRequest"
      }
    });
  }

  window.addEventListener("message", async (event) => {
    const message = event.data;
    if (event.source !== window || message?.source !== SOURCE || message?.direction !== "request") {
      return;
    }

    try {
      const requestURL = new URL(message.path, window.location.origin);
      const allowedPath = /^\/(?:(?:hk|jp|sg|uk|au)\/)?shop\/fulfillment-messages$/;
      if (requestURL.origin !== window.location.origin || !allowedPath.test(requestURL.pathname)) {
        throw new Error("仅允许请求 Apple 库存接口");
      }

      const shield = {
        initial: (await ensureShieldReady()).id,
        retry: "",
        retried: false
      };
      let response = await fetchStock(requestURL);

      if (response.status === 541) {
        clearShieldCookie();
        const shieldResult = await ensureShieldReady(true);
        shield.retry = shieldResult.id;
        if (shieldResult.ready) {
          response = await fetchStock(requestURL);
          shield.retried = true;
        }
      }

      const body = response.status === 200 ? await response.text() : "";
      window.postMessage({
        source: SOURCE,
        direction: "response",
        id: message.id,
        status: response.status,
        body,
        shield
      }, window.location.origin);
    } catch (error) {
      window.postMessage({
        source: SOURCE,
        direction: "response",
        id: message.id,
        status: 0,
        error: error instanceof Error ? error.message : String(error)
      }, window.location.origin);
    }
  });
})();
