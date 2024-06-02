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
	for i := 0; i < length; i++ {
		password += string(chatSet[rand.Intn(len(chatSet))])
	}
	return password
}
