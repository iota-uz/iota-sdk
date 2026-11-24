package sms

import "testing"

func TestNormalizePhone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, cc string
		want   string
		ok     bool
	}{
		{"+998 90 123-45-67", "", "+998901234567", true},
		{"+998901234567", "998", "+998901234567", true},
		{"00998901234567", "", "+998901234567", true},
		{"(+1) 415 555-2671", "", "+14155552671", true},
		{"901234567", "998", "+998901234567", true},
		{"0901234567", "998", "+998901234567", true},
		{"998901234567", "998", "+998901234567", true},
		{"901234567", "", "", false},
		{"", "998", "", false},
		{"abc", "998", "", false},
		{"+998 90 12x", "", "", false},
		{"+12345", "", "", false},
		{"+1234567890123456", "", "", false},
		{"+0123456789", "", "", false},
		{"12+34567890", "998", "", false},
	}
	for _, tc := range cases {
		got, ok := normalizePhone(tc.in, tc.cc)
		if got != tc.want || ok != tc.ok {
			t.Errorf("normalizePhone(%q, %q) = %q, %v; want %q, %v", tc.in, tc.cc, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCountryCode(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"+998": "998", " 998 ": "998", "1": "1", "": "", "0": "", "99a": "", "9981": ""} {
		if got := countryCode(in); got != want {
			t.Errorf("countryCode(%q) = %q, want %q", in, got, want)
		}
	}
}
