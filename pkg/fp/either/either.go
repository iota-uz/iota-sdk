package either

import opt "github.com/iota-uz/iota-sdk/pkg/fp/option"

// BaseError struct
type Either[L, R any] struct {
	isLeft bool
	left   L
	right  R
}

// Constructor for Either with a value
func Left[L, R any](value L) Either[L, R] {
	return Either[L, R]{
		isLeft: true,
		left:   value,
	}
}

// Constructor for Either with an error
func Right[L, R any](value R) Either[L, R] {
	return Either[L, R]{
		isLeft: false,
		right:  value,
	}
}

// Helper to check if the Either has an error
func IsLeft[L, R any](e Either[L, R]) bool {
	return e.isLeft
}

// Helper to check if the Either has a value
func IsRight[L, R any](e Either[L, R]) bool {
	return !e.isLeft
}

// Returns `false` if `Left` or returns the boolean result of the application of the given predicate to the `Right` value
func Exists[L, R any](predicate func(right R) bool) func(Either[L, R]) bool {
	return func(e Either[L, R]) bool {

		if IsLeft(e) {
			return false
		}

		return predicate(e.right)
	}
}

// Removes one level of nesting. Returns its bound argument into the outer level.
func Flatten[L, R any](e Either[L, Either[L, R]]) Either[L, R] {

	if IsLeft(e) {
		return Left[L, R](e.left)
	}

	return e.right
}

// Constructor of Either from any couple of mutually exclusive `value` and `error`. Returns a Left in case we have an error, Right if we have a value and error is nil.
func FromError[R any](value R, e error) Either[error, R] {

	if e != nil {
		return Left[error, R](e)
	}

	return Right[error](value)
}

// Constructor of Either from any lazy function that returns a couple of mutually exclusive `value` and `error`. Returns a Left in case we have a return error, Right if we have return value and error is nil.
func FromErrorFn[R any](fn func() (value R, e error)) Either[error, R] {

	val, err := fn()

	if err != nil {
		return Left[error, R](err)
	}

	return Right[error](val)
}

// Constructor of Either from an Option.
// Returns a Left in case of None storing the callback return value as the error argument
// Returns a Right in case of Some with the option value.
func FromOption[L, R any](onNone func() L) func(o opt.Option[R]) Either[L, R] {
	return func(o opt.Option[R]) Either[L, R] {

		if opt.IsNone(o) {
			return Left[L, R](onNone())
		}

		return Right[L](o.Value)

	}
}

// Constructor of Either from a predicate.
// Returns a Left if the predicate function over the value return false.
// Returns a Right if the predicate function over the value return true.
func FromPredicate[L, R any](predicate func(value R) bool, onLeft func() L) func(R) Either[L, R] {
	return func(value R) Either[L, R] {

		if predicate(value) {
			return Right[L](value)
		}

		return Left[L, R](onLeft())
	}
}

// Extracts the value out of the Either, if it exists. Otherwise returns the result of the callback function that takes the error as argument.
func GetOrElse[L, R any](onLeft func(left L) R) func(Either[L, R]) R {
	return func(e Either[L, R]) R {

		if IsLeft(e) {
			return onLeft(e.left)
		}

		return e.right
	}
}

// Map over the Either value if it exists. Otherwise return the Either itself
func Map[L, R, T any](onRight func(right R) T) func(Either[L, R]) Either[L, T] {
	return func(e Either[L, R]) Either[L, T] {

		if IsLeft(e) {
			return Left[L, T](e.left)
		}

		return Right[L](onRight(e.right))
	}
}

// Map over the Either error if it exists. Otherwise return the Either with the new error type
func MapLeft[L, R, T any](fn func(left L) T) func(Either[L, R]) Either[T, R] {
	return func(e Either[L, R]) Either[T, R] {

		if IsLeft(e) {
			return Left[T, R](fn(e.left))
		}

		return Right[T](e.right)
	}
}

// Extracts the value out of the Either.
// Returns a new type running the succes or error callbacks which are taking respectively the error or value as an argument.
func Match[L, R, T any](onLeft func(left L) T, onRight func(right R) T) func(Either[L, R]) T {
	return func(e Either[L, R]) T {

		if IsLeft(e) {
			return onLeft(e.left)
		}

		return onRight(e.right)
	}
}
