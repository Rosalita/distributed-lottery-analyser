package evaluator

import (
	"reflect"
	"testing"
)

func TestChoose(t *testing.T) {
	tests := map[string]struct {
		n, k     int
		expected int64
	}{
		"5 choose 3":       {5, 3, 10},
		"5 choose 0":       {5, 0, 1},
		"5 choose 5":       {5, 5, 1},
		"5 choose 6":       {5, 6, 0},
		"negative n":       {-1, 2, 0},
		"lotto max choose": {59, 6, 45057474},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := Choose(tc.n, tc.k)
			if result != tc.expected {
				t.Errorf("Choose(%d, %d) = %d; expected %d", tc.n, tc.k, result, tc.expected)
			}
		})
	}
}

func TestUnrankCombination(t *testing.T) {
	t.Run("all ranks for 5 choose 3", func(t *testing.T) {
		seen := make(map[string]bool)
		for rank := int64(0); rank < 10; rank++ {
			comb := UnrankCombination(rank, 5, 3)
			if len(comb) != 3 {
				t.Errorf("Rank %d: expected length 3, got %d", rank, len(comb))
			}
			if comb[0] >= comb[1] || comb[1] >= comb[2] {
				t.Errorf("Rank %d: expected sorted combination, got %+v", rank, comb)
			}
			for _, v := range comb {
				if v < 1 || v > 5 {
					t.Errorf("Rank %d: value %d out of bounds [1,5]", rank, v)
				}
			}
			key := string(rune(comb[0])) + "," + string(rune(comb[1])) + "," + string(rune(comb[2]))
			if seen[key] {
				t.Errorf("Rank %d: produced duplicate combination %+v", rank, comb)
			}
			seen[key] = true
		}
	})

	tests := map[string]struct {
		rank     int64
		n, k     int
		expected []int
	}{
		"rank 0 for 5 choose 3": {0, 5, 3, []int{1, 2, 3}},
		"rank 9 for 5 choose 3": {9, 5, 3, []int{3, 4, 5}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := UnrankCombination(tc.rank, tc.n, tc.k)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("expected %+v, got %+v", tc.expected, result)
			}
		})
	}
}

func TestUnrankCombinationToMask(t *testing.T) {
	t.Run("compare with SliceToMask", func(t *testing.T) {
		// Let's test 1000 ranks for a typical game config (e.g., 39 choose 5)
		n := 39
		k := 5
		for rank := int64(0); rank < 1000; rank++ {
			// 1. Get the slice combination using the trusted original function
			comb := UnrankCombination(rank, n, k)

			// 2. Convert the slice combination to a bitmask using the trusted SliceToMask
			expectedMask := SliceToMask(comb)

			// 3. Get the bitmask directly using the new optimized function
			gotMask := UnrankCombinationToMask(rank, n, k)

			if gotMask != expectedMask {
				t.Errorf("Rank %d: expected mask %064b, got %064b", rank, expectedMask, gotMask)
			}
		}
	})
}

func TestUnrankTicket(t *testing.T) {
	tests := map[string]struct {
		rank                   int64
		config                 GameConfig
		expectedPrimaryCount   int
		expectedSecondaryCount int
	}{
		"thunderball rank check": {123456, ThunderballConfig, 5, 1},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pSlice, sSlice := UnrankTicket(tc.rank, tc.config)
			if len(pSlice) != tc.expectedPrimaryCount {
				t.Errorf("expected %d primary numbers, got %d", tc.expectedPrimaryCount, len(pSlice))
			}
			if len(sSlice) != tc.expectedSecondaryCount {
				t.Errorf("expected %d secondary numbers, got %d", tc.expectedSecondaryCount, len(sSlice))
			}
		})
	}
}

func TestUnrankTicketToMasks(t *testing.T) {
	t.Run("compare with SliceToMask for all configs", func(t *testing.T) {
		configs := []GameConfig{
			ThunderballConfig,
			LottoConfig,
			SetForLifeConfig,
			EuroMillionsConfig,
		}
		for _, config := range configs {
			for rank := int64(0); rank < 500; rank++ {
				// 1. Get slices using original UnrankTicket
				pSlice, sSlice := UnrankTicket(rank, config)

				// 2. Convert slices to expected masks
				expectedPMask := SliceToMask(pSlice)
				expectedSMask := SliceToMask(sSlice)

				// 3. Get masks directly using new function
				gotPMask, gotSMask := UnrankTicketToMasks(rank, config)

				if gotPMask != expectedPMask || gotSMask != expectedSMask {
					t.Errorf("Game %s, Rank %d: expected masks (P:%b, S:%b), got (P:%b, S:%b)",
						config.Name, rank, expectedPMask, expectedSMask, gotPMask, gotSMask)
				}
			}
		}
	})
}
