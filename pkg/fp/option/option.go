package opt

import (
	"github.com/iota-uz/iota-sdk/pkg/fp"
)

// BaseError struct
type Option[T any] struct {
	Value    T
	hasValue bool
}

// Constructor for Option with a value
func Some[T any](value T) Option[T] {
	return Option[T]{value, true}
}

// Constructor for Option without a value
func None[T any]() Option[T] {
	return Option[T]{}
}

// Helper to check if the Option has a value
func IsSome[T any](o Option[T]) bool {
	return o.hasValue
}

// Helper to check if the Option is missing the value
func IsNone[T any](o Option[T]) bool {
	return !o.hasValue
}

// Execute a function that returns an Option on the Option value if it exists. Otherwise return the empty Option itself
func Chain[A, B any](fn func(a A) Option[B]) func(Option[A]) Option[B] {
	return func(a Option[A]) Option[B] {

		if IsNone(a) {
			return None[B]()
		}

		return fn(a.Value)
	}
}

// Returns `false` if `None` or returns the boolean result of the application of the given predicate to the `Some` value
func Exists[T any](predicate func(value T) bool) func(Option[T]) bool {
	return func(o Option[T]) bool {

		if IsNone(o) {
			return false
		}

		return predicate(o.Value)
	}
}

// Removes one level of nesting. Returns its bound argument into the outer level.
func Flatten[T any](o Option[Option[T]]) Option[T] {

	if IsNone(o) {
		return None[T]()
	}

	return o.Value
}

// Constructor of Option from any couple of mutually exclusive `value` and `error`. Returns a None in case we have an error, Some if we have a value and error is nil.
func FromError[R any](value R, e error) Option[R] {

	if e != nil {
		return None[R]()
	}

	return Some(value)
}

// Constructor of Option from any lazy function that returns a couple of mutually exclusive `value` and `error`. Returns a None in case we have a return error, Some if we have return value and error is nil.
func FromErrorFn[R any](fn func() (value R, e error)) Option[R] {

	val, err := fn()

	if err != nil {
		return None[R]()
	}

	return Some(val)
}

// Constructor of Option from a predicate.
// Returns a None if the predicate function over the value return false.
// Returns a Some if the predicate function over the value return true.
func FromPredicate[T any](predicate func(value T) bool) func(T) Option[T] {
	return func(value T) Option[T] {

		if predicate(value) {
			return Some(value)
		}

		return None[T]()
	}
}

// Extracts the value out of the Option, if it exists. Otherwise returns the function with a default value
func GetOrElse[T any](onNone fp.Lazy[T]) func(Option[T]) T {
	return func(o Option[T]) T {

		if IsNone(o) {
			return onNone()
		}

		return o.Value
	}
}

// Execute the function on the Option value if it exists. Otherwise return the empty Option itself
func Map[T, R any](fn func(value T) R) func(o Option[T]) Option[R] {
	return func(o Option[T]) Option[R] {

		if IsNone(o) {
			return None[R]()
		}

		return Some(fn(o.Value))
	}
}

// Extracts the value out of the Option, if it exists, with a function. Otherwise returns the function with a default value
func Match[T, R any](onNone fp.Lazy[R], onSome func(value T) R) func(Option[T]) R {
	return func(o Option[T]) R {

		if IsNone(o) {
			return onNone()
		}

		return onSome(o.Value)
	}
}
