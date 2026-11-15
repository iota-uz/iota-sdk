package formula

import (
	"fmt"
	"html"
	"strings"
	"unicode"
)

// The parser covers the subset of LaTeX that report methodology uses: fractions,
// roots, scripts, grouping, text, big operators and Greek. Anything outside it
// is an error rather than a silent misrender — the caller shows a message and
// logs the event, and the report goes on.

type node struct {
	kind     string // seq, num, ident, op, text, frac, sqrt, sup, sub, subsup, fenced, space
	value    string
	children []*node
}

type parser struct {
	src []rune
	pos int
}

func parse(src string) (*node, error) {
	p := &parser{src: []rune(src)}
	n, err := p.sequence(0)
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.src) {
		return nil, fmt.Errorf("unexpected %q at %d", string(p.src[p.pos]), p.pos)
	}
	return n, nil
}

var symbols = map[string]string{
	"cdot": "·", "times": "×", "div": "÷", "pm": "±", "leq": "≤", "le": "≤", "geq": "≥", "ge": "≥",
	"neq": "≠", "approx": "≈", "to": "→", "infty": "∞", "%": "%", "$": "$", "&": "&", "_": "_", "#": "#",
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε", "theta": "θ", "lambda": "λ",
	"mu": "μ", "pi": "π", "rho": "ρ", "sigma": "σ", "tau": "τ", "phi": "φ", "omega": "ω",
	"Delta": "Δ", "Sigma": "Σ", "Omega": "Ω", "ldots": "…", "dots": "…",
}

var bigOps = map[string]string{"sum": "∑", "prod": "∏", "int": "∫"}

var names = map[string]string{"min": "min", "max": "max", "log": "log", "ln": "ln", "exp": "exp", "avg": "avg"}

func (p *parser) peek() (rune, bool) {
	if p.pos >= len(p.src) {
		return 0, false
	}
	return p.src[p.pos], true
}

// sequence parses until end, a closing brace (depth>0) or \right.
func (p *parser) sequence(depth int) (*node, error) {
	seq := &node{kind: "seq"}
	for {
		r, ok := p.peek()
		if !ok {
			if depth > 0 {
				return nil, fmt.Errorf("unclosed group")
			}
			return seq, nil
		}
		if r == '}' {
			if depth == 0 {
				return nil, fmt.Errorf("unmatched }")
			}
			return seq, nil
		}
		if r == '\\' && p.hasPrefix(`\right`) {
			return seq, nil
		}
		atom, err := p.atom()
		if err != nil {
			return nil, err
		}
		atom, err = p.scripts(atom)
		if err != nil {
			return nil, err
		}
		seq.children = append(seq.children, atom)
	}
}

func (p *parser) hasPrefix(s string) bool {
	rs := []rune(s)
	if p.pos+len(rs) > len(p.src) {
		return false
	}
	return string(p.src[p.pos:p.pos+len(rs)]) == s
}

func (p *parser) scripts(base *node) (*node, error) {
	var sub, sup *node
	for {
		r, ok := p.peek()
		if !ok || (r != '^' && r != '_') {
			break
		}
		p.pos++
		arg, err := p.argument()
		if err != nil {
			return nil, fmt.Errorf("missing script argument: %w", err)
		}
		if r == '^' {
			if sup != nil {
				return nil, fmt.Errorf("double superscript")
			}
			sup = arg
		} else {
			if sub != nil {
				return nil, fmt.Errorf("double subscript")
			}
			sub = arg
		}
	}
	switch {
	case sub != nil && sup != nil:
		return &node{kind: "subsup", children: []*node{base, sub, sup}}, nil
	case sub != nil:
		return &node{kind: "sub", children: []*node{base, sub}}, nil
	case sup != nil:
		return &node{kind: "sup", children: []*node{base, sup}}, nil
	}
	return base, nil
}

// argument is one braced group or one atom.
func (p *parser) argument() (*node, error) {
	p.skipSpace()
	r, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("unexpected end")
	}
	if r == '{' {
		p.pos++
		n, err := p.sequence(1)
		if err != nil {
			return nil, err
		}
		p.pos++ // }
		return n, nil
	}
	if r == '}' || r == '^' || r == '_' {
		return nil, fmt.Errorf("unexpected %q", string(r))
	}
	return p.atom()
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) && unicode.IsSpace(p.src[p.pos]) {
		p.pos++
	}
}

func (p *parser) atom() (*node, error) {
	p.skipSpace()
	r, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("unexpected end")
	}
	switch {
	case r == '{':
		return p.argument()
	case r == '\\':
		return p.command()
	case unicode.IsDigit(r) || r == '.' || r == ',':
		start := p.pos
		for p.pos < len(p.src) && (unicode.IsDigit(p.src[p.pos]) || p.src[p.pos] == '.') {
			p.pos++
		}
		if p.pos == start {
			p.pos++
		}
		return &node{kind: "num", value: string(p.src[start:p.pos])}, nil
	case unicode.IsLetter(r):
		p.pos++
		return &node{kind: "ident", value: string(r)}, nil
	case strings.ContainsRune("+-=<>/*|!:;", r):
		p.pos++
		return &node{kind: "op", value: string(r)}, nil
	case r == '(' || r == ')' || r == '[' || r == ']':
		p.pos++
		return &node{kind: "op", value: string(r)}, nil
	case r == '&' || r == '#' || r == '$' || r == '~':
		return nil, fmt.Errorf("unsupported %q", string(r))
	}
	return nil, fmt.Errorf("unexpected %q", string(r))
}

func (p *parser) command() (*node, error) {
	p.pos++ // backslash
	start := p.pos
	for p.pos < len(p.src) && unicode.IsLetter(p.src[p.pos]) {
		p.pos++
	}
	name := string(p.src[start:p.pos])
	if name == "" {
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("dangling backslash")
		}
		name = string(p.src[p.pos])
		p.pos++
		if v, ok := symbols[name]; ok {
			return &node{kind: "op", value: v}, nil
		}
		if name == "," || name == ";" || name == " " || name == "!" || name == ":" {
			return &node{kind: "space"}, nil
		}
		return nil, fmt.Errorf("unknown command \\%s", name)
	}
	switch name {
	case "frac", "dfrac", "tfrac":
		a, err := p.argument()
		if err != nil {
			return nil, fmt.Errorf("\\frac: %w", err)
		}
		b, err := p.argument()
		if err != nil {
			return nil, fmt.Errorf("\\frac: %w", err)
		}
		return &node{kind: "frac", children: []*node{a, b}}, nil
	case "sqrt":
		a, err := p.argument()
		if err != nil {
			return nil, fmt.Errorf("\\sqrt: %w", err)
		}
		return &node{kind: "sqrt", children: []*node{a}}, nil
	case "text", "mathrm", "operatorname", "mathbf", "textbf":
		return p.textArg(name)
	case "htmlData":
		return p.htmlData()
	case "left":
		return p.fenced()
	case "quad", "qquad":
		return &node{kind: "space"}, nil
	}
	if v, ok := symbols[name]; ok {
		return &node{kind: "op", value: v}, nil
	}
	if v, ok := bigOps[name]; ok {
		return &node{kind: "bigop", value: v}, nil
	}
	if v, ok := names[name]; ok {
		return &node{kind: "text", value: v}, nil
	}
	return nil, fmt.Errorf("unknown command \\%s", name)
}

func (p *parser) textArg(cmd string) (*node, error) {
	p.skipSpace()
	if r, ok := p.peek(); !ok || r != '{' {
		return nil, fmt.Errorf("\\%s needs a braced argument", cmd)
	}
	p.pos++
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] != '}' {
		if p.src[p.pos] == '{' || p.src[p.pos] == '\\' {
			return nil, fmt.Errorf("\\%s takes plain text only", cmd)
		}
		p.pos++
	}
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unclosed \\%s", cmd)
	}
	text := string(p.src[start:p.pos])
	p.pos++
	return &node{kind: "text", value: text}, nil
}

// htmlData is KaTeX's \htmlData{key=value}{body}: it only tags a subexpression
// (the client uses it to tie a symbol to its variable), so here it becomes an
// mrow carrying the same data attribute.
func (p *parser) htmlData() (*node, error) {
	attrs, err := p.textArg("htmlData")
	if err != nil {
		return nil, err
	}
	key, value, ok := strings.Cut(attrs.value, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return nil, fmt.Errorf("\\htmlData needs key=value")
	}
	for _, r := range key {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return nil, fmt.Errorf("\\htmlData key %q is not allowed", key)
		}
	}
	body, err := p.argument()
	if err != nil {
		return nil, fmt.Errorf("\\htmlData: %w", err)
	}
	return &node{kind: "data", value: key + "=" + strings.TrimSpace(value), children: []*node{body}}, nil
}

func (p *parser) fenced() (*node, error) {
	open, err := p.delimiter()
	if err != nil {
		return nil, err
	}
	body, err := p.sequence(0)
	if err != nil {
		return nil, err
	}
	if !p.hasPrefix(`\right`) {
		return nil, fmt.Errorf("\\left without \\right")
	}
	p.pos += len(`\right`)
	closeDelim, err := p.delimiter()
	if err != nil {
		return nil, err
	}
	return &node{kind: "fenced", value: open + closeDelim, children: []*node{body}}, nil
}

func (p *parser) delimiter() (string, error) {
	p.skipSpace()
	r, ok := p.peek()
	if !ok {
		return "", fmt.Errorf("missing delimiter")
	}
	p.pos++
	if r == '.' {
		return "", nil
	}
	if r == '\\' && p.pos < len(p.src) && (p.src[p.pos] == '{' || p.src[p.pos] == '}' || p.src[p.pos] == '|') {
		d := string(p.src[p.pos])
		p.pos++
		return d, nil
	}
	if strings.ContainsRune("()[]|", r) {
		return string(r), nil
	}
	return "", fmt.Errorf("bad delimiter %q", string(r))
}

func esc(s string) string { return html.EscapeString(s) }

func (n *node) mathml(b *strings.Builder) {
	switch n.kind {
	case "seq":
		for _, c := range n.children {
			c.mathml(b)
		}
	case "num":
		b.WriteString("<mn>" + esc(n.value) + "</mn>")
	case "ident":
		b.WriteString("<mi>" + esc(n.value) + "</mi>")
	case "op":
		b.WriteString("<mo>" + esc(n.value) + "</mo>")
	case "bigop":
		b.WriteString("<mo>" + esc(n.value) + "</mo>")
	case "text":
		b.WriteString("<mtext>" + esc(n.value) + "</mtext>")
	case "space":
		b.WriteString(`<mspace width="0.2em"></mspace>`)
	case "frac":
		b.WriteString("<mfrac><mrow>")
		n.children[0].mathml(b)
		b.WriteString("</mrow><mrow>")
		n.children[1].mathml(b)
		b.WriteString("</mrow></mfrac>")
	case "sqrt":
		b.WriteString("<msqrt>")
		n.children[0].mathml(b)
		b.WriteString("</msqrt>")
	case "sup", "sub", "subsup":
		tag := map[string]string{"sup": "msup", "sub": "msub", "subsup": "msubsup"}[n.kind]
		if n.children[0].kind == "bigop" && n.kind != "sup" {
			tag = map[string]string{"sub": "munder", "subsup": "munderover"}[n.kind]
		}
		b.WriteString("<" + tag + "><mrow>")
		n.children[0].mathml(b)
		b.WriteString("</mrow>")
		for _, c := range n.children[1:] {
			b.WriteString("<mrow>")
			c.mathml(b)
			b.WriteString("</mrow>")
		}
		b.WriteString("</" + tag + ">")
	case "data":
		key, value, _ := strings.Cut(n.value, "=")
		b.WriteString(`<mrow data-` + esc(key) + `="` + esc(value) + `">`)
		n.children[0].mathml(b)
		b.WriteString("</mrow>")
	case "fenced":
		open, closeD := fence(n.value)
		b.WriteString("<mrow>")
		if open != "" {
			b.WriteString(`<mo fence="true">` + esc(open) + "</mo>")
		}
		n.children[0].mathml(b)
		if closeD != "" {
			b.WriteString(`<mo fence="true">` + esc(closeD) + "</mo>")
		}
		b.WriteString("</mrow>")
	}
}

// fence splits the stored open+close pair (each is 0 or 1 rune).
func fence(v string) (string, string) {
	rs := []rune(v)
	switch len(rs) {
	case 2:
		return string(rs[0]), string(rs[1])
	case 1:
		return string(rs[0]), ""
	}
	return "", ""
}

var spoken = map[string]string{
	"+": " plus ", "-": " minus ", "=": " equals ", "/": " divided by ", "*": " times ", "×": " times ", "·": " times ",
	"÷": " divided by ", "<": " less than ", ">": " greater than ", "≤": " at most ", "≥": " at least ", "≠": " not equal to ",
	"±": " plus or minus ", "%": " percent ", "∑": " sum of ", "∏": " product of ",
}

func (n *node) text(b *strings.Builder) {
	switch n.kind {
	case "seq":
		for _, c := range n.children {
			c.text(b)
		}
	case "num", "ident", "text":
		b.WriteString(" " + n.value + " ")
	case "op", "bigop":
		if s, ok := spoken[n.value]; ok {
			b.WriteString(s)
		} else {
			b.WriteString(" " + n.value + " ")
		}
	case "data":
		n.children[0].text(b)
	case "frac":
		b.WriteString(" (")
		n.children[0].text(b)
		b.WriteString(") divided by (")
		n.children[1].text(b)
		b.WriteString(") ")
	case "sqrt":
		b.WriteString(" square root of (")
		n.children[0].text(b)
		b.WriteString(") ")
	case "sup":
		n.children[0].text(b)
		b.WriteString(" to the power of (")
		n.children[1].text(b)
		b.WriteString(") ")
	case "sub":
		n.children[0].text(b)
		b.WriteString(" sub (")
		n.children[1].text(b)
		b.WriteString(") ")
	case "subsup":
		n.children[0].text(b)
		b.WriteString(" from (")
		n.children[1].text(b)
		b.WriteString(") to (")
		n.children[2].text(b)
		b.WriteString(") ")
	case "fenced":
		open, closeD := fence(n.value)
		b.WriteString(" " + open + " ")
		n.children[0].text(b)
		b.WriteString(" " + closeD + " ")
	}
}
