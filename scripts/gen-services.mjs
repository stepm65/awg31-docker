// Собирает static/services.js и static/services.css: каталог популярных сервисов для редактора
// правил маршрутизации и SVG-пути иконок из Simple Icons (CC0-1.0).
//
//   cd /tmp && npm init -y && npm install simple-icons
//   node scripts/gen-services.mjs /tmp/node_modules/simple-icons
//
// Сами пути иконок вшиваются в services.js: внешних запросов у панели нет (CSP).
// Знаки сервисов принадлежат их владельцам; здесь они лишь подписывают правила.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const lib = process.argv[2] || "node_modules/simple-icons";
const meta = JSON.parse(fs.readFileSync(path.join(lib, "data/simple-icons.json"), "utf8"));
const version = JSON.parse(fs.readFileSync(path.join(lib, "package.json"), "utf8")).version;
const hexOf = Object.fromEntries(meta.map((m) => [m.slug, m.hex]));

// id, название, slug иконки (null — буквенная плитка), условия правила
const CATALOG = [
  ["youtube", "YouTube", "youtube", ["geosite:youtube"]],
  ["telegram", "Telegram", "telegram", ["geosite:telegram", "geoip:telegram"]],
  ["google", "Google", "google", ["geosite:google"]],
  ["gemini", "Google Gemini", "googlegemini", ["geosite:google-gemini"]],
  ["openai", "OpenAI", null, ["geosite:openai"]],
  ["anthropic", "Claude (Anthropic)", "anthropic", ["geosite:anthropic"]],
  ["netflix", "Netflix", "netflix", ["geosite:netflix", "geoip:netflix"]],
  ["instagram", "Instagram", "instagram", ["geosite:instagram"]],
  ["facebook", "Facebook", "facebook", ["geosite:facebook", "geoip:facebook"]],
  ["twitter", "X (Twitter)", "x", ["geosite:twitter", "geoip:twitter"]],
  ["tiktok", "TikTok", "tiktok", ["geosite:tiktok"]],
  ["spotify", "Spotify", "spotify", ["geosite:spotify"]],
  ["discord", "Discord", "discord", ["geosite:discord"]],
  ["whatsapp", "WhatsApp", "whatsapp", ["geosite:whatsapp"]],
  ["signal", "Signal", "signal", ["geosite:signal"]],
  ["microsoft", "Microsoft", null, ["geosite:microsoft"]],
  ["steam", "Steam", "steam", ["geosite:steam"]],
  ["github", "GitHub", "github", ["geosite:github"]],
  ["twitch", "Twitch", "twitch", ["geosite:twitch"]],
  ["linkedin", "LinkedIn", null, ["geosite:linkedin"]],
  ["apple", "Apple", "apple", ["geosite:apple"]],
  ["amazon", "Amazon", null, ["geosite:amazon"]],
  ["cloudflare", "Cloudflare", "cloudflare", ["geosite:cloudflare", "geoip:cloudflare"]],
  ["reddit", "Reddit", "reddit", ["geosite:reddit"]],
  ["zoom", "Zoom", "zoom", ["geosite:zoom"]],
  ["dropbox", "Dropbox", "dropbox", ["geosite:dropbox"]],
  ["wikipedia", "Wikipedia", "wikipedia", ["geosite:wikipedia"]],
  ["ru", "Россия (IP)", null, ["geoip:ru"]],
  ["private", "Локальные сети", null, ["geoip:private"]],
];

// Тёмные цвета знаков на тёмной теме не видны: для них берём нейтральный светлый.
const lum = (hex) => {
  const c = [0, 2, 4].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
};

const icons = {}, css = [];
const catalog = CATALOG.map(([id, name, slug, conds]) => {
  let icon = null;
  if (slug) {
    const f = path.join(lib, "icons", slug + ".svg");
    if (fs.existsSync(f)) {
      const m = /<path d="([^"]+)"/.exec(fs.readFileSync(f, "utf8"));
      if (m) { icons[slug] = m[1]; icon = slug; }
    }
  }
  if (icon) {
    const hex = hexOf[slug] || "b4bac3";
    css.push(`.svc-${id}{--svc:#${lum(hex) < 0.12 ? "b4bac3" : hex}}`);
  }
  return { id, name, icon, conds };
});

const js = `"use strict";
// Создано scripts/gen-services.mjs (Simple Icons ${version}, CC0-1.0). Не править руками.
window.AWG_SERVICES = ${JSON.stringify({ catalog, icons })};
`;
fs.writeFileSync(path.join(root, "internal/web/static/services.js"), js);
fs.writeFileSync(path.join(root, "internal/web/static/services.css"),
  `/* Создано scripts/gen-services.mjs: цвета знаков сервисов. Не править руками. */\n${css.join("\n")}\n`);
console.log(`services: ${catalog.length}, иконок: ${Object.keys(icons).length}, simple-icons ${version}`);
