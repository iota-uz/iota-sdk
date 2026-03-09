package system_info

import (
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
)

func TestMetricsPartialGitCommitLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metrics  *viewmodels.SystemInfoMetrics
		wantHref string
		wantLink bool
		wantText string
	}{
		{
			name: "renders eai commit link when url is provided",
			metrics: &viewmodels.SystemInfoMetrics{
				GitCommit:    "a460c4a9",
				GitCommitURL: "https://github.com/iota-uz/eai/commit/a460c4a9",
			},
			wantHref: "https://github.com/iota-uz/eai/commit/a460c4a9",
			wantLink: true,
			wantText: "a460c4a9",
		},
		{
			name: "renders plain text when commit url is missing",
			metrics: &viewmodels.SystemInfoMetrics{
				GitCommit: "a460c4a9",
			},
			wantLink: false,
			wantText: "a460c4a9",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &viewmodels.SystemInfoViewModel{
				Metrics: tt.metrics,
			}

			var builder strings.Builder
			err := MetricsPartial(vm).Render(context.Background(), &builder)
			if err != nil {
				t.Fatalf("render metrics partial: %v", err)
			}

			doc, err := goquery.NewDocumentFromReader(strings.NewReader(builder.String()))
			if err != nil {
				t.Fatalf("parse html: %v", err)
			}

			buildCard := doc.Find("span").FilterFunction(func(_ int, sel *goquery.Selection) bool {
				return strings.TrimSpace(sel.Text()) == "Git Commit"
			}).First().Parent()
			if buildCard.Length() == 0 {
				t.Fatal("git commit row not found")
			}

			link := buildCard.Find("a")
			if tt.wantLink {
				if link.Length() != 1 {
					t.Fatalf("expected one link, got %d", link.Length())
				}

				href, ok := link.Attr("href")
				if !ok {
					t.Fatal("expected href attribute")
				}
				if href != tt.wantHref {
					t.Fatalf("href = %q, want %q", href, tt.wantHref)
				}

				if strings.TrimSpace(link.Text()) != tt.wantText {
					t.Fatalf("link text = %q, want %q", strings.TrimSpace(link.Text()), tt.wantText)
				}

				return
			}

			if link.Length() != 0 {
				t.Fatalf("expected no link, got %d", link.Length())
			}

			text := strings.TrimSpace(buildCard.Find("span.font-mono").Last().Text())
			if text != tt.wantText {
				t.Fatalf("text = %q, want %q", text, tt.wantText)
			}
		})
	}
}
