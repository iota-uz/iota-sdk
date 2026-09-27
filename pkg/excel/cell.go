package excel

import "unicode/utf8"

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
// evaluate the text as a formula (CSV/formula injection, OWASP, CWE-1236).
// That is a leading '=', '@', tab, carriage return or line feed, their
// full-width forms, or a leading '+'/'-' that does not start one number.
//
// The number exception keeps signed amounts and plain phone numbers as they
// are: "-5.00", "-1 234,56", "+998901234567", "+998 90 123 45 67" and a lone
// "-" placeholder. Anything with an operator after the sign is prefixed —
// "-1-1", and also a hyphenated phone such as "+998 (90) 123-45-67", which a
// spreadsheet would otherwise compute into a wrong number.
//
// XLSX output does not need this: excelize stores a string as a string cell,
// never as a formula.
func NeutralizeFormula(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	switch r {
	case '=', '@', '\t', '\r', '\n', '\uff1d', '\uff20', '\uff0b', '\uff0d':
		return "'" + s
	case '+', '-':
		if isNumberBody(s[size:]) {
			return s
		}
		return "'" + s
	default:
		return s
	}
}

// isNumberBody reports whether s is the unsigned part of one formatted number:
// digits with group and decimal separators (space, no-break space, dot,
// comma). It holds no arithmetic operator, so a sign followed by s stays a
// single value.
func isNumberBody(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == ' ', r == '\u00a0', r == '.', r == ',':
		default:
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
