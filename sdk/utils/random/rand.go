package random

import "math/rand"

const (
	LowerCharSet    = "abcdedfghijklmnopqrst"
	UpperCharSet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	SpecialCharSet  = "!@#$%&*"
	NumberSet       = "0123456789"
	AlphaNumericSet = LowerCharSet + UpperCharSet + NumberSet
	AllCharSet      = LowerCharSet + UpperCharSet + SpecialCharSet + NumberSet
)

func String(length int, chatSet string) string {
	var password string
	for range length {
		password += string(chatSet[rand.Intn(len(chatSet))]) //nolint:gosec
	}
	return password
}
