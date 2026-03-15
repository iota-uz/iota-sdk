package templ

import (
	templpkg "github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

type AsyncProps struct {
	PanelBasePath   string
	FilterFormID    string
	IncludeSelector string
}

type DashboardProps struct {
	Spec   lens.DashboardSpec
	Result *runtime.Result
	Async  *AsyncProps
}

type FragmentProps struct {
	Panel  panel.Spec
	Result *runtime.Result
}

type BodyProps struct {
	ClassName      string
	ContainerClass string
	Dashboard      templpkg.Component
}

type PageProps struct {
	MetaTitle      string
	Title          string
	Subtitle       string
	Icon           templpkg.Component
	Filters        templpkg.Component
	Dashboard      templpkg.Component
	BodyClass      string
	ContainerClass string
}
