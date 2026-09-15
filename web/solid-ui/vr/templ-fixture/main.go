package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/a-h/templ"
	base "github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/avatar"
	"github.com/iota-uz/iota-sdk/components/base/badge"
	"github.com/iota-uz/iota-sdk/components/base/button"
	"github.com/iota-uz/iota-sdk/components/base/dialog"
	baseinput "github.com/iota-uz/iota-sdk/components/base/input"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/components/base/radio"
	"github.com/iota-uz/iota-sdk/components/base/selects"
	"github.com/iota-uz/iota-sdk/components/base/tab"
	copybutton "github.com/iota-uz/iota-sdk/components/copy_button"
	usercomponents "github.com/iota-uz/iota-sdk/components/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"golang.org/x/text/language"
)

func main() {
	port := flag.Int("port", 61011, "fixture server port")
	flag.Parse()
	root, err := repositoryRoot()
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/standalone.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(root, "web/solid-ui/package-dist/standalone.css"))
	})
	mux.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir(filepath.Join(root, "web/solid-ui/package-dist/fonts")))))
	mux.Handle("/assets/fonts/", http.StripPrefix("/assets/fonts/", http.FileServer(http.Dir(filepath.Join(root, "modules/core/presentation/assets/fonts")))))
	mux.HandleFunc("/", fixture)
	if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", *port), mux); err != nil {
		panic(err)
	}
}

func repositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("go.mod not found above working directory")
		}
		directory = parent
	}
}

func fixture(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	specimen := r.URL.Query().Get("specimen")
	theme := r.URL.Query().Get("theme")
	if theme != "dark" {
		theme = "light"
	}
	bodyBackground := "#fff"
	if theme == "dark" {
		bodyBackground = "#191e27"
	}
	if specimen == "dialog" || specimen == "drawer" {
		if theme == "dark" {
			bodyBackground = "#11141a"
		} else {
			bodyBackground = "#f7f8fa"
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html><html class="%s" data-theme="%s"><head><meta charset="UTF-8"><link rel="stylesheet" href="/standalone.css"><style>body{margin:0;padding:48px;color:oklch(var(--clr-text-100));background:%s}.fixture{width:480px}</style></head><body><main class="fixture">`, templ.EscapeString(theme), templ.EscapeString(theme), bodyBackground)
	component := fixtureComponent(specimen, state)
	if component == nil {
		http.Error(w, "unknown fixture", http.StatusNotFound)
		return
	}
	if err := component.Render(context.Background(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if specimen == "copy-button" {
		_, _ = io.WriteString(w, `<script>{const icons=document.querySelectorAll('.fixture>button>span>span');icons[0]?.classList.add('opacity-100','scale-100');icons[1]?.classList.add('opacity-0','scale-50')}</script>`)
	}
	openDialog := state == "open"
	_, _ = fmt.Fprintf(w, `</main><script>if(%t){document.querySelector('dialog')?.showModal()}document.fonts.ready.then(()=>{document.documentElement.dataset.fixtureReady='true'})</script></body></html>`, openDialog)
}

func fixtureComponent(specimen, state string) templ.Component {
	disabled := state == "disabled"
	switch specimen {
	case "button":
		component := button.Primary(button.Props{
			Size: button.SizeNormal, Loading: state == "loading", Disabled: disabled,
			Attrs: templ.Attributes{"data-parity-target": "button"},
		})
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			return component.Render(templ.WithChildren(ctx, templ.Raw("Primary action")), w)
		})
	case "input":
		errorText := ""
		if state == "error" {
			errorText = "This field is required"
		}
		return baseinput.Text(&baseinput.Props{
			Label:       "Product name",
			Placeholder: "Enter a name",
			Error:       errorText,
			Attrs: templ.Attributes{
				"id":                 "templ-product-name",
				"value":              "Insurance product",
				"disabled":           disabled,
				"data-parity-target": "input",
			},
		})
	case "checkbox":
		return baseinput.Checkbox(&baseinput.CheckboxProps{
			ID:      "templ-enabled",
			Label:   "Product enabled",
			Checked: state == "checked",
			Attrs: templ.Attributes{
				"disabled":           disabled,
				"data-parity-target": "checkbox",
			},
		})
	case "select":
		component := base.Select(&base.SelectProps{
			Label: "Insurance type", Attrs: templ.Attributes{"data-parity-target": "select", "disabled": disabled},
		})
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			children := templ.Raw(`<option value="mandatory">Mandatory</option><option value="voluntary" selected>Voluntary</option>`)
			return component.Render(templ.WithChildren(ctx, children), w)
		})
	case "radio":
		component := radio.CardItem(radio.CardItemProps{
			Name: "frequency", Value: "monthly", Checked: state == "checked", Disabled: disabled,
			Attrs: templ.Attributes{"data-parity-target": "radio"},
		})
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			return component.Render(templ.WithChildren(ctx, templ.Raw("Monthly")), w)
		})
	case "switch":
		return baseinput.Switch(&baseinput.SwitchProps{
			ID: "templ-renewal", Label: "Automatic renewal", Checked: state == "checked",
			Attrs: templ.Attributes{"data-parity-target": "switch", "disabled": disabled},
		})
	case "tabs":
		component := tab.Link("#details", state != "selected")
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			return component.Render(templ.WithChildren(ctx, templ.Raw("Details")), w)
		})
	case "badge":
		component := badge.New(badge.Props{Variant: badge.VariantPink, Size: badge.SizeNormal})
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			return component.Render(templ.WithChildren(ctx, templ.Raw("pink")), w)
		})
	case "avatar":
		return avatar.Avatar(avatar.Props{Initials: "DK", Variant: avatar.Round})
	case "pagination":
		return pagination.Pagination(pagination.New("#page", 4, 7, 1))
	case "table":
		component := base.Table(base.TableProps{
			Columns: []*base.TableColumn{
				{Label: "Product", Key: "product", Sortable: true, SortDir: base.SortDirectionAsc},
				{Label: "Code", Key: "code"},
				{Label: "Status", Key: "status", Priority: 2},
			},
			Attrs: templ.Attributes{"data-parity-target": "table"},
		})
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			children := templ.Raw(`<tr><td class="p-4 border-r border-subtle last-of-type:border-r-0">Travel insurance</td><td class="p-4 border-r border-subtle last-of-type:border-r-0">TRV-2026</td><td class="p-4 border-r border-subtle last-of-type:border-r-0 max-md:hidden" data-col-priority="2"><div class="flex items-center justify-center rounded-lg text-sm font-medium border border-green bg-badge-green text-green h-8">Active</div></td></tr><tr><td class="p-4 border-r border-subtle last-of-type:border-r-0">Property insurance</td><td class="p-4 border-r border-subtle last-of-type:border-r-0">PRP-2026</td><td class="p-4 border-r border-subtle last-of-type:border-r-0 max-md:hidden" data-col-priority="2"><div class="flex items-center justify-center rounded-lg text-sm font-medium border border-yellow bg-badge-yellow text-yellow h-8">Draft</div></td></tr>`)
			return component.Render(templ.WithChildren(ctx, children), w)
		})
	case "dialog":
		return dialog.Confirmation(&dialog.Props{
			Heading: "Delete insurance product?", Text: "This action cannot be undone.",
			CancelText: "Cancel", ConfirmText: "Delete", Attrs: templ.Attributes{"data-parity-target": "dialog"},
		})
	case "drawer":
		component := dialog.StdViewDrawer(dialog.StdDrawerProps{
			Title: "Product details", Open: state == "open", Attrs: templ.Attributes{"data-parity-target": "drawer"},
		})
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			children := templ.Raw(`<div class="grid gap-2 p-6"><strong>Travel insurance</strong><p>Contract series TRV-2026 with worldwide coverage.</p></div>`)
			return component.Render(templ.WithChildren(ctx, children), w)
		})
	case "combobox":
		component := base.Combobox(base.ComboboxProps{
			Label: "Product", Placeholder: "Choose a product", Searchable: true, Disabled: disabled,
		})
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			options := []*base.ComboboxOption{{Value: "travel", Label: "Travel insurance"}, {Value: "property", Label: "Property insurance"}}
			return component.Render(templ.WithChildren(ctx, base.ComboboxOptions(options)), w)
		})
	case "search-select":
		return selects.SearchSelect(&selects.SearchSelectProps{
			Label: "Customer", Placeholder: "Search customers", Endpoint: "/customers", Name: "customer",
			Attrs: templ.Attributes{"data-parity-target": "search-select", "disabled": disabled},
		})
	case "date-picker":
		component := baseinput.DatePicker(baseinput.DatePickerProps{
			Label: "Start date", Placeholder: "Choose a date", Mode: baseinput.DatePickerModeSingle,
			SelectorType: baseinput.DateSelectorTypeDay, DateFormat: "Y-m-d", Locale: "en-US",
		})
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			pageURL := &url.URL{Path: "/"}
			ctx = composables.WithPageCtx(ctx, types.NewPageContext(language.English, pageURL, nil))
			return component.Render(ctx, w)
		})
	case "copy-button":
		return copybutton.CopyButton(copybutton.Props{Text: "TRV-2026"})
	case "language-select":
		return usercomponents.LanguageSelect(&usercomponents.LanguageSelectProps{
			Label: "Language", Value: "en", Attrs: templ.Attributes{"data-parity-target": "language-select"},
		})
	default:
		return nil
	}
}
