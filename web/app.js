"use strict";

(() => {
  const $ = (sel, root = document) => root.querySelector(sel);
  const form = $("#form");
  const pickBtn = $("#pick");
  const resultEl = $("#result");
  const backdropEl = $("#backdrop");
  const STORE_KEY = "matchboxd:v1";
  const USER_RE = /^[A-Za-z0-9_]{1,40}$/;

  // State of each person: { user, sources, loading, ctrl }
  const sides = {};
  // Last draw, used by "Un autre": { a, b, slug }
  let lastDraw = null;

  /* ---------- persistence (best effort) ---------- */

  function loadStore() {
    try { return JSON.parse(localStorage.getItem(STORE_KEY)) || {}; } catch { return {}; }
  }
  function saveStore() {
    const data = {};
    for (const [key, s] of Object.entries(sides)) {
      data[key] = { user: s.input.value.trim(), source: s.select.value };
    }
    try { localStorage.setItem(STORE_KEY, JSON.stringify(data)); } catch { /* ignore */ }
  }

  /* ---------- API ---------- */

  async function api(path, opts = {}) {
    const res = await fetch(path, {
      ...opts,
      headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    });
    let body = null;
    try { body = await res.json(); } catch { /* non-JSON */ }
    if (!res.ok) {
      throw new Error((body && body.error) || `Erreur inattendue (HTTP ${res.status}).`);
    }
    return body;
  }

  /* ---------- people panels ---------- */

  function setMsg(side, text, kind = "") {
    side.msg.textContent = text;
    side.msg.className = "field-msg" + (kind ? " " + kind : "");
  }

  function setOptions(side, sources, preferred) {
    const sel = side.select;
    sel.replaceChildren();
    const general = document.createElement("optgroup");
    general.label = "Général";
    const lists = document.createElement("optgroup");
    lists.label = "Listes personnalisées";
    for (const src of sources) {
      const opt = new Option(src.label + (src.count ? ` · ${src.count} films` : ""), src.id);
      (src.kind === "list" ? lists : general).append(opt);
    }
    sel.append(general);
    if (lists.children.length) sel.append(lists);
    sel.disabled = false;
    if (preferred && sources.some((s) => s.id === preferred)) sel.value = preferred;
  }

  function resetSelect(side, label) {
    side.sources = null;
    side.select.replaceChildren(new Option(label, ""));
    side.select.disabled = true;
  }

  async function loadSources(side, preferred) {
    const user = side.input.value.trim().replace(/^@/, "");
    if (user === side.user && (side.sources || side.loading)) return;
    side.user = user;
    if (side.ctrl) side.ctrl.abort();

    if (!user) {
      resetSelect(side, "Entrez un pseudo");
      setMsg(side, "");
      return refresh();
    }
    if (!USER_RE.test(user)) {
      resetSelect(side, "Pseudo invalide");
      setMsg(side, "Un pseudo Letterboxd ne contient que lettres, chiffres et _.", "error");
      return refresh();
    }

    side.ctrl = new AbortController();
    side.loading = true;
    side.panel.classList.add("loading");
    resetSelect(side, "Recherche des sources…");
    setMsg(side, "");
    refresh();
    try {
      const data = await api(`/api/sources?user=${encodeURIComponent(user)}`, { signal: side.ctrl.signal });
      side.sources = data.sources;
      setOptions(side, data.sources, preferred);
      const nLists = data.sources.filter((s) => s.kind === "list").length;
      if (data.warning) setMsg(side, data.warning, "warn");
      else setMsg(side, nLists ? `${nLists} liste${nLists > 1 ? "s" : ""} trouvée${nLists > 1 ? "s" : ""}` : "Aucune liste personnalisée", "ok");
      saveStore();
    } catch (err) {
      if (err.name === "AbortError") return;
      side.user = null; // allow retrying the same name
      resetSelect(side, "Indisponible");
      setMsg(side, err.message, "error");
    } finally {
      side.loading = false;
      side.panel.classList.remove("loading");
      refresh();
    }
  }

  function refresh() {
    const ready = Object.values(sides).every((s) => s.sources && s.select.value);
    pickBtn.disabled = !ready || pickBtn.classList.contains("busy");
  }

  for (const panel of form.querySelectorAll(".person")) {
    const side = {
      panel,
      input: $("input", panel),
      select: $("select", panel),
      msg: $(".field-msg", panel),
      user: null, sources: null, loading: false, ctrl: null, timer: 0,
    };
    sides[panel.dataset.side] = side;
    side.input.addEventListener("input", () => {
      clearTimeout(side.timer);
      side.timer = setTimeout(() => loadSources(side), 700);
    });
    side.input.addEventListener("change", () => { clearTimeout(side.timer); loadSources(side); });
    side.input.addEventListener("keydown", (e) => {
      if (e.key === "Enter") { e.preventDefault(); clearTimeout(side.timer); loadSources(side); }
    });
    side.select.addEventListener("change", () => { saveStore(); refresh(); });
  }

  /* ---------- drawing ---------- */

  function selection() {
    return {
      a: { user: sides.a.user, source: sides.a.select.value },
      b: { user: sides.b.user, source: sides.b.select.value },
    };
  }

  function sourceLabel(side) {
    const opt = side.select.selectedOptions[0];
    return opt ? opt.textContent.replace(/ · \d+ films$/, "") : "";
  }

  async function draw(again) {
    const sel = again && lastDraw ? { a: lastDraw.a, b: lastDraw.b } : selection();
    const exclude = again && lastDraw ? lastDraw.slug : "";
    const trigger = again ? $(".again", resultEl) : pickBtn;

    pickBtn.classList.add("busy");
    pickBtn.disabled = true;
    if (trigger) { trigger.classList.add("busy"); trigger.disabled = true; }
    if (!again) showLoading();

    try {
      const data = await api("/api/pick", { method: "POST", body: JSON.stringify({ ...sel, exclude }) });
      if (!data.count) {
        lastDraw = null;
        setBackdrop("");
        showNotice("🎞️", "Aucun film en commun",
          `« ${sourceLabel(sides.a)} » de ${sel.a.user} (${data.sizes[0]} films) et « ${sourceLabel(sides.b)} » de ${sel.b.user} (${data.sizes[1]} films) n’ont aucun film en commun. Essayez une autre source.`);
        return;
      }
      lastDraw = { ...sel, slug: data.film.slug };
      showFilm(data);
    } catch (err) {
      showNotice("⚠️", "Impossible de tirer un film", err.message, true);
    } finally {
      pickBtn.classList.remove("busy");
      refresh();
    }
  }

  form.addEventListener("submit", (e) => {
    e.preventDefault();
    if (!pickBtn.disabled) draw(false);
  });
  resultEl.addEventListener("click", (e) => {
    if (e.target.closest(".again")) draw(true);
  });

  /* ---------- rendering ---------- */

  function el(tag, cls, text) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  const cssURL = (url) => `url("${url.replace(/["\\\n]/g, encodeURIComponent)}")`;

  // Fades the blurred backdrop in, on the page and behind the card.
  function setBackdrop(url, cardBg) {
    if (!url) { backdropEl.classList.remove("on"); return; }
    const img = new Image();
    img.onload = () => {
      backdropEl.classList.remove("on");
      requestAnimationFrame(() => {
        backdropEl.style.backgroundImage = cssURL(url);
        if (cardBg) cardBg.style.backgroundImage = cssURL(url);
        requestAnimationFrame(() => {
          backdropEl.classList.add("on");
          if (cardBg) cardBg.classList.add("on");
        });
      });
    };
    img.src = url;
  }

  function showLoading() {
    const card = el("div", "panel loading-card");
    card.append(el("div", "skel skel-poster"));
    const lines = el("div", "skel-lines");
    for (let i = 0; i < 5; i++) lines.append(el("div", "skel"));
    card.append(lines, el("p", "loading-text", "Lecture des pages Letterboxd… la première fois, cela peut prendre quelques secondes."));
    resultEl.replaceChildren(card);
    scrollToResult();
  }

  function showNotice(icon, title, text, isError = false) {
    const box = el("div", "panel notice" + (isError ? " error" : ""));
    box.append(el("div", "notice-icon", icon), el("h2", null, title), el("p", null, text));
    resultEl.replaceChildren(box);
    scrollToResult();
  }

  const fmtNum = (n) => new Intl.NumberFormat("fr-FR").format(n);
  const fmtCompact = (n) => new Intl.NumberFormat("fr-FR", { notation: "compact", maximumFractionDigits: 1 }).format(n);
  const fmtRuntime = (m) => (m >= 60 ? `${Math.floor(m / 60)} h ${String(m % 60).padStart(2, "0")}` : `${m} min`);

  function showFilm({ film, imdb, count }) {
    const node = $("#tpl-film").content.firstElementChild.cloneNode(true);

    const img = $(".film-poster img", node);
    img.alt = `Affiche de ${film.title}`;
    img.addEventListener("load", () => img.classList.add("loaded"));
    if (film.poster) img.src = film.poster;

    $(".common", node).textContent = count === 1
      ? "Le seul film en commun"
      : `${fmtNum(count)} films en commun`;
    $(".title", node).textContent = film.title;
    $(".year", node).textContent = film.year ? `(${film.year})` : "";

    const meta = $(".film-meta", node);
    const parts = [];
    if (film.directors.length) {
      const d = el("span");
      d.append("Réalisé par ", el("b", null, film.directors.join(", ")));
      parts.push(d);
    }
    if (film.runtime) parts.push(el("span", null, fmtRuntime(film.runtime)));
    parts.forEach((p, i) => { if (i) meta.append(el("span", "sep", "•")); meta.append(p); });
    if (!parts.length) meta.remove();

    const genres = $(".genres", node);
    for (const g of film.genres) genres.append(el("li", null, g));

    const badges = $(".badges", node);
    if (film.rating) {
      badges.append(badge("lb", film.url, film.rating.toFixed(1), "/5",
        film.ratingCount ? `${fmtCompact(film.ratingCount)} notes` : "Letterboxd"));
    }
    if (imdb && imdb.value && film.imdbId) {
      badges.append(badge("imdb", `https://www.imdb.com/title/${film.imdbId}/`, imdb.value.toFixed(1), "/10",
        imdb.votes ? `${fmtCompact(imdb.votes)} votes` : "IMDb"));
    }

    $(".synopsis", node).textContent = film.synopsis || "";
    $(".lb-link", node).href = film.url;
    const imdbLink = $(".imdb-link", node);
    if (film.imdbId) imdbLink.href = `https://www.imdb.com/title/${film.imdbId}/`;
    else imdbLink.remove();

    const again = $(".again", node);
    if (count < 2) { again.disabled = true; again.title = "Il n’y a qu’un seul film en commun"; }

    resultEl.replaceChildren(node);
    setBackdrop(film.backdrop || film.poster || "", $(".film-bg", node));
    scrollToResult();
  }

  function badge(kind, href, value, scale, sub) {
    const a = el("a", `badge badge-${kind}`);
    a.href = href; a.target = "_blank"; a.rel = "noopener";
    const icon = el("span", "badge-icon");
    if (kind === "lb") { icon.append(el("i"), el("i"), el("i")); a.title = "Note moyenne Letterboxd"; }
    else { icon.textContent = "IMDb"; a.title = "Note IMDb"; }
    const txt = el("span");
    const val = el("span", "badge-value", value);
    val.append(el("small", null, scale));
    txt.append(val, el("span", "badge-sub", sub));
    a.append(icon, txt);
    return a;
  }

  function scrollToResult() {
    const top = resultEl.getBoundingClientRect().top;
    if (top > window.innerHeight * 0.6 || top < 0) {
      resultEl.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "start" });
    }
  }

  /* ---------- boot ---------- */

  const saved = loadStore();
  for (const [key, side] of Object.entries(sides)) {
    const s = saved[key];
    if (s && s.user) {
      side.input.value = s.user;
      loadSources(side, s.source);
    }
  }
})();
