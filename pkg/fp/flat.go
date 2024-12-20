package fp

// Returns a new array with all sub-array elements concatenated into it recursively up to the specified depth.
func Flat[T any](xs [][]T) []T {
	result := []T{}

	for _, item := range xs {
		result = append(result, item...)
	}

	return result
}
