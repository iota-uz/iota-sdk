package actions

import (
	"context"
	"fmt"
	"io"
	
	"github.com/a-h/templ"
)

// RenderAction creates a templ.Component from ActionProps
func RenderAction(props ActionProps) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		// Build button classes
		classes := "btn"
		
		// Add variant class
		switch props.Variant {
		case 1: // VariantPrimary
			classes += " btn-primary"
		case 2: // VariantSecondary
			classes += " btn-secondary"
		case 5: // VariantDanger
			classes += " btn-danger"
		default:
			classes += " btn-secondary"
		}
		
		// Add size class
		switch props.Size {
		case 1: // SizeNormal
			classes += " btn-normal"
		case 3: // SizeSM
			classes += " btn-sm"
		default:
			classes += " btn-normal"
		}
		
		// Add custom classes from attrs
		if customClass, ok := props.Attrs["class"]; ok {
			classes += " " + customClass.(string)
		}
		
		// Start building the HTML
		var html string
		if props.Href != "" {
			html = fmt.Sprintf(`<a href="%s" class="%s"`, props.Href, classes)
		} else {
			html = fmt.Sprintf(`<button type="button" class="%s"`, classes)
		}
		
		// Add other attributes
		for key, value := range props.Attrs {
			if key != "class" {
				html += fmt.Sprintf(` %s="%v"`, key, value)
			}
		}
		
		html += ">"
		
		// Write opening tag
		if _, err := io.WriteString(w, html); err != nil {
			return err
		}
		
		// Render icon if present
		if props.Icon != nil {
			if err := props.Icon.Render(ctx, w); err != nil {
				return err
			}
		}
		
		// Render label
		if props.Label != "" {
			if _, err := io.WriteString(w, props.Label); err != nil {
				return err
			}
		}
		
		// Close tag
		if props.Href != "" {
			if _, err := io.WriteString(w, "</a>"); err != nil {
				return err
			}
		} else {
			if _, err := io.WriteString(w, "</button>"); err != nil {
				return err
			}
		}
		
		return nil
	})
}

// RenderRowActions creates a templ.Component that renders multiple actions in a row
func RenderRowActions(actions ...ActionProps) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		// Open container div
		if _, err := io.WriteString(w, `<div class="flex gap-2">`); err != nil {
			return err
		}
		
		// Render each action
		for _, action := range actions {
			if err := RenderAction(action).Render(ctx, w); err != nil {
				return err
			}
		}
		
		// Close container div
		if _, err := io.WriteString(w, "</div>"); err != nil {
			return err
		}
		
		return nil
	})
}