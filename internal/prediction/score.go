package prediction

import (
	"github.com/cgastin/cash-five-lotto/internal/stats"
)

// Weights holds the configurable scoring weights.
type Weights struct {
	Frequency   float64 // w1: all-time frequency
	Recency     float64 // w2: 30-draw rolling frequency
	Overdue     float64 // w3: gap/overdue bonus
	Distribution float64 // w4: odd/even + low/mid/high balance
	SumAlignment float64 // w5: sum proximity to historical median
	Consecutive float64 // w6: consecutive number penalty
}

// DefaultWeights returns the baseline scoring weights from the architecture spec.
func DefaultWeights() Weights {
	return Weights{
		Frequency:    0.25,
		Recency:      0.30,
		Overdue:      0.15,
		Distribution: 0.20,
		SumAlignment: 0.05,
		Consecutive:  0.05,
	}
}

// ScoreCombo computes the composite score for a combination given statistical features.
// All component scores are normalized to [0, 1]. Higher is better (except consecutive penalty).
func ScoreCombo(combo [5]int, f stats.Features, w Weights) float64 {
	maxAllTime := stats.MaxFrequency(f.AllTimeFreq)
	maxRecent := stats.MaxFrequency(f.Rolling30Freq)

	// w1: all-time frequency score
	freqScore := 0.0
	if maxAllTime > 0 {
		sum := 0
		for _, n := range combo {
			sum += f.AllTimeFreq[n]
		}
		freqScore = float64(sum) / (float64(maxAllTime) * 5)
	}

	// w2: recency score (30-draw rolling)
	recentScore := 0.0
	if maxRecent > 0 {
		sum := 0
		for _, n := range combo {
			sum += f.Rolling30Freq[n]
		}
		recentScore = float64(sum) / (float64(maxRecent) * 5)
	}

	// w3: overdue/gap bonus — higher gap → higher bonus
	// Normalize: gap / drawCount → overdue proportion
	overdueScore := 0.0
	if f.DrawCount > 0 {
		sum := 0.0
		for _, n := range combo {
			gap := f.GapSinceLastSeen[n]
			// Invert: a number due soon (gap=0) gets 0; never seen gets 1
			sum += float64(gap) / float64(f.DrawCount)
		}
		overdueScore = sum / 5
	}

	// w4: distribution score
	distScore := stats.DistributionScore(combo)

	// w5: sum alignment
	sumScore := stats.SumAlignmentScore(combo, f.MedianSum, f.SumStdDev)

	// w6: consecutive penalty
	consecPenalty := stats.ConsecutivePenalty(combo)

	return w.Frequency*freqScore +
		w.Recency*recentScore +
		w.Overdue*overdueScore +
		w.Distribution*distScore +
		w.SumAlignment*sumScore -
		w.Consecutive*consecPenalty
}

// FeatureBreakdown returns the individual component scores for a combination.
func FeatureBreakdown(combo [5]int, f stats.Features, w Weights) map[string]float64 {
	maxAllTime := stats.MaxFrequency(f.AllTimeFreq)
	maxRecent := stats.MaxFrequency(f.Rolling30Freq)

	freqScore := 0.0
	if maxAllTime > 0 {
		sum := 0
		for _, n := range combo {
			sum += f.AllTimeFreq[n]
		}
		freqScore = float64(sum) / (float64(maxAllTime) * 5)
	}

	recentScore := 0.0
	if maxRecent > 0 {
		sum := 0
		for _, n := range combo {
			sum += f.Rolling30Freq[n]
		}
		recentScore = float64(sum) / (float64(maxRecent) * 5)
	}

	overdueScore := 0.0
	if f.DrawCount > 0 {
		sum := 0.0
		for _, n := range combo {
			gap := f.GapSinceLastSeen[n]
			sum += float64(gap) / float64(f.DrawCount)
		}
		overdueScore = sum / 5
	}

	return map[string]float64{
		"frequency_score":    freqScore,
		"recency_score":      recentScore,
		"overdue_score":      overdueScore,
		"distribution_score": stats.DistributionScore(combo),
		"sum_alignment":      stats.SumAlignmentScore(combo, f.MedianSum, f.SumStdDev),
		"consecutive_penalty": stats.ConsecutivePenalty(combo),
		"composite_score":    ScoreCombo(combo, f, w),
	}
}
