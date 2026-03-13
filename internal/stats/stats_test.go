package stats

import (
	"math"
	"testing"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

func makeDraws(sets [][5]int) []store.Draw {
	draws := make([]store.Draw, len(sets))
	base := time.Date(2020, 1, 6, 0, 0, 0, 0, time.UTC) // Monday
	for i, nums := range sets {
		draws[i] = store.Draw{
			DrawDate: base.AddDate(0, 0, i),
			Numbers:  nums,
			GameName: "Cash Five",
		}
	}
	return draws
}

func TestComputeFeatures_Empty(t *testing.T) {
	f := ComputeFeatures(nil)
	if f.DrawCount != 0 {
		t.Errorf("expected 0 draws, got %d", f.DrawCount)
	}
}

func TestComputeFeatures_AllTimeFreq(t *testing.T) {
	draws := makeDraws([][5]int{
		{1, 2, 3, 4, 5},
		{1, 6, 7, 8, 9},
		{1, 2, 10, 11, 12},
	})
	f := ComputeFeatures(draws)

	if f.AllTimeFreq[1] != 3 {
		t.Errorf("number 1 should appear 3 times, got %d", f.AllTimeFreq[1])
	}
	if f.AllTimeFreq[2] != 2 {
		t.Errorf("number 2 should appear 2 times, got %d", f.AllTimeFreq[2])
	}
	if f.AllTimeFreq[13] != 0 {
		t.Errorf("number 13 should appear 0 times, got %d", f.AllTimeFreq[13])
	}
}

func TestComputeFeatures_Rolling30(t *testing.T) {
	// Create 35 draws; only last 30 should be in rolling30
	sets := make([][5]int, 35)
	for i := range sets {
		sets[i] = [5]int{1, 2, 3, 4, 5}
	}
	// First 5 draws include number 6; last 30 don't
	sets[0] = [5]int{6, 7, 8, 9, 10}
	sets[1] = [5]int{6, 7, 8, 9, 10}
	sets[2] = [5]int{6, 7, 8, 9, 10}
	sets[3] = [5]int{6, 7, 8, 9, 10}
	sets[4] = [5]int{6, 7, 8, 9, 10}

	draws := makeDraws(sets)
	f := ComputeFeatures(draws)

	if f.Rolling30Freq[6] != 0 {
		t.Errorf("number 6 should not appear in rolling 30, got %d", f.Rolling30Freq[6])
	}
	if f.Rolling30Freq[1] != 30 {
		t.Errorf("number 1 should appear 30 times in rolling 30, got %d", f.Rolling30Freq[1])
	}
}

func TestComputeFeatures_GapSinceLastSeen(t *testing.T) {
	draws := makeDraws([][5]int{
		{1, 2, 3, 4, 5},   // index 0
		{6, 7, 8, 9, 10},  // index 1
		{11, 12, 13, 14, 15}, // index 2
	})
	f := ComputeFeatures(draws)

	// Number 1 last seen at index 0, latest index is 2 → gap = 2
	if f.GapSinceLastSeen[1] != 2 {
		t.Errorf("gap for 1 should be 2, got %d", f.GapSinceLastSeen[1])
	}
	// Number 6 last seen at index 1, gap = 1
	if f.GapSinceLastSeen[6] != 1 {
		t.Errorf("gap for 6 should be 1, got %d", f.GapSinceLastSeen[6])
	}
	// Number 11 last seen at index 2, gap = 0
	if f.GapSinceLastSeen[11] != 0 {
		t.Errorf("gap for 11 should be 0, got %d", f.GapSinceLastSeen[11])
	}
	// Number 35 never seen, gap = drawCount
	if f.GapSinceLastSeen[35] != 3 {
		t.Errorf("gap for unseen 35 should be 3 (draw count), got %d", f.GapSinceLastSeen[35])
	}
}

func TestComputeFeatures_MedianSum(t *testing.T) {
	draws := makeDraws([][5]int{
		{1, 2, 3, 4, 5},     // sum=15
		{10, 11, 12, 13, 14}, // sum=60
	})
	f := ComputeFeatures(draws)
	expected := (15.0 + 60.0) / 2
	if math.Abs(f.MedianSum-expected) > 0.001 {
		t.Errorf("median sum: got %f, want %f", f.MedianSum, expected)
	}
}

func TestDistributionScore(t *testing.T) {
	tests := []struct {
		name    string
		numbers [5]int
		wantMin float64
	}{
		{"balanced", [5]int{2, 14, 18, 26, 31}, 0.5},
		{"all low", [5]int{1, 2, 3, 4, 5}, 0.0},
		{"all odd", [5]int{1, 3, 5, 7, 9}, 0.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := DistributionScore(tc.numbers)
			if score < 0 || score > 1 {
				t.Errorf("score out of range [0,1]: %f", score)
			}
		})
	}
}

func TestConsecutivePenalty(t *testing.T) {
	tests := []struct {
		numbers [5]int
		want    float64
	}{
		{[5]int{1, 2, 3, 4, 5}, 1.0},  // all consecutive
		{[5]int{1, 3, 5, 7, 9}, 0.0},  // no consecutive
		{[5]int{1, 2, 5, 10, 15}, 0.25}, // one pair
	}
	for _, tc := range tests {
		got := ConsecutivePenalty(tc.numbers)
		if math.Abs(got-tc.want) > 0.001 {
			t.Errorf("ConsecutivePenalty(%v) = %f, want %f", tc.numbers, got, tc.want)
		}
	}
}

func TestSumAlignmentScore(t *testing.T) {
	// At median, score = 1.0
	nums := [5]int{1, 2, 3, 4, 90} // won't be used — we test with median=sum
	_ = nums
	score := SumAlignmentScore([5]int{10, 20, 30, 35, 5}, 100.0, 10.0)
	if score != 1.0 {
		t.Errorf("expected 1.0 for exact median, got %f", score)
	}

	// 2+ stddevs away → 0
	score2 := SumAlignmentScore([5]int{1, 2, 3, 4, 5}, 100.0, 5.0)
	if score2 != 0.0 {
		t.Errorf("expected 0.0 for >2 stddev away, got %f", score2)
	}
}

func TestMaxFrequency(t *testing.T) {
	freq := NumberFrequency{1: 5, 2: 3, 3: 10}
	if MaxFrequency(freq) != 10 {
		t.Errorf("expected max 10, got %d", MaxFrequency(freq))
	}
}
