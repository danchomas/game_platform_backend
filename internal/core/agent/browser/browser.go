// Package browser — тест игры в настоящем headless Chrome (chromedp/headless-shell по CDP).
// Игра открывается как мобильная страница с touch-эмуляцией, сеть заблокирована:
// агент проверяет ошибки JS, старт раунда по тапу, живой игровой цикл и реакцию на ввод.
package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"beeline-games/internal/core/agent"
)

const (
	gameURL = "https://game.test/index.html"
	vw, vh  = 390, 844
)

type Tester struct {
	wsURL   string
	timeout time.Duration
	sem     chan struct{}
}

// New: wsURL — адрес DevTools, например ws://browser:9222.
func New(wsURL string, timeout time.Duration, maxParallel int) *Tester {
	if maxParallel <= 0 {
		maxParallel = 2
	}
	if timeout <= 0 {
		timeout = 40 * time.Second
	}
	return &Tester{wsURL: wsURL, timeout: timeout, sem: make(chan struct{}, maxParallel)}
}

// Chrome принимает DevTools-запросы только с Host = IP или localhost,
// поэтому имя сервиса из docker-compose резолвим в IP.
func resolveWS(ctx context.Context, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host, port := u.Hostname(), u.Port()
	if net.ParseIP(host) == nil && host != "localhost" {
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addrs) == 0 {
			return "", fmt.Errorf("resolve %s: %w", host, err)
		}
		host = addrs[0].IP.String()
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	u.Host = host
	return u.String(), nil
}

func (t *Tester) Test(ctx context.Context, html string) (agent.TestReport, error) {
	var rep agent.TestReport
	select {
	case t.sem <- struct{}{}:
		defer func() { <-t.sem }()
	case <-ctx.Done():
		return rep, ctx.Err()
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	ws, err := resolveWS(ctx, t.wsURL)
	if err != nil {
		return rep, err
	}
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, ws)
	defer cancelAlloc()
	tab, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	var mu sync.Mutex
	body := base64.StdEncoding.EncodeToString([]byte(html))
	chromedp.ListenTarget(tab, func(ev any) {
		e, ok := ev.(*fetch.EventRequestPaused)
		if !ok {
			return
		}
		go func() {
			c := chromedp.FromContext(tab)
			exec := cdp.WithExecutor(tab, c.Target)
			if e.Request.URL == gameURL {
				_ = fetch.FulfillRequest(e.RequestID, 200).
					WithResponseHeaders([]*fetch.HeaderEntry{{Name: "Content-Type", Value: "text/html; charset=utf-8"}}).
					WithBody(body).Do(exec)
				return
			}
			if !strings.HasPrefix(e.Request.URL, "data:") && !strings.HasPrefix(e.Request.URL, "blob:") {
				mu.Lock()
				if len(rep.BlockedRequests) < 10 {
					rep.BlockedRequests = append(rep.BlockedRequests, clip(e.Request.URL, 120))
				}
				mu.Unlock()
			}
			_ = fetch.FailRequest(e.RequestID, network.ErrorReasonBlockedByClient).Do(exec)
		}()
	})

	err = chromedp.Run(tab,
		emulation.SetDeviceMetricsOverride(vw, vh, 3, true),
		emulation.SetTouchEmulationEnabled(true).WithMaxTouchPoints(5),
		fetch.Enable().WithPatterns([]*fetch.RequestPattern{{URLPattern: "*"}}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(harness).Do(ctx)
			return err
		}),
		chromedp.Navigate(gameURL),
		chromedp.Sleep(1200*time.Millisecond),
	)
	if err != nil {
		return rep, fmt.Errorf("загрузка игры: %w", err)
	}

	var st qaState
	shot0, _ := screenshot(tab)
	_ = chromedp.Run(tab, chromedp.Evaluate(stateJS, &st))
	rep.FramesIdle = st.Raf
	rafIdle := st.Raf

	// старт: DOM-кнопка «Играть», иначе тап в центр, иначе тап ниже центра
	var btn []float64
	_ = chromedp.Run(tab, chromedp.Evaluate(findStartJS, &btn))
	starts := []struct {
		name string
		x, y float64
	}{{"tap_center", vw / 2, vh / 2}, {"tap_lower", vw / 2, vh * 0.68}}
	if len(btn) == 2 {
		starts = append([]struct {
			name string
			x, y float64
		}{{"dom_button", btn[0], btn[1]}}, starts...)
	}
	for _, s := range starts {
		rep.StartMethod = s.name
		_ = chromedp.Run(tab, tap(s.x, s.y), chromedp.Sleep(900*time.Millisecond), chromedp.Evaluate(stateJS, &st))
		if st.Start > 0 {
			break
		}
	}
	rep.StartCalled = st.Start > 0
	shot1, _ := screenshot(tab)
	rep.ChangedOnStart = !bytes.Equal(shot0, shot1)

	// геймплей: жесты и клавиши
	rafStart := st.Raf
	_ = chromedp.Run(tab,
		swipe(vw/2, vh/2, 120, 0), chromedp.Sleep(250*time.Millisecond),
		tap(vw*0.3, vh*0.4), chromedp.Sleep(250*time.Millisecond),
		swipe(vw/2, vh/2, 0, -120), chromedp.Sleep(250*time.Millisecond),
	)
	shot2, _ := screenshot(tab)
	_ = chromedp.Run(tab,
		swipe(vw/2, vh/2, -120, 0), chromedp.Sleep(250*time.Millisecond),
		tap(vw*0.7, vh*0.6), chromedp.Sleep(250*time.Millisecond),
		hold(vw/2, vh*0.55, 500*time.Millisecond), chromedp.Sleep(200*time.Millisecond),
		swipe(vw/2, vh/2, 0, 120), chromedp.Sleep(250*time.Millisecond),
		key("ArrowLeft", 37), key("ArrowRight", 39), key(" ", 32),
		chromedp.Sleep(400*time.Millisecond),
	)
	shot3, _ := screenshot(tab)
	rep.ChangedOnInput = !bytes.Equal(shot1, shot2) || !bytes.Equal(shot2, shot3)

	var cv canvasInfo
	_ = chromedp.Run(tab, chromedp.Evaluate(stateJS, &st), chromedp.Evaluate(canvasJS, &cv))
	rep.FramesPlay = st.Raf - rafStart
	if !rep.StartCalled {
		rep.FramesPlay = st.Raf - rafIdle
	}
	rep.HasCanvas = cv.Has
	rep.CanvasBlank = cv.Has && cv.Is2D && cv.Colors <= 1
	rep.Errors = formatErrors(st.Errors, html)
	rep.EndCalls = st.Ends
	rep.CustomEvents = st.Events
	rep.Screenshot = shot3
	rep.DurationMs = time.Since(start).Milliseconds()
	return rep, nil
}

type qaState struct {
	Raf    int       `json:"raf"`
	Start  int       `json:"start"`
	Ends   int       `json:"ends"`
	Events []string  `json:"events"`
	Errors []qaError `json:"errors"`
}

type qaError struct {
	Msg  string `json:"m"`
	Line int    `json:"l"`
	Col  int    `json:"c"`
}

// formatErrors прикладывает к ошибке фрагмент кода: номера строк считаются
// по HTML со встроенным GameKit, а исправитель видит код без него.
func formatErrors(errs []qaError, html string) []string {
	lines := strings.Split(html, "\n")
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		s := e.Msg
		if e.Line > 0 && e.Line <= len(lines) {
			src := lines[e.Line-1]
			from := e.Col - 80
			if from < 0 {
				from = 0
			}
			to := e.Col + 80
			if to > len(src) || e.Col == 0 {
				to = len(src)
			}
			if from < to {
				if frag := strings.TrimSpace(src[from:to]); frag != "" {
					s += " — at code: " + clip(frag, 160)
				}
			}
		}
		out = append(out, s)
	}
	return out
}

type canvasInfo struct {
	Has    bool `json:"has"`
	Is2D   bool `json:"is2d"`
	Colors int  `json:"colors"`
}

func screenshot(ctx context.Context) ([]byte, error) {
	var buf []byte
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		b, err := page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatJpeg).WithQuality(55).Do(ctx)
		buf = b
		return err
	}))
	return buf, err
}

func touch(typ input.TouchType, pts ...*input.TouchPoint) chromedp.Action {
	if pts == nil {
		pts = []*input.TouchPoint{}
	}
	return input.DispatchTouchEvent(typ, pts)
}

func tap(x, y float64) chromedp.Action {
	return chromedp.Tasks{
		touch(input.TouchStart, &input.TouchPoint{X: x, Y: y}),
		chromedp.Sleep(60 * time.Millisecond),
		touch(input.TouchEnd),
	}
}

func hold(x, y float64, d time.Duration) chromedp.Action {
	return chromedp.Tasks{
		touch(input.TouchStart, &input.TouchPoint{X: x, Y: y}),
		chromedp.Sleep(d),
		touch(input.TouchEnd),
	}
}

func swipe(x, y, dx, dy float64) chromedp.Action {
	tasks := chromedp.Tasks{touch(input.TouchStart, &input.TouchPoint{X: x, Y: y})}
	const steps = 6
	for i := 1; i <= steps; i++ {
		f := float64(i) / steps
		tasks = append(tasks, chromedp.Sleep(16*time.Millisecond),
			touch(input.TouchMove, &input.TouchPoint{X: x + dx*f, Y: y + dy*f}))
	}
	return append(tasks, touch(input.TouchEnd))
}

func key(k string, code int64) chromedp.Action {
	name := k
	if k == " " {
		name = "Space"
	}
	return chromedp.Tasks{
		input.DispatchKeyEvent(input.KeyDown).WithKey(k).WithCode(name).WithWindowsVirtualKeyCode(code),
		chromedp.Sleep(80 * time.Millisecond),
		input.DispatchKeyEvent(input.KeyUp).WithKey(k).WithCode(name).WithWindowsVirtualKeyCode(code),
	}
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// harness выполняется до кода игры: подменяет GameSDK, считает кадры, ловит ошибки.
const harness = `(function(){
  var t = window.__qa = {errors:[], start:0, ends:0, events:[], raf:0}, seen = {};
  function err(m, l, c){
    m = String(m).slice(0,300); var k = m + '@' + (l||0);
    if (seen[k] || t.errors.length >= 20) return;
    seen[k] = 1; t.errors.push({m:m, l:l||0, c:c||0});
  }
  window.addEventListener('error', function(e){
    if (e.target && e.target !== window) { err('resource failed: ' + (e.target.src || e.target.href || e.target.tagName)); return; }
    var l = e.lineno, c = e.colno, m = e.error && e.error.stack && /index\.html:(\d+):(\d+)/.exec(e.error.stack);
    if (m) { l = +m[1]; c = +m[2]; }  // исходное место, даже если ошибку перебросил GameKit
    err(e.message || 'error', l, c);
  }, true);
  window.addEventListener('unhandledrejection', function(e){ err('unhandled promise rejection: ' + (e.reason && (e.reason.message || e.reason))); });
  var ce = console.error;
  console.error = function(){ try { err('console.error: ' + Array.prototype.map.call(arguments, String).join(' ')); } catch(_){} return ce.apply(console, arguments); };
  var raf = window.requestAnimationFrame;
  window.requestAnimationFrame = function(cb){ t.raf++; return raf.call(window, cb); };
  var gc = HTMLCanvasElement.prototype.getContext;
  HTMLCanvasElement.prototype.getContext = function(type){ this.__qaType = type; return gc.apply(this, arguments); };
  ['alert','confirm','prompt'].forEach(function(f){ window[f] = function(){ err('blocking dialog ' + f + '() called'); return f === 'confirm' ? true : null; }; });
  window.GameSDK = { sessionId:'qa',
    start: function(){ t.start++; },
    end: function(){ t.ends++; },
    event: function(n){ if (t.events.length < 50) t.events.push(String(n)); } };
})();`

const stateJS = `(function(){ var t = window.__qa || {}; return {raf:t.raf|0, start:t.start|0, ends:t.ends|0, events:t.events||[], errors:t.errors||[]}; })()`

const findStartJS = `(function(){
  var re = /^\s*(▶\s*)?(play|start|играть|начать|старт|поехали|go)(\s|!|$)/i;
  var els = document.querySelectorAll('button,[role=button],a,div,span');
  for (var i = 0; i < els.length; i++) {
    var e = els[i], txt = (e.innerText || '').trim();
    if (txt.length > 0 && txt.length < 40 && re.test(txt)) {
      var r = e.getBoundingClientRect();
      if (r.width > 0 && r.height > 0) return [r.left + r.width/2, r.top + r.height/2];
    }
  }
  return [];
})()`

const canvasJS = `(function(){
  var cs = document.querySelectorAll('canvas'), best = null, area = 0;
  for (var i = 0; i < cs.length; i++) { var a = cs[i].width * cs[i].height; if (a > area) { area = a; best = cs[i]; } }
  if (!best) return {has:false, is2d:false, colors:0};
  var is2d = best.__qaType === '2d';
  if (!is2d) return {has:true, is2d:false, colors:0};
  try {
    var o = document.createElement('canvas'); o.width = 24; o.height = 24;
    var c = o.getContext('2d'); c.drawImage(best, 0, 0, 24, 24);
    var d = c.getImageData(0, 0, 24, 24).data, seen = {}, n = 0;
    for (var j = 0; j < d.length; j += 4) { var k = (d[j]>>4)+'-'+(d[j+1]>>4)+'-'+(d[j+2]>>4); if (!seen[k]) { seen[k] = 1; n++; } }
    return {has:true, is2d:true, colors:n};
  } catch (e) { return {has:true, is2d:true, colors:2}; }
})()`
