package fp

// Determines whether the specified callback function returns true for any element of an array.
func Some[T any](predicate func(T) bool) func([]T) bool {
	return func(xs []T) bool {

		for _, x := range xs {
			if predicate(x) {
				return true
			}
		}

		return false
	}
}

// See Some but callback receives index of element.
func SomeWithIndex[T any](predicate func(T, int) bool) func([]T) bool {
	return func(xs []T) bool {

		for i, x := range xs {
			if predicate(x, i) {
				return true
			}
		}

		return false
	}
}

// Like Some but callback receives index of element and the whole array.
func SomeWithSlice[T any](predicate func(T, int, []T) bool) func([]T) bool {
	return func(xs []T) bool {

		for i, x := range xs {
			if predicate(x, i, xs) {
				return true
			}
		}

		return false
	}
}
