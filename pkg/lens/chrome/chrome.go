// Package chrome defines optional visual chrome metadata for Lens panels.
package chrome

import "github.com/a-h/templ"

type Icon struct {
	component templ.Component
}

func Component(component templ.Component) Icon {
	return Icon{component: component}
}

func (i Icon) Empty() bool {
	return i.component == nil
}

func (i Icon) Render() templ.Component {
	return i.component
}

type Spec struct {
	Icon        Icon
	AccentColor string
}
