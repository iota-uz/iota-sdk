package filterq_test

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/filterq"
)

func testSchema() filterq.Schema {
	return filterq.Schema{Fields: []filterq.Field{
		{Key: "status", Type: filterq.FieldTypeReference},
		{Key: "agency", Type: filterq.FieldTypeReference},
		{Key: "issue_at", Type: filterq.FieldTypeDate},
		{Key: "end_at", Type: filterq.FieldTypeDate},
		{Key: "premium", Type: filterq.FieldTypeNumber},
		{Key: "reissued", Type: filterq.FieldTypeBool},
		{Key: "restricted", Type: filterq.FieldTypeReference, Operators: []filterq.Operator{filterq.OpIs}},
	}}
}

func TestEncodeCondition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cond filterq.Condition
		want string
	}{
		{
			name: "reference multi value",
			cond: filterq.Condition{Field: "status", Op: filterq.OpIs, Values: []string{"1", "2", "15"}},
			want: "status:is:1,2,15",
		},
		{
			name: "between dates",
			cond: filterq.Condition{Field: "issue_at", Op: filterq.OpBetween, Values: []string{"2026-06-01", "2026-06-30"}},
			want: "issue_at:between:2026-06-01,2026-06-30",
		},
		{
			name: "symbolic preset keeps colon",
			cond: filterq.Condition{Field: "end_at", Op: filterq.OpBetween, Values: []string{"preset:next_30d"}},
			want: "end_at:between:preset:next_30d",
		},
		{
			name: "escapes comma and percent",
			cond: filterq.Condition{Field: "agency", Op: filterq.OpIs, Values: []string{"a,b", "50%"}},
			want: "agency:is:a%2Cb,50%25",
		},
		{
			name: "bool flag",
			cond: filterq.Condition{Field: "reissued", Op: filterq.OpIs, Values: []string{"true"}},
			want: "reissued:is:true",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := filterq.EncodeCondition(tc.cond); got != tc.want {
				t.Errorf("EncodeCondition() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	fs := filterq.FilterSet{
		{Field: "status", Op: filterq.OpIs, Values: []string{"1", "2"}},
		{Field: "agency", Op: filterq.OpIsNot, Values: []string{"0196f4aa-1111-7000-8000-000000000001"}},
		{Field: "issue_at", Op: filterq.OpBetween, Values: []string{"preset:this_year"}},
		{Field: "end_at", Op: filterq.OpBefore, Values: []string{"2026-12-31"}},
		{Field: "premium", Op: filterq.OpBetween, Values: []string{"500000", "1000000"}},
		{Field: "reissued", Op: filterq.OpIs, Values: []string{"true"}},
		{Field: "agency", Op: filterq.OpIs, Values: []string{"weird,value", "100%legit"}},
	}
	q := filterq.Encode(fs)
	got := filterq.Decode(q, testSchema())
	if !reflect.DeepEqual(got, fs) {
		t.Errorf("round trip mismatch:\n got %#v\nwant %#v", got, fs)
	}

	// Determinism: encoding the decoded set reproduces the identical query.
	if again := filterq.Encode(got); !reflect.DeepEqual(again, q) {
		t.Errorf("re-encode mismatch:\n got %#v\nwant %#v", again, q)
	}
}

func TestDecodeDropsInvalid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  []string
		want filterq.FilterSet
	}{
		{name: "unknown field", raw: []string{"nope:is:1"}, want: nil},
		{name: "unknown operator", raw: []string{"status:like:1"}, want: nil},
		{name: "operator not allowed for type", raw: []string{"status:between:1,2"}, want: nil},
		{name: "operator not in restricted list", raw: []string{"restricted:isnot:1"}, want: nil},
		{name: "between needs two dates", raw: []string{"issue_at:between:2026-06-01"}, want: nil},
		{name: "before needs one date", raw: []string{"issue_at:before:2026-06-01,2026-06-02"}, want: nil},
		{name: "malformed date", raw: []string{"issue_at:on:01.06.2026"}, want: nil},
		{name: "unknown preset", raw: []string{"issue_at:between:preset:someday"}, want: nil},
		{name: "preset only valid on date between", raw: []string{"issue_at:on:preset:this_year"}, want: nil},
		{name: "malformed number", raw: []string{"premium:gt:1e--3"}, want: nil},
		{name: "bool must be true or false", raw: []string{"reissued:is:yes"}, want: nil},
		{name: "empty values", raw: []string{"status:is:"}, want: nil},
		{name: "no colons", raw: []string{"garbage"}, want: nil},
		{name: "empty field", raw: []string{":is:1"}, want: nil},
		{
			name: "invalid dropped, valid kept",
			raw:  []string{"nope:is:1", "status:is:1"},
			want: filterq.FilterSet{{Field: "status", Op: filterq.OpIs, Values: []string{"1"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q := url.Values{filterq.ParamName: tc.raw}
			got := filterq.Decode(q, testSchema())
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Decode(%v) = %#v, want %#v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestDecodeMergesDuplicates(t *testing.T) {
	t.Parallel()
	q := url.Values{filterq.ParamName: []string{
		"status:is:1,2",
		"status:is:2,3",
		"issue_at:on:2026-06-01",
		"issue_at:on:2026-06-02", // non-set duplicate: first wins
	}}
	got := filterq.Decode(q, testSchema())
	want := filterq.FilterSet{
		{Field: "status", Op: filterq.OpIs, Values: []string{"1", "2", "3"}},
		{Field: "issue_at", Op: filterq.OpOn, Values: []string{"2026-06-01"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Decode() = %#v, want %#v", got, want)
	}
}

func TestPresence(t *testing.T) {
	t.Parallel()
	if filterq.HasPresence(url.Values{}) {
		t.Error("empty query must not have presence")
	}
	if !filterq.HasPresence(url.Values{filterq.PresenceParam: []string{"1"}}) {
		t.Error("fb=1 must report presence")
	}
}

func TestFilterSetHelpers(t *testing.T) {
	t.Parallel()
	fs := filterq.FilterSet{
		{Field: "status", Op: filterq.OpIs, Values: []string{"1"}},
		{Field: "agency", Op: filterq.OpIs, Values: []string{"x"}},
		{Field: "status", Op: filterq.OpIsNot, Values: []string{"2"}},
	}
	if got := len(fs.Field("status")); got != 2 {
		t.Errorf("Field(status) len = %d, want 2", got)
	}
	if !fs.Has("agency") || fs.Has("nope") {
		t.Error("Has() mismatch")
	}
	rest := fs.Without("status")
	if len(rest) != 1 || rest[0].Field != "agency" {
		t.Errorf("Without(status) = %#v", rest)
	}
	if fs.IsZero() || !(filterq.FilterSet{}).IsZero() {
		t.Error("IsZero() mismatch")
	}
}

func FuzzDecodeCondition(f *testing.F) {
	f.Add("status:is:1,2")
	f.Add("end_at:between:preset:next_30d")
	f.Add("agency:is:a%2Cb,50%25")
	f.Add(":::")
	f.Add("a:b")
	f.Fuzz(func(t *testing.T, raw string) {
		c, ok := filterq.DecodeCondition(raw)
		if !ok {
			return
		}
		// Re-encoding a decoded condition must be stable under decode.
		again, ok2 := filterq.DecodeCondition(filterq.EncodeCondition(c))
		if !ok2 {
			t.Fatalf("re-decode failed for %q (from %q)", filterq.EncodeCondition(c), raw)
		}
		if !reflect.DeepEqual(c, again) {
			t.Fatalf("unstable round trip: %#v vs %#v (raw %q)", c, again, raw)
		}
	})
}
