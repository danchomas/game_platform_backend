package domain

import (
	"math"
	"testing"
)

func TestWilson(t *testing.T) {
	tests := []struct {
		name              string
		successes, trials int64
		low, high         float64
	}{
		{name: "no trials", successes: 0, trials: 0, low: 0, high: 1},
		{name: "half of hundred", successes: 50, trials: 100, low: 0.4038, high: 0.5962},
		{name: "all of one", successes: 1, trials: 1, low: 0.2065, high: 1},
		{name: "none of ten", successes: 0, trials: 10, low: 0, high: 0.2775},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			low, high := Wilson(tt.successes, tt.trials)
			if math.Abs(low-tt.low) > 1e-3 || math.Abs(high-tt.high) > 1e-3 {
				t.Errorf("Wilson(%d, %d) = [%.4f, %.4f], ожидалось [%.4f, %.4f]",
					tt.successes, tt.trials, low, high, tt.low, tt.high)
			}
		})
	}
}

func TestVariantEligible(t *testing.T) {
	tests := []struct {
		name     string
		v        Variant
		eligible bool
		reason   string
	}{
		{name: "clean build", v: Variant{Version: 1, Status: StatusReady, BuildStatus: GenStatusReady}, eligible: true},
		{name: "still building", v: Variant{Status: StatusTesting}, reason: "ещё собирается"},
		{name: "failed first build", v: Variant{Status: StatusFailed}, reason: "сборка не удалась"},
		{
			name:   "unfixed issues",
			v:      Variant{Version: 1, Status: StatusReady, BuildStatus: GenStatusWithIssues},
			reason: "автопроверка нашла ошибки, которые не удалось исправить",
		},
		{name: "edit in progress", v: Variant{Version: 1, Status: StatusGenerating, BuildStatus: GenStatusReady}, reason: "ещё собирается"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.Eligible(); got != tt.eligible {
				t.Errorf("Eligible() = %v, ожидалось %v", got, tt.eligible)
			}
			if got := tt.v.RejectReason(); got != tt.reason {
				t.Errorf("RejectReason() = %q, ожидалось %q", got, tt.reason)
			}
		})
	}
}

func TestVariantStatsDerive(t *testing.T) {
	st := VariantStats{Exposures: 200, Sessions: 180, Engaged: 90, Retained: 60, ErrorSessions: 18, Starts: 50, Ends: 40, Wins: 10}
	st.Derive()

	checks := []struct {
		name      string
		got, want float64
	}{
		{"load", st.LoadRate, 0.9},
		{"engagement", st.EngagementRate, 0.5},
		{"retention per exposure", st.RetentionRate, 0.3},
		{"errors", st.ErrorRate, 0.1},
		{"completion", st.CompletionRate, 0.8},
		{"win", st.WinRate, 0.25},
	}
	for _, c := range checks {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("%s = %v, ожидалось %v", c.name, c.got, c.want)
		}
	}
	if !(st.RetentionLow < st.RetentionRate && st.RetentionRate < st.RetentionHigh) {
		t.Errorf("интервал [%v, %v] не содержит оценку %v", st.RetentionLow, st.RetentionHigh, st.RetentionRate)
	}
}
