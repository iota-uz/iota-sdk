package observability

import (
	"testing"
)

func TestAttributes(t *testing.T) {
	t.Parallel()

	attrs := NewAttributes()

	// Test SetString
	attrs.SetString(AttrModelName, "gpt-5.2")
	if val, ok := attrs.GetString(AttrModelName); !ok || val != "gpt-5.2" {
		t.Errorf("GetString failed: got %v, %v", val, ok)
	}

	// Test SetInt
	attrs.SetInt(AttrGenerationTokens, 150)
	if val, ok := attrs.GetInt(AttrGenerationTokens); !ok || val != 150 {
		t.Errorf("GetInt failed: got %v, %v", val, ok)
	}

	// Test SetInt64
	attrs.SetInt64(AttrGenerationLatencyMs, 500)
	if val, ok := attrs.GetInt64(AttrGenerationLatencyMs); !ok || val != 500 {
		t.Errorf("GetInt64 failed: got %v, %v", val, ok)
	}

	// Test SetFloat64
	attrs.SetFloat64(AttrCostUSD, 0.05)
	if val, ok := attrs.GetFloat64(AttrCostUSD); !ok || val != 0.05 {
		t.Errorf("GetFloat64 failed: got %v, %v", val, ok)
	}

	// Test SetBool
	attrs.SetBool(AttrContextOverflow, true)
	if val, ok := attrs.GetBool(AttrContextOverflow); !ok || val != true {
		t.Errorf("GetBool failed: got %v, %v", val, ok)
	}

	// Test Merge
	other := NewAttributes().SetString(AttrModelProvider, "openai")
	attrs.Merge(other)

	if val, ok := attrs.GetString(AttrModelProvider); !ok || val != "openai" {
		t.Errorf("Merge failed: got %v, %v", val, ok)
	}

	// Test Copy
	attrsCopy := attrs.Copy()
	if len(attrsCopy) != len(attrs) {
		t.Errorf("Copy failed: expected %d attributes, got %d", len(attrs), len(attrsCopy))
	}

	// Modify copy should not affect original
	attrsCopy.SetString(AttrModelName, "claude-sonnet-4-6")
	if val, _ := attrs.GetString(AttrModelName); val != "gpt-5.2" {
		t.Errorf("Copy is not independent: original modified")
	}
}
