package spotlight

import "testing"

func TestAISearchCandidateDisplayTitleUsesStructuredLabelValue(t *testing.T) {
	candidate := AISearchCandidate{
		TitleLabelKey: "Spotlight.Badge.Contract",
		TitleValue:    "9944427",
	}

	got := candidate.DisplayTitle(func(key string) string {
		if key == "Spotlight.Badge.Contract" {
			return "Contract"
		}
		return ""
	})

	if got != "Contract 9944427" {
		t.Fatalf("expected structured title, got %q", got)
	}
}

func TestAISearchCandidateHasDisplayTitleAcceptsTitleValueOnly(t *testing.T) {
	candidate := AISearchCandidate{TitleValue: "0383376"}
	if !candidate.HasDisplayTitle() {
		t.Fatal("expected title_value-only candidate to be displayable")
	}
}

func TestAISearchLinkDisplayLabelPrefersLocalizedKey(t *testing.T) {
	link := AISearchLink{
		Kind:     "policy",
		Label:    "Policy",
		LabelKey: "Spotlight.AI.Link.Policy",
		URL:      "/portfolio/contracts/policy/123",
	}

	got := link.DisplayLabel(func(key string) string {
		if key == "Spotlight.AI.Link.Policy" {
			return "Полис"
		}
		return ""
	})

	if got != "Полис" {
		t.Fatalf("expected localized link label, got %q", got)
	}
}
