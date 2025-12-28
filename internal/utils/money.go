package utils

import "math"

func ConvertToCents(a float64) int64 {
	return int64(math.Round(a * 100))
}

func ConvertFromCents(a int64) float64 {
	return float64(a) / 100
}
