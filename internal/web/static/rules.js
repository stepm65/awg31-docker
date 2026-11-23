"use strict";
/* Модель правил маршрутизации: текст <-> блоки (сервисы) без потерь.
 *
 * Текст остаётся единственным хранилищем. Редактор только читает его и пишет обратно:
 *   #@ YouTube | youtube        — заголовок блока: имя и значок (id каталога или slug Simple Icons)
 *   PROXY geosite:youtube       — условие блока (у каждой строки своё действие)
 *   #~ PROXY geoip:telegram     — выключенное условие (движок видит комментарий и пропускает его)
 * Всё остальное — комментарии, пустые строки, строки в запятой-форме, хвостовые «# заметки» —
 * сохраняется как есть, пока строку не изменили в редакторе.
 * Файл без DOM: его можно проверять в node (scripts/test-rules.mjs).
 */
(function (root) {
  var KIND = { domain: "domain", suffix: "domain", "domain-suffix": "domain", full: "full", keyword: "keyword",
    "domain-keyword": "keyword", geosite: "geosite", geoip: "geoip", cidr: "cidr", "ip-cidr": "cidr", ip: "cidr" };
  var PLAIN = { domain: 1, full: 1, keyword: 1, cidr: 1 };

  function normHost(v) {
    v = String(v).trim().toLowerCase();
    if (v.indexOf("*.") === 0) v = v.slice(2);
    if (v.charAt(0) === ".") v = v.slice(1);
    if (v.charAt(v.length - 1) === ".") v = v.slice(0, -1);
    return v;
  }

  function parseAction(s) {
    var m = /^([A-Za-z]+)(?:[@:](.*))?$/.exec(s);
    if (!m) return null;
    var head = m[1].toUpperCase(), target = m[2] === undefined ? "" : m[2].trim();
    if (head === "PROXY" || head === "VPN" || head === "TUNNEL") {
      if (m[2] !== undefined) {
        target = target.replace(/\.conf$/, "");
        if (!/^[A-Za-z0-9_-]{1,15}$/.test(target)) return null;
      }
      return { act: "PROXY", target: target };
    }
    if (m[2] !== undefined) return null;
    if (head === "DIRECT" || head === "BYPASS") return { act: "DIRECT", target: "" };
    if (head === "DROP" || head === "BLOCK" || head === "REJECT") return { act: "DROP", target: "" };
    return null;
  }

  function parseCond(c) {
    c = c.trim();
    var i = c.indexOf(":");
    if (i > 0) {
      var k = KIND[c.slice(0, i).toLowerCase()];
      if (k) {
        var v = c.slice(i + 1).trim();
        if (k === "geosite" || k === "geoip") v = v.toLowerCase();
        else if (k === "cidr") v = v.toLowerCase();
        else v = normHost(v);
        return { kind: k, value: v };
      }
    }
    if (/^[0-9a-f:.]+(\/\d{1,3})?$/i.test(c) && (c.indexOf(":") >= 0 || /^\d+\.\d+\.\d+\.\d+(\/\d+)?$/.test(c))) return { kind: "cidr", value: c.toLowerCase() };
    return { kind: "domain", value: normHost(c) };
  }

  // Одна строка правила, как её понимает движок: «ДЕЙСТВИЕ УСЛОВИЕ [# заметка]» или «ДЕЙСТВИЕ,УСЛОВИЕ[,…]».
  function parseLine(s) {
    var hash = s.indexOf("#");
    var body = (hash < 0 ? s : s.slice(0, hash)).trim();
    var note = hash < 0 ? "" : s.slice(hash + 1).trim();
    if (!body) return null;
    var act, cond;
    if (body.indexOf(",") >= 0) {
      var p = body.split(",");
      act = p[0].trim(); cond = (p[1] || "").trim();
    } else {
      var f = body.split(/\s+/);
      act = f[0]; cond = f[1] || "";
    }
    var a = parseAction(act);
    if (!a || !cond) return null;
    var c = parseCond(cond);
    return { act: a.act, target: a.target, kind: c.kind, value: c.value, off: false, offPre: null, note: note, raw: null, pre: [] };
  }

  var destKey = function (l) { return l.act === "PROXY" && l.target ? "PROXY@" + l.target : l.act; };
  var condKey = function (l) { return l.kind + ":" + l.value; };
  var isPlain = function (l) { return !!PLAIN[l.kind]; };

  function lineText(l) {
    var s = l.raw !== null ? l.raw : destKey(l) + " " + condKey(l) + (l.note ? " # " + l.note : "");
    return l.off ? (l.offPre !== null ? l.offPre : "#~ ") + s : s;
  }

  function cleanName(s) { return String(s || "").replace(/\s*[|\r\n]+\s*/g, " ").trim(); }
  function headText(b) {
    var name = cleanName(b.name) || cleanName(b.icon) || "rule", icon = cleanName(b.icon);
    return "#@ " + name + (icon ? " | " + icon : "");
  }

  function parse(text) {
    text = String(text || "").replace(/\r\n?/g, "\n");
    var eol = text === "" || text.charAt(text.length - 1) === "\n";
    var src = text.split("\n");
    if (src[src.length - 1] === "") src.pop();
    var items = [], cur = null, pend = [];

    function flush() { for (var i = 0; i < pend.length; i++) items.push({ t: "raw", text: pend[i] }); pend = []; }
    function lastBlock() { var it = items[items.length - 1]; return it && it.t === "block" ? it : null; }
    function mergeable(b, l) {
      if (b.lines[0].off !== l.off || destKey(b.lines[0]) !== destKey(l) || !isPlain(l)) return false;
      for (var i = 0; i < b.lines.length; i++) if (!isPlain(b.lines[i])) return false;
      return true;
    }
    function addLine(l) {
      if (cur && cur.head) { l.pre = pend; pend = []; cur.lines.push(l); return; }
      if (cur && pend.length === 0 && mergeable(cur, l)) { cur.lines.push(l); return; }
      var b = { t: "block", head: false, name: "", icon: "", hraw: null, pre: pend, lines: [l], trail: 0 };
      pend = []; items.push(b); cur = b;
    }

    for (var n = 0; n < src.length; n++) {
      var ln = src[n], tr = ln.trim(), m;
      if (tr === "") {
        flush(); cur = null;
        var lb = lastBlock();
        if (lb) lb.trail++; else items.push({ t: "raw", text: "" });
        continue;
      }
      if ((m = /^\s*#@(.*)$/.exec(ln))) {
        var parts = m[1].split("|");
        var b = { t: "block", head: true, name: parts[0].trim(), icon: (parts[1] || "").trim(), hraw: ln, pre: pend, lines: [], trail: 0 };
        pend = []; items.push(b); cur = b;
        continue;
      }
      if ((m = /^(\s*#~ ?)(.*)$/.exec(ln))) {
        var d = parseLine(m[2]);
        if (d) { d.off = true; d.offPre = m[1]; d.raw = m[2]; addLine(d); continue; }
      }
      if (tr.charAt(0) === "#") {
        if (!(cur && cur.head)) cur = null;
        pend.push(ln);
        continue;
      }
      var l = parseLine(ln);
      if (!l) { flush(); cur = null; items.push({ t: "raw", text: ln, bad: true }); continue; }
      l.raw = ln;
      addLine(l);
    }
    flush();
    return { items: items, eol: eol };
  }

  function serialize(m) {
    var out = [];
    m.items.forEach(function (it) {
      if (it.t === "raw") { out.push(it.text); return; }
      it.pre.forEach(function (p) { out.push(p); });
      if (it.head) out.push(it.hraw !== null ? it.hraw : headText(it));
      it.lines.forEach(function (l) { l.pre.forEach(function (p) { out.push(p); }); out.push(lineText(l)); });
      for (var i = 0; i < it.trail; i++) out.push("");
    });
    return out.join("\n") + (out.length && m.eol !== false ? "\n" : "");
  }

  var blocks = function (m) { return m.items.filter(function (it) { return it.t === "block"; }); };
  var bad = function (m) { return m.items.filter(function (it) { return it.t === "raw" && it.bad; }); };

  // Назначение блока: «PROXY», «PROXY@имя», «DIRECT», «DROP» или «mixed», если строки разошлись.
  function dest(b) {
    var k = "";
    for (var i = 0; i < b.lines.length; i++) {
      var x = destKey(b.lines[i]);
      if (k && x !== k) return "mixed";
      k = x;
    }
    return k || "PROXY";
  }
  function setDest(b, key) {
    var a = parseAction(key);
    if (!a) return;
    b.lines.forEach(function (l) { l.act = a.act; l.target = a.target; l.raw = null; });
  }
  var isOff = function (b) { return b.lines.length > 0 && b.lines.every(function (l) { return l.off; }); };
  var setOff = function (b, off) { b.lines.forEach(function (l) { l.off = !!off; }); };

  function conds(b) { return b.lines.map(function (l) { return { kind: l.kind, value: l.value, off: l.off, key: condKey(l) }; }); }
  function hasCond(b, cond) {
    var c = parseCond(cond), k = c.kind + ":" + c.value;
    return b.lines.some(function (l) { return condKey(l) === k; });
  }
  function addCond(b, cond) {
    var d = dest(b), key = d === "mixed" ? destKey(b.lines[0]) : d;
    var l = parseLine(key + " " + cond);
    if (!l || hasCond(b, cond)) return false;
    l.off = isOff(b);
    b.lines.push(l);
    return true;
  }
  function removeCond(b, i) { b.lines.splice(i, 1); }

  // Новый блок: условия добавляются как есть, затем всем строкам ставится назначение.
  function newBlock(o) {
    var b = { t: "block", head: true, name: cleanName(o.name), icon: cleanName(o.icon), hraw: null, pre: [], lines: [], trail: 1 };
    (o.conds || []).forEach(function (c) { addCond(b, c); });
    setDest(b, o.dest || "PROXY");
    return b;
  }

  function add(m, b, at) {
    if (at === undefined || at === null || at >= m.items.length) {
      var last = m.items[m.items.length - 1];
      if (last && last.t === "block" && last.trail === 0) last.trail = 1;
      m.items.push(b);
    } else {
      m.items.splice(at, 0, b);
    }
  }
  function remove(m, b) { var i = m.items.indexOf(b); if (i >= 0) m.items.splice(i, 1); }
  // Переставляет блок перед блоком «before» (null — в конец). Комментарии блока едут вместе с ним.
  function move(m, b, before) {
    var i = m.items.indexOf(b);
    if (i < 0 || b === before) return;
    m.items.splice(i, 1);
    var j = before ? m.items.indexOf(before) : -1;
    if (j < 0) m.items.push(b); else m.items.splice(j, 0, b);
  }
  function step(m, b, dir) {
    var list = blocks(m), i = list.indexOf(b), j = i + dir;
    if (i < 0 || j < 0 || j >= list.length) return;
    if (dir < 0) move(m, b, list[j]); else move(m, b, list[j + 1] || null);
  }
  function clone(b) {
    return { t: "block", head: true, name: b.name, icon: b.icon, hraw: null, pre: [], trail: 1,
      lines: b.lines.map(function (l) { return { act: l.act, target: l.target, kind: l.kind, value: l.value, off: l.off, offPre: null, note: l.note, raw: null, pre: [] }; }) };
  }

  // Какой записи каталога соответствует блок: по значку или по тому же набору условий.
  function service(b, catalog) {
    var i, c;
    if (b.icon) for (i = 0; i < catalog.length; i++) if (catalog[i].id === b.icon) return catalog[i];
    var have = b.lines.map(condKey).sort().join(" ");
    if (!have) return null;
    for (i = 0; i < catalog.length; i++) {
      c = catalog[i].conds.map(function (x) { var p = parseCond(x); return p.kind + ":" + p.value; }).sort().join(" ");
      if (c === have) return catalog[i];
    }
    return null;
  }

  // Правила в том виде, как их увидит движок: только включённые строки, по порядку.
  function effective(m) {
    var out = [];
    m.items.forEach(function (it) {
      if (it.t !== "block") return;
      it.lines.forEach(function (l) { if (!l.off) out.push(destKey(l) + " " + condKey(l)); });
    });
    return out;
  }

  var api = { parse: parse, serialize: serialize, parseLine: parseLine, parseCond: parseCond, parseAction: parseAction,
    blocks: blocks, bad: bad, dest: dest, setDest: setDest, isOff: isOff, setOff: setOff, conds: conds, hasCond: hasCond,
    addCond: addCond, removeCond: removeCond, newBlock: newBlock, add: add, remove: remove, move: move, step: step,
    clone: clone, service: service, effective: effective, destKey: destKey };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.AWG_RULES = api;
})(typeof window !== "undefined" ? window : globalThis);
