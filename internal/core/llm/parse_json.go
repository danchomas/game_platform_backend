package llm

import (
	"encoding/json"
	"errors"
	"strings"
)

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
