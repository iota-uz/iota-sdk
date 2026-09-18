package excel

import "strings"

// Formula marks an export cell as a spreadsheet formula. It is the only way a
// cell becomes a formula: every plain string is data in both CSV and XLSX
// output, whatever it starts with. Expression is stored without the leading
// '=' (one is accepted and dropped).
//
// Build Expression only from trusted parts (cell references, function names,
// numbers). Never concatenate user text into it — that re-opens the formula
// injection plain strings are protected from.
type Formula struct {
	Expression string
}

// NeutralizeFormula makes a plain string safe to put in a CSV cell: it
// prefixes an apostrophe when Excel, LibreOffice or Google Sheets would
// evaluate the text as a formula (CSV/formula injection, OWASP). That is a
// leading '=', '@', tab or carriage return, or a leading '+'/'-' that does not
// start a single value: after '-' only one (formatted) number may follow, after
// '+' also phone punctuation. So "-5.00", "-1 234,56", "+998 (90) 123-45-67"
// and a lone "-" placeholder keep their value, while "-1-1", which a
// spreadsheet would compute, is prefixed.
//
// XLSX output does not need this: excelize stores a string as a string cell,
// never as a formula.
func NeutralizeFormula(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '@', '\t', '\r':
		return "'" + s
	case '-':
		if isNumberBody(s[1:]) {
			return s
		}
		return "'" + s
	case '+':
		if isPhoneBody(s[1:]) {
			return s
		}
		return "'" + s
	default:
		return s
	}
}

// isNumberBody reports whether s is the unsigned part of one formatted number:
// digits with group and decimal separators (space, no-break space, dot,
// comma). It holds no operator, so "-" + s stays a single negative value.
func isNumberBody(s string) bool {
	return onlyDigitsAnd(s, " \u00a0.,")
}

// isPhoneBody is isNumberBody that also allows the parentheses and hyphens of
// a phone number. None of these characters can form a function call, a cell
// reference or a DDE link, so such a value is never an injection payload.
func isPhoneBody(s string) bool {
	return onlyDigitsAnd(s, " \u00a0.,()-")
}

// onlyDigitsAnd reports whether every rune of s is a digit or one of extra.
func onlyDigitsAnd(s, extra string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && !strings.ContainsRune(extra, r) {
			return false
		}
	}
	return true
}

// formulaExpression returns f's expression without a leading '='.
func formulaExpression(f Formula) string {
	if len(f.Expression) > 0 && f.Expression[0] == '=' {
		return f.Expression[1:]
	}
	return f.Expression
}

// asFormula returns the expression of a Formula / *Formula value. Only this
// explicit type is ever written as a formula.
func asFormula(v interface{}) (string, bool) {
	switch t := v.(type) {
	case Formula:
		return formulaExpression(t), true
	case *Formula:
		if t == nil {
			return "", false
		}
		return formulaExpression(*t), true
	default:
		return "", false
	}
}
