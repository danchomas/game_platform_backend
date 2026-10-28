package agent

import (
	"context"
	"fmt"
	"strings"

	"beeline-games/internal/core/domain"
)

// Tester запускает игру в настоящем браузере и проверяет, что она живая.
// Реализация — internal/core/agent/browser (headless Chrome через CDP).
type Tester interface {
	Test(ctx context.Context, html string) (TestReport, error)
}

type TestReport struct {
	Errors          []string `json:"errors"`
	BlockedRequests []string `json:"blocked_requests"`
	StartCalled     bool     `json:"start_called"`
	EndCalls        int      `json:"end_calls"`
	CustomEvents    []string `json:"custom_events"`
	FramesIdle      int      `json:"frames_idle"`
	FramesPlay      int      `json:"frames_play"`
	HasCanvas       bool     `json:"has_canvas"`
	CanvasBlank     bool     `json:"canvas_blank"`
	ChangedOnStart  bool     `json:"changed_on_start"`
	ChangedOnInput  bool     `json:"changed_on_input"`
	StartMethod     string   `json:"start_method"`
	// ScreensCompared — все снимки сделаны, ChangedOnInput можно верить
	ScreensCompared bool `json:"screens_compared"`
	// Incomplete — тест не дошёл до конца (обычно таймаут): причина; данные после неё неизвестны
	Incomplete string `json:"incomplete,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Screenshot []byte `json:"-"`
}

// Issues переводит отчёт в список проблем для агента-исправителя.
func (r TestReport) Issues() []domain.Issue {
	var out []domain.Issue
	add := func(code string, sev domain.Severity, msg string) {
		out = append(out, domain.Issue{Code: code, Severity: sev, Message: msg})
	}
	if len(r.Errors) > 0 {
		errs := r.Errors
		if len(errs) > 8 {
			errs = errs[:8]
		}
		add("runtime_error", domain.SevError, "JavaScript errors while running in a mobile browser:\n    "+strings.Join(errs, "\n    "))
	}
	if len(r.BlockedRequests) > 0 {
		add("network_request", domain.SevError, "the game tried to load external resources: "+strings.Join(r.BlockedRequests, ", "))
	}
	// Незаконченный тест — проблема стенда, а не игры: выводы о старте, кадрах и экране
	// по нему ложные, а исправитель потратил бы круг на то, что не сломано.
	if r.Incomplete != "" {
		add("test_incomplete", domain.SevWarn, "the browser test did not finish ("+r.Incomplete+"); start, game loop and screen checks were skipped")
		return out
	}
	if !r.StartCalled {
		add("not_started", domain.SevError, fmt.Sprintf("after tapping the start screen (%s) GameSDK.start() was not called: the start screen must start the round on GameKit 'action' and call GameSDK.start()", r.StartMethod))
	}
	if r.HasCanvas && r.CanvasBlank {
		add("blank_canvas", domain.SevError, "the canvas stays empty (single color) during gameplay: nothing is drawn")
	}
	if r.StartCalled && r.FramesPlay == 0 {
		add("no_game_loop", domain.SevError, "no requestAnimationFrame calls during gameplay: the game loop is not running")
	}
	if r.StartCalled && r.ScreensCompared && !r.ChangedOnInput {
		add("frozen_screen", domain.SevWarn, "the screen did not change while tapping/swiping during gameplay")
	}
	return out
}
