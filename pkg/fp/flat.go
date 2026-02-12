package fp

// Returns a new array with all sub-array elements concatenated into it recursively up to the specified depth.
func Flat[T any](xs [][]T) []T {
	n := 0
	for _, item := range xs {
		n += len(item)
	}
	result := make([]T, 0, n)
	for _, item := range xs {
		result = append(result, item...)
	}

	return result
}
