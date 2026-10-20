package agent

import (
	"fmt"
	"regexp"
	"strings"

	"beeline-games/internal/core/domain"
)

var (
	externalRe = regexp.MustCompile(`(?i)(?:\b(?:src|href)\s*=\s*["']\s*(?:https?:)?//|@import\s|url\(\s*["']?\s*(?:https?:)?//)`)
	networkRe  = regexp.MustCompile(`\bfetch\s*\(|\bXMLHttpRequest\b|\bWebSocket\s*\(|\bimport\s*\(|<script[^>]*\btype\s*=\s*["']module`)
	dialogRe   = regexp.MustCompile(`(?:^|[^.\w$])(?:window\.)?(?:alert|confirm|prompt)\s*\(`)
	ownInputRe = regexp.MustCompile(`addEventListener\(\s*["'](?:touchstart|touchend|pointerdown|pointerup|mousedown|click)["']`)
	sdkStartRe = regexp.MustCompile(`GameSDK\s*\.\s*start\s*\(`)
	sdkEndRe   = regexp.MustCompile(`GameSDK\s*\.\s*end\s*\(`)
	kitOnRe    = regexp.MustCompile(`GameKit\s*\.\s*on\s*\(`)
	canvasRe   = regexp.MustCompile(`(?i)<canvas|createElement\(\s*["']canvas|GameKit\s*\.\s*setup\s*\(`)
	gkDefRe    = regexp.MustCompile(`window\.GameKit\s*=|var\s+GameKit\s*=|const\s+GameKit\s*=|let\s+GameKit\s*=`)
)

const maxHTMLBytes = 2 << 20

// Validate — быстрые статические проверки кода игры (без запуска).
// html — код без встроенного GameKit.
func Validate(html string, spec *domain.GameSpec) []domain.Issue {
	var out []domain.Issue
	add := func(code string, sev domain.Severity, msg string) {
		out = append(out, domain.Issue{Code: code, Severity: sev, Message: msg})
	}

	if m := externalRe.FindString(html); m != "" {
		add("external_resource", domain.SevError, fmt.Sprintf("external resource reference %q: the game must be fully self-contained", strings.TrimSpace(m)))
	}
	if m := networkRe.FindString(html); m != "" {
		add("network_call", domain.SevError, fmt.Sprintf("network/module usage %q is not allowed", strings.TrimSpace(m)))
	}
	if dialogRe.MatchString(html) {
		add("blocking_dialog", domain.SevError, "alert/confirm/prompt block the WebView; draw messages on the canvas instead")
	}
	if !sdkStartRe.MatchString(html) {
		add("no_sdk_start", domain.SevError, "GameSDK.start() is never called (call it guarded at the beginning of every round)")
	}
	if !sdkEndRe.MatchString(html) {
		add("no_sdk_end", domain.SevError, "GameSDK.end(result, score) is never called (call it guarded exactly once per round)")
	}
	if gkDefRe.MatchString(html) {
		add("gamekit_redefined", domain.SevError, "the code defines its own GameKit; GameKit is injected by the platform, remove the copy")
	}
	if !kitOnRe.MatchString(html) {
		if ownInputRe.MatchString(html) {
			add("no_gamekit_input", domain.SevWarn, "input is handled with raw listeners; use GameKit.on(...) so touch works the same everywhere")
		} else {
			add("no_input", domain.SevError, "no input handling found; use GameKit.on('action'|'tap'|'swipe'|'drag'|...)")
		}
	}
	if !canvasRe.MatchString(html) {
		add("no_canvas", domain.SevWarn, "no <canvas> found: the game should render with HTML5 Canvas/WebGL via GameKit.setup()")
	}
	if len(html) > maxHTMLBytes {
		add("too_large", domain.SevWarn, fmt.Sprintf("file is %d KB; keep it under %d KB", len(html)>>10, maxHTMLBytes>>10))
	}
	if spec != nil {
		if spec.Controls.Scheme == "buttons" && !strings.Contains(html, "GameKit.buttons") {
			add("controls_mismatch", domain.SevWarn, "the plan requires on-screen buttons (GameKit.buttons) but the code does not create them")
		}
		if spec.Controls.Scheme == "swipe" && !strings.Contains(html, "'swipe'") && !strings.Contains(html, `"swipe"`) {
			add("controls_mismatch", domain.SevWarn, "the plan requires swipe controls but the code does not listen to GameKit 'swipe'")
		}
	}
	return out
}
