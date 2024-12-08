package sequence

import (
	"reflect"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func Title(str string) string {
	return cases.Title(language.English, cases.NoLower).String(str)
}

func ReverseInPlace[T any](array []T) []T {
	length := len(array)
	swap := reflect.Swapper(array)
	for i := range length / 2 {
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

func Pad(b *strings.Builder, str string) {
	if b.Len() == 0 {
		return
	}
	b.WriteString(str)
}

func RemoveNonNumeric(str string) string {
	var b strings.Builder
	for _, r := range str {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
