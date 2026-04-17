package config

import (
	"testing"
	"time"
)

// ---- test structs ----

type allKindsStruct struct {
	S       string        `default:"hello"`
	BTrue   bool          `default:"true"`
	BFalse  bool          `default:"false"`
	I       int           `default:"42"`
	I8      int8          `default:"8"`
	I16     int16         `default:"16"`
	I32     int32         `default:"32"`
	I64     int64         `default:"64"`
	U       uint          `default:"1"`
	U8      uint8         `default:"8"`
	U16     uint16        `default:"16"`
	U32     uint32        `default:"32"`
	U64     uint64        `default:"64"`
	F32     float32       `default:"3.14"`
	F64     float64       `default:"2.71"`
	D       time.Duration `default:"5m"`
	Strs    []string      `default:"a,b,c"`
}

type nestedOuter struct {
	Name  string `default:"outer"`
	Inner nestedInner
}

type nestedInner struct {
	Value string `default:"inner-default"`
}

type unsupportedStruct struct {
	M map[string]int `default:"something"`
}

// ---- tests ----

func TestApplyTagDefaults_AllKinds(t *testing.T) {
	t.Parallel()

	var s allKindsStruct
	if err := applyTagDefaults(&s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"string", s.S, "hello"},
		{"bool-true", s.BTrue, true},
		{"bool-false", s.BFalse, false},
		{"int", s.I, 42},
		{"int8", s.I8, int8(8)},
		{"int16", s.I16, int16(16)},
		{"int32", s.I32, int32(32)},
		{"int64", s.I64, int64(64)},
		{"uint", s.U, uint(1)},
		{"uint8", s.U8, uint8(8)},
		{"uint16", s.U16, uint16(16)},
		{"uint32", s.U32, uint32(32)},
		{"uint64", s.U64, uint64(64)},
		{"float32", s.F32, float32(3.14)},
		{"float64", s.F64, 2.71},
		{"duration", s.D, 5 * time.Minute},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %v (%T), want %v (%T)", tc.got, tc.got, tc.want, tc.want)
			}
		})
	}

	if len(s.Strs) != 3 || s.Strs[0] != "a" || s.Strs[1] != "b" || s.Strs[2] != "c" {
		t.Errorf("[]string: got %v, want [a b c]", s.Strs)
	}
}

func TestApplyTagDefaults_NoClobber(t *testing.T) {
	t.Parallel()

	s := allKindsStruct{
		S: "already-set",
		I: 99,
		D: 10 * time.Second,
	}
	if err := applyTagDefaults(&s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.S != "already-set" {
		t.Errorf("pre-set string was clobbered: got %q", s.S)
	}
	if s.I != 99 {
		t.Errorf("pre-set int was clobbered: got %d", s.I)
	}
	if s.D != 10*time.Second {
		t.Errorf("pre-set duration was clobbered: got %v", s.D)
	}
	// Un-set field should still get default.
	if s.BTrue != true {
		t.Errorf("unset bool should get default true, got %v", s.BTrue)
	}
}

func TestApplyTagDefaults_NestedRecursion(t *testing.T) {
	t.Parallel()

	var s nestedOuter
	if err := applyTagDefaults(&s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "outer" {
		t.Errorf("outer Name: got %q, want \"outer\"", s.Name)
	}
	if s.Inner.Value != "inner-default" {
		t.Errorf("inner Value: got %q, want \"inner-default\"", s.Inner.Value)
	}
}

func TestApplyTagDefaults_UnsupportedKindReturnsError(t *testing.T) {
	t.Parallel()

	var s unsupportedStruct
	err := applyTagDefaults(&s)
	if err == nil {
		t.Fatal("expected error for unsupported map kind, got nil")
	}
	t.Logf("error (expected): %v", err)
}

func TestApplyTagDefaults_EmptyTagNoOp(t *testing.T) {
	t.Parallel()

	type noTagStruct struct {
		X string `default:""`
		Y int
	}
	var s noTagStruct
	if err := applyTagDefaults(&s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.X != "" {
		t.Errorf("empty default tag should not set field, got %q", s.X)
	}
	if s.Y != 0 {
		t.Errorf("no default tag should leave field zero, got %d", s.Y)
	}
}
