package stats

// DistributionScore scores a combination on odd/even and low/mid/high balance.
// Returns a value in [0, 1] where 1 is perfect balance.
//
// Ranges:
//   Low:  1-12  (12 numbers)
//   Mid:  13-24 (12 numbers)
//   High: 25-35 (11 numbers)
func DistributionScore(numbers [5]int) float64 {
	odd, even := 0, 0
	low, mid, high := 0, 0, 0

	for _, n := range numbers {
		if n%2 == 0 {
			even++
		} else {
			odd++
		}
		switch {
		case n <= 12:
			low++
		case n <= 24:
			mid++
		default:
			high++
		}
	}

	// Odd/even score: ideal is 2:3 or 3:2 split
	oeScore := oddEvenScore(odd, even)

	// Range score: ideal is at least 1 in each range, no range dominant
	rangeScore := rangeBalanceScore(low, mid, high)

	return (oeScore + rangeScore) / 2
}

// oddEvenScore returns 1.0 for 2:3 or 3:2, scaling down toward 0 for 0:5 or 5:0.
func oddEvenScore(odd, even int) float64 {
	// Deviation from ideal (2.5 = perfect balance)
	diff := odd - even
	if diff < 0 {
		diff = -diff
	}
	// diff: 0 or 1 → great, 3 → poor, 5 → worst
	switch diff {
	case 1:
		return 1.0
	case 3:
		return 0.5
	case 5:
		return 0.0
	default:
		return 0.75 // diff == 0 (exactly 2.5 impossible with 5 numbers, but 0 diff means... wait, 5 numbers can't be exactly 2.5 each so we handle odd case)
	}
}

// rangeBalanceScore rewards having numbers spread across low/mid/high.
func rangeBalanceScore(low, mid, high int) float64 {
	nonZeroRanges := 0
	if low > 0 {
		nonZeroRanges++
	}
	if mid > 0 {
		nonZeroRanges++
	}
	if high > 0 {
		nonZeroRanges++
	}

	// Bonus for coverage; penalty for concentration
	base := float64(nonZeroRanges) / 3.0

	// Penalty if any range has 4+ numbers (overly concentrated)
	maxInRange := low
	if mid > maxInRange {
		maxInRange = mid
	}
	if high > maxInRange {
		maxInRange = high
	}

	if maxInRange >= 4 {
		base *= 0.5
	}

	return base
}

// ConsecutivePenalty returns a penalty [0, 1] for runs of consecutive numbers.
// More consecutive pairs → higher penalty.
func ConsecutivePenalty(numbers [5]int) float64 {
	pairs := 0
	for i := 0; i < 4; i++ {
		if numbers[i+1]-numbers[i] == 1 {
			pairs++
		}
	}
	// 0 pairs → 0 penalty, 4 pairs → 1.0
	return float64(pairs) / 4.0
}

// SumAlignmentScore returns how close the combination sum is to the historical median.
// Returns 1.0 if exactly at median, approaching 0 as deviation grows beyond 2 stddevs.
func SumAlignmentScore(numbers [5]int, medianSum, sumStdDev float64) float64 {
	sum := 0
	for _, n := range numbers {
		sum += n
	}
	if sumStdDev == 0 {
		return 1.0
	}
	dev := float64(sum) - medianSum
	if dev < 0 {
		dev = -dev
	}
	zScore := dev / sumStdDev
	if zScore >= 2.0 {
		return 0.0
	}
	return 1.0 - zScore/2.0
}
