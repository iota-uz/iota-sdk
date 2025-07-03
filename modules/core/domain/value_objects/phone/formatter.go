package phone

import (
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
)

type DisplayStyle int

const (
	StyleParentheses DisplayStyle = iota
	StyleDashes
	StyleSpaces
)

func FormatForDisplay(phone Phone) string {
	return FormatWithStyle(phone, StyleParentheses)
}

func FormatWithStyle(phone Phone, style DisplayStyle) string {
	e164 := phone.E164()
	if e164 == "" {
		return ""
	}

	detectedCountry, _ := ParseCountry(e164)
	return formatByCountry(e164, detectedCountry, style)
}

func formatByCountry(e164 string, c country.Country, style DisplayStyle) string {
	cleaned := Strip(e164)
	
	switch c {
	case country.Uzbekistan:
		return formatUzbek(cleaned, style)
	case country.UnitedStates, country.Canada:
		return formatNorthAmerican(cleaned, style)
	case country.UnitedKingdom:
		return formatUK(cleaned, style)
	case country.Germany:
		return formatGerman(cleaned, style)
	case country.France:
		return formatFrench(cleaned, style)
	case country.Russia:
		return formatRussian(cleaned, style)
	default:
		return formatDefault(cleaned, style)
	}
}

func formatUzbek(phone string, style DisplayStyle) string {
	if len(phone) != 12 || !strings.HasPrefix(phone, "998") {
		return "+" + phone
	}
	
	prefix := phone[:3]
	code := phone[3:5]
	part1 := phone[5:8]
	part2 := phone[8:10]
	part3 := phone[10:12]
	
	switch style {
	case StyleParentheses:
		return "+" + prefix + "(" + code + ")" + part1 + "-" + part2 + "-" + part3
	case StyleDashes:
		return "+" + prefix + "-" + code + "-" + part1 + "-" + part2 + "-" + part3
	case StyleSpaces:
		return "+" + prefix + " " + code + " " + part1 + " " + part2 + " " + part3
	}
	return "+" + phone
}

func formatNorthAmerican(phone string, style DisplayStyle) string {
	if len(phone) != 11 || !strings.HasPrefix(phone, "1") {
		// For invalid North American numbers, use default formatting
		return formatDefault(phone, style)
	}
	
	prefix := phone[:1]
	area := phone[1:4]
	exchange := phone[4:7]
	number := phone[7:11]
	
	switch style {
	case StyleParentheses:
		return "+" + prefix + "(" + area + ")" + exchange + "-" + number
	case StyleDashes:
		return "+" + prefix + "-" + area + "-" + exchange + "-" + number
	case StyleSpaces:
		return "+" + prefix + " " + area + " " + exchange + " " + number
	}
	return "+" + phone
}

func formatUK(phone string, style DisplayStyle) string {
	if len(phone) < 10 || !strings.HasPrefix(phone, "44") {
		return "+" + phone
	}
	
	prefix := phone[:2]
	area := phone[2:4]
	part1 := phone[4:8]
	part2 := phone[8:]
	
	switch style {
	case StyleParentheses:
		return "+" + prefix + "(" + area + ")" + part1 + "-" + part2
	case StyleDashes:
		return "+" + prefix + "-" + area + "-" + part1 + "-" + part2
	case StyleSpaces:
		return "+" + prefix + " " + area + " " + part1 + " " + part2
	}
	return "+" + phone
}

func formatGerman(phone string, style DisplayStyle) string {
	if len(phone) < 10 || !strings.HasPrefix(phone, "49") {
		return "+" + phone
	}
	
	prefix := phone[:2]
	area := phone[2:5]
	part1 := phone[5:8]
	part2 := phone[8:]
	
	switch style {
	case StyleParentheses:
		return "+" + prefix + "(" + area + ")" + part1 + "-" + part2
	case StyleDashes:
		return "+" + prefix + "-" + area + "-" + part1 + "-" + part2
	case StyleSpaces:
		return "+" + prefix + " " + area + " " + part1 + " " + part2
	}
	return "+" + phone
}

func formatFrench(phone string, style DisplayStyle) string {
	if len(phone) < 10 || !strings.HasPrefix(phone, "33") {
		return "+" + phone
	}
	
	prefix := phone[:2]
	part1 := phone[2:4]
	part2 := phone[4:6]
	part3 := phone[6:8]
	part4 := phone[8:]
	
	switch style {
	case StyleParentheses:
		return "+" + prefix + "(" + part1 + ")" + part2 + "-" + part3 + "-" + part4
	case StyleDashes:
		return "+" + prefix + "-" + part1 + "-" + part2 + "-" + part3 + "-" + part4
	case StyleSpaces:
		return "+" + prefix + " " + part1 + " " + part2 + " " + part3 + " " + part4
	}
	return "+" + phone
}

func formatRussian(phone string, style DisplayStyle) string {
	if len(phone) < 10 || !strings.HasPrefix(phone, "7") {
		return "+" + phone
	}
	
	prefix := phone[:1]
	area := phone[1:4]
	part1 := phone[4:7]
	part2 := phone[7:9]
	part3 := phone[9:]
	
	switch style {
	case StyleParentheses:
		return "+" + prefix + "(" + area + ")" + part1 + "-" + part2 + "-" + part3
	case StyleDashes:
		return "+" + prefix + "-" + area + "-" + part1 + "-" + part2 + "-" + part3
	case StyleSpaces:
		return "+" + prefix + " " + area + " " + part1 + " " + part2 + " " + part3
	}
	return "+" + phone
}

func formatDefault(phone string, style DisplayStyle) string {
	if len(phone) < 7 {
		return "+" + phone
	}
	
	switch style {
	case StyleSpaces:
		return "+" + insertSpaces(phone, 3)
	case StyleDashes:
		return "+" + insertDashes(phone, 3)
	case StyleParentheses:
		// For unknown countries, use spaces as the default format
		return "+" + insertSpaces(phone, 3)
	default:
		return "+" + phone
	}
}

func insertSpaces(s string, interval int) string {
	var result strings.Builder
	for i, char := range s {
		if i > 0 && i%interval == 0 {
			result.WriteString(" ")
		}
		result.WriteRune(char)
	}
	return result.String()
}

func insertDashes(s string, interval int) string {
	var result strings.Builder
	for i, char := range s {
		if i > 0 && i%interval == 0 {
			result.WriteString("-")
		}
		result.WriteRune(char)
	}
	return result.String()
}

func FormatString(phoneStr string) string {
	if phoneStr == "" {
		return ""
	}
	
	phone, err := NewFromE164(phoneStr)
	if err != nil {
		return phoneStr
	}
	
	return FormatForDisplay(phone)
}