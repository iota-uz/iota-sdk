package actions

import (
	"github.com/a-h/templ"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/components/base/button"
)

// ActionType defines the type of action
type ActionType string

const (
	ActionTypeCreate ActionType = "create"
	ActionTypeExport ActionType = "export"
	ActionTypeImport ActionType = "import"
	ActionTypeCustom ActionType = "custom"
)

// ActionProps defines properties for an action
type ActionProps struct {
	Type    ActionType
	Label   string
	Href    string
	Icon    templ.Component
	Variant button.Variant
	Size    button.Size
	Attrs   templ.Attributes
	OnClick string
}

// CreateAction creates a standard "Create New" action button
func CreateAction(label string, href string) ActionProps {
	return ActionProps{
		Type:    ActionTypeCreate,
		Label:   label,
		Href:    href,
		Icon:    icons.PlusCircle(icons.Props{Size: "18"}),
		Variant: button.VariantPrimary,
		Size:    button.SizeNormal,
	}
}

// ExportAction creates a standard "Export" action button
func ExportAction(label string, href string) ActionProps {
	return ActionProps{
		Type:    ActionTypeExport,
		Label:   label,
		Href:    href,
		Icon:    icons.Download(icons.Props{Size: "18"}),
		Variant: button.VariantSecondary,
		Size:    button.SizeNormal,
	}
}

// ImportAction creates a standard "Import" action button
func ImportAction(label string, href string) ActionProps {
	return ActionProps{
		Type:    ActionTypeImport,
		Label:   label,
		Href:    href,
		Icon:    icons.Upload(icons.Props{Size: "18"}),
		Variant: button.VariantSecondary,
		Size:    button.SizeNormal,
	}
}

// CustomAction creates a custom action button
func CustomAction(label string, icon templ.Component, opts ...ActionOption) ActionProps {
	action := ActionProps{
		Type:    ActionTypeCustom,
		Label:   label,
		Icon:    icon,
		Variant: button.VariantSecondary,
		Size:    button.SizeNormal,
	}

	for _, opt := range opts {
		opt(&action)
	}

	return action
}

// ActionOption is a function that modifies ActionProps
type ActionOption func(*ActionProps)

// WithHref sets the href for the action
func WithHref(href string) ActionOption {
	return func(a *ActionProps) {
		a.Href = href
	}
}

// WithOnClick sets the onclick handler for the action
func WithOnClick(onClick string) ActionOption {
	return func(a *ActionProps) {
		a.OnClick = onClick
	}
}

// WithVariant sets the button variant
func WithVariant(variant button.Variant) ActionOption {
	return func(a *ActionProps) {
		a.Variant = variant
	}
}

// WithSize sets the button size
func WithSize(size button.Size) ActionOption {
	return func(a *ActionProps) {
		a.Size = size
	}
}

// WithAttrs sets additional attributes
func WithAttrs(attrs templ.Attributes) ActionOption {
	return func(a *ActionProps) {
		a.Attrs = attrs
	}
}

// EditAction creates a standard "Edit" action button
func EditAction(href string) ActionProps {
	return ActionProps{
		Type:    ActionTypeCustom,
		Label:   "", // No label for icon-only button
		Href:    href,
		Icon:    icons.PencilSimple(icons.Props{Size: "20"}),
		Variant: button.VariantSecondary,
		Size:    button.SizeSM,
		Attrs: templ.Attributes{
			"class": "btn-fixed",
		},
	}
}

// DeleteAction creates a standard "Delete" action button
func DeleteAction(id string) ActionProps {
	return ActionProps{
		Type:    ActionTypeCustom,
		Label:   "", // No label for icon-only button
		Icon:    icons.Trash(icons.Props{Size: "20"}),
		Variant: button.VariantDanger,
		Size:    button.SizeSM,
		Attrs: templ.Attributes{
			"class":      "btn-fixed",
			"hx-delete":  id,
			"hx-confirm": "Are you sure?",
			"hx-target":  "closest tr",
			"hx-swap":    "outerHTML swap:0.5s",
		},
	}
}
