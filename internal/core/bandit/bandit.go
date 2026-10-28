// Package bandit — Thompson sampling для бернуллиевских «рук»: у каждой руки есть
// число попыток и успехов, а апостериорное распределение доли успехов — Beta(1+s, 1+f).
//
// Выбор руки — одна выборка из каждого распределения и максимум по ним. Так рука
// получает трафик пропорционально вероятности оказаться лучшей: явный аутсайдер
// почти перестаёт показываться, а при равных данных трафик делится поровну.
package bandit

import (
	"math"
	"math/rand/v2"
)

// Arm — накопленный результат одной руки.
type Arm struct {
	Successes int64
	Trials    int64
}

// Rand — источник случайности. В проде — глобальный rand/v2 (потокобезопасный),
// в тестах — генератор с фиксированным зерном.
type Rand interface {
	Float64() float64
	NormFloat64() float64
}

type globalRand struct{}

func (globalRand) Float64() float64     { return rand.Float64() }
func (globalRand) NormFloat64() float64 { return rand.NormFloat64() }

// Global — потокобезопасный источник на основе пакета math/rand/v2.
var Global Rand = globalRand{}

// Pick выбирает руку по Thompson sampling; -1 — рук нет.
func Pick(arms []Arm, r Rand) int {
	best, bestTheta := -1, -1.0
	for i, a := range arms {
		if theta := sample(a, r); theta > bestTheta {
			best, bestTheta = i, theta
		}
	}
	return best
}

// ProbBest оценивает методом Монте-Карло вероятность каждой руки иметь наибольшую
// долю успехов. Это та же величина, по которой Pick распределяет трафик.
func ProbBest(arms []Arm, r Rand, draws int) []float64 {
	probs := make([]float64, len(arms))
	if len(arms) == 0 || draws <= 0 {
		return probs
	}
	for range draws {
		probs[Pick(arms, r)]++
	}
	for i := range probs {
		probs[i] /= float64(draws)
	}
	return probs
}

func sample(a Arm, r Rand) float64 {
	failures := max(a.Trials-a.Successes, 0)
	return Beta(float64(max(a.Successes, 0))+1, float64(failures)+1, r)
}

// Beta — выборка из Beta(a, b) через две гамма-величины; a, b ≥ 1.
func Beta(a, b float64, r Rand) float64 {
	x := gamma(a, r)
	y := gamma(b, r)
	return x / (x + y)
}

// gamma — выборка из Gamma(k, 1) методом Марсальи–Цанга; k ≥ 1, что гарантирует
// равномерный априор Beta(1, 1).
func gamma(k float64, r Rand) float64 {
	d := k - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := r.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := r.Float64()
		if u < 1-0.0331*x*x*x*x || math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}
