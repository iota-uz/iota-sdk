# IOTA SDK Documentation (github.com/iota-uz/iota-sdk)

Generated automatically from source code.

## Package `tools` (.)

---

## Package `components` (components)

Package components provides UI components for building web interfaces.

It includes basic components like buttons, inputs, and select dropdowns,
as well as more complex components like tables, charts, and dialogs.
All components follow the project's design system and are built
with accessability in mind.

templ: version: v0.3.857


### Types

#### UploadInputProps

UploadInputProps defines the properties for the UploadInput component.
It provides configuration options for the file upload interface.


```go
type UploadInputProps struct {
    ID string
    Label string
    Placeholder string
    Uploads []*viewmodels.Upload
    Error string
    Accept string
    Name string
    Form string
    Class string
    Multiple bool
}
```

##### Methods

### Functions

#### `func UploadInput(props *UploadInputProps) templ.Component`

UploadInput renders a file upload input with preview capability.
It displays existing uploads and allows selecting new files.


#### `func UploadPreview(p *UploadInputProps) templ.Component`

### Variables and Constants

---

## Package `base` (components/base)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### BaseLabelProps

```go
type BaseLabelProps struct {
    Text string
}
```

#### ComboboxOption

```go
type ComboboxOption struct {
    Value string
    Label string
}
```

#### ComboboxProps

```go
type ComboboxProps struct {
    Multiple bool
    Placeholder string
    Class string
    ListClass string
    Name string
    Form string
    Label string
    Endpoint string
    Searchable bool
    NotFoundText string
    Trigger *Trigger
}
```

#### DetailsDropdownProps

```go
type DetailsDropdownProps struct {
    Summary templ.Component
    Classes templ.CSSClasses
}
```

##### Methods

#### DropdownItemProps

```go
type DropdownItemProps struct {
    Href string
}
```

#### ProgressProps

```go
type ProgressProps struct {
    Value uint
    Target uint
    TargetLabel string
    ValueLabel string
    TargetLabelComp templ.Component
    ValueLabelComp templ.Component
}
```

#### SelectProps

```go
type SelectProps struct {
    Label string
    Class string
    Placeholder string
    Attrs templ.Attributes
    Prefix string
    Error string
}
```

##### Methods

#### TableCellProps

```go
type TableCellProps struct {
    Classes templ.CSSClasses
    Attrs templ.Attributes
}
```

#### TableColumn

```go
type TableColumn struct {
    Label string
    Key string
    Class string
}
```

#### TableEmptyStateProps

```go
type TableEmptyStateProps struct {
    Title string
    Description string
    Class string
    Attrs templ.Attributes
}
```

#### TableProps

```go
type TableProps struct {
    Columns []*TableColumn
    Classes templ.CSSClasses
    Attrs templ.Attributes
    TBodyClasses templ.CSSClasses
    TBodyAttrs templ.Attributes
}
```

#### TableRowProps

```go
type TableRowProps struct {
    Attrs templ.Attributes
}
```

#### Trigger

```go
type Trigger struct {
    Render func(props *TriggerProps) templ.Component
    Component templ.Component
}
```

#### TriggerProps

```go
type TriggerProps struct {
    InputAttrs templ.Attributes
    ButtonAttrs templ.Attributes
}
```

### Functions

#### `func BaseLabel(props BaseLabelProps) templ.Component`

#### `func Combobox(props ComboboxProps) templ.Component`

#### `func ComboboxOptions(options []*ComboboxOption) templ.Component`

#### `func DetailsDropdown(props *DetailsDropdownProps) templ.Component`

#### `func DropdownIndicator() templ.Component`

#### `func DropdownItem(props DropdownItemProps) templ.Component`

#### `func Progress(p ProgressProps) templ.Component`

#### `func Select(p *SelectProps) templ.Component`

#### `func SelectedValues() templ.Component`

#### `func Table(props TableProps) templ.Component`

#### `func TableCell(props TableCellProps) templ.Component`

#### `func TableEmptyState(props TableEmptyStateProps) templ.Component`

#### `func TableRow(props TableRowProps) templ.Component`

### Variables and Constants

---

## Package `alert` (components/base/alert)

templ: version: v0.3.857


### Functions

#### `func Error() templ.Component`

### Variables and Constants

---

## Package `avatar` (components/base/avatar)

templ: version: v0.3.857


### Types

#### Props

```go
type Props struct {
    Class templ.CSSClasses
    ImageURL string
    Initials string
    Variant Variant
}
```

#### Variant

### Functions

#### `func Avatar(props Props) templ.Component`

### Variables and Constants

---

## Package `badge` (components/base/badge)

templ: version: v0.3.857


### Types

#### Props

```go
type Props struct {
    Class templ.CSSClasses
    Size Size
    Variant Variant
}
```

#### Size

#### Variant

### Functions

#### `func New(props Props) templ.Component`

### Variables and Constants

- Const: `[VariantPink VariantYellow VariantGreen VariantBlue VariantPurple VariantGray]`

- Const: `[SizeNormal SizeLG]`

---

## Package `breadcrumb` (components/base/breadcrumb)

### Functions

#### `func Item() templ.Component`

#### `func Link(href string) templ.Component`

#### `func List() templ.Component`

#### `func Separator() templ.Component`

#### `func SlashSeparator() templ.Component`

### Variables and Constants

---

## Package `button` (components/base/button)

templ: version: v0.3.857


### Types

#### Props

```go
type Props struct {
    Size Size
    Fixed bool
    Href string
    Rounded bool
    Loading bool
    Disabled bool
    Class any
    Icon templ.Component
    Attrs templ.Attributes
}
```

#### Size

#### Variant

### Functions

#### `func Danger(props Props) templ.Component`

#### `func Ghost(props Props) templ.Component`

#### `func Primary(props Props) templ.Component`

#### `func PrimaryOutline(props Props) templ.Component`

#### `func Secondary(props Props) templ.Component`

#### `func Sidebar(props Props) templ.Component`

### Variables and Constants

- Const: `[VariantPrimary VariantSecondary VariantPrimaryOutline VariantSidebar VariantDanger VariantGhost]`

- Const: `[SizeNormal SizeMD SizeSM SizeXS]`

---

## Package `card` (components/base/card)

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### AccentColorProps

```go
type AccentColorProps struct {
    Name string
    Value string
    Color string
    Form string
    Checked bool
}
```

#### Props

```go
type Props struct {
    Class string
    WrapperClass string
    Header templ.Component
    Attrs templ.Attributes
}
```

### Functions

#### `func AccentColor(props AccentColorProps) templ.Component`

#### `func Card(props Props) templ.Component`

#### `func DefaultHeader(text string) templ.Component`

### Variables and Constants

---

## Package `dialog` (components/base/dialog)

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### Direction

#### DrawerProps

```go
type DrawerProps struct {
    ID string
    Open bool
    Direction Direction
    Action string
    Attrs templ.Attributes
    Classes templ.CSSClasses
}
```

#### Props

```go
type Props struct {
    Icon templ.Component
    Heading string
    Text string
    Action string
    Attrs templ.Attributes
    CancelText string
    ConfirmText string
}
```

#### StdDrawerProps

```go
type StdDrawerProps struct {
    ID string
    Title string
    Action string
    Open bool
    Attrs templ.Attributes
}
```

### Functions

#### `func Confirmation(p *Props) templ.Component`

#### `func Drawer(props DrawerProps) templ.Component`

#### `func StdViewDrawer(props StdDrawerProps) templ.Component`

### Variables and Constants

---

## Package `input` (components/base/input)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### Addon

```go
type Addon struct {
    Render func(props *Props) templ.Component
    Component templ.Component
    Class string
    Attrs templ.Attributes
}
```

#### CheckboxProps

```go
type CheckboxProps struct {
    Label string
    LabelComp templ.Component
    Error string
    Checked bool
    Attrs templ.Attributes
    Class templ.CSSClasses
    ID string
}
```

##### Methods

#### DatePickerMode

#### DatePickerProps

```go
type DatePickerProps struct {
    Label string `json:"-"`
    Placeholder string `json:"-"`
    Mode DatePickerMode `json:"mode"`
    SelectorType DateSelectorType `json:"selectorType"`
    Attrs templ.Attributes
    DateFormat string `json:"dateFormat"`
    LabelFormat string `json:"labelFormat"`
    MinDate string `json:"minDate"`
    MaxDate string `json:"maxDate"`
    Selected []string `json:"selected"`
    Locale string `json:"locale"`
    Name string `json:"-"`
    EndName string `json:"-"`
    StartName string `json:"-"`
    Form string `json:"-"`
}
```

##### Methods

#### DateSelectorType

#### Props

```go
type Props struct {
    Placeholder string
    Label string
    Class string
    Attrs templ.Attributes
    WrapperProps templ.Attributes
    AddonRight *Addon
    AddonLeft *Addon
    Error string
}
```

##### Methods

#### SwitchProps

```go
type SwitchProps struct {
    ID string
    Label string
    LabelComp templ.Component
    LabelClasses templ.CSSClasses
    Error string
    Checked bool
    Attrs templ.Attributes
    Size SwitchSize
}
```

##### Methods

#### SwitchSize

#### TextAreaProps

```go
type TextAreaProps struct {
    Placeholder string
    Label string
    Class string
    WrapperClass string
    Attrs templ.Attributes
    Error string
    Value string
}
```

##### Methods

### Functions

#### `func Checkbox(p *CheckboxProps) templ.Component`

#### `func Color(props *Props) templ.Component`

#### `func Date(props *Props) templ.Component`

#### `func DatePicker(props DatePickerProps) templ.Component`

#### `func Email(props *Props) templ.Component`

#### `func Number(props *Props) templ.Component`

#### `func Password(props *Props) templ.Component`

#### `func Switch(p *SwitchProps) templ.Component`

#### `func Tel(props *Props) templ.Component`

#### `func Text(props *Props) templ.Component`

#### `func TextArea(props *TextAreaProps) templ.Component`

### Variables and Constants

- Const: `[DatePickerModeSingle DatePickerModeMultiple DatePickerModeRange]`

- Const: `[DateSelectorTypeDay DateSelectorTypeMonth DateSelectorTypeWeek DateSelectorTypeYear]`

---

## Package `navtabs` (components/base/navtabs)

### Types

#### Props

```go
type Props struct {
    DefaultValue string
    Tabs []string
    Class string
}
```

##### Methods

- `func (Props) Validate() error`
  Validate checks if the Props are valid and returns an error if not
  

### Functions

#### `func NavTabs(props Props) templ.Component`

### Variables and Constants

---

## Package `pagination` (components/base/pagination)

templ: version: v0.3.857


### Types

#### Page

```go
type Page struct {
    Num int
    Link string
    Filler bool
    Active bool
}
```

##### Methods

- `func (Page) Classes() string`

#### State

```go
type State struct {
    Total int
    Current int
}
```

##### Methods

- `func (State) NextLink() string`

- `func (State) NextLinkClasses() string`

- `func (State) Pages() []Page`

- `func (State) PrevLink() string`

- `func (State) PrevLinkClasses() string`

- `func (State) TotalStr() string`

### Functions

#### `func Pagination(state *State) templ.Component`

### Variables and Constants

---

## Package `radio` (components/base/radio)

### Types

#### CardItemProps

CardItemProps configures an individual radio input styled as a card.


```go
type CardItemProps struct {
    WrapperClass templ.CSSClasses
    Class templ.CSSClasses
    Name string
    Checked bool
    Disabled bool
    Attrs templ.Attributes
    Value string
    Form string
}
```

#### Orientation

Orientation defines the layout direction of radio items


#### RadioGroupProps

RadioGroupProps configures the RadioGroup component's behavior and appearance.


```go
type RadioGroupProps struct {
    Label string
    Error string
    Class string
    Attrs templ.Attributes
    WrapperProps templ.Attributes
    Orientation Orientation
}
```

### Functions

#### `func CardItem(props CardItemProps) templ.Component`

CardItem renders a styled radio input as a card-like UI element.
Children are rendered as the label content next to the radio indicator.


#### `func RadioGroup(props RadioGroupProps) templ.Component`

RadioGroup creates a container for radio inputs with optional label and error message.
Child components (typically CardItem elements) are rendered within the group.


### Variables and Constants

---

## Package `selects` (components/base/selects)

templ: version: v0.3.857


### Types

#### SearchOptionsProps

```go
type SearchOptionsProps struct {
    Options []*Value
    NothingFoundText string
}
```

#### SearchSelectProps

```go
type SearchSelectProps struct {
    Label string
    Placeholder string
    Value string
    Endpoint string
    Attrs templ.Attributes
}
```

#### Value

```go
type Value struct {
    Value string
    Label string
}
```

### Functions

#### `func SearchOptions(props *SearchOptionsProps) templ.Component`

#### `func SearchSelect(props *SearchSelectProps) templ.Component`

### Variables and Constants

---

## Package `slider` (components/base/slider)

templ: version: v0.3.857


### Types

#### SliderProps

```go
type SliderProps struct {
    Min int
    Max int
    Value float64
    Step float64
    Label string
    HelpText string
    Error string
    Disabled bool
    Class string
    Attrs templ.Attributes
    ID string
    ValueFormat string
}
```

##### Methods

### Functions

#### `func Slider(props SliderProps) templ.Component`

### Variables and Constants

---

## Package `tab` (components/base/tab)

templ: version: v0.3.857


### Types

#### BoostLinkProps

```go
type BoostLinkProps struct {
    Href string
    Push bool
}
```

#### ListProps

```go
type ListProps struct {
    Class string
}
```

#### Props

```go
type Props struct {
    DefaultValue string
    Class string
}
```

### Functions

#### `func BoostedContent(class templ.CSSClasses) templ.Component`

#### `func BoostedLink(props BoostLinkProps) templ.Component`

#### `func Button(value string) templ.Component`

#### `func Content(value string) templ.Component`

#### `func Link(href string, active bool) templ.Component`

--- Pure Tabs ---


#### `func List(props ListProps) templ.Component`

#### `func Root(props Props) templ.Component`

### Variables and Constants

---

## Package `toggle` (components/base/toggle)

templ: version: v0.3.857


### Types

#### ToggleAlignment

#### ToggleOption

```go
type ToggleOption struct {
    Value string
    Label string
}
```

##### Methods

#### ToggleProps

```go
type ToggleProps struct {
    InitialActive string
    Options []ToggleOption
    Size ToggleSize
    Rounded ToggleRounded
    Alignment ToggleAlignment
}
```

##### Methods

#### ToggleRounded

#### ToggleSize

### Functions

#### `func Toggle(props ToggleProps) templ.Component`

### Variables and Constants

---

## Package `charts` (components/charts)

### Types

#### AnnotationLabel

```go
type AnnotationLabel struct {
    BorderColor *string `json:"borderColor,omitempty"`
    BorderWidth *int `json:"borderWidth,omitempty"`
    BorderRadius *int `json:"borderRadius,omitempty"`
    Text *string `json:"text,omitempty"`
    TextAnchor *string `json:"textAnchor,omitempty"`
    Position *string `json:"position,omitempty"`
    Orientation *string `json:"orientation,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Style *AnnotationLabelStyle `json:"style,omitempty"`
}
```

#### AnnotationLabelStyle

```go
type AnnotationLabelStyle struct {
    Background *string `json:"background,omitempty"`
    Color *string `json:"color,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontWeight *string `json:"fontWeight,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    CSSClass *string `json:"cssClass,omitempty"`
    Padding *Padding `json:"padding,omitempty"`
}
```

#### Annotations

```go
type Annotations struct {
    YAxis []YAxisAnnotation `json:"yaxis,omitempty"`
    XAxis []XAxisAnnotation `json:"xaxis,omitempty"`
    Points []PointAnnotation `json:"points,omitempty"`
    Texts []TextAnnotation `json:"texts,omitempty"`
    Images []ImageAnnotation `json:"images,omitempty"`
}
```

#### AxisTitle

```go
type AxisTitle struct {
    Text string `json:"text"`
    Style TextStyle `json:"style"`
}
```

#### BarConfig

```go
type BarConfig struct {
    BorderRadius int `json:"borderRadius,omitempty"`
    ColumnWidth string `json:"columnWidth,omitempty"`
    DataLabels BarLabels `json:"dataLabels,omitempty"`
}
```

#### BarLabels

```go
type BarLabels struct {
    Position string `json:"position,omitempty"`
}
```

#### ChartConfig

```go
type ChartConfig struct {
    Type ChartType `json:"type"`
    Height string `json:"height,omitempty"`
    OffsetX int `json:"offsetX,omitempty"`
    OffsetY int `json:"offsetY,omitempty"`
    Toolbar Toolbar `json:"toolbar,omitempty"`
    Stacked bool `json:"stacked,omitempty"`
}
```

#### ChartOptions

```go
type ChartOptions struct {
    Chart ChartConfig `json:"chart"`
    Series []Series `json:"series"`
    XAxis XAxisConfig `json:"xaxis"`
    YAxis []YAxisConfig `json:"yaxis"`
    Colors []string `json:"colors,omitempty"`
    DataLabels *DataLabels `json:"dataLabels,omitempty"`
    Grid *GridConfig `json:"grid,omitempty"`
    PlotOptions *PlotOptions `json:"plotOptions,omitempty"`
    Tooltip *TooltipConfig `json:"tooltip,omitempty"`
    Title *TitleConfig `json:"title,omitempty"`
    Theme *ThemeConfig `json:"theme,omitempty"`
    Stroke *StrokeConfig `json:"stroke,omitempty"`
    Markers *MarkersConfig `json:"markers,omitempty"`
    Legend *LegendConfig `json:"legend,omitempty"`
    NoData *NoDataConfig `json:"noData,omitempty"`
    States *StatesConfig `json:"states,omitempty"`
    Fill *FillConfig `json:"fill,omitempty"`
    Annotations *Annotations `json:"annotations,omitempty"`
}
```

#### ChartType

#### ColorStop

```go
type ColorStop struct {
    Offset *int `json:"offset,omitempty"`
    Color *string `json:"color,omitempty"`
    Opacity *float64 `json:"opacity,omitempty"`
}
```

#### DataLabelStyle

```go
type DataLabelStyle struct {
    Colors []string `json:"colors,omitempty"`
    FontSize string `json:"fontSize,omitempty"`
    FontWeight string `json:"fontWeight,omitempty"`
}
```

#### DataLabels

```go
type DataLabels struct {
    Enabled bool `json:"enabled"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
    Style *DataLabelStyle `json:"style,omitempty"`
    OffsetY int `json:"offsetY,omitempty"`
    DropShadow *DropShadow `json:"dropShadow,omitempty"`
}
```

#### DonutSpecifics

```go
type DonutSpecifics struct {
    Size *string `json:"size,omitempty"`
    Labels *Labels `json:"labels,omitempty"`
}
```

#### DropShadow

```go
type DropShadow struct {
    Enabled bool `json:"enabled"`
    Top int `json:"top,omitempty"`
    Left int `json:"left,omitempty"`
    Blur int `json:"blur,omitempty"`
    Color string `json:"color,omitempty"`
    Opacity float64 `json:"opacity,omitempty"`
}
```

#### FillConfig

```go
type FillConfig struct {
    Colors []string `json:"colors,omitempty"`
    Opacity interface{} `json:"opacity,omitempty"`
    Type interface{} `json:"type,omitempty"`
    Gradient *FillGradient `json:"gradient,omitempty"`
    Image *FillImage `json:"image,omitempty"`
    Pattern *FillPattern `json:"pattern,omitempty"`
}
```

#### FillGradient

```go
type FillGradient struct {
    Shade *string `json:"shade,omitempty"`
    Type *string `json:"type,omitempty"`
    ShadeIntensity *float64 `json:"shadeIntensity,omitempty"`
    GradientToColors []string `json:"gradientToColors,omitempty"`
    InverseColors *bool `json:"inverseColors,omitempty"`
    OpacityFrom *float64 `json:"opacityFrom,omitempty"`
    OpacityTo *float64 `json:"opacityTo,omitempty"`
    Stops []float64 `json:"stops,omitempty"`
    ColorStops []ColorStop `json:"colorStops,omitempty"`
}
```

#### FillImage

```go
type FillImage struct {
    Src interface{} `json:"src,omitempty"`
    Width *int `json:"width,omitempty"`
    Height *int `json:"height,omitempty"`
}
```

#### FillPattern

```go
type FillPattern struct {
    Style interface{} `json:"style,omitempty"`
    Width *int `json:"width,omitempty"`
    Height *int `json:"height,omitempty"`
    StrokeWidth *int `json:"strokeWidth,omitempty"`
}
```

#### FillType

#### GridConfig

```go
type GridConfig struct {
    BorderColor string `json:"borderColor,omitempty"`
    Row *GridRowColumn `json:"row,omitempty"`
    Column *GridRowColumn `json:"column,omitempty"`
    Padding *Padding `json:"padding,omitempty"`
}
```

#### GridRowColumn

```go
type GridRowColumn struct {
    Colors []string `json:"colors,omitempty"`
    Opacity *float64 `json:"opacity,omitempty"`
}
```

#### HeatmapColorScale

```go
type HeatmapColorScale struct {
    Ranges []HeatmapRange `json:"ranges,omitempty"`
    Inverse *bool `json:"inverse,omitempty"`
    Min *float64 `json:"min,omitempty"`
    Max *float64 `json:"max,omitempty"`
}
```

#### HeatmapConfig

```go
type HeatmapConfig struct {
    Radius *int `json:"radius,omitempty"`
    EnableShades *bool `json:"enableShades,omitempty"`
    ShadeIntensity *float64 `json:"shadeIntensity,omitempty"`
    ReverseNegative *bool `json:"reverseNegative,omitempty"`
    Distributed *bool `json:"distributed,omitempty"`
    ColorScale *HeatmapColorScale `json:"colorScale,omitempty"`
}
```

#### HeatmapRange

```go
type HeatmapRange struct {
    From *float64 `json:"from,omitempty"`
    To *float64 `json:"to,omitempty"`
    Color *string `json:"color,omitempty"`
    Name *string `json:"name,omitempty"`
}
```

#### ImageAnnotation

```go
type ImageAnnotation struct {
    Path *string `json:"path,omitempty"`
    X *float64 `json:"x,omitempty"`
    Y *float64 `json:"y,omitempty"`
    Width *int `json:"width,omitempty"`
    Height *int `json:"height,omitempty"`
}
```

#### LabelFormatter

```go
type LabelFormatter struct {
    Style LabelStyle `json:"style"`
}
```

#### LabelNameValue

```go
type LabelNameValue struct {
    Show *bool `json:"show,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    FontWeight *string `json:"fontWeight,omitempty"`
    Color *string `json:"color,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
}
```

#### LabelStyle

```go
type LabelStyle struct {
    Colors string `json:"colors,omitempty"`
    FontSize string `json:"fontSize,omitempty"`
}
```

#### LabelTotal

```go
type LabelTotal struct {
    Show *bool `json:"show,omitempty"`
    ShowAlways *bool `json:"showAlways,omitempty"`
    Label *string `json:"label,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    FontWeight *string `json:"fontWeight,omitempty"`
    Color *string `json:"color,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
}
```

#### Labels

```go
type Labels struct {
    Show *bool `json:"show,omitempty"`
    Name *LabelNameValue `json:"name,omitempty"`
    Value *LabelNameValue `json:"value,omitempty"`
    Total *LabelTotal `json:"total,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
}
```

#### LegendConfig

```go
type LegendConfig struct {
    Show *bool `json:"show,omitempty"`
    ShowForSingleSeries *bool `json:"showForSingleSeries,omitempty"`
    ShowForNullSeries *bool `json:"showForNullSeries,omitempty"`
    ShowForZeroSeries *bool `json:"showForZeroSeries,omitempty"`
    Position *LegendPosition `json:"position,omitempty"`
    HorizontalAlign *LegendHorizontalAlign `json:"horizontalAlign,omitempty"`
    Floating *bool `json:"floating,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    FontWeight interface{} `json:"fontWeight,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
    InverseOrder *bool `json:"inverseOrder,omitempty"`
    Width *int `json:"width,omitempty"`
    Height *int `json:"height,omitempty"`
    TooltipHoverFormatter templ.JSExpression `json:"tooltipHoverFormatter,omitempty"`
    CustomLegendItems []string `json:"customLegendItems,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Labels *LegendLabelsConfig `json:"labels,omitempty"`
    Markers *LegendMarkersConfig `json:"markers,omitempty"`
    ItemMargin *LegendItemMargin `json:"itemMargin,omitempty"`
    OnItemClick *LegendOnItemClick `json:"onItemClick,omitempty"`
    OnItemHover *LegendOnItemHover `json:"onItemHover,omitempty"`
    ContainerMargin *Padding `json:"containerMargin,omitempty"`
}
```

#### LegendHorizontalAlign

#### LegendItemMargin

```go
type LegendItemMargin struct {
    Horizontal *int `json:"horizontal,omitempty"`
    Vertical *int `json:"vertical,omitempty"`
}
```

#### LegendLabelsConfig

```go
type LegendLabelsConfig struct {
    Colors interface{} `json:"colors,omitempty"`
    UseSeriesColors *bool `json:"useSeriesColors,omitempty"`
}
```

#### LegendMarkersConfig

```go
type LegendMarkersConfig struct {
    Width *int `json:"width,omitempty"`
    Height *int `json:"height,omitempty"`
    StrokeWidth *int `json:"strokeWidth,omitempty"`
    StrokeColor *string `json:"strokeColor,omitempty"`
    FillColors []string `json:"fillColors,omitempty"`
    Radius *int `json:"radius,omitempty"`
    CustomHTML templ.JSExpression `json:"customHTML,omitempty"`
    OnClick templ.JSExpression `json:"onClick,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
}
```

#### LegendOnItemClick

```go
type LegendOnItemClick struct {
    ToggleDataSeries *bool `json:"toggleDataSeries,omitempty"`
}
```

#### LegendOnItemHover

```go
type LegendOnItemHover struct {
    HighlightDataSeries *bool `json:"highlightDataSeries,omitempty"`
}
```

#### LegendPosition

#### MarkerDiscrete

```go
type MarkerDiscrete struct {
    SeriesIndex *int `json:"seriesIndex,omitempty"`
    DataPointIndex *int `json:"dataPointIndex,omitempty"`
    FillColor *string `json:"fillColor,omitempty"`
    StrokeColor *string `json:"strokeColor,omitempty"`
    Size *int `json:"size,omitempty"`
    Shape *string `json:"shape,omitempty"`
}
```

#### MarkerHover

```go
type MarkerHover struct {
    Size *int `json:"size,omitempty"`
    SizeOffset *int `json:"sizeOffset,omitempty"`
}
```

#### MarkersConfig

```go
type MarkersConfig struct {
    Size interface{} `json:"size,omitempty"`
    Colors []string `json:"colors,omitempty"`
    StrokeColor interface{} `json:"strokeColor,omitempty"`
    StrokeWidth interface{} `json:"strokeWidth,omitempty"`
    StrokeOpacity interface{} `json:"strokeOpacity,omitempty"`
    StrokeDashArray interface{} `json:"strokeDashArray,omitempty"`
    FillOpacity interface{} `json:"fillOpacity,omitempty"`
    Discrete []MarkerDiscrete `json:"discrete,omitempty"`
    Shape interface{} `json:"shape,omitempty"`
    Radius *int `json:"radius,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    OnClick templ.JSExpression `json:"onClick,omitempty"`
    OnDblClick templ.JSExpression `json:"onDblClick,omitempty"`
    ShowNullDataPoints *bool `json:"showNullDataPoints,omitempty"`
    Hover *MarkerHover `json:"hover,omitempty"`
}
```

#### NoDataConfig

```go
type NoDataConfig struct {
    Text *string `json:"text,omitempty"`
    Align *TitleAlign `json:"align,omitempty"`
    VerticalAlign *string `json:"verticalAlign,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Style *NoDataStyleConfig `json:"style,omitempty"`
}
```

#### NoDataStyleConfig

```go
type NoDataStyleConfig struct {
    Color *string `json:"color,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
}
```

#### Padding

```go
type Padding struct {
    Top *int `json:"top,omitempty"`
    Right *int `json:"right,omitempty"`
    Bottom *int `json:"bottom,omitempty"`
    Left *int `json:"left,omitempty"`
}
```

#### PieDataLabels

```go
type PieDataLabels struct {
    Offset *int `json:"offset,omitempty"`
}
```

#### PieDonutConfig

```go
type PieDonutConfig struct {
    Size *string `json:"size,omitempty"`
    Donut *DonutSpecifics `json:"donut,omitempty"`
    CustomScale *float64 `json:"customScale,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    DataLabels *PieDataLabels `json:"dataLabels,omitempty"`
}
```

#### PlotOptions

```go
type PlotOptions struct {
    Bar *BarConfig `json:"bar,omitempty"`
    Pie *PieDonutConfig `json:"pie,omitempty"`
    Donut *PieDonutConfig `json:"donut,omitempty"`
    RadialBar *RadialBarConfig `json:"radialBar,omitempty"`
    Heatmap *HeatmapConfig `json:"heatmap,omitempty"`
}
```

#### PointAnnotation

```go
type PointAnnotation struct {
    X interface{} `json:"x,omitempty"`
    Y *float64 `json:"y,omitempty"`
    YAxisIndex *int `json:"yAxisIndex,omitempty"`
    SeriesIndex *int `json:"seriesIndex,omitempty"`
    Marker *PointMarker `json:"marker,omitempty"`
    Label *AnnotationLabel `json:"label,omitempty"`
    Image *PointImage `json:"image,omitempty"`
}
```

#### PointImage

```go
type PointImage struct {
    Path *string `json:"path,omitempty"`
    Width *int `json:"width,omitempty"`
    Height *int `json:"height,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
}
```

#### PointMarker

```go
type PointMarker struct {
    Size *int `json:"size,omitempty"`
    FillColor *string `json:"fillColor,omitempty"`
    StrokeColor *string `json:"strokeColor,omitempty"`
    StrokeWidth *int `json:"strokeWidth,omitempty"`
    Shape *string `json:"shape,omitempty"`
    Radius *int `json:"radius,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    CSSClass *string `json:"cssClass,omitempty"`
}
```

#### Props

Props defines the configuration options for a Chart component.


```go
type Props struct {
    Class string
    Options ChartOptions
}
```

#### RadialBarConfig

```go
type RadialBarConfig struct {
    Size *string `json:"size,omitempty"`
    InverseOrder *bool `json:"inverseOrder,omitempty"`
    StartAngle *int `json:"startAngle,omitempty"`
    EndAngle *int `json:"endAngle,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Hollow *RadialBarHollow `json:"hollow,omitempty"`
    Track *RadialBarTrack `json:"track,omitempty"`
    DataLabels *RadialBarDataLabels `json:"dataLabels,omitempty"`
}
```

#### RadialBarDataLabels

```go
type RadialBarDataLabels struct {
    Show *bool `json:"show,omitempty"`
    Name *LabelNameValue `json:"name,omitempty"`
    Value *LabelNameValue `json:"value,omitempty"`
    Total *LabelTotal `json:"total,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
}
```

#### RadialBarHollow

```go
type RadialBarHollow struct {
    Margin *int `json:"margin,omitempty"`
    Size *string `json:"size,omitempty"`
    Background *string `json:"background,omitempty"`
    Image *string `json:"image,omitempty"`
    ImageWidth *int `json:"imageWidth,omitempty"`
    ImageHeight *int `json:"imageHeight,omitempty"`
    ImageOffsetX *int `json:"imageOffsetX,omitempty"`
    ImageOffsetY *int `json:"imageOffsetY,omitempty"`
    ImageClipped *bool `json:"imageClipped,omitempty"`
    Position *string `json:"position,omitempty"`
    DropShadow *DropShadow `json:"dropShadow,omitempty"`
}
```

#### RadialBarTrack

```go
type RadialBarTrack struct {
    Show *bool `json:"show,omitempty"`
    Background *string `json:"background,omitempty"`
    StrokeWidth *string `json:"strokeWidth,omitempty"`
    Opacity *float64 `json:"opacity,omitempty"`
    Margin *int `json:"margin,omitempty"`
    DropShadow *DropShadow `json:"dropShadow,omitempty"`
}
```

#### Series

```go
type Series struct {
    Name string `json:"name"`
    Type *ChartType `json:"type,omitempty"`
    Data []interface{} `json:"data"`
}
```

#### StateActiveConfig

```go
type StateActiveConfig struct {
    AllowMultipleDataPointsSelection *bool `json:"allowMultipleDataPointsSelection,omitempty"`
    Filter *StateFilter `json:"filter,omitempty"`
}
```

#### StateFilter

```go
type StateFilter struct {
    Type *string `json:"type,omitempty"`
    Value *float64 `json:"value,omitempty"`
}
```

#### StateFilterConfig

```go
type StateFilterConfig struct {
    Filter *StateFilter `json:"filter,omitempty"`
}
```

#### StatesConfig

```go
type StatesConfig struct {
    Normal *StateFilterConfig `json:"normal,omitempty"`
    Hover *StateFilterConfig `json:"hover,omitempty"`
    Active *StateActiveConfig `json:"active,omitempty"`
}
```

#### StrokeConfig

```go
type StrokeConfig struct {
    Show *bool `json:"show,omitempty"`
    Curve interface{} `json:"curve,omitempty"`
    LineCap StrokeLineCap `json:"lineCap,omitempty"`
    Colors []string `json:"colors,omitempty"`
    Width interface{} `json:"width,omitempty"`
    DashArray interface{} `json:"dashArray,omitempty"`
}
```

#### StrokeCurve

#### StrokeLineCap

#### TextAnnotation

```go
type TextAnnotation struct {
    X *float64 `json:"x,omitempty"`
    Y *float64 `json:"y,omitempty"`
    Text *string `json:"text,omitempty"`
    TextAnchor *string `json:"textAnchor,omitempty"`
    ForeColor *string `json:"foreColor,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    FontWeight *string `json:"fontWeight,omitempty"`
    BackgroundColor *string `json:"backgroundColor,omitempty"`
    BorderColor *string `json:"borderColor,omitempty"`
    BorderRadius *int `json:"borderRadius,omitempty"`
    BorderWidth *int `json:"borderWidth,omitempty"`
    PaddingLeft *int `json:"paddingLeft,omitempty"`
    PaddingRight *int `json:"paddingRight,omitempty"`
    PaddingTop *int `json:"paddingTop,omitempty"`
    PaddingBottom *int `json:"paddingBottom,omitempty"`
}
```

#### TextStyle

```go
type TextStyle struct {
    FontWeight string `json:"fontWeight,omitempty"`
    FontSize string `json:"fontSize,omitempty"`
    Color string `json:"color,omitempty"`
}
```

#### ThemeConfig

```go
type ThemeConfig struct {
    Mode *ThemeMode `json:"mode,omitempty"`
    Palette *ThemePalette `json:"palette,omitempty"`
    Monochrome *ThemeMonochromeConfig `json:"monochrome,omitempty"`
}
```

#### ThemeMode

#### ThemeMonochromeConfig

```go
type ThemeMonochromeConfig struct {
    Enabled *bool `json:"enabled,omitempty"`
    Color *string `json:"color,omitempty"`
    ShadeTo *ThemeMonochromeShadeTo `json:"shadeTo,omitempty"`
    ShadeIntensity *float64 `json:"shadeIntensity,omitempty"`
}
```

#### ThemeMonochromeShadeTo

#### ThemePalette

#### TitleAlign

#### TitleConfig

```go
type TitleConfig struct {
    Text *string `json:"text,omitempty"`
    Align *TitleAlign `json:"align,omitempty"`
    Margin *int `json:"margin,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Floating *bool `json:"floating,omitempty"`
    Style *TitleStyleConfig `json:"style,omitempty"`
}
```

#### TitleStyleConfig

```go
type TitleStyleConfig struct {
    FontSize *string `json:"fontSize,omitempty"`
    FontWeight interface{} `json:"fontWeight,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    Color *string `json:"color,omitempty"`
}
```

#### Toolbar

```go
type Toolbar struct {
    Show bool `json:"show"`
}
```

#### TooltipConfig

```go
type TooltipConfig struct {
    Enabled *bool `json:"enabled,omitempty"`
    EnabledOnSeries []int `json:"enabledOnSeries,omitempty"`
    Shared *bool `json:"shared,omitempty"`
    FollowCursor *bool `json:"followCursor,omitempty"`
    Intersect *bool `json:"intersect,omitempty"`
    InverseOrder *bool `json:"inverseOrder,omitempty"`
    Custom interface{} `json:"custom,omitempty"`
    HideEmptySeries *bool `json:"hideEmptySeries,omitempty"`
    FillSeriesColor *bool `json:"fillSeriesColor,omitempty"`
    Theme *string `json:"theme,omitempty"`
    Style *TooltipStyleConfig `json:"style,omitempty"`
    OnDatasetHover *TooltipOnDatasetHoverConfig `json:"onDatasetHover,omitempty"`
    X *TooltipXConfig `json:"x,omitempty"`
    Y interface{} `json:"y,omitempty"`
    Z *TooltipZConfig `json:"z,omitempty"`
    Marker *TooltipMarkerConfig `json:"marker,omitempty"`
    Items *TooltipItemsConfig `json:"items,omitempty"`
    Fixed *TooltipFixedConfig `json:"fixed,omitempty"`
}
```

#### TooltipFixedConfig

```go
type TooltipFixedConfig struct {
    Enabled *bool `json:"enabled,omitempty"`
    Position *TooltipFixedPosition `json:"position,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
}
```

#### TooltipFixedPosition

#### TooltipItemsConfig

```go
type TooltipItemsConfig struct {
    Display *string `json:"display,omitempty"`
}
```

#### TooltipMarkerConfig

```go
type TooltipMarkerConfig struct {
    Show *bool `json:"show,omitempty"`
}
```

#### TooltipOnDatasetHoverConfig

```go
type TooltipOnDatasetHoverConfig struct {
    HighlightDataSeries *bool `json:"highlightDataSeries,omitempty"`
}
```

#### TooltipStyleConfig

```go
type TooltipStyleConfig struct {
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
}
```

#### TooltipXConfig

```go
type TooltipXConfig struct {
    Show *bool `json:"show,omitempty"`
    Format *string `json:"format,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
}
```

#### TooltipYConfig

```go
type TooltipYConfig struct {
    Formatter templ.JSExpression `json:"formatter,omitempty"`
    Title *TooltipYTitleConfig `json:"title,omitempty"`
}
```

#### TooltipYTitleConfig

```go
type TooltipYTitleConfig struct {
    Formatter templ.JSExpression `json:"formatter,omitempty"`
}
```

#### TooltipZConfig

```go
type TooltipZConfig struct {
    Formatter templ.JSExpression `json:"formatter,omitempty"`
    Title *string `json:"title,omitempty"`
}
```

#### XAxisAnnotation

```go
type XAxisAnnotation struct {
    X *float64 `json:"x,omitempty"`
    X2 *float64 `json:"x2,omitempty"`
    StrokeDashArray *int `json:"strokeDashArray,omitempty"`
    FillColor *string `json:"fillColor,omitempty"`
    BorderColor *string `json:"borderColor,omitempty"`
    BorderWidth *int `json:"borderWidth,omitempty"`
    Opacity *float64 `json:"opacity,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Label *AnnotationLabel `json:"label,omitempty"`
}
```

#### XAxisBorderConfig

```go
type XAxisBorderConfig struct {
    Show *bool `json:"show,omitempty"`
    Color *string `json:"color,omitempty"`
    Height *int `json:"height,omitempty"`
    Width interface{} `json:"width,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
}
```

#### XAxisBorderType

#### XAxisConfig

```go
type XAxisConfig struct {
    Type *XAxisType `json:"type,omitempty"`
    Categories []string `json:"categories,omitempty"`
    TickAmount interface{} `json:"tickAmount,omitempty"`
    TickPlacement *XAxisTickPlacement `json:"tickPlacement,omitempty"`
    Min *float64 `json:"min,omitempty"`
    Max *float64 `json:"max,omitempty"`
    StepSize *float64 `json:"stepSize,omitempty"`
    Range *float64 `json:"range,omitempty"`
    Floating *bool `json:"floating,omitempty"`
    DecimalsInFloat *int `json:"decimalsInFloat,omitempty"`
    OverwriteCategories []string `json:"overwriteCategories,omitempty"`
    Position *XAxisPosition `json:"position,omitempty"`
    Labels *XAxisLabelsConfig `json:"labels,omitempty"`
    Group *XAxisGroupConfig `json:"group,omitempty"`
    AxisBorder *XAxisBorderConfig `json:"axisBorder,omitempty"`
    AxisTicks *XAxisTicksConfig `json:"axisTicks,omitempty"`
    Title *XAxisTitleConfig `json:"title,omitempty"`
    Crosshairs *XAxisCrosshairsConfig `json:"crosshairs,omitempty"`
    Tooltip *XAxisTooltipConfig `json:"tooltip,omitempty"`
}
```

#### XAxisCrosshairFillType

#### XAxisCrosshairPosition

#### XAxisCrosshairsConfig

```go
type XAxisCrosshairsConfig struct {
    Show *bool `json:"show,omitempty"`
    Width interface{} `json:"width,omitempty"`
    Position *XAxisCrosshairPosition `json:"position,omitempty"`
    Opacity *float64 `json:"opacity,omitempty"`
    Stroke *XAxisCrosshairsStrokeConfig `json:"stroke,omitempty"`
    Fill *XAxisCrosshairsFillConfig `json:"fill,omitempty"`
    DropShadow *DropShadow `json:"dropShadow,omitempty"`
}
```

#### XAxisCrosshairsFillConfig

```go
type XAxisCrosshairsFillConfig struct {
    Type *XAxisCrosshairFillType `json:"type,omitempty"`
    Color *string `json:"color,omitempty"`
    Gradient *XAxisCrosshairsGradientConfig `json:"gradient,omitempty"`
}
```

#### XAxisCrosshairsGradientConfig

```go
type XAxisCrosshairsGradientConfig struct {
    ColorFrom *string `json:"colorFrom,omitempty"`
    ColorTo *string `json:"colorTo,omitempty"`
    Stops []float64 `json:"stops,omitempty"`
    OpacityFrom *float64 `json:"opacityFrom,omitempty"`
    OpacityTo *float64 `json:"opacityTo,omitempty"`
}
```

#### XAxisCrosshairsStrokeConfig

```go
type XAxisCrosshairsStrokeConfig struct {
    Color *string `json:"color,omitempty"`
    Width *int `json:"width,omitempty"`
    DashArray *int `json:"dashArray,omitempty"`
}
```

#### XAxisDateTimeFormatterConfig

```go
type XAxisDateTimeFormatterConfig struct {
    Year *string `json:"year,omitempty"`
    Month *string `json:"month,omitempty"`
    Day *string `json:"day,omitempty"`
    Hour *string `json:"hour,omitempty"`
    Minute *string `json:"minute,omitempty"`
    Second *string `json:"second,omitempty"`
}
```

#### XAxisGroupConfig

```go
type XAxisGroupConfig struct {
    Groups []XAxisGroupItemConfig `json:"groups,omitempty"`
    Style *XAxisGroupStyleConfig `json:"style,omitempty"`
}
```

#### XAxisGroupItemConfig

```go
type XAxisGroupItemConfig struct {
    Title string `json:"title"`
    Cols int `json:"cols"`
}
```

#### XAxisGroupStyleConfig

```go
type XAxisGroupStyleConfig struct {
    Colors interface{} `json:"colors,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontWeight interface{} `json:"fontWeight,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    CSSClass *string `json:"cssClass,omitempty"`
}
```

#### XAxisLabelStyleConfig

```go
type XAxisLabelStyleConfig struct {
    Colors interface{} `json:"colors,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    FontWeight interface{} `json:"fontWeight,omitempty"`
    CSSClass *string `json:"cssClass,omitempty"`
}
```

#### XAxisLabelsConfig

```go
type XAxisLabelsConfig struct {
    Show *bool `json:"show,omitempty"`
    Rotate *int `json:"rotate,omitempty"`
    RotateAlways *bool `json:"rotateAlways,omitempty"`
    HideOverlappingLabels *bool `json:"hideOverlappingLabels,omitempty"`
    ShowDuplicates *bool `json:"showDuplicates,omitempty"`
    Trim *bool `json:"trim,omitempty"`
    MinHeight *int `json:"minHeight,omitempty"`
    MaxHeight *int `json:"maxHeight,omitempty"`
    Style *XAxisLabelStyleConfig `json:"style,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Format *string `json:"format,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
    DatetimeUTC *bool `json:"datetimeUTC,omitempty"`
    DatetimeFormatter *XAxisDateTimeFormatterConfig `json:"datetimeFormatter,omitempty"`
}
```

#### XAxisPosition

#### XAxisTickPlacement

#### XAxisTicksConfig

```go
type XAxisTicksConfig struct {
    Show *bool `json:"show,omitempty"`
    BorderType *XAxisBorderType `json:"borderType,omitempty"`
    Color *string `json:"color,omitempty"`
    Height *int `json:"height,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
}
```

#### XAxisTitleConfig

```go
type XAxisTitleConfig struct {
    Text *string `json:"text,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Style *XAxisTitleStyleConfig `json:"style,omitempty"`
}
```

#### XAxisTitleStyleConfig

```go
type XAxisTitleStyleConfig struct {
    Color *string `json:"color,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    FontWeight interface{} `json:"fontWeight,omitempty"`
    CSSClass *string `json:"cssClass,omitempty"`
}
```

#### XAxisTooltipConfig

```go
type XAxisTooltipConfig struct {
    Enabled *bool `json:"enabled,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Style *XAxisTooltipStyleConfig `json:"style,omitempty"`
}
```

#### XAxisTooltipStyleConfig

```go
type XAxisTooltipStyleConfig struct {
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
}
```

#### XAxisType

#### YAxisAnnotation

```go
type YAxisAnnotation struct {
    Y *float64 `json:"y,omitempty"`
    Y2 *float64 `json:"y2,omitempty"`
    StrokeDashArray *int `json:"strokeDashArray,omitempty"`
    FillColor *string `json:"fillColor,omitempty"`
    BorderColor *string `json:"borderColor,omitempty"`
    BorderWidth *int `json:"borderWidth,omitempty"`
    Opacity *float64 `json:"opacity,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    YAxisIndex *int `json:"yAxisIndex,omitempty"`
    Label *AnnotationLabel `json:"label,omitempty"`
}
```

#### YAxisBorderConfig

```go
type YAxisBorderConfig struct {
    Show *bool `json:"show,omitempty"`
    Color *string `json:"color,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
}
```

#### YAxisConfig

```go
type YAxisConfig struct {
    Show *bool `json:"show,omitempty"`
    ShowAlways *bool `json:"showAlways,omitempty"`
    ShowForNullSeries *bool `json:"showForNullSeries,omitempty"`
    SeriesName interface{} `json:"seriesName,omitempty"`
    Opposite *bool `json:"opposite,omitempty"`
    Reversed *bool `json:"reversed,omitempty"`
    Logarithmic *bool `json:"logarithmic,omitempty"`
    LogBase *int `json:"logBase,omitempty"`
    TickAmount *int `json:"tickAmount,omitempty"`
    Min interface{} `json:"min,omitempty"`
    Max interface{} `json:"max,omitempty"`
    StepSize *float64 `json:"stepSize,omitempty"`
    ForceNiceScale *bool `json:"forceNiceScale,omitempty"`
    Floating *bool `json:"floating,omitempty"`
    DecimalsInFloat *int `json:"decimalsInFloat,omitempty"`
    Labels *YAxisLabelsConfig `json:"labels,omitempty"`
    AxisBorder *YAxisBorderConfig `json:"axisBorder,omitempty"`
    AxisTicks *YAxisTicksConfig `json:"axisTicks,omitempty"`
    Title *YAxisTitleConfig `json:"title,omitempty"`
    Crosshairs *YAxisCrosshairsConfig `json:"crosshairs,omitempty"`
    Tooltip *YAxisTooltipConfig `json:"tooltip,omitempty"`
}
```

#### YAxisCrosshairsConfig

```go
type YAxisCrosshairsConfig struct {
    Show *bool `json:"show,omitempty"`
    Position *XAxisCrosshairPosition `json:"position,omitempty"`
    Stroke *YAxisCrosshairsStrokeConfig `json:"stroke,omitempty"`
}
```

#### YAxisCrosshairsStrokeConfig

```go
type YAxisCrosshairsStrokeConfig struct {
    Color *string `json:"color,omitempty"`
    Width *int `json:"width,omitempty"`
    DashArray *int `json:"dashArray,omitempty"`
}
```

#### YAxisLabelAlign

#### YAxisLabelStyleConfig

```go
type YAxisLabelStyleConfig struct {
    Colors interface{} `json:"colors,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    FontWeight interface{} `json:"fontWeight,omitempty"`
    CSSClass *string `json:"cssClass,omitempty"`
}
```

#### YAxisLabelsConfig

```go
type YAxisLabelsConfig struct {
    Show *bool `json:"show,omitempty"`
    ShowDuplicates *bool `json:"showDuplicates,omitempty"`
    Align *YAxisLabelAlign `json:"align,omitempty"`
    MinWidth *int `json:"minWidth,omitempty"`
    MaxWidth *int `json:"maxWidth,omitempty"`
    Style *YAxisLabelStyleConfig `json:"style,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Rotate *int `json:"rotate,omitempty"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
}
```

#### YAxisTicksConfig

```go
type YAxisTicksConfig struct {
    Show *bool `json:"show,omitempty"`
    BorderType *XAxisBorderType `json:"borderType,omitempty"`
    Color *string `json:"color,omitempty"`
    Width *int `json:"width,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
}
```

#### YAxisTitleConfig

```go
type YAxisTitleConfig struct {
    Text *string `json:"text,omitempty"`
    Rotate *int `json:"rotate,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
    OffsetY *int `json:"offsetY,omitempty"`
    Style *YAxisTitleStyleConfig `json:"style,omitempty"`
}
```

#### YAxisTitleStyleConfig

```go
type YAxisTitleStyleConfig struct {
    Color *string `json:"color,omitempty"`
    FontSize *string `json:"fontSize,omitempty"`
    FontFamily *string `json:"fontFamily,omitempty"`
    FontWeight interface{} `json:"fontWeight,omitempty"`
    CSSClass *string `json:"cssClass,omitempty"`
}
```

#### YAxisTooltipConfig

```go
type YAxisTooltipConfig struct {
    Enabled *bool `json:"enabled,omitempty"`
    OffsetX *int `json:"offsetX,omitempty"`
}
```

### Functions

#### `func Chart(props Props) templ.Component`

Chart renders a chart with the specified options.
It generates a random ID for the chart container and initializes
the ApexCharts library to render the chart on the client side.


### Variables and Constants

---

## Package `filters` (components/filters)

templ: version: v0.3.857


### Types

#### DrawerProps

```go
type DrawerProps struct {
    Heading string
    Action string
}
```

#### Props

Props defines configuration options for the Default filter component.


```go
type Props struct {
    Fields []SearchField
}
```

#### SearchField

SearchField represents a field that can be searched on.


```go
type SearchField struct {
    Label string
    Key string
}
```

### Functions

#### `func CreatedAt() templ.Component`

CreatedAt renders a date range filter for filtering by creation date.
It provides common options like today, yesterday, this week, etc.


#### `func Default(props *Props) templ.Component`

Default renders a complete filter bar with search, page size, and date filters.
It combines multiple filter components into a single interface.


#### `func Drawer(props DrawerProps) templ.Component`

#### `func PageSize() templ.Component`

PageSize renders a select dropdown for choosing the number of items per page.


#### `func Search(fields []SearchField) templ.Component`

Search renders a search input with field selection.
It includes a search icon and allows selecting which field to search on.


#### `func SearchFields(fields []SearchField) templ.Component`

SearchFields renders a dropdown list of available search fields.
For a single field, it creates a hidden select. For multiple fields,
it creates a combobox for selecting which field to search on.


#### `func SearchFieldsTrigger(trigger *base.TriggerProps) templ.Component`

### Variables and Constants

---

## Package `illustrations` (components/illustrations)

templ: version: v0.3.857


### Types

#### EmptyTableProps

```go
type EmptyTableProps struct {
    Width int
    Height int
}
```

### Functions

#### `func EmptyTable(props EmptyTableProps) templ.Component`

### Variables and Constants

---

## Package `loaders` (components/loaders)

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### SkeletonProps

```go
type SkeletonProps struct {
    ContainerClass templ.CSSClasses
    SkeletonClass templ.CSSClasses
    Lines int
}
```

#### SpinnerProps

```go
type SpinnerProps struct {
    ContainerClass templ.CSSClasses
    SpinnerClass templ.CSSClasses
}
```

### Functions

#### `func Hand() templ.Component`

Hand renders a hand-shaped loading animation.
It's a stylized animation for use during loading states, providing
visual feedback to users while content or data is being processed.


#### `func Skeleton(props SkeletonProps) templ.Component`

#### `func SkeletonCard(props SkeletonProps) templ.Component`

#### `func SkeletonTable(props SkeletonProps) templ.Component`

#### `func SkeletonText(props SkeletonProps) templ.Component`

#### `func Spinner(props SpinnerProps) templ.Component`

### Variables and Constants

---

## Package `filters` (components/scaffold/filters)

templ: version: v0.3.857


### Types

#### DropdownItemProps

```go
type DropdownItemProps struct {
    Class templ.CSSClasses
    Label string
    Value string
    Name string
    Checked bool
}
```

#### DropdownProps

```go
type DropdownProps struct {
    Label string
    Name string
}
```

#### Option

#### OptionItem

```go
type OptionItem struct {
    Value string
    Label string
}
```

#### TableFilter

```go
type TableFilter struct {
    Name string
}
```

##### Methods

- `func (TableFilter) Add(opts ...OptionItem) *TableFilter`

- `func (TableFilter) AsSideFilter() templ.Component`

- `func (TableFilter) Component() templ.Component`

### Functions

#### `func Dropdown(props DropdownProps) templ.Component`

#### `func DropdownItem(props DropdownItemProps) templ.Component`

### Variables and Constants

---

## Package `form` (components/scaffold/form)

templ: version: v0.3.857


### Types

#### CheckboxField

CheckboxField for boolean inputs


##### Interface Methods

- `<?>`

#### CheckboxFieldBuilder

CheckboxFieldBuilder builds a CheckboxField


##### Methods

- `func (CheckboxFieldBuilder) Attrs(a templ.Attributes) *CheckboxFieldBuilder`

- `func (CheckboxFieldBuilder) Build() CheckboxField`

- `func (CheckboxFieldBuilder) Default(val bool) *CheckboxFieldBuilder`

- `func (CheckboxFieldBuilder) Required() *CheckboxFieldBuilder`

- `func (CheckboxFieldBuilder) Validators(v []Validator) *CheckboxFieldBuilder`

#### ColorField

ColorField for color inputs


##### Interface Methods

- `<?>`

#### ColorFieldBuilder

##### Methods

- `func (ColorFieldBuilder) Attrs(a templ.Attributes) *ColorFieldBuilder`

- `func (ColorFieldBuilder) Build() ColorField`

- `func (ColorFieldBuilder) Default(val string) *ColorFieldBuilder`

- `func (ColorFieldBuilder) Required() *ColorFieldBuilder`

- `func (ColorFieldBuilder) Validators(v []Validator) *ColorFieldBuilder`

#### DateField

DateField for date inputs


##### Interface Methods

- `<?>`
- `Min() time.Time`
- `Max() time.Time`

#### DateFieldBuilder

DateFieldBuilder builds a DateField


##### Methods

- `func (DateFieldBuilder) Attrs(a templ.Attributes) *DateFieldBuilder`

- `func (DateFieldBuilder) Build() DateField`

- `func (DateFieldBuilder) Default(val time.Time) *DateFieldBuilder`

- `func (DateFieldBuilder) Max(val time.Time) *DateFieldBuilder`

- `func (DateFieldBuilder) Min(val time.Time) *DateFieldBuilder`

- `func (DateFieldBuilder) Required() *DateFieldBuilder`

- `func (DateFieldBuilder) Validators(v []Validator) *DateFieldBuilder`

#### DateTimeLocalField

DateTimeLocalField for datetime-local inputs


##### Interface Methods

- `<?>`
- `Min() time.Time`
- `Max() time.Time`

#### DateTimeLocalFieldBuilder

DateTimeLocalFieldBuilder builds a DateTimeLocalField


##### Methods

- `func (DateTimeLocalFieldBuilder) Attrs(a templ.Attributes) *DateTimeLocalFieldBuilder`

- `func (DateTimeLocalFieldBuilder) Build() DateTimeLocalField`

- `func (DateTimeLocalFieldBuilder) Default(val time.Time) *DateTimeLocalFieldBuilder`

- `func (DateTimeLocalFieldBuilder) Max(val time.Time) *DateTimeLocalFieldBuilder`

- `func (DateTimeLocalFieldBuilder) Min(val time.Time) *DateTimeLocalFieldBuilder`

- `func (DateTimeLocalFieldBuilder) Required() *DateTimeLocalFieldBuilder`

- `func (DateTimeLocalFieldBuilder) Validators(v []Validator) *DateTimeLocalFieldBuilder`

#### EmailField

EmailField for email inputs


##### Interface Methods

- `<?>`

#### EmailFieldBuilder

EmailFieldBuilder builds an EmailField


##### Methods

- `func (EmailFieldBuilder) Attrs(a templ.Attributes) *EmailFieldBuilder`

- `func (EmailFieldBuilder) Build() EmailField`

- `func (EmailFieldBuilder) Default(val string) *EmailFieldBuilder`

- `func (EmailFieldBuilder) Required() *EmailFieldBuilder`

- `func (EmailFieldBuilder) Validators(v []Validator) *EmailFieldBuilder`

#### Field

Field defines minimal metadata for form inputs and rendering


##### Interface Methods

- `Component() templ.Component`
- `Type() FieldType`
- `Key() string`
- `Label() string`
- `Required() bool`
- `Attrs() templ.Attributes`
- `Validators() []Validator`

#### FieldType

--- FieldType enumerates supported input types ---


#### FormConfig

FormConfig holds the configuration for a dynamic form


```go
type FormConfig struct {
    Title string
    SaveURL string
    DeleteURL string
    SubmitLabel string
    Fields []Field
}
```

##### Methods

- `func (FormConfig) Add(fields ...Field) *FormConfig`
  Add appends one or more Field implementations to the form and returns the config
  

- `func (FormConfig) WithMethod(method string) *FormConfig`
  WithMethod sets the HTTP method for the form
  

#### GenericField

GenericField defines minimal metadata for form inputs and rendering


##### Interface Methods

- `Field`
- `Default() T`
- `WithValue(value T) <?>`
- `Value() T`

#### MonthField

MonthField for month inputs


##### Interface Methods

- `<?>`
- `Min() string`
- `Max() string`

#### MonthFieldBuilder

MonthFieldBuilder builds a MonthField


##### Methods

- `func (MonthFieldBuilder) Attrs(a templ.Attributes) *MonthFieldBuilder`

- `func (MonthFieldBuilder) Build() MonthField`

- `func (MonthFieldBuilder) Default(val string) *MonthFieldBuilder`

- `func (MonthFieldBuilder) Max(val string) *MonthFieldBuilder`

- `func (MonthFieldBuilder) Min(val string) *MonthFieldBuilder`

- `func (MonthFieldBuilder) Required() *MonthFieldBuilder`

- `func (MonthFieldBuilder) Validators(v []Validator) *MonthFieldBuilder`

#### NumberField

NumberField for numeric inputs


##### Interface Methods

- `<?>`
- `Min() float64`
- `Max() float64`

#### NumberFieldBuilder

NumberFieldBuilder builds a NumberField


##### Methods

- `func (NumberFieldBuilder) Attrs(a templ.Attributes) *NumberFieldBuilder`

- `func (NumberFieldBuilder) Build() NumberField`

- `func (NumberFieldBuilder) Default(val float64) *NumberFieldBuilder`

- `func (NumberFieldBuilder) Max(val float64) *NumberFieldBuilder`

- `func (NumberFieldBuilder) Min(val float64) *NumberFieldBuilder`

- `func (NumberFieldBuilder) Required() *NumberFieldBuilder`

- `func (NumberFieldBuilder) Validators(v []Validator) *NumberFieldBuilder`

#### Option

Option for SelectField and RadioField choices


```go
type Option struct {
    Value string
    Label string
}
```

#### RadioField

RadioField for radio button inputs


##### Interface Methods

- `<?>`
- `Options() []Option`

#### RadioFieldBuilder

RadioFieldBuilder builds a RadioField


##### Methods

- `func (RadioFieldBuilder) Attrs(a templ.Attributes) *RadioFieldBuilder`

- `func (RadioFieldBuilder) Build() RadioField`

- `func (RadioFieldBuilder) Default(val string) *RadioFieldBuilder`

- `func (RadioFieldBuilder) Options(opts []Option) *RadioFieldBuilder`

- `func (RadioFieldBuilder) Required() *RadioFieldBuilder`

- `func (RadioFieldBuilder) Validators(v []Validator) *RadioFieldBuilder`

#### SelectField

SelectField for dropdowns


##### Interface Methods

- `<?>`
- `Options() []Option`

#### SelectFieldBuilder

SelectFieldBuilder builds a SelectField


##### Methods

- `func (SelectFieldBuilder) Attrs(a templ.Attributes) *SelectFieldBuilder`

- `func (SelectFieldBuilder) Build() SelectField`

- `func (SelectFieldBuilder) Default(val string) *SelectFieldBuilder`

- `func (SelectFieldBuilder) Options(opts []Option) *SelectFieldBuilder`

- `func (SelectFieldBuilder) Required() *SelectFieldBuilder`

- `func (SelectFieldBuilder) Validators(v []Validator) *SelectFieldBuilder`

#### TelField

TelField for telephone inputs


##### Interface Methods

- `<?>`

#### TelFieldBuilder

TelFieldBuilder builds a TelField


##### Methods

- `func (TelFieldBuilder) Attrs(a templ.Attributes) *TelFieldBuilder`

- `func (TelFieldBuilder) Build() TelField`

- `func (TelFieldBuilder) Default(val string) *TelFieldBuilder`

- `func (TelFieldBuilder) Required() *TelFieldBuilder`

- `func (TelFieldBuilder) Validators(v []Validator) *TelFieldBuilder`

#### TextField

TextField for single-line text inputs


##### Interface Methods

- `<?>`
- `MinLength() int`
- `MaxLength() int`

#### TextFieldBuilder

TextFieldBuilder builds a TextField


##### Methods

- `func (TextFieldBuilder) Attrs(a templ.Attributes) *TextFieldBuilder`

- `func (TextFieldBuilder) Build() TextField`

- `func (TextFieldBuilder) Default(val string) *TextFieldBuilder`

- `func (TextFieldBuilder) MaxLen(v int) *TextFieldBuilder`

- `func (TextFieldBuilder) MinLen(v int) *TextFieldBuilder`

- `func (TextFieldBuilder) Required() *TextFieldBuilder`

- `func (TextFieldBuilder) Validators(v []Validator) *TextFieldBuilder`

#### TextareaField

TextareaField for multi-line text inputs


##### Interface Methods

- `<?>`
- `MinLength() int`
- `MaxLength() int`

#### TextareaFieldBuilder

TextareaFieldBuilder builds a TextareaField


##### Methods

- `func (TextareaFieldBuilder) Attrs(a templ.Attributes) *TextareaFieldBuilder`

- `func (TextareaFieldBuilder) Build() TextareaField`

- `func (TextareaFieldBuilder) Default(val string) *TextareaFieldBuilder`

- `func (TextareaFieldBuilder) MaxLen(v int) *TextareaFieldBuilder`

- `func (TextareaFieldBuilder) MinLen(v int) *TextareaFieldBuilder`

- `func (TextareaFieldBuilder) Required() *TextareaFieldBuilder`

- `func (TextareaFieldBuilder) Validators(v []Validator) *TextareaFieldBuilder`

#### TimeField

TimeField for time inputs


##### Interface Methods

- `<?>`
- `Min() string`
- `Max() string`

#### TimeFieldBuilder

TimeFieldBuilder builds a TimeField


##### Methods

- `func (TimeFieldBuilder) Attrs(a templ.Attributes) *TimeFieldBuilder`

- `func (TimeFieldBuilder) Build() TimeField`

- `func (TimeFieldBuilder) Default(val string) *TimeFieldBuilder`

- `func (TimeFieldBuilder) Max(val string) *TimeFieldBuilder`

- `func (TimeFieldBuilder) Min(val string) *TimeFieldBuilder`

- `func (TimeFieldBuilder) Required() *TimeFieldBuilder`

- `func (TimeFieldBuilder) Validators(v []Validator) *TimeFieldBuilder`

#### URLField

URLField for URL inputs


##### Interface Methods

- `<?>`

#### URLFieldBuilder

URLFieldBuilder builds a URLField


##### Methods

- `func (URLFieldBuilder) Attrs(a templ.Attributes) *URLFieldBuilder`

- `func (URLFieldBuilder) Build() URLField`

- `func (URLFieldBuilder) Default(val string) *URLFieldBuilder`

- `func (URLFieldBuilder) Required() *URLFieldBuilder`

- `func (URLFieldBuilder) Validators(v []Validator) *URLFieldBuilder`

#### Validator

Validator for custom field-level checks


##### Interface Methods

- `Validate(ctx context.Context, value any) error`

### Functions

#### `func Form(cfg *FormConfig) templ.Component`

Form renders a dynamic form using a slice of scaffold.Field


#### `func FormFields(cfg *FormConfig) templ.Component`

FormFields renders all fields in the form


#### `func Page(cfg *FormConfig) templ.Component`

Page wraps Form in authenticated layout


### Variables and Constants

---

## Package `table` (components/scaffold/table)

templ: version: v0.3.857


### Types

#### ColumnOpt

#### DefaultDrawerProps

```go
type DefaultDrawerProps struct {
    Title string
    CallbackURL string
}
```

#### InfiniteScrollConfig

```go
type InfiniteScrollConfig struct {
    HasMore bool
    Page int
    PerPage int
}
```

#### RowOpt

#### TableColumn

##### Interface Methods

- `Key() string`
- `Label() string`
- `Class() string`
- `Width() string`

#### TableConfig

```go
type TableConfig struct {
    Title string
    DataURL string
    Filters []templ.Component
    Columns []TableColumn
    Rows []TableRow
    Infinite *InfiniteScrollConfig
    SideFilter templ.Component
}
```

##### Methods

- `func (TableConfig) AddCols(cols ...TableColumn) *TableConfig`

- `func (TableConfig) AddFilters(filters ...templ.Component) *TableConfig`

- `func (TableConfig) AddRows(rows ...TableRow) *TableConfig`

- `func (TableConfig) SetSideFilter(filter templ.Component) *TableConfig`

#### TableConfigOpt

#### TableRow

##### Interface Methods

- `Cells() []templ.Component`
- `Attrs() templ.Attributes`
- `ApplyOpts(opts ...RowOpt) TableRow`

### Functions

#### `func Content(config *TableConfig) templ.Component`

Content renders the complete scaffold page content with filters and table


#### `func DateTime(ts time.Time) templ.Component`

DateTime renders a timestamp with Alpine-based relative formatting


#### `func DefaultDrawer(props DefaultDrawerProps) templ.Component`

#### `func InfiniteScrollSpinner(cfg *TableConfig) templ.Component`

#### `func Page(config *TableConfig) templ.Component`

Page renders a complete authenticated page with the scaffolded content


#### `func Rows(cfg *TableConfig) templ.Component`

Rows renders the table rows for a scaffold table


#### `func Table(config *TableConfig) templ.Component`

Table renders a dynamic table based on configuration and data


#### `func TableSection(config *TableConfig) templ.Component`

TableSection combines filters and table into one form to enable unified HTMX update


### Variables and Constants

---

## Package `selects` (components/selects)

### Types

#### CountriesSelectProps

CountriesSelectProps defines the properties for the CountriesSelect component.


```go
type CountriesSelectProps struct {
    Label string
    Placeholder string
    Name string
    Selected string
    Error string
    Required bool
    Class string
    Attrs templ.Attributes
}
```

### Functions

#### `func CountriesSelect(props CountriesSelectProps) templ.Component`

CountriesSelect renders a select dropdown with a list of countries.
Countries are translated according to the current locale.


### Variables and Constants

---

## Package `sidebar` (components/sidebar)

templ: version: v0.3.857

Package sidebar provides navigation components for application layout.


### Types

#### Group

Group represents a collection of navigation items that can be expanded/collapsed.


##### Interface Methods

- `ID() string`
- `IsLink() bool`
- `Position() int`
- `Text() string`
- `Icon() templ.Component`
- `Children() []Item`
- `IsActive(ctx context.Context) bool`
- `SetPosition(position int) Group`

#### Item

Item is the base interface for navigation elements in the sidebar.


##### Interface Methods

- `IsLink() bool`
- `Position() int`
- `Icon() templ.Component`
- `IsActive(ctx context.Context) bool`

#### Link

Link represents a navigation link in the sidebar.


##### Interface Methods

- `IsLink() bool`
- `Position() int`
- `Href() string`
- `Text() string`
- `Icon() templ.Component`
- `IsActive(ctx context.Context) bool`
- `SetPosition(position int) Link`

#### Props

```go
type Props struct {
    Header templ.Component
    Items []Item
    Footer templ.Component
}
```

### Functions

#### `func AccordionGroup(group Group) templ.Component`

#### `func AccordionLink(link Link) templ.Component`

#### `func Sidebar(props Props) templ.Component`

### Variables and Constants

---

## Package `spotlight` (components/spotlight)

### Functions

#### `func LinkItem(title, link string, icon templ.Component) templ.Component`

#### `func NotFound() templ.Component`

NotFound renders a message indicating that no search results were found.


#### `func Spotlight() templ.Component`

Spotlight renders a search dialog component that can be triggered
with a button click or keyboard shortcut.


#### `func SpotlightItem(i int) templ.Component`

SpotlightItem renders a single item in the Spotlight search results.


#### `func SpotlightItems(items []templ.Component, startIdx int) templ.Component`

SpotlightItems renders a list of search results in the Spotlight component.
If no items are found, it displays a "nothing found" message.


### Variables and Constants

---

## Package `usercomponents` (components/user)

### Types

#### LanguageSelectProps

LanguageSelectProps defines the properties for the LanguageSelect component.


```go
type LanguageSelectProps struct {
    Label string
    Placeholder string
    Value string
    Error string
    Attrs templ.Attributes
}
```

### Functions

#### `func LanguageSelect(props *LanguageSelectProps) templ.Component`

LanguageSelect renders a dropdown for selecting the application language.
It displays all supported languages with their verbose names.


### Variables and Constants

---

## Package `assets` (internal/assets)

### Variables and Constants

- Var: `[FS]`

- Var: `[HashFS]`

---

## Package `server` (internal/server)

### Types

#### DefaultOptions

```go
type DefaultOptions struct {
    Logger *logrus.Logger
    Configuration *configuration.Configuration
    Application application.Application
    Pool *pgxpool.Pool
}
```

### Functions

#### `func Default(options *DefaultOptions) (*server.HTTPServer, error)`

---

## Package `application` (pkg/application)

### Types

#### Application

Application with a dynamically extendable service registry


##### Interface Methods

- `DB() *pgxpool.Pool`
- `EventPublisher() eventbus.EventBus`
- `Controllers() []Controller`
- `Middleware() []mux.MiddlewareFunc`
- `Assets() []*embed.FS`
- `HashFsAssets() []*hashfs.FS`
- `RBAC() rbac.RBAC`
- `Spotlight() spotlight.Spotlight`
- `QuickLinks() *spotlight.QuickLinks`
- `Migrations() MigrationManager`
- `NavItems(localizer *i18n.Localizer) []types.NavigationItem`
- `RegisterNavItems(items ...types.NavigationItem)`
- `RegisterControllers(controllers ...Controller)`
- `RegisterHashFsAssets(fs ...*hashfs.FS)`
- `RegisterAssets(fs ...*embed.FS)`
- `RegisterLocaleFiles(fs ...*embed.FS)`
- `RegisterGraphSchema(schema GraphSchema)`
- `GraphSchemas() []GraphSchema`
- `RegisterServices(services ...interface{})`
- `RegisterMiddleware(middleware ...mux.MiddlewareFunc)`
- `Service(service interface{}) interface{}`
- `Services() map[reflect.Type]interface{}`
- `Bundle() *i18n.Bundle`

#### Controller

##### Interface Methods

- `Register(r *mux.Router)`
- `Key() string`

#### GraphSchema

```go
type GraphSchema struct {
    Value graphql.ExecutableSchema
    BasePath string
    ExecutorCb func(*executor.Executor)
}
```

#### MigrationManager

MigrationManager is an interface for handling database migrations


##### Interface Methods

- `CollectSchema(ctx context.Context) error`
- `Run() error`
- `Rollback() error`
- `RegisterSchema(fs ...*embed.FS)`
- `SchemaFSs() []*embed.FS`

#### Module

##### Interface Methods

- `Name() string`
- `Register(app Application) error`

#### SeedFunc

#### Seeder

##### Interface Methods

- `Seed(ctx context.Context, app Application) error`
- `Register(funcs ...SeedFunc)`

### Functions

### Variables and Constants

- Var: `[ErrAppNotFound]`

---

## Package `commands` (pkg/commands)

### Types

#### LogCollector

```go
type LogCollector struct {
    LokiURL string
    AppName string
    LogPath string
    BatchSize int
    Timeout time.Duration
    Labels []string
}
```

##### Methods

- `func (LogCollector) Process(ctx context.Context) error`
  Process continuously monitors the log file and sends batches to Loki
  

- `func (LogCollector) SendBatch(ctx context.Context, client *http.Client, batch []map[string]interface{}) error`
  SendBatch sends a batch of log entries to Loki
  

#### LokiPush

```go
type LokiPush struct {
    Streams []LokiStream `json:"streams"`
}
```

#### LokiStream

```go
type LokiStream struct {
    Stream map[string]string `json:"stream"`
    Values [][2]string `json:"values"`
}
```

### Functions

#### `func CheckTrKeys(mods ...application.Module) error`

#### `func CollectLogs(ctx context.Context, options ...func(*LogCollector)) error`

CollectLogs initializes and runs a log collector that forwards logs to Loki


#### `func Migrate(mods ...application.Module) error`

#### `func WithBatchSize(batchSize int) func(*LogCollector)`

WithBatchSize allows customizing the batch size for sending logs


#### `func WithLabels(labels []string) func(*LogCollector)`

WithLabels allows customizing the labels to extract from log entries


#### `func WithLogPath(logPath string) func(*LogCollector)`

WithLogPath allows customizing the log file path


#### `func WithTimeout(timeout time.Duration) func(*LogCollector)`

WithTimeout allows customizing the timeout for HTTP requests


### Variables and Constants

- Var: `[ErrNoCommand]`

---

## Package `composables` (pkg/composables)

### Types

#### PaginationParams

```go
type PaginationParams struct {
    Limit int
    Offset int
    Page int
}
```

#### Params

```go
type Params struct {
    IP string
    UserAgent string
    Authenticated bool
    Request *http.Request
    Writer http.ResponseWriter
}
```

#### Tenant

```go
type Tenant struct {
    ID uuid.UUID
    Name string
    Domain string
}
```

### Functions

#### `func BeginTx(ctx context.Context) (pgx.Tx, error)`

#### `func CanUser(ctx context.Context, permission *permission.Permission) error`

#### `func CanUserAll(ctx context.Context, perms ...rbac.Permission) error`

#### `func CanUserAny(ctx context.Context, perms ...rbac.Permission) error`

CanUserAny checks if the user has any of the given permissions (OR logic)


#### `func InTx(ctx context.Context, fn func(context.Context) error) error`

#### `func MustUseHead(ctx context.Context) templ.Component`

MustUseHead returns the head component from the context or panics


#### `func MustUseLogo(ctx context.Context) templ.Component`

MustUseLogo returns the logo component from the context or panics


#### `func MustUseUser(ctx context.Context) user.User`

MustUseUser returns the user from the context. If no user is found, it panics.


#### `func UseAllNavItems(ctx context.Context) ([]types.NavigationItem, error)`

#### `func UseApp(ctx context.Context) (application.Application, error)`

UseApp returns the application from the context.


#### `func UseAuthenticated(ctx context.Context) bool`

UseAuthenticated returns whether the user is authenticated and the second return value is true.
If the user is not authenticated, the second return value is false.


#### `func UseFlash(w http.ResponseWriter, r *http.Request, name string) ([]byte, error)`

#### `func UseFlashMap(w http.ResponseWriter, r *http.Request, name string) (map[K]V, error)`

#### `func UseForm(v T, r *http.Request) (T, error)`

#### `func UseHead(ctx context.Context) (templ.Component, error)`

UseHead returns the head component from the context


#### `func UseIP(ctx context.Context) (string, bool)`

UseIP returns the IP address from the context.
If the IP address is not found, the second return value will be false.


#### `func UseLogger(ctx context.Context) *logrus.Entry`

UseLogger returns the logger from the context.
If the logger is not found, the second return value will be false.


#### `func UseLogo(ctx context.Context) (templ.Component, error)`

UseLogo returns the logo component from the context


#### `func UseNavItems(ctx context.Context) []types.NavigationItem`

#### `func UsePageCtx(ctx context.Context) *types.PageContext`

UsePageCtx returns the page context from the context.
If the page context is not found, function will panic.


#### `func UsePool(ctx context.Context) (*pgxpool.Pool, error)`

#### `func UseQuery(v T, r *http.Request) (T, error)`

#### `func UseSession(ctx context.Context) (*session.Session, error)`

UseSession returns the session from the context.


#### `func UseTabs(ctx context.Context) ([]*tab.Tab, error)`

#### `func UseTx(ctx context.Context) (repo.Tx, error)`

#### `func UseUser(ctx context.Context) (user.User, error)`

UseUser returns the user from the context.


#### `func UseUserAgent(ctx context.Context) (string, bool)`

UseUserAgent returns the user agent from the context.
If the user agent is not found, the second return value will be false.


#### `func UseWriter(ctx context.Context) (http.ResponseWriter, bool)`

UseWriter returns the response writer from the context.
If the response writer is not found, the second return value will be false.


#### `func WithPageCtx(ctx context.Context, pageCtx *types.PageContext) context.Context`

WithPageCtx returns a new context with the page context.


#### `func WithParams(ctx context.Context, params *Params) context.Context`

WithParams returns a new context with the request parameters.


#### `func WithPool(ctx context.Context, pool *pgxpool.Pool) context.Context`

#### `func WithSession(ctx context.Context, sess *session.Session) context.Context`

WithSession returns a new context with the session.


#### `func WithTenant(ctx context.Context, tenant *Tenant) context.Context`

#### `func WithTx(ctx context.Context, tx pgx.Tx) context.Context`

#### `func WithUser(ctx context.Context, u user.User) context.Context`

WithUser returns a new context with the user.


### Variables and Constants

- Var: `[ErrNoSessionFound ErrNoUserFound]`

- Var: `[ErrNoTx ErrNoPool]`

- Var: `[ErrInvalidPassword ErrNotFound ErrUnauthorized ErrForbidden ErrInternal]`

- Var: `[ErrNoLogoFound ErrNoHeadFound]`

- Var: `[ErrNavItemsNotFound]`

- Var: `[ErrNoLogger]`

- Var: `[ErrNoTenantFound]`

- Var: `[ErrTabsNotFound]`

---

## Package `configuration` (pkg/configuration)

### Types

#### ClickOptions

```go
type ClickOptions struct {
    URL string `env:"CLICK_URL" envDefault:"https://my.click.uz"`
    MerchantID int64 `env:"CLICK_MERCHANT_ID"`
    MerchantUserID int64 `env:"CLICK_MERCHANT_USER_ID"`
    ServiceID int64 `env:"CLICK_SERVICE_ID"`
    SecretKey string `env:"CLICK_SECRET_KEY"`
}
```

#### Configuration

```go
type Configuration struct {
    Database DatabaseOptions
    Google GoogleOptions
    Twilio TwilioOptions
    Loki LokiOptions
    OpenTelemetry OpenTelemetryOptions
    Click ClickOptions
    Payme PaymeOptions
    Octo OctoOptions
    Stripe StripeOptions
    MigrationsDir string `env:"MIGRATIONS_DIR" envDefault:"migrations"`
    ServerPort int `env:"PORT" envDefault:"3200"`
    SessionDuration time.Duration `env:"SESSION_DURATION" envDefault:"720h"`
    GoAppEnvironment string `env:"GO_APP_ENV" envDefault:"development"`
    SocketAddress string `env:"-"`
    OpenAIKey string `env:"OPENAI_KEY"`
    UploadsPath string `env:"UPLOADS_PATH" envDefault:"static"`
    Domain string `env:"DOMAIN" envDefault:"localhost"`
    Origin string `env:"ORIGIN" envDefault:"http://localhost:3200"`
    PageSize int `env:"PAGE_SIZE" envDefault:"25"`
    MaxPageSize int `env:"MAX_PAGE_SIZE" envDefault:"100"`
    LogLevel string `env:"LOG_LEVEL" envDefault:"error"`
    RequestIDHeader string `env:"REQUEST_ID_HEADER" envDefault:"X-Request-ID"`
    RealIPHeader string `env:"REAL_IP_HEADER" envDefault:"X-Real-IP"`
    SidCookieKey string `env:"SID_COOKIE_KEY" envDefault:"sid"`
    OauthStateCookieKey string `env:"OAUTH_STATE_COOKIE_KEY" envDefault:"oauthState"`
    TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN"`
}
```

##### Methods

- `func (Configuration) Address() string`

- `func (Configuration) Logger() *logrus.Logger`

- `func (Configuration) LogrusLogLevel() logrus.Level`

- `func (Configuration) Scheme() string`

- `func (Configuration) Unload()`
  unload handles a graceful shutdown.
  

#### DatabaseOptions

```go
type DatabaseOptions struct {
    Opts string `env:"-"`
    Name string `env:"DB_NAME" envDefault:"iota_erp"`
    Host string `env:"DB_HOST" envDefault:"localhost"`
    Port string `env:"DB_PORT" envDefault:"5432"`
    User string `env:"DB_USER" envDefault:"postgres"`
    Password string `env:"DB_PASSWORD" envDefault:"postgres"`
}
```

##### Methods

- `func (DatabaseOptions) ConnectionString() string`

#### GoogleOptions

```go
type GoogleOptions struct {
    RedirectURL string `env:"GOOGLE_REDIRECT_URL"`
    ClientID string `env:"GOOGLE_CLIENT_ID"`
    ClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
}
```

#### LokiOptions

```go
type LokiOptions struct {
    URL string `env:"LOKI_URL"`
    AppName string `env:"LOKI_APP_NAME" envDefault:"sdk"`
    LogPath string `env:"LOG_PATH" envDefault:"./logs/app.log"`
}
```

#### OctoOptions

```go
type OctoOptions struct {
    OctoShopID int32 `env:"OCTO_SHOP_ID"`
    OctoSecret string `env:"OCTO_SECRET"`
    OctoSecretHash string `env:"OCTO_SECRET_HASH"`
    NotifyUrl string `env:"OCTO_NOTIFY_URL"`
}
```

#### OpenTelemetryOptions

```go
type OpenTelemetryOptions struct {
    Enabled bool `env:"OTEL_ENABLED" envDefault:"false"`
    TempoURL string `env:"OTEL_TEMPO_URL" envDefault:"localhost:4318"`
    ServiceName string `env:"OTEL_SERVICE_NAME" envDefault:"sdk"`
}
```

#### PaymeOptions

```go
type PaymeOptions struct {
    URL string `env:"PAYME_URL" envDefault:"https://checkout.test.paycom.uz"`
    MerchantID string `env:"PAYME_MERCHANT_ID"`
    User string `env:"PAYME_USER" envDefault:"Paycom"`
    SecretKey string `env:"PAYME_SECRET_KEY"`
}
```

#### StripeOptions

```go
type StripeOptions struct {
    SecretKey string `env:"STRIPE_SECRET_KEY"`
    SigningSecret string `env:"STRIPE_SIGNING_SECRET"`
}
```

#### TwilioOptions

```go
type TwilioOptions struct {
    WebhookURL string `env:"TWILIO_WEBHOOK_URL"`
    AccountSID string `env:"TWILIO_ACCOUNT_SID"`
    AuthToken string `env:"TWILIO_AUTH_TOKEN"`
    PhoneNumber string `env:"TWILIO_PHONE_NUMBER"`
}
```

### Functions

#### `func LoadEnv(envFiles []string) (int, error)`

### Variables and Constants

- Const: `[Production]`

---

## Package `constants` (pkg/constants)

### Types

#### ContextKey

### Variables and Constants

- Var: `[Validate]`

---

## Package `crud` (pkg/crud)

### Types

#### DataStore

DataStore abstracts CRUD operations for type T with ID type ID


##### Interface Methods

- `List(ctx context.Context, params FindParams) ([]T, error)`
- `Get(ctx context.Context, id ID) (T, error)`
- `Create(ctx context.Context, entity T) (ID, error)`
- `Update(ctx context.Context, id ID, entity T) error`
- `Delete(ctx context.Context, id ID) error`

#### DefaultEntityFactory

DefaultEntityFactory is the default implementation of EntityFactory


##### Methods

- `func (DefaultEntityFactory) Create() T`
  Create instantiates a new entity of type T
  

#### DefaultEntityPatcher

DefaultEntityPatcher is the default implementation of EntityPatcher


##### Methods

- `func (DefaultEntityPatcher) Patch(entity T, formData map[string]string, fields []formui.Field) (T, ValidationError)`
  Patch applies form values to the entity
  

#### EntityFactory

EntityFactory creates new instances of entity type T


##### Interface Methods

- `Create() T`

#### EntityPatcher

EntityPatcher applies form values to an entity


##### Interface Methods

- `Patch(entity T, formData map[string]string, fields []formui.Field) (T, ValidationError)`

#### FieldError

FieldError represents a single field validation error


```go
type FieldError struct {
    Field string
    Message string
}
```

#### Filter

#### FindParams

FindParams defines pagination, sorting, searching, and filtering parameters


```go
type FindParams struct {
    Limit int
    Offset int
    Search string
    SortBy SortBy
    Filters []Filter
}
```

#### ModelLevelValidator

ModelLevelValidator for full-model checks


##### Interface Methods

- `ValidateModel(ctx context.Context, model T) error`

#### RenderFunc

RenderFunc abstracts the rendering logic to make it testable


#### Schema

Schema defines a runtime-driven CRUD resource for entity type T with identifier type ID.


```go
type Schema struct {
    Service *<?>
    Renderer RenderFunc
}
```

##### Methods

- `func (Schema) Key() string`
  Key returns the base path for routing identification
  

- `func (Schema) Register(r *mux.Router)`
  Register mounts CRUD HTTP handlers on the provided router
  

#### SchemaOpt

SchemaOpt configures optional settings on a Schema


#### Service

Service encapsulates the business logic of the CRUD operations


```go
type Service struct {
    Name string
    Path string
    IDField string
    Fields []formui.Field
    Store <?>
    EntityFactory <?>
    EntityPatcher <?>
    ModelValidators []<?>
}
```

##### Methods

- `func (Service) CreateEntity(ctx context.Context, formData map[string]string) (ID, error)`
  CreateEntity creates a new entity from form data
  

- `func (Service) DeleteEntity(ctx context.Context, id ID) error`
  DeleteEntity deletes an entity by ID
  

- `func (Service) Extract(entity T) map[string]string`
  Extract returns entity field values as map for UI rendering
  

- `func (Service) Get(ctx context.Context, id ID) (T, error)`
  Get retrieves a single entity by ID
  

- `func (Service) List(ctx context.Context, params FindParams) ([]T, error)`
  List retrieves entities based on the provided parameters
  

- `func (Service) UpdateEntity(ctx context.Context, id ID, formData map[string]string) error`
  UpdateEntity updates an existing entity from form data
  

#### SortBy

SortBy and Filter are generic aliases using Field


#### ValidationError

ValidationError collects field-level validation errors


```go
type ValidationError struct {
    Errors []FieldError
}
```

##### Methods

- `func (ValidationError) Error() string`

### Functions

#### `func DefaultGetPrimaryKey() (func() string)`

DefaultGetPrimaryKey returns a function that gets the primary key


#### `func DefaultRenderFunc(w http.ResponseWriter, r *http.Request, component templ.Component, options ...func(*templ.ComponentHandler))`

DefaultRenderFunc provides the default rendering implementation


### Variables and Constants

- Var: `[ErrNotFound]`
  ErrNotFound is returned when an entity is not found
  

- Var: `[ErrValidation]`
  ErrValidation is returned for validation errors
  

---

## Package `di` (pkg/di)

### Types

#### DIContext

DIContext holds context for dependency injection


##### Methods

- `func (DIContext) Invoke(ctx context.Context, fn interface{}) ([]reflect.Value, error)`
  Invoke resolves dependencies and calls the provided function with the given context
  

#### Provider

Provider is an interface that can provide a value for a given type


##### Interface Methods

- `Ok(t reflect.Type) bool`
- `Provide(t reflect.Type, ctx context.Context) (reflect.Value, error)`

### Functions

#### `func H(handler interface{}, customProviders ...Provider) http.HandlerFunc`

H creates a dependency injection HTTP handler


#### `func Invoke(ctx context.Context, fn interface{}, customProviders ...Provider) ([]reflect.Value, error)`

Invoke creates a generic DI function that can be used for any function type


---

## Package `document` (pkg/document)

### Types

#### Config

```go
type Config struct {
    SourceDir string
    OutputPath string
    Recursive bool
    ExcludeDirs []string
}
```

### Functions

#### `func Generate(config Config) error`

---

## Package `eventbus` (pkg/eventbus)

### Types

#### EventBus

##### Interface Methods

- `Publish(args ...interface{})`
- `Subscribe(handler interface{})`
- `Unsubscribe(handler interface{})`
- `Clear()`
- `SubscribersCount() int`

#### Subscriber

```go
type Subscriber struct {
    Handler interface{}
}
```

### Functions

#### `func MatchSignature(handler interface{}, args []interface{}) bool`

---

## Package `fp` (pkg/fp)

### Types

#### Lazy

Callback function that returns a specific value type


#### LazyVal

Callback function that takes an argument and return a value of the same type


### Functions

#### `func Compose10(fn10 func(T10) R, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 10 functions


#### `func Compose11(fn11 func(T11) R, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 11 functions


#### `func Compose12(fn12 func(T12) R, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 12 functions


#### `func Compose13(fn13 func(T13) R, fn12 func(T12) T13, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 13 functions


#### `func Compose14(fn14 func(T14) R, fn13 func(T13) T14, fn12 func(T12) T13, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 14 functions


#### `func Compose15(fn15 func(T15) R, fn14 func(T14) T15, fn13 func(T13) T14, fn12 func(T12) T13, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 15 functions


#### `func Compose16(fn16 func(T16) R, fn15 func(T15) T16, fn14 func(T14) T15, fn13 func(T13) T14, fn12 func(T12) T13, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 16 functions


#### `func Compose2(fn2 func(T2) R, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of two functions


#### `func Compose3(fn3 func(T3) R, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of three functions


#### `func Compose4(fn4 func(T4) R, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of four functions


#### `func Compose5(fn5 func(T5) R, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 5 functions


#### `func Compose6(fn6 func(T6) R, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 6 functions


#### `func Compose7(fn7 func(T7) R, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 7 functions


#### `func Compose8(fn8 func(T8) R, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 8 functions


#### `func Compose9(fn9 func(T9) R, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) (func(T1) R)`

Performs right-to-left function composition of 9 functions


#### `func Curry10(fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) (func(T9) (func(T10) R))))))))))`

Allow to transform a function that receives 10 params in a sequence of unary functions


#### `func Curry11(fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) (func(T9) (func(T10) (func(T11) R)))))))))))`

Allow to transform a function that receives 11 params in a sequence of unary functions


#### `func Curry12(fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) (func(T9) (func(T10) (func(T11) (func(T12) R))))))))))))`

Allow to transform a function that receives 12 params in a sequence of unary functions


#### `func Curry13(fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) (func(T9) (func(T10) (func(T11) (func(T12) (func(T13) R)))))))))))))`

Allow to transform a function that receives 13 params in a sequence of unary functions


#### `func Curry14(fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) (func(T9) (func(T10) (func(T11) (func(T12) (func(T13) (func(T14) R))))))))))))))`

Allow to transform a function that receives 14 params in a sequence of unary functions


#### `func Curry15(fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) (func(T9) (func(T10) (func(T11) (func(T12) (func(T13) (func(T14) (func(T15) R)))))))))))))))`

Allow to transform a function that receives 15 params in a sequence of unary functions


#### `func Curry16(fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) (func(T9) (func(T10) (func(T11) (func(T12) (func(T13) (func(T14) (func(T15) (func(T16) R))))))))))))))))`

Allow to transform a function that receives 16 params in a sequence of unary functions


#### `func Curry2(fn func(T1, T2) R) (func(T1) (func(T2) R))`

Allow to transform a function that receives 2 params in a sequence of unary functions


#### `func Curry3(fn func(T1, T2, T3) R) (func(T1) (func(T2) (func(T3) R)))`

Allow to transform a function that receives 3 params in a sequence of unary functions


#### `func Curry4(fn func(T1, T2, T3, T4) R) (func(T1) (func(T2) (func(T3) (func(T4) R))))`

Allow to transform a function that receives 4 params in a sequence of unary functions


#### `func Curry5(fn func(T1, T2, T3, T4, T5) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) R)))))`

Allow to transform a function that receives 5 params in a sequence of unary functions


#### `func Curry6(fn func(T1, T2, T3, T4, T5, T6) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) R))))))`

Allow to transform a function that receives 6 params in a sequence of unary functions


#### `func Curry7(fn func(T1, T2, T3, T4, T5, T6, T7) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) R)))))))`

Allow to transform a function that receives 7 params in a sequence of unary functions


#### `func Curry8(fn func(T1, T2, T3, T4, T5, T6, T7, T8) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) R))))))))`

Allow to transform a function that receives 8 params in a sequence of unary functions


#### `func Curry9(fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9) R) (func(T1) (func(T2) (func(T3) (func(T4) (func(T5) (func(T6) (func(T7) (func(T8) (func(T9) R)))))))))`

Allow to transform a function that receives 9 params in a sequence of unary functions


#### `func Every(predicate func(T) bool) (func([]T) bool)`

Determines whether all the members of an array satisfy the specified test.


#### `func EveryWithIndex(predicate func(T, int) bool) (func([]T) bool)`

See Every but callback receives index of element.


#### `func EveryWithSlice(predicate func(T, int, []T) bool) (func([]T) bool)`

Like Every but callback receives index of element and the whole array.


#### `func Filter(predicate func(T) bool) (func([]T) []T)`

Filter Returns the elements of an array that meet the condition specified in a callback function.


#### `func FilterWithIndex(predicate func(T, int) bool) (func([]T) []T)`

FilterWithIndex See Filter but callback receives index of element.


#### `func FilterWithSlice(predicate func(T, int, []T) bool) (func([]T) []T)`

FilterWithSlice Like Filter but callback receives index of element and the whole array.


#### `func Flat(xs [][]T) []T`

Returns a new array with all sub-array elements concatenated into it recursively up to the specified depth.


#### `func FlatMap(callback func(T) []R) (func([]T) []R)`

Calls a defined callback function on each element of an array. Then, flattens the result into a new array. This is identical to a map followed by flat with depth 1.


#### `func FlatMapWithIndex(callback func(T, int) []R) (func([]T) []R)`

See FlatMap but callback receives index of element.


#### `func FlatMapWithSlice(callback func(T, int, []T) []R) (func([]T) []R)`

Like FlatMap but callback receives index of element and the whole array.


#### `func Map(callback func(T) R) (func([]T) []R)`

Calls a defined callback function on each element of an array, and returns an array that contains the results.


#### `func MapWithIndex(callback func(T, int) R) (func([]T) []R)`

See Map but callback receives index of element.


#### `func MapWithSlice(callback func(T, int, []T) R) (func([]T) []R)`

Like Map but callback receives index of element and the whole array.


#### `func Pipe10(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) R) (func(T1) R)`

Performs left-to-right function composition of 10 functions


#### `func Pipe11(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) R) (func(T1) R)`

Performs left-to-right function composition of 11 functions


#### `func Pipe12(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) R) (func(T1) R)`

Performs left-to-right function composition of 12 functions


#### `func Pipe13(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) T13, fn13 func(T13) R) (func(T1) R)`

Performs left-to-right function composition of 13 functions


#### `func Pipe14(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) T13, fn13 func(T13) T14, fn14 func(T14) R) (func(T1) R)`

Performs left-to-right function composition of 14 functions


#### `func Pipe15(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) T13, fn13 func(T13) T14, fn14 func(T14) T15, fn15 func(T15) R) (func(T1) R)`

Performs left-to-right function composition of 15 functions


#### `func Pipe16(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) T13, fn13 func(T13) T14, fn14 func(T14) T15, fn15 func(T15) T16, fn16 func(T16) R) (func(T1) R)`

Performs left-to-right function composition of 16 functions


#### `func Pipe2(fn1 func(T1) T2, fn2 func(T2) R) (func(T1) R)`

Performs left-to-right function composition of two functions


#### `func Pipe3(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) R) (func(T1) R)`

Performs left-to-right function composition of three functions


#### `func Pipe4(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) R) (func(T1) R)`

Performs left-to-right function composition of four functions


#### `func Pipe5(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) R) (func(T1) R)`

Performs left-to-right function composition of five functions


#### `func Pipe6(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) R) (func(T1) R)`

Performs left-to-right function composition of 6 functions


#### `func Pipe7(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) R) (func(T1) R)`

Performs left-to-right function composition of 7 functions


#### `func Pipe8(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) R) (func(T1) R)`

Performs left-to-right function composition of 8 functions


#### `func Pipe9(fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) R) (func(T1) R)`

Performs left-to-right function composition of 9 functions


#### `func Reduce(callback func(R, T) R, acc R) (func([]T) R)`

Reduce Calls the specified callback function for all the elements in an array. The return value of the callback function is the accumulated result, and is provided as an argument in the next call to the callback function.


#### `func ReduceWithIndex(callback func(R, T, int) R, acc R) (func([]T) R)`

ReduceWithIndex See Reduce but callback receives index of element.


#### `func ReduceWithSlice(callback func(R, T, int, []T) R, acc R) (func([]T) R)`

ReduceWithSlice Like Reduce but callback receives index of element and the whole array.


#### `func Some(predicate func(T) bool) (func([]T) bool)`

Determines whether the specified callback function returns true for any element of an array.


#### `func SomeWithIndex(predicate func(T, int) bool) (func([]T) bool)`

See Some but callback receives index of element.


#### `func SomeWithSlice(predicate func(T, int, []T) bool) (func([]T) bool)`

Like Some but callback receives index of element and the whole array.


---

## Package `either` (pkg/fp/either)

### Types

#### Either

BaseError struct


### Functions

#### `func Exists(predicate func(right R) bool) (func(<?>) bool)`

Returns `false` if `Left` or returns the boolean result of the application of the given predicate to the `Right` value


#### `func FromOption(onNone func() L) (func(o <?>) <?>)`

Constructor of Either from an Option.
Returns a Left in case of None storing the callback return value as the error argument
Returns a Right in case of Some with the option value.


#### `func FromPredicate(predicate func(value R) bool, onLeft func() L) (func(R) <?>)`

Constructor of Either from a predicate.
Returns a Left if the predicate function over the value return false.
Returns a Right if the predicate function over the value return true.


#### `func GetOrElse(onLeft func(left L) R) (func(<?>) R)`

Extracts the value out of the Either, if it exists. Otherwise returns the result of the callback function that takes the error as argument.


#### `func IsLeft(e <?>) bool`

Helper to check if the Either has an error


#### `func IsRight(e <?>) bool`

Helper to check if the Either has a value


#### `func Map(onRight func(right R) T) (func(<?>) <?>)`

Map over the Either value if it exists. Otherwise return the Either itself


#### `func MapLeft(fn func(left L) T) (func(<?>) <?>)`

Map over the Either error if it exists. Otherwise return the Either with the new error type


#### `func Match(onLeft func(left L) T, onRight func(right R) T) (func(<?>) T)`

Extracts the value out of the Either.
Returns a new type running the succes or error callbacks which are taking respectively the error or value as an argument.


---

## Package `opt` (pkg/fp/option)

### Types

#### Option

BaseError struct


```go
type Option struct {
    Value T
}
```

### Functions

#### `func Chain(fn func(a A) <?>) (func(<?>) <?>)`

Execute a function that returns an Option on the Option value if it exists. Otherwise return the empty Option itself


#### `func Exists(predicate func(value T) bool) (func(<?>) bool)`

Returns `false` if `None` or returns the boolean result of the application of the given predicate to the `Some` value


#### `func FromPredicate(predicate func(value T) bool) (func(T) <?>)`

Constructor of Option from a predicate.
Returns a None if the predicate function over the value return false.
Returns a Some if the predicate function over the value return true.


#### `func GetOrElse(onNone <?>) (func(<?>) T)`

Extracts the value out of the Option, if it exists. Otherwise returns the function with a default value


#### `func IsNone(o <?>) bool`

Helper to check if the Option is missing the value


#### `func IsSome(o <?>) bool`

Helper to check if the Option has a value


#### `func Map(fn func(value T) R) (func(o <?>) <?>)`

Execute the function on the Option value if it exists. Otherwise return the empty Option itself


#### `func Match(onNone <?>, onSome func(value T) R) (func(<?>) R)`

Extracts the value out of the Option, if it exists, with a function. Otherwise returns the function with a default value


---

## Package `graphql` (pkg/graphql)

### Types

#### FieldFunc

##### Methods

- `func (FieldFunc) ExtensionName() string`

- `func (FieldFunc) InterceptField(ctx context.Context, next graphql.Resolver) (any, error)`

- `func (FieldFunc) Validate(schema graphql.ExecutableSchema) error`

#### Handler

##### Methods

- `func (Handler) AddExecutor(execs ...*executor.Executor)`

- `func (Handler) AddTransport(transport graphql.Transport)`

- `func (Handler) AroundFields(funcs map[*executor.Executor]graphql.FieldMiddleware)`
  AroundFields is a convenience method for creating an extension that only implements field middleware
  

- `func (Handler) AroundOperations(funcs map[*executor.Executor]graphql.OperationMiddleware)`
  AroundOperations is a convenience method for creating an extension that only implements operation middleware
  

- `func (Handler) AroundResponses(funcs map[*executor.Executor]graphql.ResponseMiddleware)`
  AroundResponses is a convenience method for creating an extension that only implements response middleware
  

- `func (Handler) AroundRootFields(funcs map[*executor.Executor]graphql.RootFieldMiddleware)`
  AroundRootFields is a convenience method for creating an extension that only implements field middleware
  

- `func (Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)`

- `func (Handler) SetDisableSuggestion(values map[*executor.Executor]bool)`

- `func (Handler) SetErrorPresenter(funcs map[*executor.Executor]graphql.ErrorPresenterFunc)`

- `func (Handler) SetParserTokenLimit(limits map[*executor.Executor]int)`

- `func (Handler) SetQueryCache(caches map[*executor.Executor]<?>)`

- `func (Handler) SetRecoverFunc(funcs map[*executor.Executor]graphql.RecoverFunc)`

- `func (Handler) Use(extensions map[*executor.Executor]graphql.HandlerExtension)`

#### MyPOST

```go
type MyPOST struct {
    ResponseHeaders map[string][]string
}
```

##### Methods

- `func (MyPOST) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor)`

- `func (MyPOST) Supports(r *http.Request) bool`

#### OperationFunc

##### Methods

- `func (OperationFunc) ExtensionName() string`

- `func (OperationFunc) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler`

- `func (OperationFunc) Validate(schema graphql.ExecutableSchema) error`

#### Resolver

#### ResponseFunc

##### Methods

- `func (ResponseFunc) ExtensionName() string`

- `func (ResponseFunc) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response`

- `func (ResponseFunc) Validate(schema graphql.ExecutableSchema) error`

### Functions

### Variables and Constants

---

## Package `htmx` (pkg/htmx)

### Functions

#### `func CurrentUrl(r *http.Request) string`

CurrentUrl retrieves the current URL of the browser from the HX-Current-URL request header.


#### `func IsBoosted(r *http.Request) bool`

IsBoosted checks if the request was triggered by an element with hx-boost.


#### `func IsHistoryRestoreRequest(r *http.Request) bool`

IsHistoryRestoreRequest checks if the request is for history restoration after a miss in the local history cache.


#### `func IsHxRequest(r *http.Request) bool`

IsHxRequest checks if the request is an HTMX request.


#### `func Location(w http.ResponseWriter, path, target string)`

Location sets the HX-Location header to trigger a client-side navigation.


#### `func PromptResponse(r *http.Request) string`

PromptResponse retrieves the user's response to an hx-prompt from the HX-Prompt request header.


#### `func PushUrl(w http.ResponseWriter, url string)`

PushUrl sets the HX-Push-Url header to push a new URL into the browser history stack.


#### `func Redirect(w http.ResponseWriter, path string)`

Redirect sets the HX-Redirect header to redirect the client to a new URL.


#### `func Refresh(w http.ResponseWriter)`

Refresh sets the HX-Refresh header to true, instructing the client to perform a full page refresh.


#### `func ReplaceUrl(w http.ResponseWriter, url string)`

ReplaceUrl sets the HX-Replace-Url header to replace the current URL in the browser location bar.


#### `func Reselect(w http.ResponseWriter, selector string)`

Reselect sets the HX-Reselect header to specify which part of the response should be swapped in.


#### `func Reswap(w http.ResponseWriter, swapStyle string)`

Reswap sets the HX-Reswap header to specify how the response will be swapped.


#### `func Retarget(w http.ResponseWriter, target string)`

Retarget sets the HX-Retarget header to specify a new target element.


#### `func SSEEvent(html string, event ...string) string`

SSEEvent creates a Server-Sent Event (SSE) formatted string.


#### `func SetTrigger(w http.ResponseWriter, event, detail string)`

Trigger sets the HX-Trigger header to trigger client-side events.


#### `func Target(r *http.Request) string`

Target returns the ID of the element that triggered the request.


#### `func Trigger(r *http.Request) string`

Trigger retrieves the ID of the triggered element from the HX-Trigger request header.


#### `func TriggerAfterSettle(w http.ResponseWriter, event, detail string)`

TriggerAfterSettle sets the HX-Trigger-After-Settle header to trigger client-side events after the settle step.


#### `func TriggerAfterSwap(w http.ResponseWriter, event, detail string)`

TriggerAfterSwap sets the HX-Trigger-After-Swap header to trigger client-side events after the swap step.


#### `func TriggerName(r *http.Request) string`

TriggerName retrieves the name of the triggered element from the HX-Trigger-Name request header.


---

## Package `client1c` (pkg/integrations/1c)

### Types

#### Client

##### Methods

- `func (Client) GetOdataServices(infoBase string) (*OdataServices, error)`

#### OdataService

```go
type OdataService struct {
    Name string `json:"name"`
    URL string `json:"url"`
}
```

#### OdataServices

```go
type OdataServices struct {
    OdataMetadata string `json:"odata.metadata"`
    Value []OdataService `json:"value"`
}
```

---

## Package `intl` (pkg/intl)

### Types

#### SupportedLanguage

```go
type SupportedLanguage struct {
    Code string
    VerboseName string
    Tag language.Tag
}
```

### Functions

#### `func MustT(ctx context.Context, msgID string) string`

MustT returns the translation for the given message ID.
If the translation is not found, it will panic.


#### `func UseLocale(ctx context.Context) (language.Tag, bool)`

UseLocale returns the locale from the context.
If the locale is not found, the second return value will be false.


#### `func UseLocalizer(ctx context.Context) (*i18n.Localizer, bool)`

UseLocalizer returns the localizer from the context.
If the localizer is not found, the second return value will be false.


#### `func UseUniLocalizer(ctx context.Context) (ut.Translator, error)`

#### `func WithLocale(ctx context.Context, l language.Tag) context.Context`

#### `func WithLocalizer(ctx context.Context, l *i18n.Localizer) context.Context`

### Variables and Constants

- Var: `[registerTranslations translationLock ErrNoLocalizer]`

- Var: `[SupportedLanguages]`

---

## Package `js` (pkg/js)

### Functions

#### `func ToJS(v interface{}) (string, error)`

ToJS transforms a Go struct into a JavaScript object representation.
It supports basic types, nested structs, maps, slices, and can include
function references.


---

## Package `llm` (pkg/llm)

---

## Package `functions` (pkg/llm/gpt-functions)

### Types

#### ChatFunctionDefinition

##### Interface Methods

- `Name() string`
- `Description() string`
- `Arguments() map[string]interface{}`
- `Execute(args map[string]interface{}) (string, error)`

#### ChatTools

```go
type ChatTools struct {
    Definitions []ChatFunctionDefinition
}
```

##### Methods

- `func (ChatTools) Add(def ChatFunctionDefinition)`

- `func (ChatTools) Call(name string, args string) (string, error)`

- `func (ChatTools) Funcs() map[string]CompletionFunc`

- `func (ChatTools) OpenAiTools() []llm.Tool`

#### Column

```go
type Column struct {
    Type string `json:"type"`
    Nullable bool `json:"nullable"`
    Enums []string `json:"enums"`
    Ref *Ref `json:"ref"`
}
```

#### CompletionFunc

#### DBColumn

```go
type DBColumn struct {
    ColumnName string `db:"column_name"`
    DataType string `db:"data_type"`
    UdtName string `db:"udt_name"`
    IsNullable string `db:"is_nullable"`
}
```

#### Enum

```go
type Enum struct {
    EnumLabel string `db:"enumlabel"`
    TypName string `db:"typname"`
}
```

#### Ref

```go
type Ref struct {
    To string `json:"to"`
    Column string `json:"column"`
}
```

#### Table

```go
type Table struct {
    Name string `json:"name"`
    Description string `json:"description"`
    Columns map[string]Column `json:"columns"`
}
```

### Functions

#### `func GetFkRelations(db *gorm.DB, tn string) ([]struct{...}, error)`

#### `func GetTables(db *gorm.DB) ([]string, error)`

---

## Package `logging` (pkg/logging)

### Types

#### LokiConfig

```go
type LokiConfig struct {
    Labels map[string]string
    Client *http.Client
}
```

#### LokiHook

LokiHook sends logs to Loki


```go
type LokiHook struct {
    URL string
    AppName string
    Config *LokiConfig
}
```

##### Methods

- `func (LokiHook) Fire(entry *logrus.Entry) error`

- `func (LokiHook) Levels() []logrus.Level`

#### LokiPush

LokiPush represents the payload sent to Loki's push API


```go
type LokiPush struct {
    Streams []LokiStream `json:"streams"`
}
```

#### LokiStream

LokiStream represents a stream of log entries in Loki format


```go
type LokiStream struct {
    Stream map[string]string `json:"stream"`
    Values [][2]string `json:"values"`
}
```

### Functions

#### `func AddLokiHook(logger *logrus.Logger, url, appName string) error`

#### `func ConsoleLogger(level logrus.Level) *logrus.Logger`

#### `func DataToMap(data any) logrus.Fields`

DataToMap converts any data structure to a logrus.Fields map


#### `func FileLogger(level logrus.Level, logPath string) (*os.File, *logrus.Logger, error)`

#### `func SetupTracing(ctx context.Context, serviceName string, tempoURL string) func()`

---

## Package `mapping` (pkg/mapping)

### Functions

#### `func MapDBModels(entities []T, mapFunc func(T) (V, error)) ([]V, error)`

MapDBModels maps entities to db models


#### `func MapViewModels(entities []T, mapFunc func(T) V) []V`

MapViewModels maps entities to view models


#### `func Or(args ...T) T`

Or is a utility function that returns the first non-zero value.


#### `func Pointer(v T) *T`

Pointer is a utility function that returns a pointer to the given value.


#### `func PointerSlice(v []T) []*T`

PointerSlice is a utility function that returns a slice of pointers from a slice of values.


#### `func PointerToSQLNullString(s *string) sql.NullString`

#### `func PointerToSQLNullTime(t *time.Time) sql.NullTime`

#### `func SQLNullTimeToPointer(v sql.NullTime) *time.Time`

#### `func UUIDToSQLNullString(id uuid.UUID) sql.NullString`

#### `func Value(v *T) T`

Value is a utility function that returns the value of the given pointer.


#### `func ValueSlice(v []*T) []T`

ValueSlice is a utility function that returns a slice of values from a slice of pointers.


#### `func ValueToSQLNullFloat64(f float64) sql.NullFloat64`

#### `func ValueToSQLNullInt32(i int32) sql.NullInt32`

#### `func ValueToSQLNullInt64(i int64) sql.NullInt64`

#### `func ValueToSQLNullString(s string) sql.NullString`

#### `func ValueToSQLNullTime(t time.Time) sql.NullTime`

---

## Package `middleware` (pkg/middleware)

### Types

#### GenericConstructor

#### LogTransport

LogTransport is a http.RoundTripper middleware for logging outgoing HTTP requests/responses.


```go
type LogTransport struct {
    Base http.RoundTripper
    Logger *logrus.Logger
    LogRequestBody bool
    LogResponseBody bool
}
```

##### Methods

- `func (LogTransport) RoundTrip(req *http.Request) (*http.Response, error)`
  RoundTrip implements http.RoundTripper.
  

#### LoggerOptions

```go
type LoggerOptions struct {
    LogRequestBody bool
    LogResponseBody bool
    MaxBodyLength int
}
```

### Functions

#### `func Authorize() mux.MiddlewareFunc`

#### `func ContextKeyValue(key interface{}, constructor GenericConstructor) mux.MiddlewareFunc`

#### `func Cors(allowOrigins ...string) mux.MiddlewareFunc`

#### `func NavItems() mux.MiddlewareFunc`

#### `func Provide(k constants.ContextKey, v any) mux.MiddlewareFunc`

#### `func ProvideLocalizer(bundle *i18n.Bundle) mux.MiddlewareFunc`

#### `func ProvideUser() mux.MiddlewareFunc`

#### `func RedirectNotAuthenticated() mux.MiddlewareFunc`

#### `func RequestParams() mux.MiddlewareFunc`

#### `func RequireAuthorization() mux.MiddlewareFunc`

#### `func Tabs() mux.MiddlewareFunc`

#### `func TracedMiddleware(name string) mux.MiddlewareFunc`

#### `func WithLogger(logger *logrus.Logger, opts LoggerOptions) mux.MiddlewareFunc`

#### `func WithPageContext() mux.MiddlewareFunc`

#### `func WithTransaction() mux.MiddlewareFunc`

### Variables and Constants

- Var: `[AllowMethods]`

---

## Package `money` (pkg/money)

### Types

#### Amount

Amount is a data structure that stores the amount being used for calculations.


#### Currencies

##### Methods

- `func (Currencies) Add(currency *Currency) Currencies`
  Add updates currencies list by adding a given Currency to it.
  

- `func (Currencies) CurrencyByCode(code string) *Currency`
  CurrencyByCode returns the currency given the currency code defined as a constant.
  

- `func (Currencies) CurrencyByNumericCode(code string) *Currency`
  CurrencyByNumericCode returns the currency given the numeric code defined in ISO-4271.
  

#### Currency

Currency represents money currency information required for formatting.


```go
type Currency struct {
    Code string
    NumericCode string
    Fraction int
    Grapheme string
    Template string
    Decimal string
    Thousand string
}
```

##### Methods

- `func (Currency) Formatter() *Formatter`
  Formatter returns currency formatter representing
  used currency structure.
  

#### Formatter

Formatter stores Money formatting information.


```go
type Formatter struct {
    Fraction int
    Decimal string
    Thousand string
    Grapheme string
    Template string
}
```

##### Methods

- `func (Formatter) Format(amount int64) string`
  Format returns string of formatted integer using given currency template.
  

- `func (Formatter) FormatCompact(amount int64, decimals int) string`
  FormatCompact returns a compactly formatted string for large monetary values
  with the specified number of decimal places.
  For example:
  - 1,234,567 -> 1.2M (decimals=1)
  - 1,234,567 -> 1.23M (decimals=2)
  - 22,524,232 -> 22.52M (decimals=2)
  - 1,234 -> 1.23K (decimals=2)
  If decimals is not specified (0), defaults to 1 decimal place.
  

- `func (Formatter) ToMajorUnits(amount int64) float64`
  ToMajorUnits returns float64 representing the value in sub units using the currency data
  

#### Money

Money represents monetary value information, stores
currency and amount value.


##### Methods

- `func (Money) Absolute() *Money`
  Absolute returns new Money struct from given Money using absolute monetary value.
  

- `func (Money) Add(ms ...*Money) (*Money, error)`
  Add returns new Money struct with value representing sum of Self and Other Money.
  

- `func (Money) Allocate(rs ...int) ([]*Money, error)`
  Allocate returns slice of Money structs with split Self value in given ratios.
  It lets split money by given ratios without losing pennies and as Split operations distributes
  leftover pennies amongst the parties with round-robin principle.
  

- `func (Money) Amount() int64`
  Amount returns a copy of the internal monetary value as an int64.
  

- `func (Money) AsMajorUnits() float64`
  AsMajorUnits lets represent Money struct as subunits (float64) in given Currency value
  

- `func (Money) Compare(om *Money) (int, error)`
  Compare function compares two money of the same type
  
  	if m.amount > om.amount returns (1, nil)
  	if m.amount == om.amount returns (0, nil
  	if m.amount < om.amount returns (-1, nil)
  
  If compare moneys from distinct currency, return (m.amount, ErrCurrencyMismatch)
  

- `func (Money) Currency() *Currency`
  Currency returns the currency used by Money.
  

- `func (Money) Display() string`
  Display lets represent Money struct as string in given Currency value.
  

- `func (Money) DisplayCompact(decimals ...int) string`
  DisplayCompact lets represent Money struct as a compact string for large values
  with the specified number of decimal places (e.g., 22.5M UZS with decimals=1, 22.52M UZS with decimals=2).
  If decimals is not specified (0), defaults to 1 decimal place.
  

- `func (Money) Equals(om *Money) (bool, error)`
  Equals checks equality between two Money types.
  

- `func (Money) GreaterThan(om *Money) (bool, error)`
  GreaterThan checks whether the value of Money is greater than the other.
  

- `func (Money) GreaterThanOrEqual(om *Money) (bool, error)`
  GreaterThanOrEqual checks whether the value of Money is greater or equal than the other.
  

- `func (Money) IsNegative() bool`
  IsNegative returns boolean of whether the value of Money is negative.
  

- `func (Money) IsPositive() bool`
  IsPositive returns boolean of whether the value of Money is positive.
  

- `func (Money) IsZero() bool`
  IsZero returns boolean of whether the value of Money is equals to zero.
  

- `func (Money) LessThan(om *Money) (bool, error)`
  LessThan checks whether the value of Money is less than the other.
  

- `func (Money) LessThanOrEqual(om *Money) (bool, error)`
  LessThanOrEqual checks whether the value of Money is less or equal than the other.
  

- `func (Money) MarshalJSON() ([]byte, error)`
  MarshalJSON is implementation of json.Marshaller
  

- `func (Money) Multiply(muls ...int64) *Money`
  Multiply returns new Money struct with value representing Self multiplied value by multiplier.
  

- `func (Money) Negative() *Money`
  Negative returns new Money struct from given Money using negative monetary value.
  

- `func (Money) Round() *Money`
  Round returns new Money struct with value rounded to nearest zero.
  

- `func (Money) SameCurrency(om *Money) bool`
  SameCurrency check if given Money is equals by currency.
  

- `func (Money) Split(n int) ([]*Money, error)`
  Split returns slice of Money structs with split Self value in given number.
  After division leftover pennies will be distributed round-robin amongst the parties.
  This means that parties listed first will likely receive more pennies than ones that are listed later.
  

- `func (Money) Subtract(ms ...*Money) (*Money, error)`
  Subtract returns new Money struct with value representing difference of Self and Other Money.
  

- `func (Money) UnmarshalJSON(b []byte) error`
  UnmarshalJSON is implementation of json.Unmarshaller
  

### Functions

### Variables and Constants

- Var: `[UnmarshalJSON MarshalJSON ErrCurrencyMismatch ErrInvalidJSONUnmarshal]`
  Injection points for backward compatibility.
  If you need to keep your JSON marshal/unmarshal way, overwrite them like below.
  
  	money.UnmarshalJSON = func (m *Money, b []byte) error { ... }
  	money.MarshalJSON = func (m Money) ([]byte, error) { ... }
  

- Const: `[AED AFN ALL AMD ANG AOA ARS AUD AWG AZN BAM BBD BDT BGN BHD BIF BMD BND BOB BRL BSD BTN BWP BYN BYR BZD CAD CDF CHF CLF CLP CNY COP CRC CUC CUP CVE CZK DJF DKK DOP DZD EEK EGP ERN ETB EUR FJD FKP GBP GEL GGP GHC GHS GIP GMD GNF GTQ GYD HKD HNL HRK HTG HUF IDR ILS IMP INR IQD IRR ISK JEP JMD JOD JPY KES KGS KHR KMF KPW KRW KWD KYD KZT LAK LBP LKR LRD LSL LTL LVL LYD MAD MDL MGA MKD MMK MNT MOP MUR MRU MVR MWK MXN MYR MZN NAD NGN NIO NOK NPR NZD OMR PAB PEN PGK PHP PKR PLN PYG QAR RON RSD RUB RUR RWF SAR SBD SCR SDG SEK SGD SHP SKK SLE SLL SOS SRD SSP STD STN SVC SYP SZL THB TJS TMT TND TOP TRL TRY TTD TWD TZS UAH UGX USD UYU UZS VEF VES VND VUV WST XAF XAG XAU XCD XCG XDR XOF XPF YER ZAR ZMW ZWD ZWL]`
  Constants for active currency codes according to the ISO 4217 standard.
  

---

## Package `multifs` (pkg/multifs)

Package multifs MultiHashFS combines multiple hashfs instances to serve files from each.


### Types

#### MultiHashFS

##### Methods

- `func (MultiHashFS) Open(name string) (http.File, error)`
  Open attempts to open a file from any of the hashfs instances.
  

---

## Package `rbac` (pkg/rbac)

### Types

#### Permission

##### Interface Methods

- `Can(u user.User) bool`

#### RBAC

##### Interface Methods

- `Register(permissions ...*permission.Permission)`
- `Get(id uuid.UUID) (*permission.Permission, error)`
- `Permissions() []*permission.Permission`
- `PermissionsByResource() map[string][]*permission.Permission`

### Variables and Constants

- Var: `[ErrPermissionNotFound]`

---

## Package `repo` (pkg/repo)

Package repo provides database utility functions and interfaces for working with PostgreSQL.


### Types

#### Cache

##### Interface Methods

- `Get(key string) (any, bool)`
- `Set(key string, value any) error`
- `Delete(key string)`
- `Clear()`

#### Column

##### Interface Methods

- `ToSQL() string`

#### ExtendedFieldSet

ExtendedFieldSet is an interface that must be implemented to persist custom fields with a repository.
It allows repositories to work with custom field sets by providing field names and values.


##### Interface Methods

- `Fields() []string`
- `Value(k string) interface{}`

#### FieldFilter

```go
type FieldFilter struct {
    Column T
    Filter Filter
}
```

#### Filter

Filter defines a query filter with a SQL clause generator and bound value.


##### Interface Methods

- `Value() []any`
- `String(column string, argIdx int) string`

#### InMemoryCache

##### Methods

- `func (InMemoryCache) Clear()`

- `func (InMemoryCache) Delete(key string)`

- `func (InMemoryCache) Get(key string) (any, bool)`

- `func (InMemoryCache) Set(key string, value any) error`

#### SortBy

```go
type SortBy struct {
    Fields []<?>
}
```

##### Methods

- `func (SortBy) ToSQL(mapping map[T]string) string`

#### SortByField

SortBy defines sorting criteria for queries with generic field type support.
Use with OrderBy function to generate ORDER BY clauses.


```go
type SortByField struct {
    Field T
    Ascending bool
    NullsLast bool
}
```

#### Tx

Tx is an interface that abstracts database transaction operations.
It provides a subset of pgx.Tx functionality needed for common database operations.


##### Interface Methods

- `CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)`
- `SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults`
- `Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)`
- `Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)`
- `QueryRow(ctx context.Context, sql string, args ...any) pgx.Row`

### Functions

#### `func BatchInsertQueryN(baseQuery string, rows [][]interface{}) (string, []interface{})`

BatchInsertQueryN creates a parameterized SQL query for batch inserting multiple rows.
It takes a base query like "INSERT INTO users (name, email) VALUES" and appends
the parameterized values for each row, returning both the query and the flattened arguments.

Example usage:

	baseQuery := "INSERT INTO users (name, email) VALUES"
	rows := [][]interface{}{
	    {"John", "john@example.com"},
	    {"Jane", "jane@example.com"},
	    {"Bob", "bob@example.com"},
	}
	query, args := repo.BatchInsertQueryN(baseQuery, rows)
	// query = "INSERT INTO users (name, email) VALUES ($1,$2),($3,$4),($5,$6)"
	// args = []interface{}{"John", "john@example.com", "Jane", "jane@example.com", "Bob", "bob@example.com"}

If rows is empty, it returns the baseQuery unchanged and nil for args.
Panics if rows have inconsistent lengths.


#### `func CacheKey(keys ...interface{}) string`

#### `func Exists(inner string) string`

Exists wraps a SELECT query inside SELECT EXISTS (...).

Example usage:

	query := repo.Exists("SELECT 1 FROM users WHERE phone = $1")
	// Returns: "SELECT EXISTS (SELECT 1 FROM users WHERE phone = $1)"


#### `func FormatLimitOffset(limit, offset int) string`

FormatLimitOffset generates SQL LIMIT and OFFSET clauses based on the provided values.

If both limit and offset are positive, it returns "LIMIT x OFFSET y".
If only limit is positive, it returns "LIMIT x".
If only offset is positive, it returns "OFFSET y".
If neither is positive, it returns an empty string.

Example usage:

	query := "SELECT * FROM users " + repo.FormatLimitOffset(10, 20)
	// Returns: "SELECT * FROM users LIMIT 10 OFFSET 20"


#### `func Insert(tableName string, fields []string, returning ...string) string`

Insert creates a parameterized SQL query for inserting a single row.
Optionally returns specified columns with the RETURNING clause.

Example usage:

	query := repo.Insert("users", []string{"name", "email", "password"}, "id", "created_at")
	// Returns: "INSERT INTO users (name, email, password) VALUES ($1, $2, $3) RETURNING id, created_at"


#### `func Join(expressions ...string) string`

Join combines multiple SQL expressions with spaces between them.

Example usage:

	query := repo.Join("SELECT *", "FROM users", "WHERE active = true")
	// Returns: "SELECT * FROM users WHERE active = true"


#### `func JoinWhere(expressions ...string) string`

JoinWhere creates an SQL WHERE clause by joining multiple conditions with AND.

Example usage:

	conditions := []string{"status = $1", "created_at > $2"}
	query := "SELECT * FROM orders " + repo.JoinWhere(conditions...)
	// Returns: "SELECT * FROM orders WHERE status = $1 AND created_at > $2"


#### `func Update(tableName string, fields []string, where ...string) string`

Update creates a parameterized SQL query for updating rows in a table.
The where parameters are optional conditions that will be ANDed together.

Example usage:

	query := repo.Update("users", []string{"name", "email"}, "id = $3")
	// Returns: "UPDATE users SET name = $1, email = $2 WHERE id = $3"

	// Multiple conditions
	query := repo.Update("products", []string{"name", "price", "updated_at"}, "id = $4", "category_id = $5")
	// Returns: "UPDATE products SET name = $1, price = $2, updated_at = $3 WHERE id = $4 AND category_id = $5"

	// No conditions
	query := repo.Update("settings", []string{"value", "updated_at"})
	// Returns: "UPDATE settings SET value = $1, updated_at = $2"


#### `func WithCache(ctx context.Context, cache Cache) context.Context`

---

## Package `collector` (pkg/schema/collector)

### Types

#### Collector

##### Methods

- `func (Collector) CollectMigrations(ctx context.Context) (*common.ChangeSet, *common.ChangeSet, error)`

- `func (Collector) StoreMigrations(upChanges, downChanges *common.ChangeSet) error`

#### Config

```go
type Config struct {
    MigrationsPath string
    Logger *logrus.Logger
    LogLevel logrus.Level
    EmbedFSs []*embed.FS
}
```

#### FileLoader

##### Methods

- `func (FileLoader) LoadExistingSchema(ctx context.Context) (*common.Schema, error)`

- `func (FileLoader) LoadModuleSchema(ctx context.Context) (*common.Schema, error)`

#### LoaderConfig

```go
type LoaderConfig struct {
    BaseDir string
    EmbedFSs []*embed.FS
    Logger logrus.FieldLogger
}
```

#### SchemaLoader

##### Interface Methods

- `LoadExistingSchema(ctx context.Context) (*common.Schema, error)`
- `LoadModuleSchema(ctx context.Context) (*common.Schema, error)`

### Functions

#### `func CollectSchemaChanges(oldSchema, newSchema *common.Schema) (*common.ChangeSet, *common.ChangeSet, error)`

CollectSchemaChanges compares two schemas and generates both up and down change sets


#### `func CompareTables(oldTable, newTable *tree.CreateTable) ([]interface{}, []interface{}, error)`

---

## Package `common` (pkg/schema/common)

### Types

#### ChangeSet

ChangeSet represents a collection of related schema changes


```go
type ChangeSet struct {
    Changes []interface{}
    Timestamp int64
    Version string
    Hash string
}
```

#### Schema

Schema represents a database schema containing all objects


```go
type Schema struct {
    Tables map[string]*tree.CreateTable
    Indexes map[string]*tree.CreateIndex
    Columns map[string]map[string]*tree.ColumnTableDef
}
```

#### SchemaObject

SchemaObject represents a generic schema object that can be different types
from the postgresql-parser tree package


### Functions

#### `func AllReferencesSatisfied(t *tree.CreateTable, tables []*tree.CreateTable) bool`

#### `func HasReferences(table *tree.CreateTable) bool`

#### `func SortTableDefs(tables []*tree.CreateTable) ([]*tree.CreateTable, error)`

---

## Package `serrors` (pkg/serrors)

### Types

#### Base

##### Interface Methods

- `Error() string`
- `Localize(l *i18n.Localizer) string`

#### BaseError

```go
type BaseError struct {
    Code string `json:"code"`
    Message string `json:"message"`
    LocaleKey string `json:"locale_key,omitempty"`
    TemplateData map[string]string `json:"-"`
}
```

##### Methods

- `func (BaseError) Error() string`

- `func (BaseError) Localize(l *i18n.Localizer) string`

- `func (BaseError) WithTemplateData(data map[string]string) *BaseError`
  WithTemplateData adds template data to the error for localization
  

#### ValidationError

ValidationError represents a field validation error


```go
type ValidationError struct {
    Field string `json:"field"`
}
```

##### Methods

- `func (ValidationError) WithDetails(details string) *ValidationError`
  WithDetails adds error details to the template data
  

- `func (ValidationError) WithFieldName(fieldLocaleKey string) *ValidationError`
  WithFieldName adds the field name to the template data
  

#### ValidationErrors

ValidationErrors is a map of field names to validation errors


### Functions

#### `func LocalizeValidationErrors(errs ValidationErrors, l *i18n.Localizer) map[string]string`

LocalizeValidationErrors localizes all validation errors in the map


#### `func UnauthorizedGQLError(path ast.Path) *gqlerror.Error`

#### `func Unmarshal(body []byte, errInstance map[string]interface{}) error`

---

## Package `server` (pkg/server)

### Types

#### HTTPServer

```go
type HTTPServer struct {
    Controllers []application.Controller
    Middlewares []mux.MiddlewareFunc
    NotFoundHandler http.Handler
    MethodNotAllowedHandler http.Handler
}
```

##### Methods

- `func (HTTPServer) Start(socketAddress string) error`

### Functions

#### `func WsHub() *ws.Hub`

### Variables and Constants

- Const: `[ChannelChat]`

---

## Package `shared` (pkg/shared)

### Types

#### DateOnly

#### FormAction

##### Methods

- `func (FormAction) IsValid() bool`

### Functions

#### `func GetInitials(firstName, lastName string) string`

GetInitials safely extracts the first character from first and last names,
properly handling non-ASCII characters and converting to uppercase.
Returns "NA" if both names are empty.


#### `func ParseID(r *http.Request) (uint, error)`

#### `func Redirect(w http.ResponseWriter, r *http.Request, path string)`

#### `func SetFlash(w http.ResponseWriter, name string, value []byte)`

#### `func SetFlashMap(w http.ResponseWriter, name string, value map[K]V)`

### Variables and Constants

- Var: `[Decoder]`

- Var: `[Encoder]`

---

## Package `spotlight` (pkg/spotlight)

Package spotlight is a package that provides a way to show a list of items in a spotlight.


### Types

#### DataSource

DataSource provides external items for Spotlight.


##### Interface Methods

- `Find(ctx context.Context, q string) []Item`

#### Item

Item represents a renderable spotlight entry.


##### Interface Methods

- `templ.Component`

#### QuickLink

##### Methods

- `func (QuickLink) Render(ctx context.Context, w io.Writer) error`

#### QuickLinks

##### Methods

- `func (QuickLinks) Add(links ...*QuickLink)`

- `func (QuickLinks) Find(ctx context.Context, q string) []Item`

#### Spotlight

Spotlight streams items matching a query over a channel.


##### Interface Methods

- `Find(ctx context.Context, q string) (chan Item)`
- `Register(ds DataSource)`

---

## Package `testutils` (pkg/testutils)

### Types

#### TestFixtures

```go
type TestFixtures struct {
    SQLDB *sql.DB
    Pool *pgxpool.Pool
    Context context.Context
    Tx pgx.Tx
    App application.Application
}
```

### Functions

#### `func CreateDB(name string)`

#### `func CreateTestTenant(ctx context.Context, pool *pgxpool.Pool) (*composables.Tenant, error)`

CreateTestTenant creates a test tenant for testing


#### `func DbOpts(name string) string`

#### `func DefaultParams() *composables.Params`

#### `func MockSession() *session.Session`

#### `func MockUser(permissions ...*permission.Permission) user.User`

#### `func NewPool(dbOpts string) *pgxpool.Pool`

#### `func SetupApplication(pool *pgxpool.Pool, mods ...application.Module) (application.Application, error)`

---

## Package `tgserver` (pkg/tgServer)

### Types

#### DBSession

##### Methods

- `func (DBSession) LoadSession(context.Context) ([]byte, error)`
  LoadSession loads session from memory.
  

- `func (DBSession) StoreSession(_ context.Context, data []byte) error`
  StoreSession stores session to memory.
  

#### Server

```go
type Server struct {
    DB *sqlx.DB
}
```

##### Methods

- `func (Server) Start()`

---

## Package `types` (pkg/types)

### Types

#### NavigationItem

```go
type NavigationItem struct {
    Name string
    Href string
    Children []NavigationItem
    Icon templ.Component
    Permissions []*permission.Permission
}
```

##### Methods

- `func (NavigationItem) HasPermission(user user.User) bool`

#### PageContext

```go
type PageContext struct {
    Locale language.Tag
    URL *url.URL
    Localizer *i18n.Localizer
}
```

##### Methods

- `func (PageContext) T(k string, args ...map[string]interface{}) string`

#### PageData

```go
type PageData struct {
    Title string
    Description string
}
```

---

## Package `validators` (pkg/validators)

### Types

#### ValidationError

```go
type ValidationError struct {
    Fields map[string]string
}
```

##### Methods

- `func (ValidationError) Error() string`

- `func (ValidationError) Field(field string) string`

- `func (ValidationError) FieldsMap() map[string]string`

- `func (ValidationError) HasField(field string) bool`

### Functions

#### `func FieldLabel(dto T, err validator.FieldError) string`

---

## Package `ws` (pkg/ws)

### Types

#### Connection

##### Methods

- `func (Connection) Channels() <?>`

- `func (Connection) Close() error`

- `func (Connection) GetContext(key string) (any, bool)`

- `func (Connection) SendMessage(message []byte) error`

- `func (Connection) Session() *session.Session`

- `func (Connection) SetContext(key string, value any)`

- `func (Connection) Subscribe(channel string)`

- `func (Connection) Unsubscribe(channel string)`

- `func (Connection) UserID() uint`

#### Connectioner

##### Interface Methods

- `UserID() uint`
- `Session() *session.Session`
- `Channels() <?>`
- `SendMessage(message []byte) error`
- `Subscribe(channel string)`
- `Unsubscribe(channel string)`
- `SetContext(key string, value any)`
- `GetContext(key string) (any, bool)`

#### Hub

##### Methods

- `func (Hub) BroadcastToAll(message []byte)`

- `func (Hub) BroadcastToChannel(channel string, message []byte)`

- `func (Hub) BroadcastToUser(userID uint, message []byte)`

- `func (Hub) ConnectionsAll() []*Connection`

- `func (Hub) ConnectionsInChannel(channel string) []*Connection`

- `func (Hub) ServeHTTP(w http.ResponseWriter, r *http.Request)`

#### Huber

##### Interface Methods

- `BroadcastToAll(message []byte)`
- `BroadcastToUser(userID uint, message []byte)`
- `BroadcastToChannel(channel string, message []byte)`
- `ConnectionsInChannel(channel string) []*Connection`
- `ConnectionsAll() []*Connection`

#### Set

#### SubscriptionMessage

```go
type SubscriptionMessage struct {
    Subscribe string `json:"subscribe,omitempty"`
    Unsubscribe string `json:"unsubscribe,omitempty"`
}
```

### Variables and Constants

---

