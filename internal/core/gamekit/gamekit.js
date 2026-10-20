/* GameKit: touch-first ввод, экранные кнопки, звук и адаптация экрана для игр в WebView. */
(function () {
  if (window.GameKit) return;
  var doc = document, win = window;
  var handlers = {}, held = {}, stats = { tap: 0, swipe: 0, drag: 0, hold: 0, button: 0, key: 0, miss: 0 };
  var roundStats = null, ui = { scale: 1 }, canvasEl = null, ctx2d = null, resizeFns = [];
  var muted = false, actx = null;

  function emit(type, e) {
    var list = handlers[type];
    if (!list) return;
    for (var i = 0; i < list.length; i++) {
      try { list[i](e || {}); } catch (err) { setTimeout(function () { throw err; }); }
    }
  }
  function count(k) { stats[k]++; if (roundStats) roundStats[k]++; }

  /* ---------- экран ---------- */
  function ensureMeta() {
    var head = doc.head || doc.getElementsByTagName("head")[0];
    if (!head || doc.querySelector('meta[name="viewport"]')) return;
    var m = doc.createElement("meta");
    m.name = "viewport";
    m.content = "width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no,viewport-fit=cover";
    head.appendChild(m);
  }
  function baseStyle() {
    var s = doc.createElement("style");
    s.textContent =
      "html,body{margin:0;padding:0;width:100%;height:100%;overflow:hidden;overscroll-behavior:none;" +
      "touch-action:none;-webkit-user-select:none;user-select:none;-webkit-touch-callout:none;" +
      "-webkit-tap-highlight-color:transparent}" +
      ".gk-layer{position:fixed;top:0;left:0;right:0;bottom:0;pointer-events:none;z-index:2147483000}" +
      ".gk-btn{position:absolute;pointer-events:auto;display:flex;align-items:center;justify-content:center;" +
      "border-radius:50%;background:rgba(255,255,255,.22);border:3px solid rgba(255,255,255,.75);color:#fff;" +
      "font:700 28px/1 system-ui,sans-serif;text-shadow:0 1px 3px rgba(0,0,0,.6);box-sizing:border-box;touch-action:none}" +
      ".gk-btn.gk-on{background:rgba(255,255,255,.5);transform:scale(.94)}" +
      ".gk-mute{position:absolute;pointer-events:auto;width:48px;height:48px;border-radius:12px;border:0;" +
      "background:rgba(0,0,0,.35);color:#fff;font-size:24px}";
    (doc.head || doc.documentElement).appendChild(s);
  }
  function safeArea() {
    var probe = doc.createElement("div");
    probe.style.cssText = "position:fixed;visibility:hidden;padding:env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left)";
    doc.body.appendChild(probe);
    var cs = getComputedStyle(probe);
    var r = { top: parseFloat(cs.paddingTop) || 0, right: parseFloat(cs.paddingRight) || 0, bottom: parseFloat(cs.paddingBottom) || 0, left: parseFloat(cs.paddingLeft) || 0 };
    doc.body.removeChild(probe);
    return r;
  }

  var view = { width: win.innerWidth, height: win.innerHeight, dpr: 1 };
  function resize() {
    view.width = win.innerWidth; view.height = win.innerHeight;
    view.dpr = Math.min(win.devicePixelRatio || 1, 3);
    if (canvasEl) {
      canvasEl.width = Math.round(view.width * view.dpr);
      canvasEl.height = Math.round(view.height * view.dpr);
      canvasEl.style.width = view.width + "px";
      canvasEl.style.height = view.height + "px";
      if (ctx2d) ctx2d.setTransform(view.dpr, 0, 0, view.dpr, 0, 0);
    }
    GK.safeArea = doc.body ? safeArea() : GK.safeArea;
    layoutButtons();
    for (var i = 0; i < resizeFns.length; i++) {
      try { resizeFns[i](view.width, view.height); } catch (e) { setTimeout(function () { throw e; }); }
    }
  }

  /* ---------- жесты ---------- */
  var TAP_MS = 300, MOVE_PX = 12, SWIPE_PX = 36, HOLD_MS = 420;
  var g = null;
  function pos(e) {
    var r = canvasEl ? canvasEl.getBoundingClientRect() : { left: 0, top: 0 };
    return { x: e.clientX - r.left, y: e.clientY - r.top };
  }
  function onDown(e) {
    if (e.target && e.target.closest && e.target.closest(".gk-btn,.gk-mute")) return;
    unlockAudio();
    if (g) return;
    var p = pos(e);
    g = { id: e.pointerId, x0: p.x, y0: p.y, x: p.x, y: p.y, t0: Date.now(), moved: false, swiped: false, held: false };
    g.timer = setTimeout(function () {
      if (g && !g.moved) { g.held = true; count("hold"); emit("hold", { x: g.x, y: g.y }); }
    }, HOLD_MS);
    GK.pointer.down = true; GK.pointer.x = p.x; GK.pointer.y = p.y;
    emit("pointerdown", p);
  }
  function onMove(e) {
    if (!g || e.pointerId !== g.id) return;
    var p = pos(e), dx = p.x - g.x0, dy = p.y - g.y0, dist = Math.sqrt(dx * dx + dy * dy);
    GK.pointer.x = p.x; GK.pointer.y = p.y;
    if (!g.moved && dist > MOVE_PX) {
      g.moved = true; clearTimeout(g.timer);
      count("drag"); emit("dragstart", { x: g.x0, y: g.y0 });
    }
    if (g.moved) emit("drag", { x: p.x, y: p.y, dx: p.x - g.x, dy: p.y - g.y, totalDx: dx, totalDy: dy });
    if (!g.swiped && !g.held && dist > SWIPE_PX) {
      g.swiped = true; count("swipe");
      emit("swipe", { dir: Math.abs(dx) > Math.abs(dy) ? (dx > 0 ? "right" : "left") : (dy > 0 ? "down" : "up"), dx: dx, dy: dy, x: g.x0, y: g.y0 });
    }
    g.x = p.x; g.y = p.y;
  }
  function onUp(e) {
    if (!g || e.pointerId !== g.id) return;
    clearTimeout(g.timer);
    var p = pos(e);
    if (g.moved) emit("dragend", { x: p.x, y: p.y });
    if (g.held) emit("release", { x: p.x, y: p.y });
    if (!g.moved && !g.held && Date.now() - g.t0 < TAP_MS) {
      count("tap"); emit("tap", { x: p.x, y: p.y }); emit("action", { source: "tap", x: p.x, y: p.y });
    }
    GK.pointer.down = false;
    emit("pointerup", p);
    g = null;
  }

  /* ---------- экранные кнопки ---------- */
  var layer = null, buttons = [];
  var PRESETS = {
    leftright: [{ id: "left", label: "◀", anchor: "left" }, { id: "right", label: "▶", anchor: "left" }],
    dpad: [{ id: "up", label: "▲", anchor: "left" }, { id: "left", label: "◀", anchor: "left" }, { id: "right", label: "▶", anchor: "left" }, { id: "down", label: "▼", anchor: "left" }],
    jump: [{ id: "a", label: "⤒", anchor: "right" }],
    action: [{ id: "a", label: "●", anchor: "right" }]
  };
  function ensureLayer() {
    if (!layer) { layer = doc.createElement("div"); layer.className = "gk-layer"; doc.body.appendChild(layer); }
    return layer;
  }
  function press(id, down) {
    if (!!held[id] === down) return;
    held[id] = down;
    if (down) { count("button"); emit("buttondown", { id: id }); if (id === "a") emit("action", { source: "button" }); }
    else emit("buttonup", { id: id });
  }
  function makeButton(b) {
    var el = doc.createElement("div");
    el.className = "gk-btn"; el.textContent = b.label || b.id;
    var on = function (e) { e.preventDefault(); unlockAudio(); el.classList.add("gk-on"); press(b.id, true); };
    var off = function (e) { e.preventDefault(); el.classList.remove("gk-on"); press(b.id, false); };
    el.addEventListener("pointerdown", on); el.addEventListener("pointerup", off);
    el.addEventListener("pointercancel", off); el.addEventListener("pointerleave", off);
    ensureLayer().appendChild(el);
    return { def: b, el: el };
  }
  function layoutButtons() {
    if (!buttons.length) { GK.controlsHeight = 0; return; }
    var size = Math.round(Math.max(64, Math.min(view.width, view.height) * 0.17) * ui.scale);
    var gap = Math.round(size * 0.25), sa = GK.safeArea, bottom = sa.bottom + gap;
    var groups = { left: [], right: [], center: [] };
    buttons.forEach(function (b) { (groups[b.def.anchor] || groups.right).push(b); });
    function place(b, x, y) {
      b.el.style.width = b.el.style.height = size + "px";
      b.el.style.fontSize = Math.round(size * 0.42) + "px";
      b.el.style.left = Math.round(x) + "px"; b.el.style.top = Math.round(y) + "px";
    }
    var isDpad = groups.left.length === 4 && groups.left.some(function (b) { return b.def.id === "up"; });
    if (isDpad) {
      var cx = sa.left + gap + size, cy = view.height - bottom - size * 2;
      groups.left.forEach(function (b) {
        var o = { up: [0, -1], down: [0, 1], left: [-1, 0], right: [1, 0] }[b.def.id] || [0, 0];
        place(b, cx + o[0] * size, cy + o[1] * size);
      });
    } else {
      groups.left.forEach(function (b, i) { place(b, sa.left + gap + i * (size + gap), view.height - bottom - size); });
    }
    groups.right.forEach(function (b, i) { place(b, view.width - sa.right - gap - size - i * (size + gap), view.height - bottom - size); });
    var total = groups.center.length * size + (groups.center.length - 1) * gap;
    groups.center.forEach(function (b, i) { place(b, (view.width - total) / 2 + i * (size + gap), view.height - bottom - size); });
    GK.controlsHeight = (isDpad ? size * 3 : size) + bottom + gap;
  }

  /* ---------- клавиатура (десктоп) ---------- */
  var KEYMAP = { ArrowLeft: "left", ArrowRight: "right", ArrowUp: "up", ArrowDown: "down", a: "left", d: "right", w: "up", s: "down", A: "left", D: "right", W: "up", S: "down", " ": "a", Enter: "a", z: "b", x: "b", Z: "b", X: "b" };
  function onKey(e, down) {
    var id = KEYMAP[e.key];
    if (!id) return;
    if (e.key.indexOf("Arrow") === 0 || e.key === " ") e.preventDefault();
    unlockAudio();
    if (down && !e.repeat) {
      count("key");
      emit("key", { key: e.key, id: id });
      if (id === "left" || id === "right" || id === "up" || id === "down") emit("swipe", { dir: id, dx: 0, dy: 0, source: "key" });
    }
    press(id, down);
  }

  /* ---------- звук ---------- */
  function unlockAudio() {
    if (actx) { if (actx.state === "suspended") actx.resume(); return; }
    try { actx = new (win.AudioContext || win.webkitAudioContext)(); } catch (e) { actx = null; }
  }
  function tone(o) {
    if (muted || !actx) return;
    try {
      var t = actx.currentTime, osc = actx.createOscillator(), gain = actx.createGain();
      osc.type = o.type || "sine";
      osc.frequency.setValueAtTime(o.freq || 440, t);
      if (o.slide) osc.frequency.exponentialRampToValueAtTime(Math.max(30, o.slide), t + (o.dur || 0.15));
      gain.gain.setValueAtTime(o.vol == null ? 0.2 : o.vol, t);
      gain.gain.exponentialRampToValueAtTime(0.0001, t + (o.dur || 0.15));
      osc.connect(gain); gain.connect(actx.destination);
      osc.start(t + (o.delay || 0)); osc.stop(t + (o.delay || 0) + (o.dur || 0.15) + 0.02);
    } catch (e) {}
  }
  var SFX = {
    click: [{ freq: 600, dur: 0.05, type: "square", vol: 0.1 }],
    move: [{ freq: 300, dur: 0.04, type: "triangle", vol: 0.08 }],
    pop: [{ freq: 500, slide: 900, dur: 0.08, type: "sine" }],
    coin: [{ freq: 880, dur: 0.08, type: "square", vol: 0.12 }, { freq: 1320, dur: 0.12, type: "square", vol: 0.12, delay: 0.07 }],
    jump: [{ freq: 300, slide: 700, dur: 0.18, type: "square", vol: 0.12 }],
    powerup: [{ freq: 400, slide: 1200, dur: 0.35, type: "sawtooth", vol: 0.1 }],
    hit: [{ freq: 200, slide: 60, dur: 0.2, type: "sawtooth", vol: 0.18 }],
    error: [{ freq: 180, dur: 0.18, type: "square", vol: 0.12 }],
    win: [{ freq: 523, dur: 0.12 }, { freq: 659, dur: 0.12, delay: 0.12 }, { freq: 784, dur: 0.25, delay: 0.24 }],
    lose: [{ freq: 392, dur: 0.18 }, { freq: 330, dur: 0.18, delay: 0.18 }, { freq: 262, dur: 0.35, delay: 0.36 }]
  };

  /* ---------- статистика ввода ---------- */
  function wrapSDK() {
    var sdk = win.GameSDK;
    if (!sdk || sdk.__gk) return;
    var start = sdk.start, end = sdk.end;
    sdk.start = function () { roundStats = { tap: 0, swipe: 0, drag: 0, hold: 0, button: 0, key: 0, miss: 0 }; return start.apply(sdk, arguments); };
    sdk.end = function () {
      var r = end.apply(sdk, arguments);
      if (roundStats) { try { sdk.event("round_input", roundStats); } catch (e) {} roundStats = null; }
      return r;
    };
    sdk.__gk = true;
  }
  var statsSent = false;
  function sendStats() {
    if (statsSent || !win.GameSDK) return;
    statsSent = true;
    try { win.GameSDK.event("input_stats", stats); } catch (e) {}
  }

  var GK = {
    version: "1.0",
    pointer: { down: false, x: 0, y: 0 },
    safeArea: { top: 0, right: 0, bottom: 0, left: 0 },
    controlsHeight: 0,
    get width() { return view.width; },
    get height() { return view.height; },
    get dpr() { return view.dpr; },
    get muted() { return muted; },
    on: function (type, fn) { (handlers[type] = handlers[type] || []).push(fn); return GK; },
    off: function (type, fn) { var l = handlers[type]; if (l) handlers[type] = l.filter(function (f) { return f !== fn; }); return GK; },
    isDown: function (id) { return !!held[id]; },
    setup: function (opts) {
      opts = opts || {};
      if (!doc.body) throw new Error("GameKit.setup() must run after <body> exists (put the game script at the end of <body>)");
      if (opts.uiScale) ui.scale = opts.uiScale;
      canvasEl = opts.canvas || doc.getElementById("game") || doc.querySelector("canvas");
      if (!canvasEl) { canvasEl = doc.createElement("canvas"); canvasEl.id = "game"; doc.body.appendChild(canvasEl); }
      canvasEl.style.display = "block"; canvasEl.style.touchAction = "none";
      if (opts.background) doc.body.style.background = opts.background;
      var ctx = opts.webgl ? (canvasEl.getContext("webgl2") || canvasEl.getContext("webgl")) : canvasEl.getContext("2d");
      ctx2d = opts.webgl ? null : ctx;
      resize();
      return { canvas: canvasEl, ctx: ctx, width: view.width, height: view.height, dpr: view.dpr };
    },
    onResize: function (fn) { resizeFns.push(fn); return GK; },
    buttons: function (list, opts) {
      if (typeof list === "string") list = list.split("+").reduce(function (a, p) { return a.concat(PRESETS[p] || []); }, []);
      if (opts && opts.uiScale) ui.scale = opts.uiScale;
      buttons.forEach(function (b) { b.el.remove(); });
      buttons = (list || []).map(makeButton);
      layoutButtons();
      return GK;
    },
    showButtons: function (v) { if (layer) layer.style.display = v === false ? "none" : ""; return GK; },
    tone: tone,
    sfx: function (name) { (SFX[name] || SFX.click).forEach(tone); },
    mute: function (v) { muted = v == null ? !muted : !!v; if (GK._muteEl) GK._muteEl.textContent = muted ? "🔇" : "🔊"; return muted; },
    muteButton: function () {
      if (GK._muteEl) return GK;
      var b = doc.createElement("button");
      b.className = "gk-mute"; b.textContent = "🔊";
      b.style.top = (GK.safeArea.top + 8) + "px"; b.style.right = (GK.safeArea.right + 8) + "px";
      b.addEventListener("pointerdown", function (e) { e.preventDefault(); e.stopPropagation(); unlockAudio(); GK.mute(); });
      ensureLayer().appendChild(b); GK._muteEl = b;
      return GK;
    },
    vibrate: function (ms) { try { navigator.vibrate && navigator.vibrate(ms || 30); } catch (e) {} },
    stats: function () { return stats; }
  };
  win.GameKit = GK;
  wrapSDK();

  function init() {
    baseStyle(); ensureMeta(); wrapSDK();
    GK.safeArea = safeArea();
    win.addEventListener("pointerdown", onDown, { passive: true });
    win.addEventListener("pointermove", onMove, { passive: true });
    win.addEventListener("pointerup", onUp, { passive: true });
    win.addEventListener("pointercancel", onUp, { passive: true });
    win.addEventListener("keydown", function (e) { onKey(e, true); });
    win.addEventListener("keyup", function (e) { onKey(e, false); });
    doc.addEventListener("touchmove", function (e) { e.preventDefault(); }, { passive: false });
    doc.addEventListener("gesturestart", function (e) { e.preventDefault(); });
    doc.addEventListener("contextmenu", function (e) { e.preventDefault(); });
    doc.addEventListener("dblclick", function (e) { e.preventDefault(); });
    win.addEventListener("resize", resize);
    win.addEventListener("orientationchange", function () { setTimeout(resize, 150); });
    // capture-фаза: статистика уходит раньше, чем SDK закроет сессию
    win.addEventListener("visibilitychange", function () { if (doc.hidden) sendStats(); }, true);
    win.addEventListener("pagehide", sendStats, true);
    resize();
  }
  if (doc.body) init(); else doc.addEventListener("DOMContentLoaded", init);
})();
