package agent

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Progress сообщает наружу, сколько кода уже написано в текущем вызове модели
// и до какой метки «// @step …» дошла модель. Вызывается не чаще progressEvery.
type Progress func(role string, chars, expectedChars int, step string)

const (
	progressEvery = time.Second
	// defaultExpectedChars — размер игры, пока нет истории сборок
	defaultExpectedChars = 30000
	maxStepRunes         = 60
)

// watcher создаёт следилку за потоком на каждый вызов модели (повтор начинает счёт заново).
type watcher func() func(delta string)

func (p Progress) watch(role string, expected int) watcher {
	if p == nil {
		return nil
	}
	return func() func(string) {
		t := &stepTracker{}
		p(role, 0, expected, "")
		var last time.Time
		return func(d string) {
			if t.feed(d) || time.Since(last) >= progressEvery {
				last = time.Now()
				p(role, t.chars, expected, t.step)
			}
		}
	}
}

// stepTracker считает байты ответа и ловит метки в законченных строках:
// метка может прийти по кускам, поэтому хвост без перевода строки ждёт продолжения.
type stepTracker struct {
	chars int
	step  string
	tail  string
}

// feed возвращает true, если появилась новая метка.
func (t *stepTracker) feed(d string) bool {
	t.chars += len(d)
	t.tail += d
	changed := false
	for {
		i := strings.IndexByte(t.tail, '\n')
		if i < 0 {
			break
		}
		if s := stepOf(t.tail[:i]); s != "" && s != t.step {
			t.step, changed = s, true
		}
		t.tail = t.tail[i+1:]
	}
	if len(t.tail) > 512 {
		t.tail = t.tail[len(t.tail)-512:]
	}
	return changed
}

// stepOf достаёт текст метки из строки кода: «// @step Рисуем поле», «/* @step Стили */».
func stepOf(line string) string {
	_, rest, ok := strings.Cut(line, "@step")
	if !ok {
		return ""
	}
	rest = strings.TrimLeft(rest, ": \t")
	for _, end := range []string{"*/", "-->"} {
		rest, _, _ = strings.Cut(rest, end)
	}
	rest = strings.TrimSpace(rest)
	if utf8.RuneCountInString(rest) > maxStepRunes {
		rest = string([]rune(rest)[:maxStepRunes]) + "…"
	}
	return rest
}
