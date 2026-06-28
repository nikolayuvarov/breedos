package eval

import (
	"math"
	"strconv"
)

// math helpers — kept in one place so the rest of the package can stay
// math-free at the import line.

func mathPow(x, p float64) float64 {
	return math.Pow(x, p)
}

func itoaFloat(v float64, prec int) string {
	return strconv.FormatFloat(v, 'f', prec, 64)
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func variance(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := mean(xs)
	sum := 0.0
	for _, x := range xs {
		d := x - m
		sum += d * d
	}
	return sum / float64(len(xs))
}

func robustness(xs []float64) float64 {
	v := variance(xs)
	// Clamp to [0, 1]: with scores in [0, 1] variance is bounded by 0.25.
	r := 1 - 4*v
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	return r
}
