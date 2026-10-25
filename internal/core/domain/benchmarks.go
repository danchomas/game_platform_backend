package domain

// Benchmark — ориентир для метрики: с чем сравнивать игры на дашборде
// и на что целиться планировщику. Source = "assumption" — наше допущение,
// его нужно перепроверить на реальных данных платформы.
type Benchmark struct {
	Metric string
	Unit   string
	Median float64
	Good   float64
	Source string
	Note   string
}

var Benchmarks = []Benchmark{
	{Metric: "avg_session_sec", Unit: "sec", Median: 300, Good: 510,
		Source: "GameAnalytics Mobile Benchmarks 2024–2025",
		Note:   "медиана 4.5–6 мин, топ-25% — 8–9 мин (мобильные игры в целом)"},
	{Metric: "d1_retention", Unit: "ratio", Median: 0.25, Good: 0.40,
		Source: "GameAnalytics 2019; отраслевой ориентир 40% D1",
		Note:   "доля игроков, вернувшихся на следующий день"},
	{Metric: "d30_retention", Unit: "ratio", Median: 0.03, Good: 0.06,
		Source: "Appodeal Mobile Casual Benchmarks 2025", Note: "hyper-casual"},
	{Metric: "round_sec", Unit: "sec", Median: 90, Good: 180,
		Source: "assumption", Note: "один раунд мини-игры в WebView: 1–3 мин, для пожилых — до 5 мин"},
	{Metric: "completion_rate", Unit: "ratio", Median: 0.6, Good: 0.8,
		Source: "assumption", Note: "game_end / game_start: доля раундов, доведённых до конца"},
	{Metric: "win_rate", Unit: "ratio", Median: 0.5, Good: 0.55,
		Source: "assumption", Note: "здоровый диапазон 0.4–0.7: ниже — слишком сложно, выше — скучно"},
	{Metric: "engagement_rate", Unit: "ratio", Median: 0.7, Good: 0.9,
		Source: "assumption", Note: "доля сессий с хотя бы одним касанием"},
}
