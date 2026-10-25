package htmlguard

import (
	"errors"
	"strings"
	"testing"

	"beeline-games/internal/core/apperr"
)

func game(body string) string {
	return "<html><head></head><body><canvas id=\"c\"></canvas><script>" +
		body + strings.Repeat("// отступ до минимального размера\n", 10) +
		"</script></body></html>"
}

func TestValidateAcceptsSelfContainedGame(t *testing.T) {
	if err := Validate(game("const x = 1;")); err != nil {
		t.Fatalf("самодостаточная игра отклонена: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name string
		html string
	}{
		{"пусто", ""},
		{"обрезанный ответ", "<html><body>"},
		{"текст вместо игры", strings.Repeat("Вот ваша игра про змейку. ", 40)},
		{"слишком большой", game(strings.Repeat("x", MaxBytes))},
		{"внешний скрипт", game("") + `<script src="https://cdn.example.com/phaser.js"></script>`},
		{"внешний скрипт без схемы", game("") + `<script src="//cdn.example.com/p.js"></script>`},
		{"внешний стиль", game("") + `<link href='http://fonts.example.com/f.css'>`},
		{"шрифт из css", game("") + `<style>@font-face{src:url(https://f.example.com/a.woff)}</style>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.html)
			if err == nil {
				t.Fatal("ожидалась ошибка, получено nil")
			}
			if !errors.Is(err, apperr.ErrLLMFailed) {
				t.Fatalf("ошибка не разворачивается в ErrLLMFailed: %v", err)
			}
		})
	}
}

func TestSanitizeStripsCodeFence(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"обёртка html", "```html\n<html></html>\n```", "<html></html>"},
		{"обёртка без языка", "```\n<html></html>\n```", "<html></html>"},
		{"без обёртки", "  <html></html>  ", "<html></html>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Sanitize(tt.in); got != tt.want {
				t.Fatalf("Sanitize() = %q, ожидалось %q", got, tt.want)
			}
		})
	}
}

func TestValidateAllowsInlineDataURI(t *testing.T) {
	html := game("") + `<img src="data:image/png;base64,iVBORw0KGgo=">`
	if err := Validate(html); err != nil {
		t.Fatalf("инлайновый ресурс отклонён: %v", err)
	}
}
