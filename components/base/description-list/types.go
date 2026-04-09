package descriptionlist

import "github.com/a-h/templ"

type DLItemType int

const (
	DLItemTypeText = DLItemType(iota + 1)
	DLItemTypeLink
	DLItemTypeDetails
	DLItemTypeCustom
)

type DLItem interface {
	Component() templ.Component
	Attrs() templ.Attributes
	Classes() templ.CSSClasses
	Lists() []DLList
}

type DLList interface {
	Title() templ.Component
	Subtitle() templ.Component
	Header() templ.Component
	Items() []DLItem
}

type dlListImpl struct {
	title           string
	subtitle        templ.Component
	header          templ.Component
	items           []DLItem
	titleClasses    templ.CSSClasses
	titleAttrs      templ.Attributes
	subtitleClasses templ.CSSClasses
	subtitleAttrs   templ.Attributes
}

type DLListOption func(dl *dlListImpl)

func WithListHeader(header templ.Component) DLListOption {
	return func(dl *dlListImpl) {
		dl.header = header
	}
}

func WithListTitle(title string) DLListOption {
	return func(dl *dlListImpl) {
		dl.title = title
	}
}

func WithListSubtitle(subtitle templ.Component) DLListOption {
	return func(dl *dlListImpl) {
		dl.subtitle = subtitle
	}
}

func WithListItems(items ...DLItem) DLListOption {
	return func(dl *dlListImpl) {
		dl.items = items
	}
}

func WithListTitleClasses(classes templ.CSSClasses) DLListOption {
	return func(dl *dlListImpl) {
		dl.titleClasses = classes
	}
}

func WithListTitleAttrs(attrs templ.Attributes) DLListOption {
	return func(dl *dlListImpl) {
		dl.titleAttrs = attrs
	}
}

func WithListSubtitleClasses(classes templ.CSSClasses) DLListOption {
	return func(dl *dlListImpl) {
		dl.subtitleClasses = classes
	}
}

func WithListSubtitleAttrs(attrs templ.Attributes) DLListOption {
	return func(dl *dlListImpl) {
		dl.subtitleAttrs = attrs
	}
}

func New(opts ...DLListOption) DLList {
	list := &dlListImpl{}
	list.header = listHeader(list)
	for _, opt := range opts {
		opt(list)
	}
	return list
}

func (d *dlListImpl) Title() templ.Component {
	if d.title != "" {
		return ListTitle(d.title, d.titleClasses, d.titleAttrs)
	}
	return nil
}

func (d *dlListImpl) Subtitle() templ.Component {
	if d.subtitle != nil {
		return ListSubtitle(d.subtitle, d.subtitleClasses, d.subtitleAttrs)
	}
	return nil
}

func (d *dlListImpl) Header() templ.Component {
	return d.header
}

func (d *dlListImpl) Items() []DLItem {
	return d.items
}

type dlItemImpl struct {
	typ                   DLItemType
	label                 string
	text                  string
	href                  string
	lists                 []DLList
	custom                templ.Component
	attrs                 templ.Attributes
	classes               templ.CSSClasses
	detailsContentClasses templ.CSSClasses
}

type DLItemOption func(dt *dlItemImpl)

func WithItemText(text string) DLItemOption {
	return func(dt *dlItemImpl) {
		dt.text = text
	}
}

func WithItemLink(href string) DLItemOption {
	return func(dt *dlItemImpl) {
		dt.href = href
		if dt.text == "" {
			dt.text = href
		}
		dt.typ = DLItemTypeLink
	}
}

func WithItemDetails(lists ...DLList) DLItemOption {
	return func(dt *dlItemImpl) {
		dt.typ = DLItemTypeDetails
		dt.lists = lists
	}
}

func WithItemCustom(custom templ.Component) DLItemOption {
	return func(dt *dlItemImpl) {
		dt.custom = custom
		dt.typ = DLItemTypeCustom
	}
}

func WithItemAttrs(attrs templ.Attributes) DLItemOption {
	return func(dt *dlItemImpl) {
		dt.attrs = attrs
	}
}

func WithItemClasses(classes templ.CSSClasses) DLItemOption {
	return func(dt *dlItemImpl) {
		dt.classes = classes
	}
}

func WithItemDetailsContentClasses(classes templ.CSSClasses) DLItemOption {
	return func(dt *dlItemImpl) {
		dt.detailsContentClasses = classes
	}
}

func NewItem(label string, opts ...DLItemOption) DLItem {
	item := &dlItemImpl{label: label, typ: DLItemTypeText}
	for _, opt := range opts {
		opt(item)
	}
	return item
}

func (t *dlItemImpl) Component() templ.Component {
	if t.typ == DLItemTypeText || t.typ == DLItemTypeLink {
		return RegularItem(
			t.labelComponent(),
			t.valueComponent(),
		)
	} else if t.typ == DLItemTypeDetails {
		return DetailsItem(t.labelComponent(), t.detailsContentClasses, t.lists...)
	} else if t.typ == DLItemTypeCustom {
		return RegularItem(
			t.labelComponent(),
			t.custom,
		)
	} else {
		return nil
	}
}

func (t *dlItemImpl) labelComponent() templ.Component {
	return Label(t.label, t.classes, t.attrs)
}

func (t *dlItemImpl) valueComponent() templ.Component {
	if t.typ == DLItemTypeText {
		return Text(templ.Raw(t.text), t.classes, t.attrs)
	} else if t.typ == DLItemTypeLink {
		return Link(t.text, t.href, t.classes, t.attrs)
	} else {
		return t.custom
	}
}

func (t *dlItemImpl) Attrs() templ.Attributes {
	return t.attrs
}

func (t *dlItemImpl) Classes() templ.CSSClasses {
	return t.classes
}

func (d *dlItemImpl) Lists() []DLList {
	return d.lists
}
