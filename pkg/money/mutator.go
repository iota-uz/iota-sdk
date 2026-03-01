// Package money provides this package.
package money

type mutator struct {
	calc *calculator
}

// initialize our default mutator here.
var mutate = mutator{calc: &calculator{}}
