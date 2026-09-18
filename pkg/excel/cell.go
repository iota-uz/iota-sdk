package excel

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
// leading '=', '@', tab or carriage return, or a leading '+'/'-' followed by
// anything but digits and number punctuation. Numbers and formatted numbers
// such as "-5.00", "-1 234,56", "+998 90 123 45 67" and a lone "-" placeholder
// stay untouched, so numeric columns and phone numbers keep their value.
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
	case '+', '-':
		if isNumberLike(s[1:]) {
			return s
		}
		return "'" + s
	default:
		return s
	}
}

// isNumberLike reports whether s holds only digits and the punctuation of a
// formatted number or phone (space, no-break space, dot, comma, parentheses,
// minus). A formula payload needs letters or other operators.
func isNumberLike(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == ' ', r == ' ', r == '.', r == ',', r == '(', r == ')', r == '-':
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
