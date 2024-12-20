# fp-go

[![Go Reference](https://pkg.go.dev/badge/github.com/repeale/fp-go.svg)](https://pkg.go.dev/github.com/repeale/fp-go)
[![Go Report](https://goreportcard.com/badge/github.com/repeale/fp-go)](https://goreportcard.com/badge/github.com/repeale/fp-go)
[![codecov](https://codecov.io/gh/repeale/fp-go/branch/main/graph/badge.svg?token=ORP8NR634Q)](https://codecov.io/gh/repeale/fp-go)

Fp-go is a collection of Functional Programming helpers powered by Golang [1.18](https://tip.golang.org/doc/go1.18)+ [generics](https://tip.golang.org/doc/go1.18#generics).

**Inspired by**

- [fp-ts](https://github.com/gcanti/fp-ts)

<p align="center">
  <img
    width="500"
    height="313"
    src="https://user-images.githubusercontent.com/9580458/162070974-4367f4b8-bb3d-451c-8114-dd578bad4e46.png"
  >
</p>

## Contents

- [Install](#install)
- [Features](#features)
  - [Currying](#currying)
  - [Variations](#variations)
- [Helpers](#helpers)
  - [Every](#every)
  - [Filter](#filter)
  - [Flat](#flat)
  - [FlatMap](#flatmap)
  - [Map](#map)
  - [Reduce](#reduce)
  - [Some](#some)
  - [Compose](#compose)
  - [Pipe](#pipe)
  - [Curry](#curry)
- [Structs](#structs)
  - [Option](#option)
  - [Either](#either)

## Install

Requires go 1.18+

```sh
go get github.com/repeale/fp-go
```

## Features

- [Currying](#currying)
- [Variations](#variations)

### Currying

By default! Data last!

```go
func isPositive(x int) bool {
	return x > 0
}

func main() {
    filterPositive := fp.Filter(isPositive)
    numbers := []int{1, 2, 3, 4, 5}

    positives := filterPositive(numbers)
}
```

### Variations

Variations allows you to get different parameters in the callback function so that you get only only what is really needed.

[Default](#default) \
[WithIndex](#withindex) \
[WithSlice](#withslice)

#### Default

Only the current item is available:

```go
fp.Map[int, string](func(x int) { ... })
```

#### WithIndex

Current element and index are available:

```go
fp.MapWithIndex[int, string](func(x int, i int) { ... })
```

#### WithSlice

Current element, index and the whole slice are available:

```go
fp.MapWithSlice[int, string](func(x int, i int, xs: []int) { ... })
```

## Helpers

#### Every

Determines whether all the members of an array satisfy the specified test.

Variations `EveryWithIndex` and `EveryWithSlice`

```go
fp.Every(func(x int) bool { return x > 0 })([]int{1, 2, 3})

// => true
```

#### Filter

Returns the elements of an array that meet the condition specified in a callback function.

Variations `FilterWithIndex` and `FilterWithSlice`

```go
fp.Filter(func(x int) bool { return x > 0 })([]int{-1, 2, -3, 4})

// => []int{2, 4}
```

#### Flat

Returns a new array with all sub-array elements concatenated into it.

```go
fp.Flat([][]int{{1, 2}, {3, 4}})

// => []int{1, 2, 3, 4}
```

#### FlatMap

Calls a defined callback function on each element of an array. Then, flattens the result into a new array. This is identical to a map followed by flat.

Variations `FlatMapWithIndex` and `FlatMapWithSlice`

```go
fp.FlatMap(func(x int) []int { return []int{x, x} })([]int{1, 2})

// => []int{1, 1, 2, 2}
```

#### Map

Calls a defined callback function on each element of an array, and returns an array that contains the results.

Variations `MapWithIndex` and `MapWithSlice`

```go
fp.Map(func(x int64) string {
    return strconv.FormatInt(x, 10)
})([]int64{1, 2, 3})

// => []string{"1", "2", "3"}
```

#### Reduce

Calls the specified callback function for all the elements in an array. The return value of the callback function is the accumulated result, and is provided as an argument in the next call to the callback function.

Variations `ReduceWithIndex` and `ReduceWithSlice`

```go
fp.Reduce(func(acc int, curr int) int { return acc + curr }, 0)([]int{1, 2, 3})

// => 6
```

#### Some

Determines whether the specified callback function returns true for any element of an array.

Variations `SomeWithIndex` and `SomeWithSlice`

```go
fp.Some(func(x int) bool { return x < 0 })([]int{1, 2, 3})

// => false
```

---

#### Compose

Performs right-to-left function composition.

Variations `Compose2` to `Compose16` stating the number of functions you are going to compose.

```go
func isPositive(x int) bool {
	return x > 0
}

func sumTwo(x int) int {
	return x + 2
}

Compose2(fp.Filter(isPositive), fp.Map(sumTwo))([]int{1, 2, 3, -1})

// => []int{3, 4, 5, 1}
```

#### Pipe

Performs left-to-right function composition.

Variations `Pipe2` to `Pipe16` stating the number of functions you are going to compose.

```go
func isPositive(x int) bool {
	return x > 0
}

func sumTwo(x int) int {
	return x + 2
}

Pipe2(fp.Filter(isPositive), fp.Map(sumTwo))([]int{1, 2, 3, -1})

// => []int{3, 4, 5}
```

#### Curry

Allow to transfrom a function that receives a certain amount of params in single functions that take one single param each.

Variations `Curry2` to `Curry16` stating the number of params will be curried individually.

```go
curryedSum := Curry2(func(a int, b int) int { return a + b })
curryedSum(1)(2)
```

## Structs

#### Option

Option represents encapsulation of an optional value, it might be used as the return type of functions which may or may not return a meaningful value when they are applied.

Option exports `Some`, `None`, `IsSome`, `IsNone`, `Chain`, `Exists`, `Flatten`, `FromError`, `FromErrorFn`, `FromPredicate`, `GetOrElse`, `Map`, `Match`.

#### Either

Either represent a value that can have two possible types. It is common to see Either used to represent a success value `Right` or a failure value `Left`, although that doesn't have to be the case.
An instance of `Either` is either an instance of `Left` or `Right`.

Either exports `Left`, `Right`, `IsLeft`, `IsRight`, `Exists`, `Flatten`, `FromError`, `FromErrorFn`, `FromOption`, `FromPredicate`, `GetOrElse`, `Map`, `MapLeft`, `Match`,
