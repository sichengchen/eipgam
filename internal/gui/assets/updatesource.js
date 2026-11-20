// Settings › About’s download mirror.
window.hideBrewUpdateControls = function () {
  for (const row of document.querySelectorAll("#about :is(.update-pill-row, .update-download-row, .update-restart-row, .update-mirror-row)")) {
    row.hidden = true;
  }
};

window.setUpdateMirror = function (mirror) {
  return writingPrefs(api("settings/update-mirror", { mirror }))
    .then((ns) => { prefs = ns; mirrorEditing = false; mirrorDraft = mirrorErr = ""; renderSettings(); status(t("Saved"), "ok", 1500); return ns; });
};
// Use a mirror… and Change open the field from mirroredit.js
const editMirror = (r, s, v) => import("./mirroredit.js").then(() => { mirrorEditing = true; mirrorDraft = v; renderUpdateMirror(r, s); }, statusErr);
window.renderUpdateMirror = function (r, s) {
  r.classList.add("update-mirror-row");
  const who = r.querySelector(".who"), val = r.querySelector(".val");
  who.querySelector(".sub")?.remove();
  val.replaceChildren();
  const sub = el("div", "sub");
  who.append(sub);
  const m = s.updateMirror || "";
  sub.textContent = mirrorErr || (m
    ? t("Downloads use this mirror. Checksums and build attestations come from GitHub.")
    : t("Downloads come from the eipgam releases on GitHub."));
  const more = el("button", "sub-more", t("How mirrors work"));
  more.onclick = () => api("open", { url: "https://github.com/sichengchen/eipgam/blob/main/docs/reference.md#updates" }).catch(() => {});
  sub.append(" ", more);
  sub.classList.toggle("err", !!mirrorErr);
  sub.title = t("A download must match the SHA-256 in its GitHub build attestation.");
  if (mirrorEditing) return mirrorEdit(r, s, sub, val);
  if (m) {
    const official = el("button", "text", t("Remove mirror"));
    official.onclick = () => setUpdateMirror("").catch(statusTErr);
    const change = el("button", "text", t("Change"));
    change.onclick = () => editMirror(r, s, m);
    val.append(el("code", "", m), change, official);
  } else {
    const add = el("button", "text", t("Use a mirror…"));
    add.onclick = () => editMirror(r, s, "");
    val.append(el("code", "", "github.com/sichengchen/eipgam"), add);
  }
};
