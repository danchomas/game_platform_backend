package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// UnmarshalJSON читает план строго, а если модель вольно обошлась с типами — терпимо:
// список вместо строки склеивается построчно, строка вместо списка становится списком,
// число строкой ("120", "0.5") — числом. Иначе одна такая вольность роняла весь план.
func (s *GameSpec) UnmarshalJSON(b []byte) error {
	type plain GameSpec
	err := json.Unmarshal(b, (*plain)(s))
	var typeErr *json.UnmarshalTypeError
	if err == nil || !errors.As(err, &typeErr) {
		return err
	}
	var raw any
	if json.Unmarshal(b, &raw) != nil {
		return err
	}
	fixed, merr := json.Marshal(coerce(raw, reflect.TypeOf(plain{})))
	if merr != nil {
		return err
	}
	*s = GameSpec{}
	return json.Unmarshal(fixed, (*plain)(s))
}

// coerce подгоняет значение из JSON под тип поля Go; чего не умеет — оставляет как есть.
func coerce(v any, t reflect.Type) any {
	switch t.Kind() {
	case reflect.String:
		return asText(v)
	case reflect.Int, reflect.Int64, reflect.Float64:
		if str, ok := v.(string); ok {
			if n, err := strconv.ParseFloat(strings.TrimSpace(str), 64); err == nil {
				return n
			}
		}
		return v
	case reflect.Slice:
		items, ok := v.([]any)
		if !ok {
			if v == nil {
				return v
			}
			items = []any{v}
		}
		out := make([]any, len(items))
		for i, it := range items {
			out[i] = coerce(it, t.Elem())
		}
		return out
	case reflect.Map:
		m, ok := v.(map[string]any)
		if !ok {
			return v
		}
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[k] = coerce(val, t.Elem())
		}
		return out
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return v
		}
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[k] = val
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if val, ok := out[name]; ok && name != "" && name != "-" {
				out[name] = coerce(val, f.Type)
			}
		}
		return out
	default:
		return v
	}
}

func asText(v any) any {
	switch x := v.(type) {
	case []any:
		parts := make([]string, 0, len(x))
		for _, it := range x {
			if s, ok := asText(it).(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	case float64, bool:
		return fmt.Sprint(x)
	case map[string]any:
		b, _ := json.Marshal(x)
		return string(b)
	default:
		return v
	}
}
