package sms

import (
	"strings"
	"unicode"
)

// Bounds of an E.164 number: the digits after "+", country code included.
const (
	minE164Digits = 8
	maxE164Digits = 15
	// minNationalDigits is the shortest national number that is taken as one already starting with the
	// default country code (a number "998901234567" for the default "998").
	minNationalDigits = 7
)

// normalizePhone turns a number a graph author wrote ("+998 90 123-45-67", "00998901234567", "90 123 45 67")
// into E.164. A number without "+" or "00" gets the default country code (digits only, "" when none is
// configured); one that already starts with the code and is long enough to hold a national number
// after it is taken as complete. ok is false when the text is not a number or has no country code.
func normalizePhone(raw, defaultCC string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	international := false
	var digits strings.Builder
	for _, r := range s {
		switch {
		case r == '+' && !international && digits.Len() == 0:
			international = true
		case unicode.IsDigit(r) && r < unicode.MaxASCII:
			digits.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.' || r == ' ':
		default:
			return "", false
		}
	}
	d := digits.String()
	if !international && strings.HasPrefix(d, "00") {
		international = true
		d = strings.TrimLeft(d[2:], "0")
	}
	if !international {
		if defaultCC == "" {
			return "", false
		}
		d = strings.TrimLeft(d, "0") // a national trunk prefix
		if !(strings.HasPrefix(d, defaultCC) && len(d)-len(defaultCC) >= minNationalDigits) {
			d = defaultCC + d
		}
	}
	if len(d) < minE164Digits || len(d) > maxE164Digits || d[0] == '0' {
		return "", false
	}
	return "+" + d, true
}

// countryCode reduces a configured code ("+998", "998", " 998 ") to its digits; "" when it is not one.
func countryCode(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "+")
	if s == "" || len(s) > 3 {
		return ""
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return ""
		}
	}
	if s[0] == '0' {
		return ""
	}
	return s
}
