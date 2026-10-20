// Package gamekit встраивает runtime-библиотеку GameKit в HTML игры.
// GameKit хранится внутри сохранённого HTML (игра остаётся одним файлом),
// а перед отправкой кода в LLM вырезается, чтобы не тратить токены.
package gamekit

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed gamekit.js
var js string

const openTag = `<script data-gamekit="1">`

var (
	headRe  = regexp.MustCompile(`(?i)<head(\s[^>]*)?>`)
	stripRe = regexp.MustCompile(`(?s)<script data-gamekit="[^"]*">.*?</script>`)
)

func Source() string { return js }

// Embed вставляет GameKit сразу после <head> (заменяя уже встроенную копию).
func Embed(html string) string {
	html = Strip(html)
	tag := openTag + js + "</script>"
	if loc := headRe.FindStringIndex(html); loc != nil {
		return html[:loc[1]] + tag + html[loc[1]:]
	}
	if i := strings.Index(strings.ToLower(html), "<body"); i >= 0 {
		return html[:i] + "<head>" + tag + "</head>" + html[i:]
	}
	return tag + html
}

// Strip убирает встроенный GameKit.
func Strip(html string) string {
	return stripRe.ReplaceAllString(html, "")
}
