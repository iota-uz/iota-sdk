package phone_test

import (
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/components/base/phone"
)

func TestLink(t *testing.T) {
	tests := []struct {
		name           string
		props          phone.Props
		expectedHref   string
		expectedText   string
		expectedClasses []string
		hasIcon        bool
	}{
		{
			name: "Basic Uzbekistan phone link",
			props: phone.Props{
				Phone:    "+998993303030",
				Class:    "custom-class",
				Style:    phone.StyleParentheses,
				ShowIcon: false,
			},
			expectedHref:   "tel:998993303030",
			expectedText:   "+998(99)330-30-30",
			expectedClasses: []string{"custom-class", "text-blue-600"},
			hasIcon:        false,
		},
		{
			name: "US phone link with icon",
			props: phone.Props{
				Phone:    "+14155551234",
				Class:    "font-medium",
				Style:    phone.StyleParentheses,
				ShowIcon: true,
			},
			expectedHref:   "tel:14155551234",
			expectedText:   "+1(415)555-1234",
			expectedClasses: []string{"font-medium", "text-blue-600"},
			hasIcon:        true,
		},
		{
			name: "Phone with dashes style",
			props: phone.Props{
				Phone:    "+998993303030",
				Style:    phone.StyleDashes,
				ShowIcon: false,
			},
			expectedHref:   "tel:998993303030",
			expectedText:   "+998-99-330-30-30",
			expectedClasses: []string{"text-blue-600"},
			hasIcon:        false,
		},
		{
			name: "Phone with spaces style",
			props: phone.Props{
				Phone:    "+998993303030",
				Style:    phone.StyleSpaces,
				ShowIcon: false,
			},
			expectedHref:   "tel:998993303030",
			expectedText:   "+998 99 330 30 30",
			expectedClasses: []string{"text-blue-600"},
			hasIcon:        false,
		},
		{
			name: "Empty phone number",
			props: phone.Props{
				Phone:    "",
				Style:    phone.StyleParentheses,
				ShowIcon: false,
			},
			expectedHref:   "",
			expectedText:   "",
			expectedClasses: nil,
			hasIcon:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			
			component := phone.Link(tt.props)
			
			var builder strings.Builder
			err := component.Render(ctx, &builder)
			require.NoError(t, err)
			
			html := builder.String()
			
			if tt.props.Phone == "" {
				assert.Empty(t, html)
				return
			}
			
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
			require.NoError(t, err)
			
			link := doc.Find("a")
			assert.Equal(t, 1, link.Length(), "Should have exactly one link")
			
			href, exists := link.Attr("href")
			assert.True(t, exists, "Link should have href attribute")
			assert.Equal(t, tt.expectedHref, href)
			
			text := strings.TrimSpace(link.Text())
			assert.Equal(t, tt.expectedText, text)
			
			class, exists := link.Attr("class")
			assert.True(t, exists, "Link should have class attribute")
			for _, expectedClass := range tt.expectedClasses {
				assert.Contains(t, class, expectedClass, "Should contain expected class")
			}
			
			if tt.hasIcon {
				svg := link.Find("svg")
				assert.Equal(t, 1, svg.Length(), "Should have exactly one icon")
			} else {
				svg := link.Find("svg")
				assert.Equal(t, 0, svg.Length(), "Should not have any icons")
			}
		})
	}
}

func TestText(t *testing.T) {
	tests := []struct {
		name           string
		props          phone.Props
		expectedText   string
		expectedClasses []string
		hasIcon        bool
	}{
		{
			name: "Basic Uzbekistan phone text",
			props: phone.Props{
				Phone:    "+998993303030",
				Class:    "text-gray-600",
				Style:    phone.StyleParentheses,
				ShowIcon: false,
			},
			expectedText:   "+998(99)330-30-30",
			expectedClasses: []string{"text-gray-600"},
			hasIcon:        false,
		},
		{
			name: "US phone text with icon",
			props: phone.Props{
				Phone:    "+14155551234",
				Class:    "font-bold",
				Style:    phone.StyleParentheses,
				ShowIcon: true,
			},
			expectedText:   "+1(415)555-1234",
			expectedClasses: []string{"font-bold"},
			hasIcon:        true,
		},
		{
			name: "Phone with dashes style",
			props: phone.Props{
				Phone:    "+998993303030",
				Style:    phone.StyleDashes,
				ShowIcon: false,
			},
			expectedText:   "+998-99-330-30-30",
			expectedClasses: []string{"inline-flex"},
			hasIcon:        false,
		},
		{
			name: "Empty phone number",
			props: phone.Props{
				Phone:    "",
				Style:    phone.StyleParentheses,
				ShowIcon: false,
			},
			expectedText:   "",
			expectedClasses: nil,
			hasIcon:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			
			component := phone.Text(tt.props)
			
			var builder strings.Builder
			err := component.Render(ctx, &builder)
			require.NoError(t, err)
			
			html := builder.String()
			
			if tt.props.Phone == "" {
				assert.Empty(t, html)
				return
			}
			
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
			require.NoError(t, err)
			
			span := doc.Find("span")
			assert.Equal(t, 1, span.Length(), "Should have exactly one span")
			
			text := strings.TrimSpace(span.Text())
			assert.Equal(t, tt.expectedText, text)
			
			class, exists := span.Attr("class")
			assert.True(t, exists, "Span should have class attribute")
			for _, expectedClass := range tt.expectedClasses {
				assert.Contains(t, class, expectedClass, "Should contain expected class")
			}
			
			if tt.hasIcon {
				svg := span.Find("svg")
				assert.Equal(t, 1, svg.Length(), "Should have exactly one icon")
			} else {
				svg := span.Find("svg")
				assert.Equal(t, 0, svg.Length(), "Should not have any icons")
			}
		})
	}
}

func TestFormatPhoneDisplay(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		style    phone.DisplayStyle
		expected string
	}{
		{
			name:     "Uzbekistan parentheses",
			phone:    "+998993303030",
			style:    phone.StyleParentheses,
			expected: "+998(99)330-30-30",
		},
		{
			name:     "Uzbekistan dashes",
			phone:    "+998993303030",
			style:    phone.StyleDashes,
			expected: "+998-99-330-30-30",
		},
		{
			name:     "Uzbekistan spaces",
			phone:    "+998993303030",
			style:    phone.StyleSpaces,
			expected: "+998 99 330 30 30",
		},
		{
			name:     "US parentheses",
			phone:    "+14155551234",
			style:    phone.StyleParentheses,
			expected: "+1(415)555-1234",
		},
		{
			name:     "US dashes",
			phone:    "+14155551234",
			style:    phone.StyleDashes,
			expected: "+1-415-555-1234",
		},
		{
			name:     "Empty phone",
			phone:    "",
			style:    phone.StyleParentheses,
			expected: "",
		},
		{
			name:     "Invalid phone returns original",
			phone:    "invalid",
			style:    phone.StyleParentheses,
			expected: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since formatPhoneDisplay is not exported, we test it indirectly through components
			ctx := context.Background()
			
			component := phone.Text(phone.Props{
				Phone: tt.phone,
				Style: tt.style,
			})
			
			var builder strings.Builder
			err := component.Render(ctx, &builder)
			require.NoError(t, err)
			
			html := builder.String()
			
			if tt.phone == "" {
				assert.Empty(t, html)
				return
			}
			
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
			require.NoError(t, err)
			
			span := doc.Find("span")
			text := strings.TrimSpace(span.Text())
			assert.Equal(t, tt.expected, text)
		})
	}
}

func TestDisplayStyleConstants(t *testing.T) {
	tests := []struct {
		name  string
		style phone.DisplayStyle
		value int
	}{
		{
			name:  "StyleParentheses",
			style: phone.StyleParentheses,
			value: 0,
		},
		{
			name:  "StyleDashes",
			style: phone.StyleDashes,
			value: 1,
		},
		{
			name:  "StyleSpaces",
			style: phone.StyleSpaces,
			value: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.value, int(tt.style))
		})
	}
}

func TestPropsWithAttributes(t *testing.T) {
	ctx := context.Background()
	
	attrs := templ.Attributes{
		"data-testid": "phone-link",
		"title":       "Call this number",
	}
	
	props := phone.Props{
		Phone:    "+998993303030",
		Style:    phone.StyleParentheses,
		ShowIcon: false,
		Attrs:    attrs,
	}
	
	component := phone.Link(props)
	
	var builder strings.Builder
	err := component.Render(ctx, &builder)
	require.NoError(t, err)
	
	html := builder.String()
	
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	
	link := doc.Find("a")
	
	testid, exists := link.Attr("data-testid")
	assert.True(t, exists)
	assert.Equal(t, "phone-link", testid)
	
	title, exists := link.Attr("title")
	assert.True(t, exists)
	assert.Equal(t, "Call this number", title)
}