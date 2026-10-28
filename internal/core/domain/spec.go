package domain

import (
	"math"
	"strings"
)

// GameSpec — план игры, который агент составляет до написания кода
// и который пользователь может поправить перед подтверждением.
// json-теги здесь — формат обмена с моделью и хранения в JSONB, а не транспорт.
type GameSpec struct {
	Title         string   `json:"title"`
	Genre         string   `json:"genre"`
	Audience      string   `json:"audience"`
	Language      string   `json:"language"`
	Concept       string   `json:"concept"`
	Goal          string   `json:"goal"`
	CoreLoop      []string `json:"core_loop"`
	WinCondition  string   `json:"win_condition"`
	LoseCondition string   `json:"lose_condition"`
	Controls      Controls `json:"controls"`
	// Camera, Mechanics, Tuning — замысел, который кодер реализует буквально, а ревьюер сверяет с кодом:
	// без них модель сама додумывает вид и физику, и игра выходит противоречивой.
	Camera    Camera   `json:"camera"`
	Mechanics []string `json:"mechanics"`
	// Tuning — ключевые числа механики в долях экрана и секундах; в коде — const TUNING.
	Tuning map[string]float64 `json:"tuning"`
	// Decisions — какой из вариантов запроса выбран и почему.
	Decisions    []string   `json:"decisions"`
	Difficulty   Difficulty `json:"difficulty"`
	RoundSeconds int        `json:"round_seconds"`
	Levels       int        `json:"levels"`
	UI           UISpec     `json:"ui"`
	Visual       Visual     `json:"visual"`
	Assets       []Asset    `json:"assets"`
	Sounds       []string   `json:"sounds"`
	Events       []string   `json:"events"`
	Summary      string     `json:"summary"`
}

type Controls struct {
	// tap | swipe | drag | hold | mixed (несколько жестов) | buttons (экранные кнопки, только по просьбе пользователя)
	Scheme   string   `json:"scheme"`
	Gestures []string `json:"gestures"`
	// Buttons — только при Scheme "buttons": кнопки закрывают поле и играются хуже жестов.
	Buttons     []string `json:"buttons"`
	Description string   `json:"description"`
}

type Camera struct {
	// side | top_down | behind | board | static
	View        string `json:"view"`
	Description string `json:"description"`
}

type Difficulty struct {
	Start string `json:"start"`
	Ramp  string `json:"ramp"`
}

type UISpec struct {
	// normal | large | xlarge
	Scale      string `json:"scale"`
	MinTouchPx int    `json:"min_touch_px"`
	FontPx     int    `json:"font_px"`
	Contrast   string `json:"contrast"`
}

type Visual struct {
	Style      string   `json:"style"`
	Palette    []string `json:"palette"`
	Background string   `json:"background"`
}

type Asset struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

var (
	controlSchemes = map[string]bool{"tap": true, "swipe": true, "drag": true, "hold": true, "buttons": true, "mixed": true}
	uiScales       = map[string]int{"normal": 64, "large": 80, "xlarge": 96}
)

// Normalize приводит план к допустимым значениям, чтобы кодеру не пришлось угадывать.
func (s *GameSpec) Normalize() {
	s.Title = clipRunes(strings.TrimSpace(s.Title), 80)
	s.Genre = clipRunes(strings.ToLower(strings.TrimSpace(s.Genre)), 40)
	s.Audience = clipRunes(strings.TrimSpace(s.Audience), 60)
	if s.Language == "" {
		s.Language = "ru"
	}
	s.Controls.Scheme = strings.ToLower(strings.TrimSpace(s.Controls.Scheme))
	if !controlSchemes[s.Controls.Scheme] {
		s.Controls.Scheme = "tap"
	}
	if s.Controls.Scheme != "buttons" {
		s.Controls.Buttons = nil
	}
	s.UI.Scale = strings.ToLower(strings.TrimSpace(s.UI.Scale))
	minPx, ok := uiScales[s.UI.Scale]
	if !ok {
		s.UI.Scale, minPx = "normal", uiScales["normal"]
	}
	if s.UI.MinTouchPx < minPx {
		s.UI.MinTouchPx = minPx
	}
	if s.UI.FontPx < 18 {
		s.UI.FontPx = 18
	}
	if s.RoundSeconds <= 0 || s.RoundSeconds > 900 {
		s.RoundSeconds = 120
	}
	if s.Levels < 1 {
		s.Levels = 1
	}
	if len(s.Assets) > 12 {
		s.Assets = s.Assets[:12]
	}
	s.Camera.View = strings.ToLower(strings.TrimSpace(s.Camera.View))
	s.Mechanics = clipList(s.Mechanics, 10, 300)
	s.Decisions = clipList(s.Decisions, 5, 300)
	for k, v := range s.Tuning {
		if math.IsNaN(v) || math.IsInf(v, 0) || len(s.Tuning) > maxTuning {
			delete(s.Tuning, k)
		}
	}
}

const maxTuning = 16

func clipList(items []string, n, runes int) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it = strings.TrimSpace(it); it != "" && len(out) < n {
			out = append(out, clipRunes(it, runes))
		}
	}
	return out
}

func clipRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
