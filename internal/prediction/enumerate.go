// Package prediction generates and scores Cash Five candidate combinations.
package prediction

// EnumerateAll generates all C(35,5) = 324,632 combinations from the pool 1-35.
// Each combination is returned sorted ascending.
func EnumerateAll() [][5]int {
	const pool = 35
	const pick = 5
	// C(35,5) = 324,632
	result := make([][5]int, 0, 324632)

	for a := 1; a <= pool-4; a++ {
		for b := a + 1; b <= pool-3; b++ {
			for c := b + 1; c <= pool-2; c++ {
				for d := c + 1; d <= pool-1; d++ {
					for e := d + 1; e <= pool; e++ {
						result = append(result, [5]int{a, b, c, d, e})
					}
				}
			}
		}
	}
	return result
}

// Overlap returns the count of numbers two combinations share.
func Overlap(a, b [5]int) int {
	count := 0
	for _, x := range a {
		for _, y := range b {
			if x == y {
				count++
				break
			}
		}
	}
	return count
}
