(() => {
  if (globalThis.__QIANGNIMEI18_CONTENT__) {
    return;
  }
  globalThis.__QIANGNIMEI18_CONTENT__ = true;

  const SOURCE = "qiangnimei18-page-bridge";
  const pending = new Map();

  window.addEventListener("message", (event) => {
    const message = event.data;
    if (event.source !== window || message?.source !== SOURCE || message?.direction !== "response") {
      return;
    }
    const request = pending.get(message.id);
    if (!request) {
      return;
    }
    clearTimeout(request.timer);
    pending.delete(message.id);
    request.resolve(message);
  });

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message?.type === "ping") {
      sendResponse({ ok: true });
      return false;
    }
    if (message?.type !== "stock-check") {
      return false;
    }

    const id = crypto.randomUUID();
    const responsePromise = new Promise((resolve) => {
      const timer = setTimeout(() => {
        pending.delete(id);
        resolve({ status: 0, error: "Apple 页面请求超时" });
      }, 25000);
      pending.set(id, { resolve, timer });
    });

    window.postMessage({
      source: SOURCE,
      direction: "request",
      id,
      path: message.path
    }, window.location.origin);

    responsePromise.then(sendResponse);
    return true;
  });
})();
