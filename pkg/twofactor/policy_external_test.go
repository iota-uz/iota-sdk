package twofactor

import "testing"

func TestAuthMethodExternalValue(t *testing.T) {
	if AuthMethodExternal != AuthMethod("external") {
		t.Fatalf("unexpected external auth method value: %s", AuthMethodExternal)
	}
}
