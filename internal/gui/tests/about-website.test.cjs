const assert = require("node:assert/strict");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");
const { fixture } = require("./fixtures/caller-keys.cjs");
const { openPart } = require("./reader.cjs");

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const width of [1100, 420]) {
    test(`${engine} ${width}: About links the eipgam repository`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      t.after(() => browser.close());
      const page = await (await browser.newContext({ viewport: { width, height: 800 }, reducedMotion: "reduce" })).newPage();
      page.setDefaultTimeout(6000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await page.route("**/*", fixture("en", "light", []));
      await page.goto("http://eipgam.test/");
      await page.locator("#prefs").click();
      await openPart(page, "about");
      assert.equal(await page.locator("#about").getByText("Website", { exact: true }).count(), 0);
      const opened = page.waitForRequest((r) => r.url().endsWith("/api/open"));
      await page.locator("#about").getByRole("button", { name: "GitHub", exact: true }).click();
      assert.equal((await opened).postDataJSON().url, "https://github.com/sichengchen/eipgam");
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, "About wider than the window");
      assert.deepEqual(errors, []);
    });
  }
}
