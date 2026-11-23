// Проверка модели правил редактора (internal/web/static/rules.js) без браузера.
//   node scripts/test-rules.mjs          — проверки; код выхода 0 при успехе
//   node scripts/test-rules.mjs --dump   — для Go-теста: сверка с движком policy.Parse
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import assert from "node:assert/strict";

const require = createRequire(import.meta.url);
const R = require("../internal/web/static/rules.js");
const corpus = JSON.parse(readFileSync(new URL("../internal/web/testdata/rules_corpus.json", import.meta.url), "utf8"));
const all = [...corpus.valid, ...corpus.loose];

// Вариации: что получится после действий в редакторе.
function variants(text) {
  const out = [];
  const keep = (name, m) => out.push({ name, text: R.serialize(m), effective: R.effective(m) });
  let m = R.parse(text); keep("orig", m);
  m = R.parse(text); R.blocks(m).forEach((b) => R.setOff(b, true)); keep("alloff", m);
  m = R.parse(text); R.blocks(m).forEach((b) => R.setOff(b, false)); keep("allon", m);
  m = R.parse(text); R.blocks(m).forEach((b) => R.setDest(b, "DIRECT")); keep("direct", m);
  m = R.parse(text); R.blocks(m).forEach((b) => R.setDest(b, "PROXY@awg9")); keep("tunnel", m);
  m = R.parse(text); [...R.blocks(m)].reverse().forEach((b, i, a) => R.move(m, b, null)); keep("reversed", m);
  m = R.parse(text); { const b = R.blocks(m)[0]; if (b) R.step(m, b, 1); } keep("down", m);
  m = R.parse(text); R.add(m, R.newBlock({ name: "Новый | блок", icon: "x", conds: ["geosite:new", "Example.org", "10.0.0.5/8"], dest: "DROP" })); keep("added", m);
  return out;
}

if (process.argv.includes("--dump")) {
  process.stdout.write(JSON.stringify(corpus.valid.map((text) => ({ text, variants: variants(text) }))));
  process.exit(0);
}

// 1. Текст без правок возвращается байт в байт.
for (const t of all) {
  assert.equal(R.serialize(R.parse(t)), t, "круговой обход: " + JSON.stringify(t));
}

// 2. Повторный разбор канонического текста даёт тот же текст, а правки не трогают чужие строки.
for (const t of all) for (const v of variants(t)) {
  assert.equal(R.serialize(R.parse(v.text)), v.text, `идемпотентность ${v.name}: ` + JSON.stringify(t));
  assert.deepEqual(R.effective(R.parse(v.text)), v.effective, `эффективные правила ${v.name}: ` + JSON.stringify(t));
}

// 3. Структура: заголовок, выключенные строки, неявные блоки.
{
  const m = R.parse(corpus.valid[2]);
  const b = R.blocks(m);
  assert.equal(b.length, 2);
  assert.deepEqual([b[0].name, b[0].icon, b[0].head], ["YouTube", "youtube", true]);
  assert.equal(R.dest(b[0]), "PROXY");
  assert.deepEqual(R.conds(b[1]).map((c) => [c.key, c.off]), [["geosite:telegram", false], ["geoip:telegram", true]]);
  assert.equal(R.isOff(b[1]), false);
  assert.deepEqual(R.effective(m), ["PROXY geosite:youtube", "PROXY domain:youtu.be", "PROXY geosite:telegram"]);
}
{
  const b = R.blocks(R.parse(corpus.valid[5]));
  assert.equal(b.length, 4, "обычные домены с одним действием склеиваются, geosite — по одной строке");
  assert.equal(b[0].lines.length, 2);
  assert.equal(b[0].head, false);
}
{
  const b = R.blocks(R.parse(corpus.valid[9]))[0];
  assert.equal(R.dest(b), "mixed");
  assert.equal(b.lines[2].pre.length, 1, "комментарий внутри блока едет со строкой");
}

// 4. Правка одного блока не меняет остальной текст.
{
  const t = "# шапка\n#@ A | youtube\nPROXY geosite:youtube\n\n#@ B\n  proxy,domain:b.com,note\nDIRECT 10.0.0.0/8 # lan\n";
  const m = R.parse(t);
  R.setDest(R.blocks(m)[0], "DIRECT");
  assert.equal(R.serialize(m), "# шапка\n#@ A | youtube\nDIRECT geosite:youtube\n\n#@ B\n  proxy,domain:b.com,note\nDIRECT 10.0.0.0/8 # lan\n");
  R.setOff(R.blocks(m)[1], true);
  assert.ok(R.serialize(m).endsWith("#@ B\n#~   proxy,domain:b.com,note\n#~ DIRECT 10.0.0.0/8 # lan\n"));
  assert.deepEqual(R.effective(m), ["DIRECT geosite:youtube"]);
}

// 5. Добавление, дубликаты, перемещение вместе с комментариями.
{
  const m = R.parse("# выше\n#@ A\nPROXY domain:a.com\n#@ B\nPROXY domain:b.com\n");
  const [a, b] = R.blocks(m);
  assert.equal(R.addCond(b, "domain:B.com"), false, "дубликат не добавляется");
  assert.equal(R.addCond(b, "geoip:ru"), true);
  R.move(m, b, a);
  assert.equal(R.serialize(m), "#@ B\nPROXY domain:b.com\nPROXY geoip:ru\n\n# выше\n#@ A\nPROXY domain:a.com\n".replace("\n\n# выше", "\n# выше"));
}
{
  const m = R.parse("");
  R.add(m, R.newBlock({ name: "Имя | с чертой", icon: "x", conds: ["geosite:new"], dest: "PROXY@awg2" }));
  assert.equal(R.serialize(m), "#@ Имя с чертой | x\nPROXY@awg2 geosite:new\n\n");
}

// 6. Каталог: блок без заголовка узнаётся по набору условий.
{
  const catalog = [{ id: "telegram", conds: ["geosite:telegram", "geoip:telegram"] }, { id: "youtube", conds: ["geosite:youtube"] }];
  const m = R.parse("PROXY geosite:youtube\n#@ X | telegram\nPROXY domain:t.me\n");
  const [a, b] = R.blocks(m);
  assert.equal(R.service(a, catalog).id, "youtube");
  assert.equal(R.service(b, catalog).id, "telegram");
}
console.log("rules.js: все проверки пройдены");
