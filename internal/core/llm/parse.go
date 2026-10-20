package llm

import (
	"encoding/json"
	"errors"
	"strings"
)

type Result struct {
	Reply, HTML, Title, Genre, Audience string
}

var ErrBadFormat = errors.New("модель вернула ответ не в ожидаемом формате")

func between(s, open, close string) (string, bool) {
	i := strings.Index(s, open)
	if i < 0 {
		return "", false
	}
	i += len(open)
	j := strings.LastIndex(s, close)
	if j < i {
		return strings.TrimSpace(s[i:]), false
	}
	return strings.TrimSpace(s[i:j]), true
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl >= 0 {
			s = s[nl+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

// Parse разбирает ответ кодера: <reply>, <meta>, <game>.
func Parse(raw string) (Result, error) {
	var r Result
	if v, ok := between(raw, "<reply>", "</reply>"); ok {
		r.Reply = v
	}
	if v, ok := between(raw, "<meta>", "</meta>"); ok {
		var m struct{ Title, Genre, Audience string }
		if json.Unmarshal([]byte(stripFence(v)), &m) == nil {
			r.Title, r.Genre, r.Audience = clip(m.Title, 80), clip(m.Genre, 40), clip(m.Audience, 60)
		}
	}
	game, closed := between(raw, "<game>", "</game>")
	if game == "" {
		if i := strings.Index(raw, "```html"); i >= 0 {
			game = stripFence(raw[i:])
			closed = true
		} else if i := strings.Index(strings.ToLower(raw), "<!doctype html"); i >= 0 {
			game, closed = raw[i:], true
		}
	}
	game = stripFence(game)
	low := strings.ToLower(game)
	if len(game) < 200 || !strings.Contains(low, "<html") || !closed || !strings.Contains(low, "</html>") {
		return r, ErrBadFormat
	}
	r.HTML = game
	if r.Reply == "" {
		r.Reply = "Готово! Игра обновлена."
	}
	return r, nil
}

// ParseJSON достаёт первый JSON-объект из ответа (с ```json-обёрткой или без) в dst.
func ParseJSON(raw string, dst any) error {
	s := stripFence(raw)
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return ErrBadFormat
	}
	if err := json.Unmarshal([]byte(s[i:j+1]), dst); err != nil {
		return errors.Join(ErrBadFormat, err)
	}
	return nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
