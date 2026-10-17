import React, { useEffect, useRef, useState } from "react";
import QRCode from "qrcode";
import {
  ArrowUpRight,
  ChevronLeft,
  ChevronRight,
  QrCode,
  X,
} from "lucide-react";
import products from "./shop-products.json";
import storeLogo from "./assets/store-logo.png";

const images = import.meta.glob("./assets/products/*.webp", {
  eager: true,
  import: "default",
});

export default function ShopRail({ onOpen, onCopy }) {
  const rail = useRef(null);
  const [purchase, setPurchase] = useState(null);
  const [qr, setQR] = useState("");
  const [qrError, setQRError] = useState(false);
  useEffect(() => {
    let cancelled = false;
    setQR("");
    setQRError(false);
    if (!purchase) return;
    QRCode.toDataURL(purchase.url, {
      width: 260,
      margin: 1,
      errorCorrectionLevel: "M",
    })
      .then((value) => {
        if (!cancelled) setQR(value);
      })
      .catch(() => {
        if (!cancelled) setQRError(true);
      });
    const escape = (e) => {
      if (e.key === "Escape") setPurchase(null);
    };
    window.addEventListener("keydown", escape);
    return () => {
      cancelled = true;
      window.removeEventListener("keydown", escape);
    };
  }, [purchase]);
  const move = (direction) =>
    rail.current?.scrollBy({
      left: direction * 430,
      behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches
        ? "instant"
        : "smooth",
    });
  return (
    <>
      <section className="shop-rack" aria-label="老孙的店最新商品">
        <header className="rack-heading">
          <div className="rack-title">
            <img src={storeLogo} alt="" />
            <strong>老孙的店</strong>
            <span>最近上新</span>
          </div>
          <p>帮你抢到手机，也欢迎来店里逛逛。</p>
          <div className="rack-arrows">
            <button
              className="icon-button small"
              aria-label="上一组商品"
              onClick={() => move(-1)}
            >
              <ChevronLeft size={16} />
            </button>
            <button
              className="icon-button small"
              aria-label="下一组商品"
              onClick={() => move(1)}
            >
              <ChevronRight size={16} />
            </button>
          </div>
        </header>
        <div className="product-rail" ref={rail}>
          {products.map((p) => (
            <article className="featured-product" key={p.id}>
              <button
                className="product-buy"
                onClick={() => setPurchase(p)}
                aria-label={p.name + "，微信扫码购买"}
              >
                <img
                  src={images["./assets/products/" + p.image]}
                  alt={p.name}
                />
                <span className="product-copy">
                  <strong>{p.name}</strong>
                  <span className="product-price">¥{p.price}</span>
                  <span className="product-destination">
                    微信扫码购买 <ArrowUpRight size={11} />
                  </span>
                </span>
              </button>
              <button
                className="product-qr"
                onClick={() => setPurchase(p)}
                aria-label={"扫码购买 " + p.name}
                title="微信扫码购买"
              >
                <QrCode size={15} />
              </button>
            </article>
          ))}
        </div>
        <div className="rack-caption">
          如果帮到你抢到心仪的
          iPhone，希望你也来我们店里买点喜欢的东西。商品及价格以小程序为准。
        </div>
      </section>
      {purchase && (
        <div className="overlay" onClick={() => setPurchase(null)}>
          <section
            className="dialog purchase-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="purchase-title"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              autoFocus
              className="icon-button dialog-close"
              aria-label="关闭购买二维码"
              onClick={() => setPurchase(null)}
            >
              <X size={18} />
            </button>
            <img className="purchase-logo" src={storeLogo} alt="老孙的店" />
            <h2 id="purchase-title">微信小程序购买</h2>
            <p>{purchase.name}</p>
            <div className="purchase-code">
              {qr ? (
                <img src={qr} alt={"微信扫码购买 " + purchase.name} />
              ) : (
                <p>
                  {qrError
                    ? "二维码暂不可用，请点击下方购买按钮。"
                    : "正在生成二维码…"}
                </p>
              )}
            </div>
            <p>手机微信扫一扫，进入这件商品的小程序页面。</p>
            <button
              className="button primary"
              onClick={() => onCopy(purchase.id)}
            >
              复制小程序购买链接 <ArrowUpRight size={15} />
            </button>
            <button className="text-button" onClick={() => onOpen(purchase.id)}>
              用浏览器打开链接
            </button>
          </section>
        </div>
      )}
    </>
  );
}
