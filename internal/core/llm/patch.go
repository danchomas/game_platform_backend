package llm

import (
	"errors"
	"fmt"
	"strings"
)

// Правки исправителя: вместо всего файла (12–20 тыс. токенов) модель присылает блоки
// «найти → заменить», это в разы быстрее и не трогает остальной код.
const (
	searchMark  = "<<<<<<< SEARCH"
	dividerMark = "======="
	replaceMark = ">>>>>>> REPLACE"
)

// Edit — одна замена: Search должен ровно один раз встречаться в коде.
type Edit struct {
	Search, Replace string
}

var ErrNoEdits = errors.New("в ответе нет блоков правок")

// ParseEdits разбирает ответ исправителя: <reply> и блоки SEARCH/REPLACE внутри <patch>.
func ParseEdits(raw string) (reply string, edits []Edit, err error) {
	if v, ok := between(raw, "<reply>", "</reply>"); ok {
		reply = v
	}
	body := raw
	if v, ok := between(raw, "<patch>", "</patch>"); ok {
		body = v
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != searchMark {
			continue
		}
		var search, repl []string
		j := i + 1
		for ; j < len(lines) && strings.TrimSpace(lines[j]) != dividerMark; j++ {
			search = append(search, lines[j])
		}
		k := j + 1
		for ; k < len(lines) && strings.TrimSpace(lines[k]) != replaceMark; k++ {
			repl = append(repl, lines[k])
		}
		if j >= len(lines) || k >= len(lines) {
			return reply, nil, fmt.Errorf("правка %d не закрыта (%s / %s)", len(edits)+1, dividerMark, replaceMark)
		}
		edits = append(edits, Edit{Search: strings.Join(search, "\n"), Replace: strings.Join(repl, "\n")})
		i = k
	}
	if len(edits) == 0 {
		return reply, nil, ErrNoEdits
	}
	return reply, edits, nil
}

// ApplyEdits применяет правки по порядку. Сначала ищется точное совпадение, затем —
// построчное без учёта отступов (модель часто сбивает пробелы). Ненайденный или
// неоднозначный фрагмент — ошибка: лучше переписать файл целиком, чем испортить код.
func ApplyEdits(src string, edits []Edit) (string, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	for n, e := range edits {
		if strings.TrimSpace(e.Search) == "" {
			return "", fmt.Errorf("правка %d: пустой фрагмент для поиска", n+1)
		}
		switch strings.Count(src, e.Search) {
		case 1:
			src = strings.Replace(src, e.Search, e.Replace, 1)
			continue
		case 0:
		default:
			return "", fmt.Errorf("правка %d: фрагмент встречается в коде несколько раз", n+1)
		}
		out, err := replaceLines(src, e)
		if err != nil {
			return "", fmt.Errorf("правка %d: %w", n+1, err)
		}
		src = out
	}
	return src, nil
}

func replaceLines(src string, e Edit) (string, error) {
	lines := strings.Split(src, "\n")
	want := trimmed(strings.Split(e.Search, "\n"))
	for len(want) > 0 && want[0] == "" {
		want = want[1:]
	}
	for len(want) > 0 && want[len(want)-1] == "" {
		want = want[:len(want)-1]
	}
	if len(want) == 0 {
		return "", errors.New("пустой фрагмент для поиска")
	}
	have := trimmed(lines)
	at := -1
	for i := 0; i+len(want) <= len(have); i++ {
		if equalLines(have[i:i+len(want)], want) {
			if at >= 0 {
				return "", errors.New("фрагмент встречается в коде несколько раз")
			}
			at = i
		}
	}
	if at < 0 {
		return "", errors.New("фрагмент не найден в текущем коде")
	}
	out := append(append(append([]string{}, lines[:at]...), strings.Split(e.Replace, "\n")...), lines[at+len(want):]...)
	return strings.Join(out, "\n"), nil
}

func trimmed(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimSpace(l)
	}
	return out
}

func equalLines(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
