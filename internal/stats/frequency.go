// Package stats computes statistical features over historical Cash Five draws.
package stats

import (
	"sort"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

// NumberFrequency maps each number (1-35) to its appearance count.
type NumberFrequency map[int]int

// Features holds all computed statistical features for a draw history.
type Features struct {
	// AllTimeFreq is appearance count per number across all draws.
	AllTimeFreq NumberFrequency

	// Rolling30Freq is appearance count per number in the last 30 draws.
	Rolling30Freq NumberFrequency

	// Rolling60Freq is appearance count per number in the last 60 draws.
	Rolling60Freq NumberFrequency

	// Rolling90Freq is appearance count per number in the last 90 draws.
	Rolling90Freq NumberFrequency

	// Rolling180Freq is appearance count per number in the last 180 draws.
	Rolling180Freq NumberFrequency

	// GapSinceLastSeen is draws elapsed since each number last appeared.
	// If a number has never appeared, gap = total draw count.
	GapSinceLastSeen map[int]int

	// MedianSum is the median sum of 5 numbers across all draws.
	MedianSum float64

	// SumStdDev is the standard deviation of sums.
	SumStdDev float64

	// DrawCount is the total number of draws in the history.
	DrawCount int

	// LatestDrawDate is the date of the most recent draw in history.
	LatestDrawDate time.Time
}

// ComputeFeatures computes all statistical features from a sorted draw history.
// draws must be sorted ascending by date (oldest first).
func ComputeFeatures(draws []store.Draw) Features {
	f := Features{
		AllTimeFreq:      make(NumberFrequency),
		Rolling30Freq:    make(NumberFrequency),
		Rolling60Freq:    make(NumberFrequency),
		Rolling90Freq:    make(NumberFrequency),
		Rolling180Freq:   make(NumberFrequency),
		GapSinceLastSeen: make(map[int]int),
		DrawCount:        len(draws),
	}

	if len(draws) == 0 {
		return f
	}

	f.LatestDrawDate = draws[len(draws)-1].DrawDate

	// All-time frequency
	for _, d := range draws {
		for _, n := range d.Numbers {
			f.AllTimeFreq[n]++
		}
	}

	// Rolling window frequencies (last N draws)
	computeRolling(draws, 30, f.Rolling30Freq)
	computeRolling(draws, 60, f.Rolling60Freq)
	computeRolling(draws, 90, f.Rolling90Freq)
	computeRolling(draws, 180, f.Rolling180Freq)

	// Gap analysis: scan from most recent draw backwards
	lastSeen := make(map[int]int) // number → index of last draw (ascending)
	for i, d := range draws {
		for _, n := range d.Numbers {
			lastSeen[n] = i
		}
	}
	lastIdx := len(draws) - 1
	for n := 1; n <= 35; n++ {
		if idx, ok := lastSeen[n]; ok {
			f.GapSinceLastSeen[n] = lastIdx - idx
		} else {
			f.GapSinceLastSeen[n] = len(draws) // never seen
		}
	}

	// Sum statistics
	sums := make([]float64, len(draws))
	for i, d := range draws {
		s := 0
		for _, n := range d.Numbers {
			s += n
		}
		sums[i] = float64(s)
	}
	f.MedianSum = median(sums)
	f.SumStdDev = stddev(sums, f.MedianSum)

	return f
}

// computeRolling fills freq with counts from the last n draws in draws.
func computeRolling(draws []store.Draw, n int, freq NumberFrequency) {
	start := len(draws) - n
	if start < 0 {
		start = 0
	}
	for _, d := range draws[start:] {
		for _, num := range d.Numbers {
			freq[num]++
		}
	}
}

// median returns the median of a float64 slice (modifies a copy).
func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := make([]float64, len(vals))
	copy(cp, vals)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

// stddev computes population standard deviation.
func stddev(vals []float64, mean float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		d := v - mean
		sum += d * d
	}
	variance := sum / float64(len(vals))
	return sqrt(variance)
}

// sqrt is a simple integer square root using Newton's method.
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 100; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

// MaxFrequency returns the maximum value in a NumberFrequency map.
func MaxFrequency(freq NumberFrequency) int {
	max := 0
	for _, v := range freq {
		if v > max {
			max = v
		}
	}
	return max
}
