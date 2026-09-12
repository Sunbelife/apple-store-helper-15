import assert from "node:assert/strict";
import test from "node:test";
import {
  appleBaseURL,
  buildFulfillmentPath,
  groupProductsByModel,
  parseStockResponse,
  productPurchaseURL
} from "./shared.js";

const task = {
  areaCode: "cn",
  store: { StoreNumber: "R761", CityStoreName: "深圳万象城", District: "518001" },
  product: {
    Model: "iPhone 18 Pro",
    Capacity: "256GB",
    Color: "Silver",
    Code: "MJT84CH/A",
    Type: "iphone18pro"
  }
};

test("builds locale-specific product URLs", () => {
  assert.equal(appleBaseURL("cn"), "https://www.apple.com.cn");
  assert.equal(appleBaseURL("hk"), "https://www.apple.com/hk");
  assert.equal(productPurchaseURL(task), "https://www.apple.com.cn/shop/product/MJT84CH/A");
});

test("builds locale-specific inventory paths", () => {
  const cnPath = buildFulfillmentPath([task]);
  assert.match(cnPath, /^\/shop\/fulfillment-messages\?/);
  assert.match(cnPath, /parts\.0=MJT84CH%2FA/);
  assert.match(cnPath, /store=R761/);
  const hkPath = buildFulfillmentPath([{ ...task, areaCode: "hk" }]);
  assert.match(hkPath, /^\/hk\/shop\/fulfillment-messages\?/);
});

test("parses an available store response", () => {
  const payload = {
    body: {
      stores: [{
        storeNumber: "R761",
        partsAvailability: {
          "MJT84CH/A": { pickupDisplay: "available" }
        }
      }]
    }
  };
  assert.deepEqual(
    parseStockResponse(payload, "R761", "MJT84CH/A"),
    { known: true, available: true, pickupDisplay: "available" }
  );
});

test("separates products by their real model name", () => {
  const products = groupProductsByModel({
    "iPhone 18 Pro": [
      { Model: "iPhone 18 Pro Max", Code: "MAX" },
      { Model: "iPhone 18 Pro", Code: "PRO" }
    ]
  });
  assert.deepEqual(Object.keys(products), ["iPhone 18 Pro Max", "iPhone 18 Pro"]);
  assert.equal(products["iPhone 18 Pro"][0].Code, "PRO");
});
