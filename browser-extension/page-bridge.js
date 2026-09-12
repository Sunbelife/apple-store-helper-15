(() => {
  if (window.__QIANGNIMEI18_BRIDGE__) {
    return;
  }
  window.__QIANGNIMEI18_BRIDGE__ = true;

  const SOURCE = "qiangnimei18-page-bridge";

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

      const response = await window.fetch(requestURL.href, {
        method: "GET",
        credentials: "include",
        headers: {
          Accept: "application/json, text/javascript, */*; q=0.01",
          "X-Requested-With": "XMLHttpRequest"
        }
      });

      const body = response.status === 200 ? await response.text() : "";
      window.postMessage({
        source: SOURCE,
        direction: "response",
        id: message.id,
        status: response.status,
        body
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
