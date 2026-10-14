// Acceptatietest per klantreis. Leest het project uit project.js, de reis uit <script id="journey">, de vakken op de
// screenshots uit shots/<reis>/annotations.js en eerdere oordelen uit feedback/<reis>.js.
// Oordelen en opmerkingen gaan als Lavish-feedback naar de agent die de pagina als Review opende (window.lavish).
// De pagina verklaart in haar head dat ze haar eigen feedback heeft, dus Lavish toont er niets omheen.
const PROJECT = window.ACCEPTANCE_PROJECT;
const JOURNEY = JSON.parse(document.getElementById("journey").textContent);
const ENVS = JOURNEY.envs ? PROJECT.envs.filter((e) => JOURNEY.envs.includes(e.id)) : PROJECT.envs;
document.body.insertAdjacentHTML("afterbegin", `<div class="top">
  <div class="logo">${PROJECT.logo}</div>
  <button type="button" class="btn" id="aiBtn" title="Kopieer een opdracht voor een AI-assistent die deze test samen met jou doet">AI-prompt kopiëren</button>
  <div class="title" id="title"></div>
  <div class="chips" id="chips"></div>
  <div class="grow"></div>
  <div class="seg" id="envs" role="group" aria-label="Omgeving"></div>
  <span class="real"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2.5 20h19L12 3Z"/><path d="M12 10v4M12 17v.01"/></svg>Echte gegevens</span>
  <div class="seg" id="levels" role="group" aria-label="Hoeveel tekst"></div>
  <div class="progress"><div class="bar"><i id="barFill"></i></div><span id="progressText"></span></div>
  <div class="save" id="saveState" aria-live="polite"></div>
  <span class="done-chip" id="doneChip"></span>
  <button type="button" class="btn" id="saveBtn">Opslaan</button>
  <button type="button" class="btn primary" id="finishBtn">Afronden</button>
</div>

<div class="work">
  <div class="app">
    <iframe id="frame" title="${PROJECT.name}"></iframe>
    <div class="zoom" id="zoom"></div>
    <div class="sheet-wrap" id="sheetWrap"><div class="sheet" id="sheet" role="dialog" aria-modal="true" aria-labelledby="sheetTitle"></div></div>
  </div>
  <div class="panel" id="panel"></div>
</div>`);

const ICON = {
  check: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="m5 12.5 4.5 4.5L19 7.5"/></svg>',
  cross: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M6 6l12 12M18 6 6 18"/></svg>',
  open: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/></svg>',
  save: '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M5 4h11l3 3v13H5z"/><path d="M8 4v5h7V4M8 20v-6h8v6"/></svg>',
  warn: '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2.5 20h19L12 3Z"/><path d="M12 10v4M12 17v.01"/></svg>',
  chev: '<svg class="chev" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>',
  proven: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#047857" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="m5 12.5 4.5 4.5L19 7.5"/></svg>',
  pending: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#9a3412" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="12" r="8.5"/><path d="M12 8v4.5"/></svg>',
  target: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="12" r="3.5"/></svg>',
  turn: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 14 4 9l5-5"/><path d="M4 9h11a5 5 0 0 1 0 10h-3"/></svg>',
};
const LEVELS = ["Kort", "Normaal", "Uitgebreid"];
const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;");
// [n] in een tekst verwijst naar vak n op de screenshot.
const rich = (s) => esc(s).replace(/\[(\d+)\]/g, '<span class="num in">$1</span>');
const plain = (s) => s.replace(/\s*\[\d+\]/g, "");
const lv = (t, cls = "", tag = "span") => t.map((x, i) => x ? `<${tag} class="${cls}" data-l="${i + 1}">${rich(x)}</${tag}>` : "").join("");
const $ = (s, r = document) => r.querySelector(s);

const ANN = {};
const state = { level: "2", env: JOURNEY.env, parts: {}, steps: {}, general: "", finishedAt: null };
const scenarios = [...JOURNEY.scenarios, ...JOURNEY.failures.scenarios];
const allSteps = scenarios.flatMap((sc) => sc.steps.map((s) => Object.assign(s, { scenario: sc })));
const SHOTS = `shots/${JOURNEY.slug}`;

function shotHtml(name) {
  return `<figure class="shot" data-shot="${name}"><div class="pic"><img src="${SHOTS}/${name}.png" alt="" loading="lazy"></div></figure>`;
}
function drawMarks(fig) {
  const a = ANN[fig.dataset.shot]; if (!a) return;
  const pic = $(".pic", fig);
  a.forEach((r, i) => {
    const m = document.createElement("span"); m.className = "mk";
    m.style.cssText = `left:calc(${r.x}% - 4px);top:calc(${r.y}% - 3px);width:calc(${r.w}% + 8px);height:calc(${r.h}% + 6px)`;
    if (a.length > 1) m.innerHTML = `<b>${i + 1}</b>`;
    pic.appendChild(m);
  });
}
function levelSwitch(scope) {
  return `<div class="seg small" data-scope="${scope}" role="group" aria-label="Hoeveel tekst">${LEVELS.map((n, i) => `<button type="button" data-set="${i + 1}">${n}</button>`).join("")}</div>`;
}
function part(id, title, inner, count = "") {
  return `<section class="part" id="part-${id}" data-part="${id}"><header><h2>${title}</h2>${count}<div class="grow"></div>${levelSwitch(id)}${ICON.chev}</header><div class="body">${inner}</div></section>`;
}
function stepHtml(s, n) {
  const tags = [
    s.saves ? `<span class="tag save">${ICON.save}Slaat op</span>` : "",
    s.warn ? `<span class="tag warn">${ICON.warn}${lv(s.warn)}</span>` : "",
  ].join("");
  return `<article class="step" data-step="${s.id}">
    <div class="head"><div class="sn">${n}</div><p class="do">${lv(s.when)}</p></div>
    <div class="checks"><div class="checks-h">Controleer</div><ul>${s.then.map((c) => `<li>${ICON.target}<div>${lv(c)}</div></li>`).join("")}</ul>
      ${s.ifNot ? `<div class="ifnot">${ICON.turn}<div><b>Niet zo?</b> ${lv(s.ifNot)}</div></div>` : ""}</div>
    ${s.why ? lv(s.why, "why", "p") : ""}
    ${tags ? `<div class="tags">${tags}</div>` : ""}
    ${shotHtml(s.shot)}
    <div class="acts">
      <button type="button" class="btn ok" data-verdict="ok">${ICON.check}Klopt</button>
      <button type="button" class="btn bad" data-verdict="bad">${ICON.cross}Klopt niet</button>
      <button type="button" class="btn link" data-note>Opmerking</button>
      <span class="sp"></span>
      <button type="button" class="btn link" data-go="${s.path}">${ICON.open}Toon in app</button>
    </div>
    <textarea class="note" placeholder="Wat zag je? Wat had je verwacht?" aria-label="Opmerking bij stap ${n}"></textarea>
  </article>`;
}

function render() {
  const J = JOURNEY;
  $("#title").textContent = J.title;
  const storyChips = J.stories.map((s) => `<span class="chip" title="${esc(s.title)}">${esc(s.tracker_id)} ${esc(s.label)}</span>`).join("");
  $("#chips").innerHTML = storyChips;
  $("#envs").innerHTML = ENVS.map((e) => `<button type="button" data-env="${e.id}">${e.label}</button>`).join("");
  $("#levels").innerHTML = LEVELS.map((n, i) => `<button type="button" data-set="${i + 1}">${n}</button>`).join("");

  const made = `<div class="stories">${storyChips}</div><p class="lead">${lv(J.made.lead)}</p>${shotHtml(J.made.shot)}
    <ol class="legend">${J.made.legend.map(([name, t], i) => `<li><span class="num">${i + 1}</span><div><strong>${name}</strong> ${lv(t, "muted")}</div></li>`).join("")}</ol>`;
  const day = (d) => new Date(d).toLocaleDateString("nl-NL", { day: "numeric", month: "long" });
  const sources = J.sources.map((q) => `<figure class="quote"><p>${lv(q.text)}</p><cite>${esc(q.cite)} · ${day(q.date)}</cite></figure>`).join("");
  const STATUS = { partial: "Deels", gap: "Ontbreekt" };
  const journey = `<ol class="track">${J.stages.map((st, i) => `<li><span class="dot">${i + 1}</span><div><strong>${esc(st.name)}</strong>${STATUS[st.status] ? ` <span class="tag warn">${STATUS[st.status]}</span>` : ""}${lv(st.text, "muted")}</div></li>`).join("")}</ol>`;
  const group = (sc) => `<div class="group">${esc(sc.name)}<span class="ref">${sc.stories.map(esc).join(" · ")}</span></div>`;
  let n = 0;
  const tests = J.scenarios.map((sc) => `${group(sc)}${sc.steps.map((s) => ((s.label = `Stap ${++n} · ${sc.label}`), stepHtml(s, n))).join("")}`).join("");
  const testCount = n;
  const F = J.failures;
  let m = 0;
  const other = `<div class="notice">${ICON.warn}<div>${lv(F.lead)}</div></div>${F.scenarios.map((sc) => `${group(sc)}${sc.steps.map((s) => ((s.label = `Overig ${++m} · ${sc.name}`), stepHtml(s, m))).join("")}`).join("")}
    <h3>${lv(J.proven.title)}</h3><ul class="list">${J.proven.items.map((t) => `<li>${ICON.proven}<div>${lv(t)}</div></li>`).join("")}</ul>${lv(J.proven.note, "why", "p")}
    <h3>${lv(J.open.title)}</h3><ul class="list">${J.open.items.map((t) => `<li>${ICON.pending}<div>${lv(t)}</div></li>`).join("")}</ul>`;
  const general = `<textarea id="general" placeholder="Algemene opmerking over deze klantreis" aria-label="Algemene opmerking"></textarea><div class="replies" id="replies" aria-live="polite"></div>`;

  $("#panel").innerHTML = [
    part("gemaakt", "Gemaakt", made),
    part("bronnen", "Bronnen", sources),
    part("journey", "Journey", journey),
    part("testen", "Testen", tests, `<span class="count" data-count="testen" data-total="${testCount}"></span>`),
    part("overig", "Overig", other, `<span class="count" data-count="overig" data-total="${m}"></span>`),
    part("opmerking", "Opmerking", general),
  ].join("");
  document.querySelectorAll("figure.shot").forEach(drawMarks);
}

function applyLevels() {
  const style = document.getElementById("lvstyle") || Object.assign(document.head.appendChild(document.createElement("style")), { id: "lvstyle" });
  const rules = [`body[data-level="1"] [data-l]:not([data-l="1"]),body[data-level="2"] [data-l]:not([data-l="2"]),body[data-level="3"] [data-l]:not([data-l="3"]){display:none}`];
  for (const [id, l] of Object.entries(state.parts)) {
    if (!l) continue;
    rules.push(`#part-${id} [data-l]{display:revert!important}`, `#part-${id} [data-l]:not([data-l="${l}"]){display:none!important}`);
  }
  style.textContent = rules.join("\n");
  document.body.dataset.level = state.level;
  document.querySelectorAll("#levels button").forEach((b) => b.classList.toggle("on", b.dataset.set === state.level));
  document.querySelectorAll(".seg[data-scope]").forEach((sw) => {
    const l = state.parts[sw.dataset.scope] || state.level;
    sw.querySelectorAll("button").forEach((b) => b.classList.toggle("on", b.dataset.set === l));
  });
}

let appUrl = "";
// Een omgeving zonder url is een app op de computer van de tester: de pagina toont dan geen app-frame en geen
// "Toon in app", en de tester werkt in de app naast de pagina.
function applyEnv(path) {
  const env = ENVS.find((e) => e.id === state.env) || ENVS[0];
  document.body.dataset.real = env.real ? "1" : "0";
  document.body.dataset.app = env.url ? "frame" : "none";
  document.querySelectorAll("#envs button").forEach((b) => b.classList.toggle("on", b.dataset.env === env.id));
  appUrl = env.url ? env.url + (path || JOURNEY.start) : "";
  if (appUrl) $("#frame").src = appUrl;
  else $("#frame").removeAttribute("src");
}

function applySteps() {
  let done = 0;
  const per = { testen: 0, overig: 0 };
  allSteps.forEach((s) => {
    const el = $(`[data-step="${s.id}"]`);
    const v = state.steps[s.id] || {};
    el.classList.toggle("ok", v.verdict === "ok");
    el.classList.toggle("bad", v.verdict === "bad");
    el.classList.toggle("noted", !!v.note);
    $('[data-verdict="ok"]', el).classList.toggle("on", v.verdict === "ok");
    $('[data-verdict="bad"]', el).classList.toggle("on", v.verdict === "bad");
    const ta = $(".note", el); if (document.activeElement !== ta) ta.value = v.note || "";
    if (v.verdict) { done++; per[el.closest("[data-part]").dataset.part]++; }
  });
  document.querySelectorAll("[data-count]").forEach((c) => (c.textContent = `${per[c.dataset.count]}/${c.dataset.total}`));
  $("#progressText").textContent = `${done}/${allSteps.length}`;
  $("#barFill").style.width = `${(done / allSteps.length) * 100}%`;
  const g = $("#general"); if (document.activeElement !== g) g.value = state.general || "";
}

// Eén bericht per stap. Een nieuwer bericht over dezelfde stap vervangt het vorige zolang het niet verstuurd is.
// Lavish voegt window.lavish pas na dit script toe, dus het wordt bij elk gebruik opgezocht. Of alles is aangekomen,
// meldt Lavish via onReview (showReview).
const saveEl = $("#saveState");
let sendTimer = null;
function disconnected() {
  if (window.lavish) return false;
  saveEl.textContent = "Niet verbonden"; saveEl.classList.add("bad");
  return true;
}
function queue(key, lines, label, element) {
  if (disconnected()) return;
  window.lavish.queuePrompt([`Acceptatietest ${JOURNEY.slug}, ${key}`, ...lines].join("\n"), { queueKey: `${JOURNEY.slug}:${key}`, tag: "acceptatie", text: label, element });
  saveEl.textContent = "Nog niet verstuurd"; saveEl.classList.remove("bad");
}
function send(delay = 0) {
  if (disconnected()) return;
  clearTimeout(sendTimer);
  sendTimer = setTimeout(() => window.lavish.sendQueuedPrompts(), delay);
}
// Lavish meldt de stand van de Review: wat nog niet is aangekomen, waarom versturen misgaat, of de test nog open is, en
// onze antwoorden. Het Lavish-gesprek is verborgen, dus de antwoorden staan onder de algemene opmerking.
const CLOSED = { finished: "Deze test is afgerond", cancelled: "Deze test is gesloten", "moved-on": "Er is een nieuwe versie: herlaad de pagina" };
function showReview(r) {
  const time = new Date().toLocaleTimeString("nl-NL", { hour: "2-digit", minute: "2-digit" });
  saveEl.textContent = CLOSED[r.state] || (r.problem ? "Versturen lukt niet, probeer Opslaan" : r.unsent ? "Nog niet verstuurd" : `Verstuurd ${time}`);
  saveEl.classList.toggle("bad", !!r.problem && !CLOSED[r.state]);
  if (r.problem) console.warn("[acceptatie] versturen lukt niet", r.problem);
  if (r.state === "finished" && !state.finishedAt) { state.finishedAt = new Date().toISOString(); applyFinished(); }
  if (r.state === "finished" && finishing) { finishing = false; openThanks(); }
  if (CLOSED[r.state]) { document.body.dataset.finished = "1"; document.querySelectorAll(".note, #general").forEach((ta) => (ta.readOnly = true)); }
  $("#replies").innerHTML = r.replies.map((x) => `<figure class="quote"><p>${esc(x.text)}</p><cite>Antwoord · ${fmt(x.at)}</cite></figure>`).join("");
}
const VERDICT = { ok: "Klopt", bad: "Klopt niet" };
function queueStep(id) {
  const s = allSteps.find((x) => x.id === id), v = state.steps[id] || {};
  queue(`stap ${id}`, [
    `oordeel: ${v.verdict || "geen"}`,
    `omgeving: ${v.environment ? `${v.environment.env} ${v.environment.app}` : state.env}`,
    `at: ${v.at || new Date().toISOString()}`,
    `opmerking: ${v.note || ""}`,
  ], `${s.label}: ${VERDICT[v.verdict] || "geen oordeel"}${v.note ? `, "${v.note}"` : ""}`, $(`[data-step="${id}"]`));
}
function queueGeneral() {
  queue("algemeen", [`opmerking: ${state.general}`], `Algemene opmerking: "${state.general}"`, $("#general"));
}
function load() {
  const saved = window.ACCEPTANCE_FEEDBACK;
  if (saved) Object.assign(state, { env: saved.env || state.env, steps: saved.steps || {}, general: saved.general || "", finishedAt: saved.finishedAt || null });
  for (const [k, v] of Object.entries(window.ACCEPTANCE_ANNOTATIONS || {})) ANN[k] = v.t.map((r) => ({ x: Math.max(r.x, 0), y: Math.max(r.y, 0), w: Math.min(r.w + Math.min(r.x, 0), 100 - Math.max(r.x, 0)), h: Math.min(r.h, 100 - Math.max(r.y, 0)) }));
  window.addEventListener("load", () => { if (!disconnected()) window.lavish.onReview(showReview); });
}

const fmt = (iso) => new Date(iso).toLocaleString("nl-NL", { day: "numeric", month: "long", hour: "2-digit", minute: "2-digit" });
function applyFinished() {
  const done = !!state.finishedAt;
  document.body.dataset.finished = done ? "1" : "0";
  $("#doneChip").innerHTML = done ? `${ICON.check}Afgerond ${fmt(state.finishedAt)}` : "";
  document.querySelectorAll(".note, #general").forEach((ta) => (ta.readOnly = done));
}
// Afronden beëindigt de ronde van de Review, zoals Finish in Lavish. Lavish verstuurt eerst wat nog in de wachtrij staat.
// De pagina sluit en bedankt pas als Lavish meldt dat de ronde klaar is (showReview): lukt het niet, dan kan de tester
// het opnieuw.
let finishing = false;
function finish() {
  if (disconnected()) return;
  finishing = true;
  saveEl.textContent = "Afronden…"; saveEl.classList.remove("bad");
  window.lavish.endSession();
}
// Een vaste opdracht voor een AI-assistent: alleen de link. Wat de assistent moet doen, staat op de pagina zelf
// in #ai-instructions, zodat deze tekst nooit per klant of reis verandert. In een Review is de link die van de
// Review-pagina (document.referrer): het adres van het frame zelf verloopt.
function aiPrompt() {
  const link = window.lavish && document.referrer ? document.referrer : location.href.split("#")[0];
  return `Open deze acceptatietest in je browser: ${link}
Lees de pagina, ook het blok "Instructions for an AI assistant", en begeleid mij daarna door de test.`;
}
// Het klembord kan in het frame van een Review dicht zijn: dan staat de tekst geselecteerd in een venster om zelf te
// kopiëren.
async function copyPrompt() {
  const btn = $("#aiBtn"), label = btn.textContent, text = aiPrompt();
  try {
    await navigator.clipboard.writeText(text);
    btn.textContent = "Gekopieerd";
    setTimeout(() => (btn.textContent = label), 2000);
  } catch (e) {
    console.info("[acceptatie] klembord dicht, handmatig kopiëren", e);
    openSheet(`<h2 id="sheetTitle">AI-prompt</h2><p class="lead muted">Kopieer deze tekst en plak hem bij je AI-assistent.</p>
      <textarea id="aiText" readonly rows="4">${esc(text)}</textarea>
      <div class="acts"><button type="button" class="btn primary" data-sheet-close>Sluiten</button></div>`);
    $("#aiText").select();
  }
}
function openSheet(html) { $("#sheet").innerHTML = html; $("#sheetWrap").classList.add("on"); }
function closeSheet() { $("#sheetWrap").classList.remove("on"); }
function openFinish() {
  const ok = allSteps.filter((s) => state.steps[s.id]?.verdict === "ok");
  const bad = allSteps.filter((s) => state.steps[s.id]?.verdict === "bad");
  const open = allSteps.length - ok.length - bad.length;
  openSheet(`<h2 id="sheetTitle">${esc(JOURNEY.title)} afronden</h2>
    <div class="tally"><span class="t-ok">${ok.length} klopt</span><span class="t-bad">${bad.length} klopt niet</span><span>${open} niet bekeken</span></div>
    ${bad.length ? `<ul class="issues">${bad.map((s) => `<li><b>${esc(s.label)}</b>: ${esc(plain(s.when[1]))}${state.steps[s.id].note ? `<p>${esc(state.steps[s.id].note)}</p>` : ""}</li>`).join("")}</ul>` : ""}
    <p class="lead muted">${open ? `Nog ${open} ${open === 1 ? "stap" : "stappen"} niet bekeken. Afronden kan toch.` : "Alle stappen bekeken."} Wij krijgen je oordeel direct binnen.</p>
    <div class="acts"><button type="button" class="btn" data-sheet-close>Terug</button><button type="button" class="btn primary" data-confirm-finish>Afronden</button></div>`);
}
function openThanks() {
  const bad = allSteps.some((s) => state.steps[s.id]?.verdict === "bad");
  openSheet(`<h2 id="sheetTitle">Bedankt</h2><p class="lead">Je oordeel over ${esc(JOURNEY.title.toLowerCase())} is bij ons binnen.${bad ? " Wij nemen contact op over wat niet klopte." : ""}</p>
    <div class="acts"><button type="button" class="btn primary" data-sheet-close>Sluiten</button></div>`);
}

function bind() {
  document.addEventListener("click", (e) => {
    const t = e.target.closest("button, header, figure.shot, .zoom");
    if (!t) return;
    if (t.matches("#levels button")) { state.level = t.dataset.set; state.parts = {}; applyLevels(); return; }
    if (t.matches(".seg[data-scope] button")) { state.parts[t.parentElement.dataset.scope] = t.dataset.set; applyLevels(); return; }
    if (t.matches("#envs button")) { state.env = t.dataset.env; applyEnv(); return; }
    if (t.matches("#saveBtn")) { send(); return; }
    if (t.matches("#aiBtn")) { copyPrompt(); return; }
    if (t.matches("#finishBtn")) { openFinish(); return; }
    if (t.matches("[data-sheet-close]")) { closeSheet(); return; }
    if (t.matches("[data-confirm-finish]")) { closeSheet(); finish(); return; }
    if (t.matches("[data-verdict]")) {
      const id = t.closest("[data-step]").dataset.step, v = state.steps[id] || (state.steps[id] = {});
      v.verdict = v.verdict === t.dataset.verdict ? null : t.dataset.verdict; v.at = new Date().toISOString();
      const env = ENVS.find((e) => e.id === state.env);
      v.environment = { env: env.id, app: appUrl, browser: navigator.userAgent };
      applySteps(); queueStep(id); send(1500);
      if (v.verdict === "bad") $(".note", t.closest("[data-step]")).focus();
      return;
    }
    if (t.matches("[data-note]")) { const st = t.closest("[data-step]"); st.classList.add("noted"); $(".note", st).focus(); return; }
    if (t.matches("[data-go]")) { applyEnv(t.dataset.go); return; }
    if (t.matches(".part > header")) { if (!e.target.closest(".seg")) t.parentElement.classList.toggle("closed"); return; }
    if (t.matches("#panel figure.shot")) {
      const z = $("#zoom"); z.innerHTML = ""; const c = t.cloneNode(true); z.appendChild(c); z.classList.add("on"); return;
    }
    if (t.closest(".zoom")) { $("#zoom").classList.remove("on"); }
  });
  document.addEventListener("input", (e) => {
    if (e.target.matches(".note")) { (state.steps[e.target.closest("[data-step]").dataset.step] ||= {}).note = e.target.value; }
    if (e.target.id === "general") { state.general = e.target.value; }
  });
  document.addEventListener("change", (e) => {
    if (e.target.matches(".note")) { queueStep(e.target.closest("[data-step]").dataset.step); send(); }
    if (e.target.id === "general") { queueGeneral(); send(); }
  });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") { $("#zoom").classList.remove("on"); closeSheet(); } });
  $("#sheetWrap").addEventListener("click", (e) => { if (e.target.id === "sheetWrap") closeSheet(); });
}

load(); render(); bind();
applyLevels(); applyEnv(); applySteps(); applyFinished();
