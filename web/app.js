"use strict";

(() => {
  /* ---------- translations ---------- */

  const I18N = {
    fr: {
      locale: "fr-FR",
      title: "matchboxd — un film pour deux",
      tagline: "Un film pour deux, tiré au sort dans vos Letterboxd.",
      person1: "Personne 1",
      person2: "Personne 2",
      username: "Pseudo Letterboxd",
      source: "Source",
      placeholderA: "ex. davidehrlich",
      placeholderB: "ex. karsten",
      pick: "Tirer un film",
      again: "Un autre",
      footer: "Données lues sur",
      notAffiliated: "Projet non affilié.",
      enterUser: "Entrez un pseudo",
      invalidUserOpt: "Pseudo invalide",
      invalidUserMsg: "Un pseudo Letterboxd ne contient que lettres, chiffres et _.",
      searching: "Recherche des sources…",
      unavailable: "Indisponible",
      groupGeneral: "Général",
      groupLists: "Listes personnalisées",
      watchlist: "Watchlist",
      films: "Films vus",
      likes: "Films aimés",
      filmsCount: (n) => `${n} films`,
      listsFound: (n) => (n === 1 ? "1 liste trouvée" : `${n} listes trouvées`),
      noLists: "Aucune liste personnalisée",
      partial: "Letterboxd a limité nos requêtes : certaines listes personnalisées peuvent manquer.",
      loading: "Lecture des pages Letterboxd… la première fois, cela peut prendre quelques secondes.",
      noneTitle: "Aucun film en commun",
      noneText: (a, b) => `« ${a.label} » de ${a.user} (${a.size} films) et « ${b.label} » de ${b.user} (${b.size} films) n’ont aucun film en commun. Essayez une autre source.`,
      errorTitle: "Impossible de tirer un film",
      onlyOne: "Le seul film en commun",
      common: (n) => `${n} films en commun`,
      onlyOneTitle: "Il n’y a qu’un seul film en commun",
      directedBy: "Réalisé par ",
      runtime: (m) => (m >= 60 ? `${Math.floor(m / 60)} h ${String(m % 60).padStart(2, "0")}` : `${m} min`),
      ratings: (n) => `${n} notes`,
      votes: (n) => `${n} votes`,
      lbRating: "Note moyenne Letterboxd",
      imdbRating: "Note IMDb",
      posterOf: (t) => `Affiche de ${t}`,
      errors: {
        invalid_user: () => "Nom d’utilisateur invalide.",
        invalid_source: () => "Source invalide.",
        bad_request: () => "Requête invalide.",
        user_not_found: (u) => `Utilisateur « ${u} » introuvable sur Letterboxd.`,
        list_not_found: (u) => `La liste de « ${u} » est introuvable ou privée.`,
        film_not_found: () => "Film introuvable sur Letterboxd.",
        private: (u) => `Cette source de « ${u} » est privée.`,
        blocked: () => "Letterboxd bloque temporairement nos requêtes (protection anti-robots). Réessayez dans quelques minutes.",
        timeout: () => "Letterboxd met trop de temps à répondre. Réessayez.",
        upstream: () => "Impossible de joindre Letterboxd pour le moment.",
        network: () => "Le serveur ne répond pas. Vérifiez qu’il est bien lancé.",
        unknown: (_, status) => `Erreur inattendue (HTTP ${status}).`,
      },
      genres: {
        Action: "Action", Adventure: "Aventure", Animation: "Animation", Comedy: "Comédie", Crime: "Crime",
        Documentary: "Documentaire", Drama: "Drame", Family: "Familial", Fantasy: "Fantastique", History: "Histoire",
        Horror: "Horreur", Music: "Musique", Mystery: "Mystère", Romance: "Romance", "Science Fiction": "Science-fiction",
        Thriller: "Thriller", "TV Movie": "Téléfilm", War: "Guerre", Western: "Western",
      },
    },
    en: {
      locale: "en-US",
      title: "matchboxd — one movie for two",
      tagline: "One movie for two, picked at random from your Letterboxd.",
      person1: "Person 1",
      person2: "Person 2",
      username: "Letterboxd username",
      source: "Source",
      placeholderA: "e.g. davidehrlich",
      placeholderB: "e.g. karsten",
      pick: "Pick a movie",
      again: "Pick another",
      footer: "Data read from",
      notAffiliated: "Not affiliated.",
      enterUser: "Enter a username",
      invalidUserOpt: "Invalid username",
      invalidUserMsg: "A Letterboxd username only contains letters, digits and _.",
      searching: "Looking for sources…",
      unavailable: "Unavailable",
      groupGeneral: "General",
      groupLists: "Custom lists",
      watchlist: "Watchlist",
      films: "Watched films",
      likes: "Liked films",
      filmsCount: (n) => `${n} films`,
      listsFound: (n) => (n === 1 ? "1 list found" : `${n} lists found`),
      noLists: "No custom lists",
      partial: "Letterboxd rate-limited us: some custom lists may be missing.",
      loading: "Reading Letterboxd pages… the first time can take a few seconds.",
      noneTitle: "No movies in common",
      noneText: (a, b) => `${a.user}’s “${a.label}” (${a.size} films) and ${b.user}’s “${b.label}” (${b.size} films) have no movies in common. Try another source.`,
      errorTitle: "Couldn’t pick a movie",
      onlyOne: "The only movie in common",
      common: (n) => `${n} movies in common`,
      onlyOneTitle: "There is only one movie in common",
      directedBy: "Directed by ",
      runtime: (m) => (m >= 60 ? `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m` : `${m} min`),
      ratings: (n) => `${n} ratings`,
      votes: (n) => `${n} votes`,
      lbRating: "Letterboxd average rating",
      imdbRating: "IMDb rating",
      posterOf: (t) => `Poster of ${t}`,
      errors: {
        invalid_user: () => "Invalid username.",
        invalid_source: () => "Invalid source.",
        bad_request: () => "Invalid request.",
        user_not_found: (u) => `User “${u}” not found on Letterboxd.`,
        list_not_found: (u) => `${u}’s list doesn’t exist or is private.`,
        film_not_found: () => "Movie not found on Letterboxd.",
        private: (u) => `This source of “${u}” is private.`,
        blocked: () => "Letterboxd is temporarily blocking our requests (bot protection). Try again in a few minutes.",
        timeout: () => "Letterboxd is taking too long to respond. Try again.",
        upstream: () => "Can’t reach Letterboxd right now.",
        network: () => "The server isn’t responding. Make sure it is running.",
        unknown: (_, status) => `Unexpected error (HTTP ${status}).`,
      },
      genres: {},
    },
  };

  const $ = (sel, root = document) => root.querySelector(sel);
  const form = $("#form");
  const pickBtn = $("#pick");
  const resultEl = $("#result");
  const backdropEl = $("#backdrop");
  const STORE_KEY = "matchboxd:v1";
  const USER_RE = /^[A-Za-z0-9_]{1,40}$/;

  let lang = "fr";
  const t = (key, ...args) => {
    const v = I18N[lang][key];
    return typeof v === "function" ? v(...args) : v;
  };
  const fmtNum = (n) => new Intl.NumberFormat(t("locale")).format(n);
  const fmtCompact = (n) => new Intl.NumberFormat(t("locale"), { notation: "compact", maximumFractionDigits: 1 }).format(n);

  // Errors carry a code from the API so they can be re-rendered in any language.
  class ApiError extends Error {
    constructor(code, user, status, fallback) {
      super(fallback || code);
      Object.assign(this, { code, user, status });
    }
  }
  const errorText = (e) => {
    const fn = I18N[lang].errors[e.code];
    return fn ? fn(e.user || "", e.status) : e.message || I18N[lang].errors.unknown("", e.status);
  };

  // State of each person.
  const sides = {};
  // Last draw, used by the "again" button: { a, b, slug }
  let lastDraw = null;
  // What the result area currently shows, so it can be re-rendered on language change.
  let view = null;

  /* ---------- persistence (best effort) ---------- */

  function loadStore() {
    try { return JSON.parse(localStorage.getItem(STORE_KEY)) || {}; } catch { return {}; }
  }
  function saveStore() {
    const data = { lang };
    for (const [key, s] of Object.entries(sides)) {
      data[key] = { user: s.input.value.trim(), source: s.select.value };
    }
    try { localStorage.setItem(STORE_KEY, JSON.stringify(data)); } catch { /* ignore */ }
  }

  /* ---------- API ---------- */

  async function api(path, opts = {}) {
    let res;
    try {
      res = await fetch(path, { ...opts, headers: { "Content-Type": "application/json", ...(opts.headers || {}) } });
    } catch (err) {
      if (err.name === "AbortError") throw err;
      throw new ApiError("network", "", 0);
    }
    let body = null;
    try { body = await res.json(); } catch { /* non-JSON */ }
    if (!res.ok) {
      throw new ApiError((body && body.code) || "unknown", body && body.user, res.status, body && body.error);
    }
    return body;
  }

  /* ---------- people panels ---------- */

  // A field message is stored as a render function so it follows the language.
  function setMsg(side, render, kind = "") {
    side.msgState = render ? { render, kind } : null;
    renderMsg(side);
  }
  function renderMsg(side) {
    const st = side.msgState;
    side.msg.textContent = st ? st.render() : "";
    side.msg.className = "field-msg" + (st && st.kind ? " " + st.kind : "");
  }

  function sourceLabel(src) {
    return src.kind === "list" ? src.label : t(src.kind);
  }

  function renderOptions(side) {
    const sel = side.select;
    if (!side.sources) {
      sel.replaceChildren(new Option(t(side.placeholder), ""));
      sel.disabled = true;
      return;
    }
    const current = sel.value;
    const general = document.createElement("optgroup");
    general.label = t("groupGeneral");
    const lists = document.createElement("optgroup");
    lists.label = t("groupLists");
    for (const src of side.sources) {
      const text = sourceLabel(src) + (src.count ? ` · ${t("filmsCount", fmtNum(src.count))}` : "");
      (src.kind === "list" ? lists : general).append(new Option(text, src.id));
    }
    sel.replaceChildren(general);
    if (lists.children.length) sel.append(lists);
    sel.disabled = false;
    if (current) sel.value = current;
  }

  function resetSelect(side, placeholder) {
    side.sources = null;
    side.placeholder = placeholder;
    renderOptions(side);
  }

  async function loadSources(side, preferred) {
    const user = side.input.value.trim().replace(/^@/, "");
    if (user === side.user && (side.sources || side.loading)) return;
    side.user = user;
    if (side.ctrl) side.ctrl.abort();

    if (!user) {
      resetSelect(side, "enterUser");
      setMsg(side, null);
      return refresh();
    }
    if (!USER_RE.test(user)) {
      resetSelect(side, "invalidUserOpt");
      setMsg(side, () => t("invalidUserMsg"), "error");
      return refresh();
    }

    side.ctrl = new AbortController();
    side.loading = true;
    side.panel.classList.add("loading");
    resetSelect(side, "searching");
    setMsg(side, null);
    refresh();
    try {
      const data = await api(`/api/sources?user=${encodeURIComponent(user)}`, { signal: side.ctrl.signal });
      side.sources = data.sources;
      renderOptions(side);
      if (preferred && data.sources.some((s) => s.id === preferred)) side.select.value = preferred;
      const nLists = data.sources.filter((s) => s.kind === "list").length;
      if (data.partial) setMsg(side, () => t("partial"), "warn");
      else setMsg(side, () => (nLists ? t("listsFound", nLists) : t("noLists")), "ok");
      saveStore();
    } catch (err) {
      if (err.name === "AbortError") return;
      side.user = null; // allow retrying the same name
      resetSelect(side, "unavailable");
      setMsg(side, () => errorText(err), "error");
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
      user: null, sources: null, placeholder: "enterUser", msgState: null,
      loading: false, ctrl: null, timer: 0,
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
    const pick = (s) => ({ user: s.user, source: s.select.value });
    return { a: pick(sides.a), b: pick(sides.b) };
  }

  // Label of a source as currently known in a panel (falls back to its ID).
  function labelFor(side, id) {
    const src = (side.sources || []).find((s) => s.id === id);
    return src ? sourceLabel(src) : id;
  }

  async function draw(again) {
    const sel = again && lastDraw ? { a: lastDraw.a, b: lastDraw.b } : selection();
    const exclude = again && lastDraw ? lastDraw.slug : "";
    const trigger = again ? $(".again", resultEl) : pickBtn;

    pickBtn.classList.add("busy");
    pickBtn.disabled = true;
    if (trigger) { trigger.classList.add("busy"); trigger.disabled = true; }
    if (!again) render({ type: "loading" });

    try {
      const data = await api("/api/pick", { method: "POST", body: JSON.stringify({ ...sel, exclude }) });
      if (!data.count) {
        lastDraw = null;
        setBackdrop("");
        render({
          type: "none",
          a: { user: sel.a.user, id: sel.a.source, size: data.sizes[0] },
          b: { user: sel.b.user, id: sel.b.source, size: data.sizes[1] },
        });
        return;
      }
      lastDraw = { ...sel, slug: data.film.slug };
      render({ type: "film", data }, true);
    } catch (err) {
      render({ type: "error", err });
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

  // fresh = a new result (animate + scroll); otherwise a re-render after a language change.
  function render(next, fresh = true) {
    view = next;
    if (!view) { resultEl.replaceChildren(); return; }
    if (view.type === "loading") renderLoading();
    else if (view.type === "none") {
      const a = { ...view.a, label: labelFor(sides.a, view.a.id), size: fmtNum(view.a.size) };
      const b = { ...view.b, label: labelFor(sides.b, view.b.id), size: fmtNum(view.b.size) };
      renderNotice("🎞️", t("noneTitle"), t("noneText", a, b));
    } else if (view.type === "error") renderNotice("⚠️", t("errorTitle"), errorText(view.err), true);
    else if (view.type === "film") renderFilm(view.data, fresh);
    resultEl.classList.toggle("static", !fresh);
    if (fresh) scrollToResult();
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

  function renderLoading() {
    const card = el("div", "panel loading-card");
    card.append(el("div", "skel skel-poster"));
    const lines = el("div", "skel-lines");
    for (let i = 0; i < 5; i++) lines.append(el("div", "skel"));
    card.append(lines, el("p", "loading-text", t("loading")));
    resultEl.replaceChildren(card);
  }

  function renderNotice(icon, title, text, isError = false) {
    const box = el("div", "panel notice" + (isError ? " error" : ""));
    box.append(el("div", "notice-icon", icon), el("h2", null, title), el("p", null, text));
    resultEl.replaceChildren(box);
  }

  function renderFilm({ film, imdb, count }, fresh) {
    const node = $("#tpl-film").content.firstElementChild.cloneNode(true);
    $("[data-i18n=again]", node).textContent = t("again");

    const img = $(".film-poster img", node);
    img.alt = t("posterOf", film.title);
    img.addEventListener("load", () => img.classList.add("loaded"));
    if (film.poster) img.src = film.poster;

    $(".common", node).textContent = count === 1 ? t("onlyOne") : t("common", fmtNum(count));
    $(".title", node).textContent = film.title;
    $(".year", node).textContent = film.year ? `(${film.year})` : "";

    const meta = $(".film-meta", node);
    const parts = [];
    if (film.directors.length) {
      const d = el("span");
      d.append(t("directedBy"), el("b", null, film.directors.join(", ")));
      parts.push(d);
    }
    if (film.runtime) parts.push(el("span", null, t("runtime", film.runtime)));
    parts.forEach((p, i) => { if (i) meta.append(el("span", "sep", "•")); meta.append(p); });
    if (!parts.length) meta.remove();

    const genres = $(".genres", node);
    for (const g of film.genres) genres.append(el("li", null, I18N[lang].genres[g] || g));

    const badges = $(".badges", node);
    if (film.rating) {
      badges.append(badge("lb", film.url, film.rating.toFixed(1), "/5",
        film.ratingCount ? t("ratings", fmtCompact(film.ratingCount)) : "Letterboxd", t("lbRating")));
    }
    if (imdb && imdb.value && film.imdbId) {
      badges.append(badge("imdb", `https://www.imdb.com/title/${film.imdbId}/`, imdb.value.toFixed(1), "/10",
        imdb.votes ? t("votes", fmtCompact(imdb.votes)) : "IMDb", t("imdbRating")));
    }

    $(".synopsis", node).textContent = film.synopsis || "";
    $(".lb-link", node).href = film.url;
    const imdbLink = $(".imdb-link", node);
    if (film.imdbId) imdbLink.href = `https://www.imdb.com/title/${film.imdbId}/`;
    else imdbLink.remove();

    const again = $(".again", node);
    if (count < 2) { again.disabled = true; again.title = t("onlyOneTitle"); }

    const cardBg = $(".film-bg", node);
    if (!fresh && backdropEl.style.backgroundImage) {
      // Language change: keep the backdrop already shown, without fading again.
      cardBg.style.backgroundImage = backdropEl.style.backgroundImage;
      cardBg.classList.add("on");
      img.classList.add("loaded");
    }
    resultEl.replaceChildren(node);
    if (fresh) setBackdrop(film.backdrop || film.poster || "", cardBg);
  }

  function badge(kind, href, value, scale, sub, title) {
    const a = el("a", `badge badge-${kind}`);
    a.href = href; a.target = "_blank"; a.rel = "noopener"; a.title = title;
    const icon = el("span", "badge-icon");
    if (kind === "lb") icon.append(el("i"), el("i"), el("i"));
    else icon.textContent = "IMDb";
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

  /* ---------- language ---------- */

  function setLang(next, persist = true) {
    lang = I18N[next] ? next : "fr";
    document.documentElement.lang = lang;
    document.title = t("title");
    for (const n of document.querySelectorAll("[data-i18n]")) n.textContent = t(n.dataset.i18n);
    for (const n of document.querySelectorAll("[data-i18n-placeholder]")) n.placeholder = t(n.dataset.i18nPlaceholder);
    for (const b of document.querySelectorAll(".lang button")) b.setAttribute("aria-pressed", String(b.dataset.lang === lang));
    for (const side of Object.values(sides)) { renderOptions(side); renderMsg(side); }
    if (view) render(view, false);
    if (persist) saveStore();
  }

  for (const b of document.querySelectorAll(".lang button")) {
    b.addEventListener("click", () => setLang(b.dataset.lang));
  }

  /* ---------- boot ---------- */

  const saved = loadStore();
  const browserLang = (navigator.language || "fr").toLowerCase().startsWith("fr") ? "fr" : "en";
  setLang(saved.lang || browserLang, false);
  for (const [key, side] of Object.entries(sides)) {
    const s = saved[key];
    if (s && s.user) {
      side.input.value = s.user;
      loadSources(side, s.source);
    }
  }
})();
