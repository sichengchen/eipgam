// magpie speaks English. Other languages' tables would go in I18N, by
// language, with the English strings as written in app.js and index.html
// as keys; a missing key or language falls back to English. {name} slots
// are filled by t(str, vars).
const I18N = {};

// Every notice magpie's agents give after a change (Notices in
// internal/agent/notice.go), as tNotice matches them; each has its
// translation above. TestEveryNoticeIsTranslated keeps the two the same.
const NOTICES = [
  "Start Codex with codex -p magpie; plain codex keeps its own provider.",
  "Claude Desktop reads this at start-up — quit and reopen it to run on magpie (Code and Cowork, no Anthropic sign-in).",
  "Claude Desktop reads this at start-up — quit and reopen it to sign in with Anthropic again.",
  "Chats and projects from your Anthropic sign-in stay in Desktop's own profile, not shown on magpie; nothing is deleted, and turning this off brings them back.",
  "Your chats and projects from before are back there.",
  "To go on with them on magpie, open Settings → Import in Desktop: it can copy in your Claude.ai chats and projects and this computer's earlier sessions, and leaves the originals as they are.",
  "Open Claude Magpie from Applications to use Claude Desktop on magpie. Your own Claude Desktop stays signed in with Anthropic.",
  "Quit Claude Magpie if it is open: it no longer runs on magpie.",
  "Codex builds its model list at start-up — restart the Codex app, open codex sessions and the app-server they share ({command}) to see this.",
  "Codex builds its model list at start-up, and the app-server open codex sessions share started before this: restart it (Restart on magpie's Agents page, or {command}) and the Codex app to see this.",
  "Codex's own multi_agent_v2 is on in the config magpie routes, which takes over the multi-agent V1 magpie's models ask for; turn it off in Codex's config to run subagents on magpie's models.",
  "Codex's multi-agent V1 is on, but magpie couldn't read Codex's config to check its own multi_agent_v2.",
  "Devin reads its model at start-up — restart open Devin sessions to use this.",
  "Gemini CLI reads its settings at start-up — restart open gemini sessions to see this.",
  "Grok Build reads its settings at start-up — restart open grok sessions to use this.",
  "Install GitHub Copilot Chat in {agent} to see magpie's models in its Chat.",
  "Kimi Code reads its settings at start-up — restart open kimi sessions to use this.",
  "Kimi Code reads this as it runs; in its web or desktop app, reopen Settings › Providers or reload the page to see new models.",
  "Magpie is a custom provider in Grok App's Settings › Account. A chat can pick magpie's models in its model picker, and Grok App stays on its current route until you press Use on magpie there.",
  "Muse Code reads its settings at start-up — restart open muse sessions to use this.",
  "Restart Zed if it still asks for an API key: magpie has configured its gateway credential in the system credential store.",
  "WSL {distro} is in {net} networking with localhostForwarding=false in %UserProfile%\\.wslconfig, so 127.0.0.1 there, where its {agent} was pointed, isn't relayed to Windows, where magpie listens. Remove localhostForwarding=false under [wsl2] (it is on by default) and run wsl --shutdown.",
  "WSL {distro} is in {net} networking, not mirrored, so its {agent} was pointed at Windows ({url}) rather than 127.0.0.1; magpie listens there for WSL, and it answers once Windows' firewall lets WSL in. For 127.0.0.1, set networkingMode=mirrored under [wsl2] in %UserProfile%\\.wslconfig and run wsl --shutdown, then pick the model again.",
  "WSL {distro} isn't running: magpie shows what it last saw there, and starts it only to change something.",
  "{agent} takes magpie's gateway and model only from how it is started — start it with {command}",
  "Type {bin} in a new terminal: magpie added it to {profiles}, to start {agent} on magpie's gateway and model (in a terminal open now: {command}).",
  "agy takes magpie's gateway and model only from how it is started — start it with {command}",
  "Type agy in a new terminal: magpie added it to {profiles}, to start agy on magpie's gateway and model (in a terminal open now: {command}).",
  "an open Claude Code session keeps the effort it started with — restart it to use this.",
  "an open Claude Code session keeps the ultracode it started with — restart it to use this.",
  "magpie couldn't ask WSL {distro} its networking mode (it has no wslinfo) and %UserProfile%\\.wslconfig doesn't set it to mirrored, so its {agent} was pointed at Windows ({url}) rather than 127.0.0.1; magpie listens there for WSL, and it answers once Windows' firewall lets WSL in. For 127.0.0.1, set networkingMode=mirrored under [wsl2] in %UserProfile%\\.wslconfig and run wsl --shutdown, then pick the model again.",
  "magpie restarted the app-server codex sessions share, which none was on, so new codex sessions have this; restart the Codex app, if it is open, to see it there.",
  "magpie's models are in {agent}'s Chat model picker under magpie, from GitHub Copilot Chat {version}'s OpenAI Compatible provider, which it offers only while it is signed in to GitHub with a personal Copilot plan.",
  "magpie's models are in {agent}'s Chat model picker, under magpie, in each of its profiles (VS Code 1.122 or later). If they don't show, run Developer: Reload Window in {agent}.",
  "{agent} in WSL {distro} builds its model list at start-up — restart it, the app-server its sessions share ({command} there) and the Codex app's WSL connection to see this.",
  "{agent} in WSL {distro} reads its config at start-up — restart open opencode sessions to use this.",
  "{agent} in WSL {distro} reads its settings at start-up — restart open gemini sessions to see this.",
  "{agent} in WSL {distro} reads its settings at start-up — restart open grok sessions to use this.",
  "{agent} in WSL {distro} reads its settings at start-up — restart open kimi sessions to use this.",
  "{agent} in WSL {distro} reads this as it runs; in its web or desktop app, reopen Settings › Providers or reload the page to see new models.",
  "{agent} in WSL {distro} takes magpie's gateway and model only from how it is started — type agy in a new terminal there: magpie put it in the shell's profile.",
  "{agent} in WSL {distro} reads its settings at start-up — restart open muse sessions to use this.",
  "{agent} reads magpie's gateway from CURSOR_LOCAL_AGENT_BASE_URL and CURSOR_LOCAL_AGENT_API_KEY, now set for your user: quit it (the app, not only its window) and open it again, as it keeps the model list it was first given until it quits. A base URL or API key set in its Open configuration or a model's settings comes first, so leave both empty (or set the base URL to {url} and the key to {key}).",
  "{agent}'s Chat features must be enabled (chat.disableAIFeatures=false) and its product.json must include defaultChatAgent and trustedExtensionAuthAccess for GitHub.copilot-chat. Then run Developer: Reload Window in {agent}.",
  "{agent}'s GitHub Copilot Chat {version} has neither a Custom Endpoint nor an OpenAI Compatible provider, so it can't list magpie's models.",
  "{agent}'s daemon runs on magpie's gateway: open {agent} again (reload its window if it is open). It still needs you signed in with your Claude account, which magpie never sends on; pick a model of magpie's in its model menu.",
  "{file} sets {var}, so Claude Code sends its requests to {cloud}, not to magpie: take it out there to go through magpie.",
  "{var} is set in your shell's environment, so Claude Code started there sends its requests to {cloud}, not to magpie: take it out of your shell's profile to go through magpie.",
  "magpie doesn't restart {agent} on this computer.",
  "{agent} isn't running on the old settings any more.",
  "magpie couldn't ask {agent} to quit. Quit it yourself, then open it again.",
  "{agent} is still open; it may be asking something in its window. Answer it there, or quit it, then open it again.",
  "{agent} is still running. Quit it, then open it again.",
  "{agent} quit, but didn't open again. Open it yourself.",
  "The {agent} app isn't running any more. It opens on the new account.",
  "magpie couldn't restart the {agent} app's app-server. Quit the app and open it again.",
  "The {agent} app's app-server ended, but the app didn't start another. Quit the app and open it again.",
  "magpie couldn't tell whether the {agent} app restarted. Quit the app and open it again.",
];

// The active language: a ?locale= URL parameter wins, then the saved
// preference, then the system.
let locale = "en";
function setLocale(pref) {
  const p = params.get("locale") || pref || "system";
  const sys = (navigator.language || "").toLowerCase();
  // a language with no table here reads in English
  const want = p === "system" ? sysLocale(sys) : p;
  locale = I18N[want] ? want : "en";
  // set only when it changes: settings come back on every refresh, and a
  // view that redraws for a new language shouldn't redraw for the same one
  const lang = LANG_TAGS[locale] || "en";
  if (document.documentElement.lang !== lang) document.documentElement.lang = lang;
  translateStatic();
}

// sysLocale is the language a system tag reads in: Traditional Chinese for
// Taiwan, Hong Kong, Macau and any zh-Hant tag, Simplified for other Chinese.
function sysLocale(sys) {
  if (/^zh-(tw|hk|mo|hant)\b/.test(sys)) return "zh-TW";
  return sys.startsWith("zh") ? "zh" : sys.startsWith("ja") ? "ja" : sys.startsWith("de") ? "de" : "en";
}

// intlLang is the tag every date, time and number the GUI shows is
// formatted in: the GUI's language, never the system's alone. English takes
// the system's own English (en-GB keeps its day-month order; so does the
// region a Mac is set to, boot.js "locale", over the webview's en-US) and is plain
// "en" on a system in another language, so a Chinese Mac set to English
// reads "Oct 11", not "10月11日" (#1843). Never format with no locale, [] or
// undefined: those follow the system, not the language picked here.
const LANG_TAGS = { zh: "zh-CN", "zh-TW": "zh-TW", ja: "ja-JP", de: "de-DE" };
function intlLang() {
  const sys = window.bootPrefs?.locale || navigator.language || "", hc = window.bootPrefs?.hourCycle;
  const base = LANG_TAGS[locale] || (/^en(-|$)/i.test(sys) ? sys : "en");
  return hc ? base.replace(/-u-.*/, "") + "-u-hc-" + hc : base;
}

// hourClock: the GUI's 24-hour times, 12 when the system is (boot.js).
function hourClock() {
  return { hour12: window.bootPrefs?.hourCycle === "h12" };
}

// dateFmt is intlLang()'s date formatter for opts, made once: a
// toLocale*String call makes a new one each time, and a card of hundreds
// of accounts formats every account's resets on each render (#5310).
const dateFmts = new Map();
function dateFmt(opts) {
  const lang = intlLang(), key = lang + JSON.stringify(opts);
  let f = dateFmts.get(key);
  if (!f) dateFmts.set(key, (f = new Intl.DateTimeFormat(lang, opts)));
  return f;
}

function t(s, vars) {
  let out = I18N[locale]?.[s] ?? s;
  if (vars) for (const k in vars) out = out.split("{" + k + "}").join(vars[k]);
  return out;
}

// tNotice is an agent's advice after a change (state.notice) in the
// reader's language (#1508). magpie writes it from the templates in
// NOTICES, with names, paths and commands in their {slots}, and may join
// several with a space: each is matched back to its template, the longest
// first, and translated. One that doesn't match whole (a plugin's own
// advice) reads as given.
let noticeRes;
function tNotice(s) {
  if (!s || !I18N[locale]) return s;
  if (I18N[locale][s]) return I18N[locale][s];
  noticeRes ??= NOTICES.map((en) => {
    const parts = en.split(/\{(\w+)\}/), names = [];
    let re = "";
    parts.forEach((p, i) => {
      if (i % 2 === 0) re += p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
      // a slot never holds a sentence's end, so it can't take in the
      // notice joined after it
      else names.push(p), (re += i === parts.length - 2 && parts[i + 1] === "" ? "((?:(?!\\. ).)+)" : "((?:(?!\\. ).)+?)");
    });
    return { en, names, re: new RegExp("^" + re + "(?= |$)"), fixed: parts.filter((_, i) => i % 2 === 0).join("").length };
  });
  const out = [];
  let rest = s;
  while (rest) {
    let hit = null;
    for (const n of noticeRes) {
      const m = n.re.exec(rest);
      if (m && (!hit || m[0].length > hit.m[0].length || (m[0].length === hit.m[0].length && n.fixed > hit.n.fixed))) hit = { n, m };
    }
    if (!hit) return s;
    const vars = {};
    hit.n.names.forEach((k, i) => (vars[k] ??= hit.m[i + 1]));
    out.push(t(hit.n.en, vars));
    rest = rest.slice(hit.m[0].length).replace(/^ /, "");
  }
  return out.join(/^(zh|ja)/.test(locale) ? "" : " ");
}

// tHeading is a release's section heading in the GUI's language (Wei He
// on Discord: past versions' headings switched between Chinese and
// English). The Chinese notes are a model's translation, which keeps the
// English headings in some releases and words them its own way in others
// (功能 or 新功能; 错误修复, 缺陷修复 or Bug 修复), and a release without
// Chinese, or a GUI in a language the notes don't come in, has English
// ones. A heading said either way is the GUI's own word for it; one it
// doesn't know reads as written.
const NOTE_HEADINGS = {
  "Features": ["features", "new features", "功能", "新功能", "新特性", "特性"],
  "Bug Fixes": ["bug fixes", "bug fix", "bugfixes", "fixes", "错误修复", "缺陷修复", "bug 修复", "bug修复", "问题修复", "修复", "錯誤修正", "錯誤修復"],
  "Improvements": ["improvements", "改进", "优化", "改進"],
  "Changes": ["changes", "更新", "变更", "變更"],
  "Documentation": ["documentation", "docs", "文档"],
  "Performance Improvements": ["performance improvements", "performance", "性能改进", "性能优化", "效能改進"],
  "Security": ["security", "安全", "安全性"],
  "Provider Changes": ["provider changes", "提供商变更", "供应商变更", "供應商變更"],
};
let noteHeadingKeys;
function tHeading(s) {
  noteHeadingKeys ??= new Map(Object.entries(NOTE_HEADINGS).flatMap(([k, ws]) => ws.map((w) => [w, k])));
  const k = noteHeadingKeys.get(s.trim().replace(/[:：]$/, "").replace(/\s+/g, " ").toLowerCase());
  return k ? t(k) : s;
}

// withNotice is a status line followed by the agent's advice, as the
// language ends a sentence.
function withNotice(msg, notice) {
  return msg + (/^(zh|ja)/.test(locale) ? "。" : ". ") + tNotice(notice);
}

// Static text in index.html: elements marked data-t (text) or data-tt
// (title) keep their English in a data attribute and are re-rendered on a
// language change. A data-tt with a value is the title's English itself
// (#653: the Hide accounts buttons had no title, so no tooltip showed).
function translateStatic() {
  for (const e of document.querySelectorAll("[data-t]")) {
    if (e.dataset.en === undefined) e.dataset.en = e.textContent;
    e.textContent = t(e.dataset.en);
  }
  for (const e of document.querySelectorAll("[data-tt]")) {
    if (e.dataset.enTitle === undefined) e.dataset.enTitle = e.dataset.tt || e.title;
    e.title = t(e.dataset.enTitle);
    // data-keys: its shortcut, the Mac's before the | and the others' after
    // (Settings' ⌘, #670); a browser tab keeps its own
    const keys = e.dataset.keys?.split("|")[/^Mac/.test(navigator.platform) ? 0 : 1];
    if (keys && !window.bootPrefs?.web) e.title += " (" + keys + ")";
  }
  for (const e of document.querySelectorAll("[data-tp]")) {
    if (e.dataset.enPlaceholder === undefined) e.dataset.enPlaceholder = e.placeholder;
    e.placeholder = t(e.dataset.enPlaceholder);
  }
  // another language makes the tabs wider in the header as it was fitted, and
  // they ran under the magpie and the window's buttons (kiwiflysky on X)
  window.fitTop?.();
}
