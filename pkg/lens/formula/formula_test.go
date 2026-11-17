package formula

import (
	"strings"
	"testing"
)

func TestMarkupAndAlt(t *testing.T) {
	s := Spec{Latex: `\mathrm{ROE} = \frac{N}{E_{avg}} \cdot 100\%`}
	m, err := s.Markup()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<mfrac>", "<msub>", "<mtext>ROE</mtext>", "<mo>·</mo>"} {
		if !strings.Contains(m, want) {
			t.Errorf("markup missing %s: %s", want, m)
		}
	}
	alt, err := s.TextAlternative()
	if err != nil || !strings.Contains(alt, "divided by") {
		t.Fatalf("alt = %q, %v", alt, err)
	}
	s.Alt = "Return on equity"
	if alt, _ := s.TextAlternative(); alt != "Return on equity" {
		t.Fatalf("author alt ignored: %q", alt)
	}
}

func TestInvalid(t *testing.T) {
	for _, src := range []string{`\frac{a}`, `\unknown`, `{a`, `a}`, `a^`, `\left( a`, `\text{\x}`} {
		if err := (Spec{Latex: src}).Validate(); err == nil {
			t.Errorf("%q should be invalid", src)
		}
	}
}

func TestValidateVariables(t *testing.T) {
	s := Spec{Latex: `a+b`, Variables: []Variable{{Key: "a"}, {Key: "a"}}}
	if s.Validate() == nil {
		t.Fatal("duplicate key accepted")
	}
	s.Variables = []Variable{{Key: "a", Ref: &Ref{Kind: "bogus", Name: "x"}}}
	if s.Validate() == nil {
		t.Fatal("bad ref accepted")
	}
	s.Variables = []Variable{{Key: "a", Ref: &Ref{Kind: RefField, Name: "x"}}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.Variables = []Variable{{Key: "a", Ref: &Ref{Kind: RefStage, Name: "x", Availability: "x_state"}}}
	if s.Validate() == nil {
		t.Fatal("availability column on a stage ref accepted")
	}
}

func TestHTMLData(t *testing.T) {
	s := Spec{Latex: `\htmlData{var=N}{N} / 2`}
	m, err := s.Markup()
	if err != nil || !strings.Contains(m, `<mrow data-var="N">`) {
		t.Fatalf("%v %s", err, m)
	}
	if (Spec{Latex: `\htmlData{bad key=x}{N}`}).Validate() == nil {
		t.Fatal("bad key accepted")
	}
}

func TestParserEdgeCases(t *testing.T) {
	for _, src := range []string{"a ", "{a }", "a ^2", `\frac12`, `\sqrt2`, `\left.x\right|`} {
		if err := (Spec{Latex: src}).Validate(); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
	if (Spec{Latex: `\sqrt[3]{x}`}).Validate() == nil {
		t.Error("indexed root must be rejected")
	}
	m, _ := Spec{Latex: `\frac12`}.Markup()
	if !strings.Contains(m, "<mn>1</mn></mrow><mrow><mn>2</mn>") {
		t.Errorf("frac12: %s", m)
	}
	m, _ = Spec{Latex: `\left.x\right|`}.Markup()
	if strings.Index(m, "<mi>x</mi>") > strings.Index(m, `fence="true">|`) {
		t.Errorf("closing delimiter before body: %s", m)
	}
	alt, _ := Spec{Latex: `x_i^2`}.TextAlternative()
	if strings.Contains(alt, "from") {
		t.Errorf("alt: %s", alt)
	}
}
