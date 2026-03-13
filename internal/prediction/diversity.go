package prediction

import (
	"sort"

	"github.com/cgastin/cash-five-lotto/internal/stats"
	"github.com/cgastin/cash-five-lotto/internal/store"
)

// RankedCombo pairs a combination with its composite score.
type RankedCombo struct {
	Numbers      [5]int
	Score        float64
	FeaturesUsed map[string]float64
}

// GenerateTop5 enumerates all C(35,5) combinations, scores them, and returns
// the top 5 with diversity enforcement (no two candidates share >maxOverlap numbers).
// draws must be sorted ascending by date.
// Only current-format draws (pool size 35) are used for feature computation.
func GenerateTop5(draws []store.Draw, w Weights) []RankedCombo {
	return GenerateTopN(draws, w, 5, 2)
}

// GenerateTopN is the generalized form of GenerateTop5.
// n is how many candidates to return; maxOverlap is the maximum shared numbers
// between any two selected candidates.
// Only draws with PoolSize == 35 (or PoolSize == 0 for backward compat) are used.
func GenerateTopN(draws []store.Draw, w Weights, n, maxOverlap int) []RankedCombo {
	// Filter to current-format draws only (1-35 pool).
	// Legacy draws (1-39 pool) skew frequency stats for numbers that no longer exist.
	var current []store.Draw
	for _, d := range draws {
		if d.PoolSize == 0 || d.PoolSize == 35 {
			current = append(current, d)
		}
	}
	if len(current) == 0 {
		current = draws // fallback: use all if none tagged
	}
	f := stats.ComputeFeatures(current)
	combos := EnumerateAll()

	ranked := make([]RankedCombo, len(combos))
	for i, combo := range combos {
		ranked[i] = RankedCombo{
			Numbers: combo,
			Score:   ScoreCombo(combo, f, w),
		}
	}

	// Sort descending by score
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	// Diversity selection
	selected := make([]RankedCombo, 0, n)
	for _, candidate := range ranked {
		if len(selected) >= n {
			break
		}
		if diversityOK(candidate.Numbers, selected, maxOverlap) {
			candidate.FeaturesUsed = FeatureBreakdown(candidate.Numbers, f, w)
			selected = append(selected, candidate)
		}
	}

	return selected
}

// diversityOK returns true if combo overlaps ≤ maxOverlap numbers with every selected combo.
func diversityOK(combo [5]int, selected []RankedCombo, maxOverlap int) bool {
	for _, s := range selected {
		if Overlap(combo, s.Numbers) > maxOverlap {
			return false
		}
	}
	return true
}
