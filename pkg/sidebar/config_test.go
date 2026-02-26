package sidebar

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/types"
)

func TestDefaultTabGroupBuilderSkipsEmptyGroups(t *testing.T) {
	items := []types.NavigationItem{
		{Name: "CRM", Href: "/crm", Children: nil},
		{Name: "EDO", Href: "/edo"},
	}

	groups := DefaultTabGroupBuilder(items, nil)
	if len(groups.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups.Groups))
	}
	if groups.Groups[0].Value != "core" {
		t.Fatalf("expected group value core, got %q", groups.Groups[0].Value)
	}
	if groups.DefaultValue != "core" {
		t.Fatalf("expected default value core, got %q", groups.DefaultValue)
	}
}

func TestDefaultTabGroupBuilderUsesCRMAsDefaultWhenCoreEmpty(t *testing.T) {
	items := []types.NavigationItem{
		{
			Name: "CRM",
			Href: "/crm",
			Children: []types.NavigationItem{
				{Name: "Chats", Href: "/crm/chats"},
			},
		},
	}

	groups := DefaultTabGroupBuilder(items, nil)
	if len(groups.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups.Groups))
	}
	if groups.Groups[0].Value != "crm" {
		t.Fatalf("expected group value crm, got %q", groups.Groups[0].Value)
	}
	if groups.DefaultValue != "crm" {
		t.Fatalf("expected default value crm, got %q", groups.DefaultValue)
	}
}

func TestDefaultTabGroupBuilderReturnsEmptyForNoItems(t *testing.T) {
	groups := DefaultTabGroupBuilder(nil, nil)
	if len(groups.Groups) != 0 {
		t.Fatalf("expected no groups, got %d", len(groups.Groups))
	}
	if groups.DefaultValue != "" {
		t.Fatalf("expected empty default value, got %q", groups.DefaultValue)
	}
}
