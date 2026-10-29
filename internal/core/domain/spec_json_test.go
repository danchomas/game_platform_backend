package domain

import (
	"encoding/json"
	"testing"
)

func TestSpecToleratesLooseTypesFromTheModel(t *testing.T) {
	// так ответил DeepSeek: summary списком; плюс другие частые вольности моделей
	raw := `{"title":"Раннер","summary":["• Идея: бег","• Управление: тап"],
		"concept":["бежит","прыгает"],"mechanics":"герой бежит вправо","round_seconds":"120",
		"tuning":{"jump_height_h":"0.3","run_speed_w_per_s":0.5},
		"controls":{"scheme":"tap","gestures":"tap","description":["тап — прыжок"]},
		"assets":[{"name":"hero","kind":"sprite","description":["круглый","жёлтый"]}]}`
	var s GameSpec
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if s.Summary != "• Идея: бег\n• Управление: тап" || s.Concept != "бежит\nпрыгает" {
		t.Errorf("text fields: %q / %q", s.Summary, s.Concept)
	}
	if len(s.Mechanics) != 1 || s.Mechanics[0] != "герой бежит вправо" || len(s.Controls.Gestures) != 1 {
		t.Errorf("lists: %+v / %+v", s.Mechanics, s.Controls.Gestures)
	}
	if s.RoundSeconds != 120 || s.Tuning["jump_height_h"] != 0.3 || s.Tuning["run_speed_w_per_s"] != 0.5 {
		t.Errorf("numbers: %d %+v", s.RoundSeconds, s.Tuning)
	}
	if s.Controls.Description != "тап — прыжок" || s.Assets[0].Description != "круглый\nжёлтый" {
		t.Errorf("nested: %q / %q", s.Controls.Description, s.Assets[0].Description)
	}
}

func TestSpecStillRejectsGarbage(t *testing.T) {
	var s GameSpec
	if err := json.Unmarshal([]byte(`{"title":`), &s); err == nil {
		t.Fatal("broken JSON must stay an error")
	}
	if err := json.Unmarshal([]byte(`{"title":"x","round_seconds":"много"}`), &s); err == nil {
		t.Fatal("a value that cannot be a number must stay an error")
	}
}
