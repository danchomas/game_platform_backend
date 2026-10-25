package domain

import "strings"

// GameSpec — план игры, который агент составляет до написания кода
// и который пользователь может поправить перед подтверждением.
// json-теги здесь — формат обмена с моделью и хранения в JSONB, а не транспорт.
type GameSpec struct {
	Title         string     `json:"title"`
	Genre         string     `json:"genre"`
	Audience      string     `json:"audience"`
	Language      string     `json:"language"`
	Concept       string     `json:"concept"`
	Goal          string     `json:"goal"`
	CoreLoop      []string   `json:"core_loop"`
	WinCondition  string     `json:"win_condition"`
	LoseCondition string     `json:"lose_condition"`
	Controls      Controls   `json:"controls"`
	Difficulty    Difficulty `json:"difficulty"`
	RoundSeconds  int        `json:"round_seconds"`
	Levels        int        `json:"levels"`
	UI            UISpec     `json:"ui"`
	Visual        Visual     `json:"visual"`
	Assets        []Asset    `json:"assets"`
	Sounds        []string   `json:"sounds"`
	Events        []string   `json:"events"`
	Summary       string     `json:"summary"`
}

type Controls struct {
	// tap | swipe | drag | hold | buttons | mixed
	Scheme      string   `json:"scheme"`
	Gestures    []string `json:"gestures"`
	Buttons     []string `json:"buttons"`
	Description string   `json:"description"`
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
}

func clipRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
