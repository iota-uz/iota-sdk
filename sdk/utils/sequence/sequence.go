package sequence

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"reflect"
)

func Includes[T comparable](array []T, elem T) bool {
	for _, e := range array {
		if e == elem {
			return true
		}
	}
	return false
}

func Title(str string) string {
	return cases.Title(language.English, cases.NoLower).String(str)
}

func ReverseInPlace[T any](array []T) []T {
	length := len(array)
	swap := reflect.Swapper(array)
	for i := 0; i < length/2; i++ {
		swap(i, length-1-i)
	}
	return array
}

func Reverse[T any](array []T) []T {
	length := len(array)
	result := make([]T, length)
	for i, elem := range array {
		result[length-1-i] = elem
	}
	return result
}
