package browser

import (
	"context"
	"os"
	"testing"
	"time"

	"beeline-games/internal/core/agent"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/gamekit"
)

type agentReport = agent.TestReport

// Интеграционный тест: нужен запущенный chromedp/headless-shell.
//
//	BROWSER_WS_URL=ws://localhost:9222 go test ./internal/core/agent/browser/
const workingGame = `<!DOCTYPE html><html><head><title>t</title></head><body>
<script>
const {ctx} = GameKit.setup({background:'#123'});
let state = 'start', x = 100, dir = 1, t = 0;
GameKit.on('action', () => { if (state === 'start') { state = 'play'; window.GameSDK && GameSDK.start(); } });
GameKit.on('swipe', e => { dir = (e.dir === 'left') ? -1 : 1; });
function loop(ts) {
  t++;
  ctx.fillStyle = '#123'; ctx.fillRect(0, 0, GameKit.width, GameKit.height);
  ctx.fillStyle = '#fc0';
  if (state === 'start') { ctx.font = '40px sans-serif'; ctx.fillText('Тапни, чтобы начать', 20, 200); }
  else { x += dir * 3; if (x > GameKit.width) x = 0; if (x < 0) x = GameKit.width; ctx.fillRect(x, 300 + Math.sin(t / 10) * 20, 40, 40); }
  requestAnimationFrame(loop);
}
requestAnimationFrame(loop);
</script></body></html>`

const brokenGame = `<!DOCTYPE html><html><head><title>t</title><link rel="stylesheet" href="https://cdn.example.com/x.css"></head><body>
<script>
const {ctx} = GameKit.setup({});
GameKit.on('action', () => { undefinedFunction(); });
</script></body></html>`

func TestBrowserIntegration(t *testing.T) {
	ws := os.Getenv("BROWSER_WS_URL")
	if ws == "" {
		t.Skip("BROWSER_WS_URL not set")
	}
	tester := New(ws, 40*time.Second, 1)
	ctx := context.Background()

	rep, err := tester.Test(ctx, gamekit.Embed(workingGame))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("working: %+v", withoutShot(rep))
	if issues := rep.Issues(); domain.HasErrors(issues) {
		t.Fatalf("working game flagged: %+v", issues)
	}
	if len(rep.Screenshot) == 0 {
		t.Fatal("no screenshot")
	}

	rep, err = tester.Test(ctx, gamekit.Embed(brokenGame))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("broken: %+v", withoutShot(rep))
	codes := map[string]bool{}
	for _, i := range rep.Issues() {
		codes[i.Code] = true
	}
	for _, c := range []string{"runtime_error", "network_request", "not_started"} {
		if !codes[c] {
			t.Errorf("broken game: expected %s, got %v", c, codes)
		}
	}
}

func withoutShot(r agentReport) agentReport {
	r.Screenshot = nil
	return r
}
