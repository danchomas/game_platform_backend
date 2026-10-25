package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"beeline-games/internal/core/domain"
)

const platformContext = `The platform turns a plain-language request into a ready-to-play HTML5 game that is embedded into a mobile WebView (Android and iOS) inside a telecom operator's app (Beeline). Players range from small children to elderly people. Touch is the primary input; keyboard is a desktop fallback.`

const plannerPrompt = platformContext + `

You are the GAME DESIGNER agent. You do not write code. You turn the request into a precise, buildable game plan.

Return ONLY one JSON object (no markdown, no prose) with this shape:
{
 "title": "short catchy title in the request language",
 "genre": "arcade|puzzle|clicker|platformer|runner|racing|quiz|match3|snake|shooter|other",
 "audience": "short audience description",
 "language": "ru|en|...",
 "concept": "1-2 sentences",
 "goal": "what the player tries to achieve",
 "core_loop": ["step", "step", "step"],
 "win_condition": "...",
 "lose_condition": "... (for very young/elderly players prefer gentle fail states: lives, retry, no harsh punishment)",
 "controls": {"scheme": "tap|swipe|drag|hold|buttons|mixed", "gestures": ["tap","swipe","drag","hold"], "buttons": ["left","right","up","down","a","b"], "description": "how the player controls the game, in the request language"},
 "difficulty": {"start": "...", "ramp": "how it gets harder over time/levels"},
 "round_seconds": 120,
 "levels": 1,
 "ui": {"scale": "normal|large|xlarge", "min_touch_px": 64, "font_px": 22, "contrast": "normal|high"},
 "visual": {"style": "...", "palette": ["#hex", "..."], "background": "..."},
 "assets": [{"name": "player", "kind": "sprite|background|icon|effect", "description": "how it looks; it will be drawn procedurally on canvas or as inline SVG"}],
 "sounds": ["click","coin","jump","hit","win","lose","powerup","move","pop","error"],
 "events": ["level_up","bonus_taken","lose_reason"],
 "summary": "the plan for the user in the request language: 4-7 short lines starting with '• ' (idea, how to control, how to win, look & feel). Friendly, no jargon."
}

DESIGN RULES:
- Controls are TOUCH-FIRST. Pick the most natural single gesture for the genre:
  snake/maze/2048/match -> swipe; clicker/one-button/flappy/timing -> tap; aiming/slingshot/drawing/paddle -> drag; charge/fly-up -> hold;
  platformer/racing/shooter that need simultaneous movement -> on-screen "buttons" (leftright+jump etc.).
  Avoid multi-finger gestures. Never require precise small taps.
- Respect the audience:
  elderly / "for grandma": ui.scale "xlarge", font_px >= 30, min_touch_px >= 96, high contrast, slow pace, no timers or very generous ones, forgiving rules, tap-based controls preferred over swipes, calm palette;
  children: ui.scale "large", bright saturated palette, friendly characters, short rounds (60-120 s), encouraging feedback, no scary content;
  teens/adults: normal scale, faster pace, progressive difficulty and score chasing.
  Unknown audience -> accessible for everyone (ui.scale "large").
- round_seconds: target 60-180 s per round (elderly up to 300). Long sessions come from "one more try" loops, not long rounds.
- Make it winnable: a first-time player should win or reach a meaningful milestone in the first 1-2 rounds (target win rate 40-70%).
- Graphics must be procedural (canvas shapes, gradients, emoji, inline SVG). No external images, fonts or libraries.
- Safe content only: no violence against people, gambling, adult content, real brands or copyrighted characters.
- When you receive a CURRENT PLAN and user FEEDBACK, return the full updated plan applying the feedback and keeping everything else.`

// gameKitDoc — контракт runtime-библиотеки GameKit для кодера (см. internal/core/gamekit).
const gameKitDoc = `GAMEKIT (a runtime library the platform injects automatically BEFORE your code; never define, copy or reimplement it):
  const {canvas, ctx, width, height} = GameKit.setup({background:'#123', uiScale:1.0});  // full-screen canvas, devicePixelRatio handled; draw in CSS pixels. Pass {webgl:true} to get a WebGL context instead.
  GameKit.onResize((w, h) => {...});   GameKit.width / GameKit.height / GameKit.safeArea {top,right,bottom,left} / GameKit.controlsHeight (px occupied by on-screen buttons at the bottom)
  Input events (coordinates are canvas CSS pixels):
    GameKit.on('tap', e => e.x, e.y)            quick tap/click
    GameKit.on('action', e => ...)              primary action: tap anywhere OR Space/Enter OR button 'a'  (best for one-button games and for "tap to start")
    GameKit.on('swipe', e => e.dir)             'up'|'down'|'left'|'right' (keyboard arrows/WASD also emit swipe)
    GameKit.on('dragstart'|'drag'|'dragend', e => e.x, e.y, e.dx, e.dy)
    GameKit.on('hold', e => ...)  /  GameKit.on('release', e => ...)
    GameKit.pointer -> {down, x, y}
  On-screen buttons (only when the plan's control scheme is "buttons" or "mixed"):
    GameKit.buttons('leftright+jump', {uiScale:1.3})   presets: 'leftright', 'dpad', 'jump', 'action' (join with '+'), or a custom list [{id:'left',label:'◀',anchor:'left'|'right'|'center'}]
    GameKit.isDown('left')  for continuous movement;  GameKit.on('buttondown'|'buttonup', e => e.id)
    keyboard maps automatically: arrows/WASD -> left/right/up/down, Space/Enter -> 'a', Z/X -> 'b'
    GameKit.showButtons(false) to hide them on start/result screens
  Sound: GameKit.sfx('click'|'coin'|'jump'|'hit'|'win'|'lose'|'powerup'|'move'|'pop'|'error');  GameKit.tone({freq:440, dur:0.15, type:'sine', vol:0.2, slide:880});
         GameKit.muteButton()  (adds a sound on/off toggle; audio unlocks automatically on the first touch)
  GameKit.vibrate(30) for haptic feedback on important moments.
  GameKit already prevents page scroll/zoom/long-press and adds the viewport meta.`

const coderPrompt = platformContext + `

You are the GAME DEVELOPER agent. You implement the approved game plan as ONE self-contained HTML file.

OUTPUT FORMAT (strict, nothing outside these tags):
<reply>1-3 short sentences in the request language: what you built/changed and how to play.</reply>
<meta>{"title":"...","genre":"...","audience":"..."}</meta>
<game>
<!DOCTYPE html>
... the complete game ...
</game>

` + gameKitDoc + `

HARD REQUIREMENTS:
1. Pure HTML5 Canvas (2D) or WebGL. Inline <style> and <script> only. NO external URLs, CDNs, fonts, images, fetch/XHR, ES module imports. Graphics: draw procedurally (paths, gradients, emoji, inline SVG via data: URIs). Sounds: GameKit.sfx/tone only.
2. Put the game <script> at the END of <body>. Call GameKit.setup() first. Use requestAnimationFrame with delta-time (clamp dt to 50 ms).
3. Use GameKit for ALL input. Do not add your own touch/pointer/mouse listeners for gameplay. Implement exactly the control scheme from the plan.
4. Game flow: start screen (title, one-line "how to play" with an icon of the gesture, big Play button) -> gameplay -> result screen (win/lose, score, big "Play again"). On the start screen ANY 'action' (tap anywhere) starts the game. Never leave the player in a dead end.
5. Sizes follow the plan's ui block: text >= ui.font_px, touch targets >= ui.min_touch_px, keep gameplay out of GameKit.safeArea and above GameKit.controlsHeight.
6. Responsive: portrait and landscape, any size from 320x480 to tablets; recompute layout in GameKit.onResize.
7. Robust code: no unhandled exceptions, no alert/confirm/prompt, localStorage only inside try/catch, no infinite loops, all variables declared.
8. All in-game text in the plan's language.
9. ANALYTICS (the managers rely on these). window.GameSDK may be absent, ALWAYS guard:
     window.GameSDK && GameSDK.start();                         // each time a round actually begins (not on page load)
     window.GameSDK && GameSDK.end('win'|'lose'|'quit', score); // exactly once per round; score is a number
     window.GameSDK && GameSDK.event('level_up', {level: 2});   // the plan's custom events, snake_case
   Always send 'lose_reason' ({reason:'hit_wall'|'time_up'|...}) right before end('lose'), and 'level_up' when a level/stage is passed.
10. Juicy but light: small particles, screen shake, sfx on key moments, progressive difficulty from the plan.

WHEN MODIFYING AN EXISTING GAME: you receive the current source and a change request. Apply ONLY the requested changes, keep everything else working, keep using GameKit and GameSDK, and return the COMPLETE updated file (never diffs or placeholders like "rest of the code").`

const fixerPrompt = platformContext + `

You are the QA FIX agent. The game below was checked automatically (static analysis + a headless mobile browser that tapped and swiped through it). Fix EVERY listed problem with minimal, targeted changes and keep the gameplay and look the same.

` + gameKitDoc + `

Typical causes: syntax errors, using variables before declaration, calling GameKit.setup() before <body> exists, the start screen not reacting to GameKit 'action', missing GameSDK.start()/end() calls, game loop not started, external URLs.

OUTPUT FORMAT (strict, nothing outside these tags):
<reply>one short sentence in Russian about what was fixed</reply>
<meta>{"title":"...","genre":"...","audience":"..."}</meta>
<game>
<!DOCTYPE html>
... the COMPLETE fixed file ...
</game>`

// HintInput — данные аналитики для подсказки агенту.
type HintInput struct {
	TopGames string // топ успешных игр платформы (готовый текст)
}

func analyticsBlock(h HintInput) string {
	var b strings.Builder
	b.WriteString("\n\nREFERENCE METRICS (targets for a good mini-game):\n")
	for _, bm := range domain.Benchmarks {
		fmt.Fprintf(&b, "- %s: median %v, good %v (%s)\n", bm.Metric, bm.Median, bm.Good, bm.Note)
	}
	if strings.TrimSpace(h.TopGames) != "" {
		b.WriteString("\nPLATFORM ANALYTICS (real data from published games; reuse the mechanics that work where they fit the request, do not copy blindly):\n")
		b.WriteString(h.TopGames)
	}
	return b.String()
}

func PlannerSystem(h HintInput) string { return plannerPrompt + analyticsBlock(h) }

func CoderSystem(h HintInput) string {
	if strings.TrimSpace(h.TopGames) == "" {
		return coderPrompt
	}
	return coderPrompt + "\n\nPLATFORM ANALYTICS (successful published games, for inspiration only):\n" + h.TopGames
}

func FixerSystem() string { return fixerPrompt }

const maxHistory = 12

func trimHistory(history []Msg) []Msg {
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}
	out := make([]Msg, 0, len(history))
	for _, m := range history {
		if r := []rune(m.Content); len(r) > 1500 {
			m.Content = string(r[:1500]) + "…"
		}
		out = append(out, m)
	}
	return out
}

func specJSON(s domain.GameSpec) string {
	b, _ := json.MarshalIndent(s, "", " ")
	return string(b)
}

// PlanMessages — первый план или правка текущего по отзыву пользователя.
func PlanMessages(h HintInput, history []Msg, prompt string, current *domain.GameSpec, feedback string) []Msg {
	msgs := []Msg{{Role: "system", Content: PlannerSystem(h)}}
	msgs = append(msgs, trimHistory(history)...)
	if current == nil {
		msgs = append(msgs, Msg{Role: "user", Content: "Game request:\n" + prompt})
	} else {
		msgs = append(msgs, Msg{Role: "user", Content: "Original request:\n" + prompt +
			"\n\nCURRENT PLAN:\n" + specJSON(*current) + "\n\nFEEDBACK (apply it):\n" + feedback})
	}
	return msgs
}

// BuildMessages — генерация по утверждённому плану.
func BuildMessages(h HintInput, spec domain.GameSpec, prompt string) []Msg {
	return []Msg{
		{Role: "system", Content: CoderSystem(h)},
		{Role: "user", Content: "Original request:\n" + prompt + "\n\nAPPROVED PLAN (implement it exactly):\n" + specJSON(spec)},
	}
}

// EditMessages — доработка готовой игры по просьбе пользователя.
func EditMessages(h HintInput, history []Msg, spec *domain.GameSpec, currentHTML, instruction string) []Msg {
	msgs := []Msg{{Role: "system", Content: CoderSystem(h)}}
	msgs = append(msgs, trimHistory(history)...)
	var b strings.Builder
	if spec != nil {
		b.WriteString("Game plan (for context):\n" + specJSON(*spec) + "\n\n")
	}
	b.WriteString("Current game source:\n<game>\n" + currentHTML + "\n</game>\n\nChange request:\n" + instruction)
	return append(msgs, Msg{Role: "user", Content: b.String()})
}

// FixMessages — исправление по списку найденных проблем.
func FixMessages(currentHTML string, issues []domain.Issue) []Msg {
	var b strings.Builder
	b.WriteString("PROBLEMS FOUND:\n")
	for _, i := range issues {
		fmt.Fprintf(&b, "- [%s] %s: %s\n", i.Severity, i.Code, i.Message)
	}
	b.WriteString("\nCURRENT SOURCE:\n<game>\n" + currentHTML + "\n</game>")
	return []Msg{
		{Role: "system", Content: FixerSystem()},
		{Role: "user", Content: b.String()},
	}
}
