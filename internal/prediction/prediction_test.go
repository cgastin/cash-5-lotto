package prediction

import (
	"testing"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

func TestEnumerateAll(t *testing.T) {
	combos := EnumerateAll()
	// C(35,5) = 324,632
	if len(combos) != 324632 {
		t.Fatalf("expected 324632 combinations, got %d", len(combos))
	}

	// Verify first and last combinations
	first := combos[0]
	if first != [5]int{1, 2, 3, 4, 5} {
		t.Errorf("first combo: got %v, want [1 2 3 4 5]", first)
	}
	last := combos[len(combos)-1]
	if last != [5]int{31, 32, 33, 34, 35} {
		t.Errorf("last combo: got %v, want [31 32 33 34 35]", last)
	}

	// Verify all are sorted and in range
	for i, c := range combos {
		for j := 0; j < 4; j++ {
			if c[j] >= c[j+1] {
				t.Errorf("combo %d not sorted: %v", i, c)
				break
			}
		}
		for _, n := range c {
			if n < 1 || n > 35 {
				t.Errorf("combo %d has out-of-range number %d", i, n)
			}
		}
	}
}

func TestOverlap(t *testing.T) {
	tests := []struct {
		a, b [5]int
		want int
	}{
		{[5]int{1, 2, 3, 4, 5}, [5]int{1, 2, 3, 4, 5}, 5},
		{[5]int{1, 2, 3, 4, 5}, [5]int{6, 7, 8, 9, 10}, 0},
		{[5]int{1, 2, 3, 4, 5}, [5]int{3, 4, 5, 6, 7}, 3},
	}
	for _, tc := range tests {
		got := Overlap(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("Overlap(%v, %v) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func makeTestDraws(n int) []store.Draw {
	draws := make([]store.Draw, n)
	base := time.Date(2020, 1, 6, 0, 0, 0, 0, time.UTC)
	for i := range draws {
		draws[i] = store.Draw{
			DrawDate: base.AddDate(0, 0, i),
			Numbers:  [5]int{1 + (i % 31), 2 + (i % 30), 3 + (i % 29), 4 + (i % 28), 5 + (i % 27)},
		}
	}
	return draws
}

func TestGenerateTop5_ReturnsFive(t *testing.T) {
	draws := makeTestDraws(100)
	w := DefaultWeights()
	candidates := GenerateTop5(draws, w)

	if len(candidates) != 5 {
		t.Fatalf("expected 5 candidates, got %d", len(candidates))
	}
}

func TestGenerateTop5_DiversityEnforced(t *testing.T) {
	draws := makeTestDraws(100)
	w := DefaultWeights()
	candidates := GenerateTop5(draws, w)

	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			overlap := Overlap(candidates[i].Numbers, candidates[j].Numbers)
			if overlap > 2 {
				t.Errorf("candidates %d and %d share %d numbers (max 2): %v vs %v",
					i, j, overlap, candidates[i].Numbers, candidates[j].Numbers)
			}
		}
	}
}

func TestGenerateTop5_ScoresDescending(t *testing.T) {
	draws := makeTestDraws(100)
	w := DefaultWeights()
	candidates := GenerateTop5(draws, w)

	for i := 1; i < len(candidates); i++ {
		if candidates[i].Score > candidates[i-1].Score {
			t.Errorf("candidates not in descending score order at index %d: %f > %f",
				i, candidates[i].Score, candidates[i-1].Score)
		}
	}
}

func TestGenerateTop5_HasFeatureBreakdown(t *testing.T) {
	draws := makeTestDraws(50)
	w := DefaultWeights()
	candidates := GenerateTop5(draws, w)

	for i, c := range candidates {
		if len(c.FeaturesUsed) == 0 {
			t.Errorf("candidate %d missing feature breakdown", i)
		}
		if _, ok := c.FeaturesUsed["composite_score"]; !ok {
			t.Errorf("candidate %d missing composite_score in features", i)
		}
	}
}

func TestScoreCombo_Deterministic(t *testing.T) {
	draws := makeTestDraws(50)
	w := DefaultWeights()

	candidates1 := GenerateTopN(draws, w, 1, 0)
	candidates2 := GenerateTopN(draws, w, 1, 0)

	if candidates1[0].Score != candidates2[0].Score {
		t.Errorf("score is not deterministic: %f vs %f",
			candidates1[0].Score, candidates2[0].Score)
	}
}
