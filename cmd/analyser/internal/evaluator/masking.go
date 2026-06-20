package evaluator

// SliceToMask converts a slice of integers to a uint64 bitmask.
func SliceToMask(slice []int) uint64 {
	var mask uint64
	for _, val := range slice {
		if val >= 0 && val < 64 {
			mask |= (1 << uint(val))
		}
	}
	return mask
}
