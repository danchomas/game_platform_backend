package htmlguard

import (
	"fmt"
	"regexp"
	"strings"

	"beeline-games/internal/core/apperr"
)

const (
	MinBytes = 256
	MaxBytes = 2 << 20
)

var externalRef = regexp.MustCompile(`(?i)(?:\bsrc\s*=|\bhref\s*=|\burl\s*\()\s*["'(]?\s*(?:https?:)?//`)

var markers = []string{"<html", "<body", "<canvas", "<script"}

func Validate(html string) error {
	switch {
	case len(html) < MinBytes:
		return fail("документ слишком мал (%d байт), похоже на обрезанный ответ", len(html))
	case len(html) > MaxBytes:
		return fail("документ слишком велик (%d байт)", len(html))
	}

	lower := strings.ToLower(html)

	if !hasMarker(lower) {
		return fail("в ответе нет разметки игры")
	}

	if loc := externalRef.FindStringIndex(lower); loc != nil {
		return fail("игра обращается к внешнему ресурсу: %q", excerpt(html, loc[0]))
	}

	return nil
}

func Sanitize(html string) string {
	html = strings.TrimSpace(html)

	if !strings.HasPrefix(html, "```") {
		return html
	}
	if i := strings.IndexByte(html, '\n'); i >= 0 {
		html = html[i+1:]
	}
	if i := strings.LastIndex(html, "```"); i >= 0 {
		html = html[:i]
	}
	return strings.TrimSpace(html)
}

func hasMarker(lower string) bool {
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func excerpt(s string, at int) string {
	end := min(at+60, len(s))
	return strings.TrimSpace(s[at:end])
}

func fail(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{apperr.ErrLLMFailed}, args...)...)
}
