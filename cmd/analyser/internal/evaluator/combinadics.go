package evaluator

import (
	"sort"
)

var chooseTable [61][61]int64

func init() {
	for n := 0; n <= 60; n++ {
		chooseTable[n][0] = 1
		for k := 1; k <= n; k++ {
			chooseTable[n][k] = chooseTable[n-1][k-1] + chooseTable[n-1][k]
		}
	}
}

// Choose returns the binomial coefficient (n choose k) from a precomputed Pascal's triangle.
// - n: the total numbers in the pool (e.g., 59 in Lotto)
// - k: how many of those numbers are picked (e.g., 6 in Lotto)
// Supports n up to 60.
func Choose(n, k int) int64 {
	if n < 0 || k < 0 || k > n || n > 60 {
		return 0
	}
	return chooseTable[n][k]
}

// UnrankCombination converts a rank in [0, Choose(n, k)-1] to a unique, sorted combination of k elements chosen from [1, n].
// - n: the total numbers in the pool
// - k: how many numbers are picked
// Uses the combinatorial number system (combinadics) representation.
func UnrankCombination(rank int64, n, k int) []int {
	combination := make([]int, k)
	next := rank
	for i := k; i >= 1; i-- {
		// Find the largest c such that Choose(c, i) <= next
		c := i - 1
		for Choose(c+1, i) <= next {
			c++
		}
		combination[k-i] = c + 1
		next -= Choose(c, i)
	}
	// Sort to return the combination in ascending order
	sort.Ints(combination)
	return combination
}

// UnrankCombinationToMask converts a rank in [0, Choose(n, k)-1] to a unique bitmask of k elements chosen from [1, n].
// - n: the total numbers in the pool
// - k: how many numbers are picked
// Uses the combinatorial number system (combinadics) representation.
// NOTE: Bit positions are 1-indexed, so the mask will have bits set at positions 1 through n.
func UnrankCombinationToMask(rank int64, n, k int) uint64 {
	mask := uint64(0)
	next := rank
	for i := k; i >= 1; i-- {
		// Find the largest c such that Choose(c, i) <= next
		c := i - 1
		for Choose(c+1, i) <= next {
			c++
		}
		mask |= 1 << (c + 1)
		next -= Choose(c, i)
	}
	return mask
}

// UnrankTicket converts a flat combination rank to primary and secondary number slices.
func UnrankTicket(rank int64, config GameConfig) (primarySlice, secondarySlice []int) {
	var primaryRank, secondaryRank int64

	if config.SecondarySelect > 0 {
		numSecondaryCombinations := Choose(config.SecondaryCount, config.SecondarySelect)
		primaryRank = rank / numSecondaryCombinations
		secondaryRank = rank % numSecondaryCombinations
	} else {
		primaryRank = rank
		secondaryRank = 0
	}

	primarySlice = UnrankCombination(primaryRank, config.PrimaryCount, config.PrimarySelect)

	if config.SecondarySelect > 0 {
		secondarySlice = UnrankCombination(secondaryRank, config.SecondaryCount, config.SecondarySelect)
	}
	return primarySlice, secondarySlice
}

// UnrankTicketToMasks converts a flat combination rank to primary and secondary number bitmasks.
func UnrankTicketToMasks(rank int64, config GameConfig) (primaryMask, secondaryMask uint64) {
	var primaryRank, secondaryRank int64

	if config.SecondarySelect > 0 {
		numSecondaryCombinations := Choose(config.SecondaryCount, config.SecondarySelect)
		primaryRank = rank / numSecondaryCombinations
		secondaryRank = rank % numSecondaryCombinations
	} else {
		primaryRank = rank
		secondaryRank = 0
	}

	primaryMask = UnrankCombinationToMask(primaryRank, config.PrimaryCount, config.PrimarySelect)

	if config.SecondarySelect > 0 {
		secondaryMask = UnrankCombinationToMask(secondaryRank, config.SecondaryCount, config.SecondarySelect)
	}
	return primaryMask, secondaryMask
}
