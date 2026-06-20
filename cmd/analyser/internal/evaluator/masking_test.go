package evaluator

import (
	"testing"
)

func TestSliceToMask(t *testing.T) {
	tests := map[string]struct {
		slice    []int
		expected uint64
	}{
		"empty slice": {
			slice:    []int{},
			expected: 0,
		},
		"single value (5)": {
			slice:    []int{5},
			expected: 0b100000, // Bit 5 set (decimal 32)
		},
		"multiple values (1, 3, 5)": {
			slice:    []int{1, 3, 5},
			expected: 0b101010, // Bits 5, 3, and 1 set (decimal 42)
		},
		"value out of upper bound": {
			slice:    []int{64},
			expected: 0,
		},
		"negative value": {
			slice:    []int{-1},
			expected: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := SliceToMask(tc.slice)
			if result != tc.expected {
				t.Errorf("expected %b (binary), got %b", tc.expected, result)
			}
		})
	}
}
