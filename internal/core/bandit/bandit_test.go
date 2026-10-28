package bandit

import (
	"math"
	"math/rand/v2"
	"testing"
)

func seeded() Rand { return rand.New(rand.NewPCG(1, 2)) }

func TestBetaMean(t *testing.T) {
	tests := []struct {
		name string
		a, b float64
	}{
		{name: "uniform prior", a: 1, b: 1},
		{name: "skewed to success", a: 30, b: 10},
		{name: "rare success", a: 2, b: 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := seeded()
			const n = 20000
			sum := 0.0
			for range n {
				x := Beta(tt.a, tt.b, r)
				if x < 0 || x > 1 {
					t.Fatalf("выборка %v вне [0, 1]", x)
				}
				sum += x
			}
			want := tt.a / (tt.a + tt.b)
			if got := sum / n; math.Abs(got-want) > 0.01 {
				t.Errorf("среднее Beta(%v, %v) = %.4f, ожидалось %.4f", tt.a, tt.b, got, want)
			}
		})
	}
}

func TestPickFavoursBetterArm(t *testing.T) {
	r := seeded()
	arms := []Arm{{Successes: 20, Trials: 100}, {Successes: 50, Trials: 100}}

	picks := [2]int{}
	for range 2000 {
		picks[Pick(arms, r)]++
	}
	if picks[1] < 1950 {
		t.Errorf("лучшая рука выбрана %d раз из 2000, ожидалось почти всегда", picks[1])
	}
}

func TestPickExploresWithoutData(t *testing.T) {
	r := seeded()
	arms := make([]Arm, 3)

	picks := [3]int{}
	for range 3000 {
		picks[Pick(arms, r)]++
	}
	for i, n := range picks {
		// без данных трафик делится поровну: по 1000 ± случайный разброс
		if n < 850 || n > 1150 {
			t.Errorf("рука %d получила %d из 3000: без данных деление должно быть равным (%v)", i, n, picks)
		}
	}
}

func TestPickEmpty(t *testing.T) {
	if got := Pick(nil, seeded()); got != -1 {
		t.Errorf("Pick(nil) = %d, ожидалось -1", got)
	}
}

func TestProbBest(t *testing.T) {
	tests := []struct {
		name    string
		arms    []Arm
		check   int
		atLeast float64
		atMost  float64
	}{
		{name: "clear winner", arms: []Arm{{10, 100}, {40, 100}}, check: 1, atLeast: 0.99, atMost: 1},
		{name: "equal arms", arms: []Arm{{30, 100}, {30, 100}}, check: 0, atLeast: 0.45, atMost: 0.55},
		{name: "small sample is uncertain", arms: []Arm{{1, 2}, {2, 3}}, check: 1, atLeast: 0.5, atMost: 0.8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probs := ProbBest(tt.arms, seeded(), 10000)
			sum := 0.0
			for _, p := range probs {
				sum += p
			}
			if math.Abs(sum-1) > 1e-9 {
				t.Errorf("сумма вероятностей %v, ожидалась 1", sum)
			}
			if p := probs[tt.check]; p < tt.atLeast || p > tt.atMost {
				t.Errorf("P(рука %d лучшая) = %.3f, ожидалось в [%.2f, %.2f]", tt.check, p, tt.atLeast, tt.atMost)
			}
		})
	}
}

func TestSampleToleratesInconsistentCounts(t *testing.T) {
	// успехов больше попыток бывает, если события пришли раньше показа в окно выборки
	r := seeded()
	for range 100 {
		if x := sample(Arm{Successes: 5, Trials: 3}, r); math.IsNaN(x) || x < 0 || x > 1 {
			t.Fatalf("выборка %v для несогласованных счётчиков", x)
		}
	}
}
