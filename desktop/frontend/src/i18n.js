import { useEffect, useState } from "react";
import translations from "./translations.json" with { type: "json" };
export function resolveLanguage(mode, languages = []) {
  if (mode === "zh" || mode === "en") return mode;
  return /^zh(?:-|_|$)/i.test(languages[0] || "en") ? "zh" : "en";
}
export function readLanguageMode() {
  try {
    const value = localStorage.getItem("qnm-language");
    return ["zh", "en"].includes(value) ? value : "auto";
  } catch {
    return "auto";
  }
}
let language = resolveLanguage(
  readLanguageMode(),
  globalThis.navigator?.languages || [globalThis.navigator?.language],
);
export const locale = () => (language === "zh" ? "zh-CN" : "en-US");
const phrases = Object.keys(translations)
  .filter(
    (key) => key.length >= 4 || key.endsWith("色") || key.endsWith("金属"),
  )
  .sort((a, b) => b.length - a.length);
const escaped = phrases.map((key) =>
  key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"),
);
const phrasePattern = new RegExp(escaped.join("|"), "g");
export function translate(value, targetLanguage) {
  if (targetLanguage !== "en" || typeof value !== "string") return value;
  if (Object.prototype.hasOwnProperty.call(translations, value))
    return translations[value];
  const trimmed = value.trim();
  if (Object.prototype.hasOwnProperty.call(translations, trimmed))
    return value.replace(trimmed, translations[trimmed]);
  return value.replace(phrasePattern, (key) => translations[key]);
}
export function t(value) {
  return translate(value, language);
}
export function useLanguage() {
  const [mode, setMode] = useState(readLanguageMode);
  const [languages, setLanguages] = useState(
    () => navigator.languages || [navigator.language],
  );
  language = resolveLanguage(mode, languages);
  useEffect(() => {
    document.documentElement.lang = locale();
    try {
      localStorage.setItem("qnm-language", mode);
    } catch {}
  }, [mode, languages]);
  useEffect(() => {
    const changed = () =>
      setLanguages(navigator.languages || [navigator.language]);
    window.addEventListener("languagechange", changed);
    return () => window.removeEventListener("languagechange", changed);
  }, []);
  return [mode, setMode, language];
}
