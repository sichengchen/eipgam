const assert = require("node:assert/strict");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");
const { fixture } = require("./fixtures/caller-keys.cjs");
const { openPart } = require("./reader.cjs");

// Settings › About's Report a problem copies the version, the system and
// the architecture, says so, then opens the issue form on GitHub. What it
// copied is never in the link (River_WLan on X).
const words = {
  en: ["Report a problem", "Details copied. Paste them into your GitHub issue"],
};
const block = "eipgam 0.1.9\nOS: macOS 26.2\nArch: arm64\nOpened in: app";
const post = "https://github.com/sichengchen/eipgam/issues/new/choose";

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of Object.keys(words)) {
    for (const width of [1100, 420]) {
      test(`${engine} ${lang} ${width}: About reports a problem`, async (t) => {
        const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
        t.after(() => browser.close());
        const page = await (await browser.newContext({ viewport: { width, height: 800 }, reducedMotion: "reduce" })).newPage();
        page.setDefaultTimeout(6000);
        page.setDefaultNavigationTimeout(60000);
        const errors = [];
        page.on("pageerror", (e) => errors.push(e.message));
        const events = [];
        await page.route("**/*", fixture(lang, "light", events));
        await page.route("**/api/feedback*", (r) => r.fulfill({ json: { text: block, url: post } }));
        await page.route("**/api/open", (r) => { events.push({ action: "open", body: r.request().postDataJSON() }); return r.fulfill({ status: 204 }); });
        await page.goto("http://eipgam.test/");
        await page.locator("#prefs").click();
        await openPart(page, "about");
        const row = page.locator("#about .row", { has: page.locator(".who", { hasText: words[lang][0] }) });
        await row.waitFor();
        await row.locator("button.text").click();
        await page.locator("#status", { hasText: words[lang][1] }).waitFor();
        assert.equal(events.find((e) => e.action === "clipboard").body.text, block);
        const opened = events.find((e) => e.action === "open");
        assert.equal(opened.body.url, post);
        assert.equal(opened.body.url.includes(encodeURIComponent("eipgam 0.1.9")), false);
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, "About wider than the window");
        assert.deepEqual(errors, []);
      });
    }
  }
}
