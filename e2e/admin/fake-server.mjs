// A small in-memory stand-in for the admin API in docs/admin-api.md, for the
// browser tests and for working on the admin without the Go server. Dev and
// test only: no auth, no persistence, and a fake preview. The admin document
// and its files come from Go (server/main.go), which passes the rest here.
//
//   node e2e/admin/fake-server.mjs --port 18097
//
// Test hooks (not part of the contract): POST /__reset, POST /__touch/<id>.
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const arg = (name, def) => {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : def;
};
const port = Number(arg("port", 18097));

const MAX_UPLOAD = 40 * 1024 * 1024;
// As images.Kind: shape as whole numbers, and the narrowest crop allowed.
const KINDS = {
  card: { an: 4, ad: 5, min: 480 },
  banner: { an: 16, ad: 9, min: 800 },
  share: { an: 40, ad: 21, min: 1200 },
};
const MAX_PIXELS = 100_000_000;
// Variants a photo's sizes take; any count would do, the app shows it as is.
const VARIANTS = 12;

let db;
let clock;

// Strictly increasing timestamps, so the updatedAt precondition is exact.
const now = () =>
  new Date(Date.UTC(2026, 10, 10, 9, 0, 0) + ++clock * 1000).toISOString();

// images.CentredCrop: the largest centred rectangle of the kind's shape.
function centredCrop(k, w, h) {
  const { an, ad } = KINDS[k];
  let cw = w;
  let ch = Math.floor((w * ad) / an);
  if (w * ad > h * an) {
    cw = Math.floor((h * an) / ad);
    ch = h;
  }
  return {
    x: Math.floor((w - cw) / 2),
    y: Math.floor((h - ch) / 2),
    w: cw,
    h: ch,
  };
}

const usable = (k, w, h) => centredCrop(k, w, h).w >= KINDS[k].min;

// The stored crops of a new photo: only the kinds it is large enough for.
function defaultCrops(w, h) {
  const out = {};
  for (const k of Object.keys(KINDS))
    if (usable(k, w, h)) out[k] = centredCrop(k, w, h);
  return out;
}

async function addPhoto(bytes, mime, alt = "", ready = false) {
  const size = imageSize(bytes);
  const id = ++db.seq;
  db.photos.set(id, {
    id,
    bytes,
    mime,
    width: size.width,
    height: size.height,
    alt,
    photographerId: null,
    crops: defaultCrops(size.width, size.height),
    // As the server: the preview is made before the upload answers.
    status: ready
      ? {
          state: "ready",
          done: VARIANTS,
          total: VARIANTS,
          previewReady: true,
          reason: "",
        }
      : {
          state: "preparing",
          done: 0,
          total: VARIANTS,
          previewReady: true,
          reason: "",
        },
  });
  return db.photos.get(id);
}

async function reset() {
  clock = 0;
  db = {
    seq: 0,
    photos: new Map(),
    events: new Map(),
    photographers: new Map(),
    gallery: [],
  };
  const land = await readFile(join(here, "fixtures", "landskap.jpg"));
  const port_ = await readFile(join(here, "fixtures", "portrett.jpg"));
  const p1 = await addPhoto(
    land,
    "image/jpeg",
    "Boder med julepynt i låven",
    true,
  );
  const p2 = await addPhoto(
    port_,
    "image/jpeg",
    "Kursholder ved kjøkkenbenken",
    true,
  );
  const ph = { id: ++db.seq, name: "Ola Nordmann", url: "https://example.no" };
  db.photographers.set(ph.id, ph);
  p1.photographerId = ph.id;
  db.gallery = [p1.id, p2.id];
  const ev = {
    id: ++db.seq,
    status: "published",
    slug: "takkebakstkurs",
    year: 2026,
    title: "Takkebakstkurs",
    summary: "Lær å bake lefser og flatbrød.",
    body: "Velkommen til kurs!",
    start: "2026-10-31",
    startTime: "10:00",
    end: "",
    endTime: "15:00",
    extraDates: [],
    priceText: "Kr 1650,-",
    ticketUrl: "",
    showInMenu: false,
    menuLabel: "",
    cancelled: false,
    mainPhoto: { id: p2.id },
    extraPhotos: [],
    publishedAt: now(),
    createdAt: now(),
    updatedAt: now(),
  };
  db.events.set(ev.id, ev);
}

// --- image size (JPEG SOF / PNG IHDR), enough for the fixtures ---

function imageSize(b) {
  if (b.length > 24 && b.readUInt32BE(0) === 0x89504e47) {
    return { width: b.readUInt32BE(16), height: b.readUInt32BE(20) };
  }
  if (b[0] === 0xff && b[1] === 0xd8) {
    let i = 2;
    while (i < b.length) {
      if (b[i] !== 0xff) return null;
      const marker = b[i + 1];
      const len = b.readUInt16BE(i + 2);
      if (
        marker >= 0xc0 &&
        marker <= 0xcf &&
        ![0xc4, 0xc8, 0xcc].includes(marker)
      ) {
        return { height: b.readUInt16BE(i + 5), width: b.readUInt16BE(i + 7) };
      }
      i += 2 + len;
    }
  }
  return null;
}

// --- JSON shapes ---

const photographerJson = (id) => (id && db.photographers.get(id)) || null;

function photoJson(p) {
  return {
    id: p.id,
    width: p.width,
    height: p.height,
    alt: p.alt,
    photographer: photographerJson(p.photographerId),
    previewUrl: `/fake-img/${p.id}/preview`,
    thumbUrl: `/fake-img/${p.id}/thumb`,
    // Every kind, as the server: an unset one shows its centred rectangle.
    crops: Object.fromEntries(
      Object.keys(KINDS).map((k) => [
        k,
        p.crops[k] || centredCrop(k, p.width, p.height),
      ]),
    ),
    status: p.status,
  };
}

const months = [
  "januar",
  "februar",
  "mars",
  "april",
  "mai",
  "juni",
  "juli",
  "august",
  "september",
  "oktober",
  "november",
  "desember",
];
function norwegianDate(s) {
  const [y, m, d] = s.split("-").map(Number);
  return `${String(d).padStart(2, "0")}. ${months[m - 1]} ${y}`;
}

function dateLine(e) {
  if (!e.start) return "";
  let s;
  if (e.extraDates.length) {
    s = [e.start, ...e.extraDates].sort().map(norwegianDate).join(", ");
    if (e.startTime) s += ` kl ${e.startTime}`;
    if (e.endTime) s += ` - ${e.endTime}`;
  } else {
    s = norwegianDate(e.start);
    if (e.startTime) s += ` kl ${e.startTime}`;
    if (!e.end || e.end === e.start) {
      if (e.endTime) s += ` - ${e.endTime}`;
    } else {
      s += ` - ${norwegianDate(e.end)}`;
      if (e.endTime) s += ` kl ${e.endTime}`;
    }
  }
  return `${s} på Aspargesgården`;
}

const today = "2026-11-10";
const lastDay = (e) =>
  [e.start, e.end, ...e.extraDates].filter(Boolean).sort().at(-1) || "";
const isOver = (e) => !!e.start && lastDay(e) < today;

function eventJson(e) {
  const path = e.year && e.slug ? `/arrangementer/${e.year}/${e.slug}` : "";
  const photo = (r) => {
    const p = db.photos.get(r.id);
    return p ? photoJson(p) : null;
  };
  return {
    id: e.id,
    status: e.status,
    slug: e.slug,
    slugFrozen: !!e.publishedAt,
    year: e.year,
    path,
    url: path ? `https://aspargesgaarden.no${path}` : "",
    dateText: dateLine(e),
    isOver: isOver(e),
    title: e.title,
    summary: e.summary,
    body: e.body,
    start: e.start,
    startTime: e.startTime,
    end: e.end,
    endTime: e.endTime,
    extraDates: e.extraDates,
    priceText: e.priceText,
    ticketUrl: e.ticketUrl,
    showInMenu: e.showInMenu,
    menuLabel: e.menuLabel,
    cancelled: e.cancelled,
    mainPhoto: e.mainPhoto ? photo(e.mainPhoto) : null,
    extraPhotos: e.extraPhotos.map(photo).filter(Boolean),
    publishedAt: e.publishedAt,
    createdAt: e.createdAt,
    updatedAt: e.updatedAt,
  };
}

// --- rules (a subset of site/, enough to exercise the app) ---

function slugify(t) {
  return t
    .toLowerCase()
    .replaceAll("æ", "ae")
    .replaceAll("ø", "o")
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

const validUrl = (s) => /^https?:\/\/[^/\s]+/.test(s);

function storedRules(e) {
  const errs = [];
  if (e.slug && !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(e.slug))
    errs.push({ code: "slug_invalid", field: "slug" });
  for (const f of ["startTime", "endTime"]) {
    if (e[f] && !/^([01]\d|2[0-3]):[0-5]\d$/.test(e[f]))
      errs.push({ code: "invalid_time", field: f });
  }
  if (e.end && (!e.start || e.end < e.start))
    errs.push({ code: "end_before_start", field: "end" });
  else if (
    (!e.end || e.end === e.start) &&
    e.startTime &&
    e.endTime &&
    e.endTime <= e.startTime
  ) {
    errs.push({ code: "end_before_start", field: "endTime" });
  }
  if (
    e.extraDates.some(
      (d, i) => !e.start || d <= e.start || d === e.extraDates[i - 1],
    )
  ) {
    errs.push({ code: "extra_date", field: "extraDates" });
  }
  if (e.ticketUrl && !validUrl(e.ticketUrl))
    errs.push({ code: "ticket_url", field: "ticketUrl" });
  if (e.status === "published") errs.push(...publishRules(e));
  return errs;
}

function publishRules(e) {
  const errs = [];
  if (!e.title.trim()) errs.push({ code: "title_required", field: "title" });
  if (!e.start) errs.push({ code: "start_required", field: "start" });
  if (!e.mainPhoto)
    errs.push({ code: "main_photo_required", field: "mainPhoto" });
  const refs = [
    ...(e.mainPhoto ? [["mainPhoto", e.mainPhoto]] : []),
    ...e.extraPhotos.map((r) => ["extraPhotos", r]),
  ];
  for (const [field, r] of refs) {
    if (!db.photos.get(r.id)?.alt.trim())
      errs.push({ code: "alt_required", field, photoId: r.id });
  }
  return errs;
}

// Applies an EventInput to e (new or stored), as site.Revise does.
function revise(prev, input) {
  const trim = (s) => String(s ?? "").trim();
  const e = {
    ...(prev || { status: "draft", publishedAt: null, createdAt: now() }),
    title: trim(input.title),
    summary: String(input.summary ?? ""),
    body: String(input.body ?? ""),
    start: input.start || "",
    startTime: input.startTime || "",
    end: input.end || "",
    endTime: input.endTime || "",
    extraDates: [...(input.extraDates || [])].sort(),
    priceText: trim(input.priceText),
    ticketUrl: trim(input.ticketUrl),
    showInMenu: !!input.showInMenu,
    menuLabel: trim(input.menuLabel),
    cancelled: !!input.cancelled,
    mainPhoto: input.mainPhoto ? { id: input.mainPhoto.id } : null,
    extraPhotos: (input.extraPhotos || []).map((r) => ({ id: r.id })),
  };
  const frozen = prev && prev.publishedAt;
  if (frozen) {
    e.slug = prev.slug;
    e.year = prev.year;
  } else {
    e.slug = trim(input.slug) || slugify(e.title);
    e.year = e.start ? Number(e.start.slice(0, 4)) : 0;
  }
  return e;
}

function applyPhotoRefs(input) {
  for (const r of [input.mainPhoto, ...(input.extraPhotos || [])].filter(
    Boolean,
  )) {
    const p = db.photos.get(r.id);
    if (!p) return { code: "not_found", photoId: r.id };
    if (r.photographerId != null && !db.photographers.has(r.photographerId)) {
      return { code: "not_found", photoId: r.id };
    }
  }
  for (const r of [input.mainPhoto, ...(input.extraPhotos || [])].filter(
    Boolean,
  )) {
    const p = db.photos.get(r.id);
    p.alt = String(r.alt ?? "").trim();
    p.photographerId = r.photographerId ?? null;
  }
  return null;
}

function slugTaken(e) {
  return [...db.events.values()].some(
    (o) => o.id !== e.id && o.year === e.year && o.slug === e.slug && e.year,
  );
}

// --- preview: plainly fake, never mistaken for the real renderer ---

const esc = (s) =>
  String(s).replace(
    /[&<>"]/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c],
  );

// No inline styles: the frame inherits the admin's CSP, as the real one does.
function previewHtml(view, input, saved) {
  const e = revise(saved || null, input);
  const main = e.mainPhoto && db.photos.get(e.mainPhoto.id);
  const img = main
    ? `<img src="/fake-img/${main.id}/preview" alt="${esc(input.mainPhoto.alt)}" width="100%">`
    : "";
  const body =
    view === "list"
      ? `<h1>Arrangementer</h1><article data-preview="list">${img}<h2>${esc(e.title)}</h2><p>${esc(dateLine(e))}</p><p>${esc(e.summary)}</p></article>`
      : `<div data-preview="page">${img}<h1>${esc(e.title)}</h1><p><b>Dato:</b> ${esc(dateLine(e))}</p><p>${esc(e.summary)}</p><pre>${esc(e.body)}</pre></div>`;
  return `<!doctype html><html lang="nb"><head><meta charset="utf-8"><title>Forhåndsvisning</title></head><body><p>FAKE PREVIEW (e2e/fake-server.mjs)</p>${body}</body></html>`;
}

// --- HTTP ---

function send(res, status, body, type = "application/json; charset=utf-8") {
  res.writeHead(status, {
    "Content-Type": type,
    "Cache-Control": "no-store",
  });
  res.end(
    type.startsWith("application/json") && body !== undefined
      ? JSON.stringify(body)
      : body,
  );
}

const fail = (res, status, errors, extra = {}) =>
  send(res, status, { errors, ...extra });

async function readBody(req, limit = 1 << 20) {
  const chunks = [];
  let n = 0;
  for await (const c of req) {
    n += c.length;
    if (n > limit) return null;
    chunks.push(c);
  }
  return Buffer.concat(chunks);
}

// As readJSON: only application/json is read.
async function json(req, res) {
  if (
    (req.headers["content-type"] || "").split(";")[0].trim() !==
    "application/json"
  ) {
    fail(res, 400, [{ code: "bad_request" }]);
    return undefined;
  }
  try {
    return JSON.parse((await readBody(req)).toString("utf8") || "null");
  } catch {
    fail(res, 400, [{ code: "bad_request" }]);
    return undefined;
  }
}

// Finds the "file" part of a multipart body.
function filePart(buf, contentType) {
  const boundary = /boundary=(?:"([^"]+)"|([^;]+))/.exec(contentType || "");
  if (!boundary) return null;
  const sep = Buffer.from(`--${boundary[1] || boundary[2]}`);
  let start = buf.indexOf(sep);
  while (start >= 0) {
    const next = buf.indexOf(sep, start + sep.length);
    if (next < 0) break;
    const part = buf.subarray(start + sep.length + 2, next - 2);
    const headEnd = part.indexOf("\r\n\r\n");
    const head = part.subarray(0, headEnd).toString();
    if (/name="file"/.test(head)) {
      return {
        name: /filename="([^"]*)"/.exec(head)?.[1] || "",
        bytes: part.subarray(headEnd + 4),
      };
    }
    start = next;
  }
  return null;
}

async function api(req, res, path, url) {
  const m = (re) => re.exec(path);
  let r;

  if (req.method === "GET" && path === "me")
    return send(res, 200, {
      name: "Utvikler",
      siteUrl: "https://aspargesgaarden.no",
    });

  if (req.method === "GET" && path === "events") {
    const all = [...db.events.values()];
    const pub = all.filter((e) => e.status === "published");
    return send(res, 200, {
      upcoming: pub
        .filter((e) => !isOver(e))
        .sort((a, b) => (a.start || "").localeCompare(b.start || ""))
        .map(eventJson),
      drafts: all
        .filter((e) => e.status === "draft")
        .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
        .map(eventJson),
      past: pub
        .filter(isOver)
        .sort((a, b) => b.start.localeCompare(a.start))
        .map(eventJson),
    });
  }

  if (req.method === "POST" && path === "events") {
    const input = await json(req, res);
    if (input === undefined) return;
    const bad = applyPhotoRefs(input);
    if (bad) return fail(res, 404, [bad]);
    const e = revise(null, input);
    const errs = storedRules(e);
    if (slugTaken(e)) errs.push({ code: "slug_taken", field: "slug" });
    if (errs.length) return fail(res, 422, errs);
    e.id = ++db.seq;
    e.updatedAt = now();
    db.events.set(e.id, e);
    return send(res, 201, eventJson(e));
  }

  if ((r = m(/^events\/(\d+)(?:\/(publish|unpublish|copy))?$/))) {
    const prev = db.events.get(Number(r[1]));
    if (!prev) return fail(res, 404, [{ code: "not_found" }]);
    const action = r[2];

    if (req.method === "GET" && !action) return send(res, 200, eventJson(prev));
    if (req.method === "DELETE" && !action) {
      db.events.delete(prev.id);
      return send(res, 204, "", "text/plain");
    }
    if (req.method === "POST" && action === "copy") {
      const c = structuredClone(prev);
      Object.assign(c, {
        id: ++db.seq,
        year: 0,
        start: "",
        end: "",
        extraDates: [],
        status: "draft",
        cancelled: false,
        publishedAt: null,
        createdAt: now(),
      });
      c.updatedAt = now();
      db.events.set(c.id, c);
      return send(res, 201, eventJson(c));
    }

    const input = await json(req, res);
    if (input === undefined) return;
    if (!input?.updatedAt || input.updatedAt !== prev.updatedAt)
      return fail(res, 409, [{ code: "conflict" }], {
        current: eventJson(prev),
      });

    let next;
    if (req.method === "PUT" && !action) {
      const bad = applyPhotoRefs(input);
      if (bad) return fail(res, 404, [bad]);
      next = revise(prev, input);
      if (!prev.publishedAt && input.slug && slugTaken(next))
        return fail(res, 422, [{ code: "slug_taken", field: "slug" }]);
    } else if (req.method === "POST" && action === "publish") {
      next = {
        ...prev,
        status: "published",
        publishedAt: prev.publishedAt || now(),
      };
    } else if (req.method === "POST" && action === "unpublish") {
      next = { ...prev, status: "draft" };
    } else {
      return fail(res, 404, [{ code: "not_found" }]);
    }
    const errs = storedRules(next);
    if (errs.length) return fail(res, 422, errs);
    next.updatedAt = now();
    db.events.set(next.id, next);
    return send(res, 200, eventJson(next));
  }

  if (req.method === "POST" && path === "preview") {
    const body = await json(req, res);
    if (body === undefined) return;
    return send(
      res,
      200,
      previewHtml(body.view, body.event, body.id && db.events.get(body.id)),
      "text/html; charset=utf-8",
    );
  }

  if (req.method === "POST" && path === "photos") {
    const buf = await readBody(req, (MAX_UPLOAD + 1) << 20);
    if (!buf) return fail(res, 413, [{ code: "too_large" }]);
    const part = filePart(buf, req.headers["content-type"]);
    if (!part) return fail(res, 400, [{ code: "bad_request" }]);
    if (part.bytes.length > MAX_UPLOAD)
      return fail(res, 413, [{ code: "too_large" }]);
    const isJpeg = part.bytes[0] === 0xff && part.bytes[1] === 0xd8;
    const isPng =
      part.bytes.length > 8 && part.bytes.readUInt32BE(0) === 0x89504e47;
    if (!isJpeg && !isPng)
      return fail(res, 415, [{ code: "unsupported_type" }]);
    const size = imageSize(part.bytes);
    if (!size) return fail(res, 422, [{ code: "damaged" }]);
    if (size.width * size.height > MAX_PIXELS)
      return fail(res, 422, [{ code: "too_many_pixels" }]);
    if (size.width < 480) return fail(res, 422, [{ code: "too_small" }]);
    const same = [...db.photos.values()].find((x) =>
      x.bytes.equals(part.bytes),
    );
    if (same) return send(res, 200, photoJson(same));
    const p = await addPhoto(part.bytes, isJpeg ? "image/jpeg" : "image/png");
    return send(res, 201, photoJson(p));
  }

  if (req.method === "GET" && path === "photos/status") {
    const statuses = {};
    for (const id of (url.searchParams.get("ids") || "")
      .split(",")
      .map(Number)) {
      const p = db.photos.get(id);
      if (!p) continue;
      // Each poll moves a preparing photo along.
      const s = p.status;
      if (s.state === "preparing") {
        s.done = Math.min(s.total, s.done + s.total / 2);
        if (s.done === s.total) s.state = "ready";
      }
      statuses[id] = s;
    }
    return send(res, 200, { statuses });
  }

  if ((r = m(/^photos\/(\d+)(?:\/(crops|prepare))?$/))) {
    const p = db.photos.get(Number(r[1]));
    if (!p) return fail(res, 404, [{ code: "not_found" }]);
    if (req.method === "GET" && !r[2]) return send(res, 200, photoJson(p));
    if (req.method === "DELETE" && !r[2]) {
      const usedBy = [...db.events.values()]
        .filter(
          (e) =>
            e.mainPhoto?.id === p.id ||
            e.extraPhotos.some((x) => x.id === p.id),
        )
        .map((e) => ({ id: e.id, title: e.title }));
      if (usedBy.length)
        return fail(res, 409, [{ code: "photo_in_use" }], { usedBy });
      db.gallery = db.gallery.filter((id) => id !== p.id);
      db.photos.delete(p.id);
      return send(res, 204, "", "text/plain");
    }
    if (req.method === "POST" && r[2] === "prepare") {
      if (p.status.state === "failed")
        p.status = {
          state: "preparing",
          done: 0,
          total: VARIANTS,
          previewReady: p.status.previewReady,
          reason: "",
        };
      return send(res, 202, p.status);
    }
    const body = await json(req, res);
    if (body === undefined) return;
    if (req.method === "PUT" && !r[2]) {
      if (
        body.photographerId != null &&
        !db.photographers.has(body.photographerId)
      ) {
        return fail(res, 404, [
          { code: "not_found", field: "photographerId", photoId: p.id },
        ]);
      }
      p.alt = String(body.alt ?? "").trim();
      p.photographerId = body.photographerId ?? null;
      return send(res, 200, photoJson(p));
    }
    if (req.method === "PUT" && r[2] === "crops") {
      const errs = [];
      const keep = {};
      for (const [k, c] of Object.entries(body || {})) {
        const kind = KINDS[k];
        const centred = kind && centredCrop(k, p.width, p.height);
        // The centred rectangle the photo shows for a kind it is too small
        // for comes back with the rest: accepted, nothing stored.
        if (
          kind &&
          !usable(k, p.width, p.height) &&
          JSON.stringify(c) === JSON.stringify(centred)
        )
          continue;
        const want = kind && (c.w * kind.ad) / kind.an;
        if (!kind) errs.push({ code: "crop_kind", field: k });
        else if (
          c.x < 0 ||
          c.y < 0 ||
          c.x + c.w > p.width ||
          c.y + c.h > p.height
        )
          errs.push({ code: "crop_outside", field: k });
        else if (Math.abs(c.h - want) > Math.max(2, want * 0.01))
          errs.push({ code: "crop_aspect", field: k });
        else if (c.w < kind.min)
          errs.push({ code: "crop_too_small", field: k });
        else keep[k] = c;
      }
      if (errs.length) return fail(res, 422, errs);
      Object.assign(p.crops, keep);
      p.status = { ...p.status, state: "preparing", done: 0, total: VARIANTS };
      return send(res, 200, photoJson(p));
    }
  }

  if (req.method === "GET" && path === "photographers") {
    return send(res, 200, {
      photographers: [...db.photographers.values()].sort((a, b) =>
        a.name.localeCompare(b.name),
      ),
    });
  }

  if (
    (r = m(/^photographers(?:\/(\d+))?$/)) &&
    (req.method === "POST" || req.method === "PUT")
  ) {
    const body = await json(req, res);
    if (body === undefined) return;
    const id = r[1] ? Number(r[1]) : null;
    if (id && !db.photographers.has(id))
      return fail(res, 404, [{ code: "not_found" }]);
    const name = String(body.name ?? "").trim();
    const link = String(body.url ?? "").trim();
    const errs = [];
    if (!name) errs.push({ code: "photographer_name_required", field: "name" });
    if (link && !validUrl(link))
      errs.push({ code: "photographer_url", field: "url" });
    if (errs.length) return fail(res, 422, errs);
    const existing = [...db.photographers.values()].find(
      (x) => x.id !== id && x.name.toLowerCase() === name.toLowerCase(),
    );
    if (existing)
      return fail(
        res,
        409,
        [{ code: "photographer_name_taken", field: "name" }],
        { existing },
      );
    const ph = { id: id || ++db.seq, name, url: link };
    db.photographers.set(ph.id, ph);
    return send(res, id ? 200 : 201, ph);
  }

  if (path === "gallery" && req.method === "GET")
    return send(res, 200, {
      photos: db.gallery.map((id) => photoJson(db.photos.get(id))),
    });
  if (path === "gallery" && req.method === "PUT") {
    const body = await json(req, res);
    if (body === undefined) return;
    if (!Array.isArray(body?.photoIds))
      return fail(res, 400, [{ code: "bad_request" }]);
    const ids = body.photoIds;
    const missing = ids.find((id) => !db.photos.has(id));
    if (missing)
      return fail(res, 404, [{ code: "not_found", photoId: missing }]);
    const dup = ids.find((id, i) => ids.indexOf(id) !== i);
    if (dup) return fail(res, 422, [{ code: "duplicate_photo", photoId: dup }]);
    db.gallery = ids;
    return send(res, 200, {
      photos: ids.map((id) => photoJson(db.photos.get(id))),
    });
  }

  return fail(res, 404, [{ code: "not_found" }]);
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url, "http://localhost");
  const p = url.pathname;
  try {
    if (req.method === "POST" && p === "/__reset") {
      await reset();
      return send(res, 204, "", "text/plain");
    }
    let t;
    if (req.method === "POST" && (t = /^\/__touch\/(\d+)$/.exec(p))) {
      const e = db.events.get(Number(t[1]));
      if (e) e.updatedAt = now();
      return send(res, 204, "", "text/plain");
    }
    if (p.startsWith("/admin/api/"))
      return await api(req, res, p.slice("/admin/api/".length), url);
    if ((t = /^\/fake-img\/(\d+)\/(preview|thumb)$/.exec(p))) {
      const photo = db.photos.get(Number(t[1]));
      if (!photo || !photo.status.previewReady)
        return send(res, 404, "not found", "text/plain");
      return send(res, 200, photo.bytes, photo.mime);
    }
    send(res, 404, "not found", "text/plain");
  } catch (err) {
    console.error(err);
    if (!res.headersSent) fail(res, 500, [{ code: "internal" }]);
  }
});

await reset();
server.listen(port, "127.0.0.1", () =>
  console.log(`ready: fake admin API on http://127.0.0.1:${port}/admin/api/`),
);
