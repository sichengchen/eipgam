// Run with Node's test runner and Playwright on the module path; see README.md.
// The warm-ups, each in its service's provider editor (they were a tab per
// service in Settings › Usage, #124 Mrhe525, #471; the owner moved them to
// the provider on 11-07): Codex's and Claude Code's Warm up on reset and
// Daily warm-up, the lines under them short, with how the last warm-up went
// still on them. Settings' Usage part keeps none of them. A daily warm-up's
// time field shows only while it is on. Every control posts the same
// setting it did, and a click on a control moves nothing. English and
// Chinese; no backend, the API is faked here.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");
const { provPrefsProviders, openProvPrefs, inEditor } = require("./reader.cjs");

const assets = path.resolve(__dirname, "../assets");

function settingsPayload(over) {
  return {
    theme: "light", lang: "en", tray: "panel", quotaLeft: false, currency: "usd",
    dock: false, dockWindow: false, proxy: "", redact: false, redactPersonal: false, redactWords: [],
    codexWarmup: "", claudeWarmup: "", codexWarmAt: "", claudeWarmAt: "", noStats: false,
    trayUsage: "", trayUsageEvery: 3, vision: "", imageGen: "",
    version: "0.1.400", dir: "~/.config/eipgam", gateway: "http://127.0.0.1:3425",
    proxyNow: "none", proxySource: "none", login: false,
    visionModels: [], imageGenModels: [], lanURLs: [],
    fx: { rate: 7.2, at: new Date().toISOString(), stale: false },
    ...over,
  };
}

function server(lang, posts, over) {
  let cur = settingsPayload({ lang, ...over });
  return async (route) => {
    const req = route.request(), url = new URL(req.url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:false};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json({ agents: [], profiles: [], settings: { lang, theme: "light" }, fx: cur.fx });
    if (url.pathname === "/api/settings") {
      if (req.method() === "POST") {
        const body = req.postDataJSON();
        posts.push(body);
        cur = { ...cur, ...body };
      }
      return json(cur);
    }
    if (url.pathname === "/api/providers") return json(provPrefsProviders(["codex", "claude"], { codex: "Codex", claude: "Claude Code" }));
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/groups") return json({ groups: [], models: [] });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

const L = {
  en: {
    lean: "Short Claude system prompt", extra: "Spend extra usage", reset: "Warm up on reset", daily: "Daily warm-up",
    on: "On", off: "Off", started: "last started",
  },
  zh: {
    lean: "精简 Claude 系统提示词", extra: "花费额外用量", reset: "窗口重置时预热", daily: "每日定时预热",
    on: "开启", off: "关闭", started: "上次启动于",
  },
};

const LISTS = { codex: "codexWarmList", claude: "claudeWarmList" };
const EDIT = { codex: "codex", claude: "claude" };
// still: till no box has scrolled for ten frames
const still = (page) => page.evaluate(() => new Promise((done) => {
  let last = "", same = 0;
  const at = () => [scrollY, ...[...document.querySelectorAll("*")].map((e) => e.scrollTop)].join(",");
  const f = () => { const now = at(); same = now === last ? same + 1 : 0; last = now; if (same >= 10) done(); else requestAnimationFrame(f); };
  requestAnimationFrame(f);
}));
// the services' lists the open editor shows, as the page says them
const panes = (page) => page.evaluate((lists) => Object.entries(lists).filter(([, id]) => {
  const e = document.getElementById(id);
  return !!e.closest(".editor .prov-prefs") && getComputedStyle(e).visibility === "visible" && e.getBoundingClientRect().height > 0;
}).map(([k]) => k), LISTS);

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  test(engine + ": warm-ups in each service's provider editor", async (t) => {
    assert(["chromium", "webkit"].includes(engine), "BROWSER must be chromium or webkit");
    const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
    const pages = [];
    t.after(async () => {
      if (process.env.ARTIFACT_DIR) {
        await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
        for (const [i, p] of pages.entries()) await p.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-settings-groups-${i}.png`) });
      }
      await browser.close();
    });
    const open = async (lang, posts, over) => {
      const errors = [];
      const ctx = await browser.newContext({ viewport: { width: 900, height: 480 }, reducedMotion: "reduce" });
      const page = await ctx.newPage();
      pages.push(page);
      page.setDefaultTimeout(5000);
      page.setDefaultNavigationTimeout(60000);
      page.on("pageerror", (e) => {
        // a goto in WebKit can cut off a fetch "due to access control
        // checks" (#1350): not this test's
        if (engine === "webkit" && /due to access control checks/.test(e.message)) return;
        errors.push(e.message);
      });
      await page.route("**/*", server(lang, posts, over));
      // edit: the service's editor opened, its lists drawn
      const edit = async (k) => {
        await openProvPrefs(page, EDIT[k]);
        await page.locator(`.editor .prov-prefs #${LISTS[k]} .segs .opt`).first().waitFor();
      };
      return { page, errors, edit };
    };

    for (const lang of ["en", "zh"]) {
      const w = L[lang];
      await t.test(lang, async () => {
        const posts = [];
        const { page, errors, edit } = await open(lang, posts, {
          codexWarmed: new Date(Date.now() - 3600e3).toISOString(),
        });

        // Settings' Usage part: its list, its Advanced fold (the menu bar's
        // currency) and the Alerts, no service's rows
        await page.goto("http://eipgam.test/?view=settings&tab=usage");
        await page.locator("#setPage-usage #currencySegs .opt").first().waitFor();
        const parts = await page.evaluate(() => [...document.querySelectorAll("#setPage-usage > *")]
          .map((h) => h.querySelector("#currencySegs") ? "usage" : h.id));
        assert.deepEqual(parts, ["usage", "usageMore", "alertHead", "alertList"]);
        assert.equal(await page.locator("#warmTabs, .warm-panes").count(), 0, "no tabs left");
        for (const id of ["#warmSegs", "#warmAtSegs", "#claudeWarmSegs", "#claudeWarmAtSegs"]) {
          assert.equal(await page.locator("#setPage-usage").locator(id).count(), 0, id + " is out of Settings");
        }

        // each editor shows its own service's rows alone
        const visibleRows = () => page.locator(".editor .prov-prefs .row.pref:visible .who .name").allTextContents();
        // each pill shown has its thumb under the option picked, drawn
        // while its list was in the hidden holder or not. A ResizeObserver
        // refits a thumb after the editor sizes its pill, so wait for that
        // (on a busy machine it lands after the editor shows, #5839)
        const thumbsOff = (l) => [...l.querySelectorAll(".segs")].filter((s) => {
          const th = s.querySelector(":scope > .thumb").getBoundingClientRect(), on = s.querySelector(":scope > .on").getBoundingClientRect();
          return !(Math.abs(th.left - on.left) <= 1 && Math.abs(th.width - on.width) <= 1);
        }).length;
        const thumbsUnderPicks = async (k) => {
          const list = await page.locator("#" + LISTS[k]).elementHandle();
          await page.waitForFunction((l) => [...l.querySelectorAll(".segs")].every((s) => {
            const th = s.querySelector(":scope > .thumb").getBoundingClientRect(), on = s.querySelector(":scope > .on").getBoundingClientRect();
            return Math.abs(th.left - on.left) <= 1 && Math.abs(th.width - on.width) <= 1;
          }), list, { timeout: 3000 }).catch(() => {});
          assert.equal(await list.evaluate(thumbsOff), 0, k + ": the pills' thumbs under their picks");
        };
        // no coloured stripe down a list's or a row's left side
        const stripes = () => page.evaluate(() => [...document.querySelectorAll(".editor .prov-prefs .list, .editor .prov-prefs .row")]
          .filter((e) => { const c = getComputedStyle(e); return c.borderLeftWidth !== c.borderRightWidth || c.borderLeftColor !== c.borderRightColor; }).length);
        // a control pressed where the reader sees it; then it has to be
        // where it was, under the pointer (the editor may scroll to keep
        // it there when a time's field opens before it)
        let before = 0, pressed = null;
        const top = (loc) => loc.evaluate((e) => Math.round(e.getBoundingClientRect().top));
        const press = async (loc) => {
          await inEditor(page, loc);
          pressed = loc;
          before = await top(loc);
          await loc.click();
        };
        const last = async (key, want) => {
          for (let i = 0; i < 50 && !(posts.length && JSON.stringify(posts.at(-1)[key]) === JSON.stringify(want)); i++) await page.waitForTimeout(40);
          assert.deepEqual(posts.at(-1)?.[key], want, key);
          await still(page);
          assert(Math.abs(await top(pressed) - before) <= 1, key + ": what was clicked stays under the pointer");
        };
        const pick = (box, name) => press(page.locator(`.editor ${box} .opt`, { hasText: name }).first());

        await edit("codex");
        assert.deepEqual(await panes(page), ["codex"]);
        assert.deepEqual(await visibleRows(), [w.reset, w.daily]);
        await thumbsUnderPicks("codex");

        // short lines, the status still on them
        const warmSub = await page.locator("#warmSub").textContent();
        assert(warmSub.includes(w.started), `the last warm-up is shown: ${warmSub}`);
        assert((await page.locator("#warmAtSub").textContent()).length < 60, "the daily line is short");
        assert.equal(await stripes(), 0, "no left-border accents");

        // no time field while a daily warm-up is off
        assert.equal(await page.locator("#warmAtSegs input.at").count(), 0);
        assert.equal(await page.locator("#claudeWarmAtSegs input.at").count(), 0);

        await press(page.locator(".editor #warmSegs .opt").nth(1));
        await last("codexWarmup", "week");
        await pick("#warmAtSegs", w.on);
        await last("codexWarmAt", "06:00");
        const field = page.locator(".editor #warmAtSegs input.at");
        await field.waitFor();
        assert.equal(await field.inputValue(), "06:00", "the time field comes with the daily warm-up on");
        await inEditor(page, field);
        await field.focus();
        pressed = field;
        before = await top(field);
        await field.fill("07:30");
        await field.dispatchEvent("change");
        await last("codexWarmAt", "07:30");

        await edit("claude");
        assert.deepEqual(await panes(page), ["claude"]);
        // Claude Code's own run row is Claude's too
        assert.deepEqual(await visibleRows(), [w.reset, w.daily, w.lean, w.extra]);
        await thumbsUnderPicks("claude");
        await press(page.locator(".editor #claudeWarmSegs .opt").nth(2));
        await last("claudeWarmup", "all");
        await pick("#claudeWarmAtSegs", w.on);
        await last("claudeWarmAt", "06:00");
        await page.locator(".editor #claudeWarmAtSegs input.at").waitFor();

        // back in Codex's editor, its rows as they were left
        await edit("codex");
        assert.equal(await field.inputValue(), "07:30");
        await pick("#warmAtSegs", w.off);
        await last("codexWarmAt", "");
        await page.waitForFunction(() => !document.querySelector("#warmAtSegs input.at"));
        // the others kept as they were set
        assert.equal(posts.at(-1).codexWarmup, "week");
        assert.equal(posts.at(-1).claudeWarmup, "all");
        assert.equal(posts.at(-1).claudeWarmAt, "06:00");
        assert.deepEqual(errors, []);
      });
    }
  });
}
