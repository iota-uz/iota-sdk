package middleware

import (
	"testing"

	sidebarui "github.com/iota-uz/iota-sdk/components/sidebar"
)

func TestNormalizeTabGroupsDropsEmptyGroupsAndKeepsOrder(t *testing.T) {
	collection := sidebarui.TabGroupCollection{
		Groups: []sidebarui.TabGroup{
			{Label: "Empty", Value: "empty", Items: nil},
			{Label: "ERP", Value: "erp", Items: []sidebarui.Item{sidebarui.NewLink("/a", "A", nil)}},
			{Label: "CRM", Value: "crm", Items: []sidebarui.Item{sidebarui.NewLink("/b", "B", nil)}},
		},
		DefaultValue: "erp",
	}

	normalized := normalizeTabGroups(collection)
	if len(normalized.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(normalized.Groups))
	}
	if normalized.Groups[0].Value != "erp" {
		t.Fatalf("expected first group to be erp, got %q", normalized.Groups[0].Value)
	}
	if normalized.Groups[1].Value != "crm" {
		t.Fatalf("expected second group to be crm, got %q", normalized.Groups[1].Value)
	}
	if normalized.DefaultValue != "erp" {
		t.Fatalf("expected default value erp, got %q", normalized.DefaultValue)
	}
}

func TestNormalizeTabGroupsRewritesMissingDefault(t *testing.T) {
	collection := sidebarui.TabGroupCollection{
		Groups: []sidebarui.TabGroup{
			{Label: "ERP", Value: "erp", Items: []sidebarui.Item{sidebarui.NewLink("/a", "A", nil)}},
		},
		DefaultValue: "edo",
	}

	normalized := normalizeTabGroups(collection)
	if normalized.DefaultValue != "erp" {
		t.Fatalf("expected default value to fall back to erp, got %q", normalized.DefaultValue)
	}
}

func TestNormalizeTabGroupsHandlesAllEmpty(t *testing.T) {
	collection := sidebarui.TabGroupCollection{
		Groups: []sidebarui.TabGroup{
			{Label: "ERP", Value: "erp", Items: nil},
			{Label: "CRM", Value: "crm", Items: nil},
		},
		DefaultValue: "erp",
	}

	normalized := normalizeTabGroups(collection)
	if len(normalized.Groups) != 0 {
		t.Fatalf("expected no groups, got %d", len(normalized.Groups))
	}
	if normalized.DefaultValue != "" {
		t.Fatalf("expected empty default value, got %q", normalized.DefaultValue)
	}
}
