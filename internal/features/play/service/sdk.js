/* Beeline Game SDK: встраивается сервером в каждую игру при отдаче (/g/{id}) и экспорте. */
(function () {
  var CFG = { game: "__GAME_ID__", url: "__ENDPOINT__", on: __ENABLED__ };
  var q = [], errs = 0, engaged = false, endSent = false;

  function rid() {
    try {
      var a = new Uint8Array(8); crypto.getRandomValues(a);
      return Array.prototype.map.call(a, function (b) { return ("0" + b.toString(16)).slice(-2); }).join("");
    } catch (e) { return Math.random().toString(16).slice(2) + Date.now().toString(16); }
  }
  function playerId() {
    try { var u = new URLSearchParams(location.search).get("uid"); if (u) return String(u).slice(0, 64); } catch (e) {}
    try {
      var p = localStorage.getItem("gsdk_pid");
      if (!p) { p = rid(); localStorage.setItem("gsdk_pid", p); }
      return p;
    } catch (e) { return rid(); }
  }
  var sid = rid(), pid = playerId();

  var active = 0, last = Date.now(), vis = !document.hidden;
  function acc() { var n = Date.now(); if (vis) active += n - last; last = n; vis = !document.hidden; }

  function notifyHost(type, data) {
    try {
      var m = { source: "game-sdk", game_id: CFG.game, type: type, data: data || {} };
      if (window.parent && window.parent !== window) window.parent.postMessage(m, "*");
      if (window.ReactNativeWebView) window.ReactNativeWebView.postMessage(JSON.stringify(m));
    } catch (e) {}
  }

  function flush() {
    if (!CFG.on || !q.length) { q.length = 0; return; }
    var body = JSON.stringify({ game_id: CFG.game, session_id: sid, player_id: pid, events: q.splice(0, q.length) });
    try { if (navigator.sendBeacon && navigator.sendBeacon(CFG.url, new Blob([body], { type: "text/plain" }))) return; } catch (e) {}
    try { fetch(CFG.url, { method: "POST", body: body, keepalive: true, headers: { "Content-Type": "text/plain" } }); } catch (e) {}
  }
  function push(type, o) {
    o = o || {};
    q.push({ type: type, result: o.result, score: o.score, duration_ms: o.duration_ms, payload: o.payload });
    if (q.length >= 20) flush();
  }
  function endSession() {
    if (endSent) return;
    endSent = true; acc();
    push("session_end", { duration_ms: Math.round(active) });
    flush();
  }

  push("session_start");
  var t = setInterval(flush, 5000);
  ["pointerdown", "touchstart", "keydown", "mousedown"].forEach(function (ev) {
    window.addEventListener(ev, function () { if (!engaged) { engaged = true; push("engaged"); } }, { passive: true, capture: true });
  });
  document.addEventListener("visibilitychange", function () { acc(); if (document.hidden) endSession(); });
  window.addEventListener("pagehide", endSession);
  window.addEventListener("error", function (e) {
    if (errs++ < 3) push("js_error", { payload: { msg: String(e.message || "").slice(0, 200), line: e.lineno } });
  });

  window.GameSDK = {
    sessionId: sid,
    start: function () { push("game_start"); notifyHost("start"); },
    end: function (result, score) {
      result = result === "win" || result === "lose" ? result : "quit";
      var s = typeof score === "number" && isFinite(score) ? score : undefined;
      push("game_end", { result: result, score: s });
      notifyHost("end", { result: result, score: s });
    },
    event: function (name, data) {
      name = String(name || "").toLowerCase().replace(/[^a-z0-9_]/g, "_").slice(0, 40);
      if (name) { push(name, { payload: data }); notifyHost("event", { name: name, data: data }); }
    }
  };
})();
