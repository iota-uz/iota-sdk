package showcase

import _ "embed"

//go:embed components/input.templ
var InputComponentSource string

//go:embed components/textarea.templ
var TextareaComponentSource string

//go:embed components/number.templ
var NumberComponentSource string

//go:embed components/select.templ
var SelectComponentSource string

//go:embed components/combobox.templ
var ComboboxComponentSource string

//go:embed components/radio.templ
var RadioComponentSource string

//go:embed components/avatar.templ
var AvatarComponentSource string

//go:embed components/card.templ
var CardComponentSource string

//go:embed components/datepicker.templ
var DatepickerComponentSource string

//go:embed components/table.templ
var TableComponentSource string

//go:embed components/buttons.templ
var ButtonsComponentSource string

//go:embed components/slider.templ
var SliderComponentSource string
