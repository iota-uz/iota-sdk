package fp

// Calls a defined callback function on each element of an array. Then, flattens the result into a new array. This is identical to a map followed by flat with depth 1.
func FlatMap[T any, R any](callback func(T) []R) func([]T) []R {
	return func(xs []T) []R {

		result := []R{}

		for _, x := range xs {
			result = append(result, callback(x)...)
		}

		return result
	}
}

// See FlatMap but callback receives index of element.
func FlatMapWithIndex[T any, R any](callback func(T, int) []R) func([]T) []R {
	return func(xs []T) []R {

		result := []R{}

		for i, x := range xs {
			result = append(result, callback(x, i)...)
		}

		return result
	}
}

// Like FlatMap but callback receives index of element and the whole array.
func FlatMapWithSlice[T any, R any](callback func(T, int, []T) []R) func([]T) []R {
	return func(xs []T) []R {

		result := []R{}

		for i, x := range xs {
			result = append(result, callback(x, i, xs)...)
		}

		return result
	}
}
