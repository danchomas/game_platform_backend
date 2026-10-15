package llm

import (
	"strings"
	"testing"
)

var doc = "<!DOCTYPE html><html><head></head><body>" + strings.Repeat("<p>x</p>", 40) + "</body></html>"

func TestParseOK(t *testing.T) {
	raw := "<reply>Готово</reply><meta>{\"title\":\"Змейка\",\"genre\":\"arcade\",\"audience\":\"бабушки\"}</meta><game>\n" + doc + "\n</game>"
	r, err := Parse(raw)
	if err != nil || r.Title != "Змейка" || r.Genre != "arcade" || r.Reply != "Готово" || r.HTML != doc {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestParseStripsFence(t *testing.T) {
	r, err := Parse("<reply>ok</reply><game>\n```html\n" + doc + "\n```\n</game>")
	if err != nil || strings.Contains(r.HTML, "```") {
		t.Fatalf("fence not stripped: %v", err)
	}
}

func TestParseFallbackFence(t *testing.T) {
	if _, err := Parse("Вот игра:\n```html\n" + doc + "\n```"); err != nil {
		t.Fatal(err)
	}
}

func TestParseRejectsTruncatedAndGarbage(t *testing.T) {
	if _, err := Parse("<reply>x</reply><game>\n" + strings.TrimSuffix(doc, "</body></html>")); err == nil {
		t.Fatal("обрезанный html должен быть отклонён")
	}
	if _, err := Parse("просто текст"); err == nil {
		t.Fatal("ответ без игры должен быть отклонён")
	}
}
