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

#### `func DateTime(props *Props) templ.Component`

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
    Class string
    Attrs templ.Attributes
}
```

### Functions

#### `func Button(value string) templ.Component`

Button renders an individual tab button


#### `func Content(value string) templ.Component`

Content renders tab content that shows/hides based on active tab


#### `func List(class string) templ.Component`

List renders the tab navigation buttons


#### `func Root(props Props) templ.Component`

Root provides a container for navtabs with content switching functionality


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
    Name string
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

#### AreaChartBuilder

AreaChartBuilder provides a fluent interface for building area charts


##### Methods

- `func (AreaChartBuilder) AsStacked() *AreaChartBuilder`
  AsStacked makes the area chart stacked
  

- `func (AreaChartBuilder) Build() ChartOptions`
  Build converts the builder to standard ChartOptions
  

- `func (AreaChartBuilder) WithCategories(categories []string) *AreaChartBuilder`
  WithCategories sets the X-axis categories
  

- `func (AreaChartBuilder) WithColors(colors ...string) *AreaChartBuilder`
  WithColors sets the chart colors
  

- `func (AreaChartBuilder) WithHeight(height string) *AreaChartBuilder`
  WithHeight sets the chart height
  

- `func (AreaChartBuilder) WithSeries(name string, data []float64) *AreaChartBuilder`
  WithSeries adds a data series to the area chart
  

- `func (AreaChartBuilder) WithTitle(title string) *AreaChartBuilder`
  WithTitle sets the chart title
  

#### AreaChartOptions

AreaChartOptions represents type-safe options for area charts


```go
type AreaChartOptions struct {
    Height string
    Series []NamedSeries
    Categories []string
    Colors []string
    DataLabels bool
    Stacked bool
    Title string
}
```

#### AxisTitle

```go
type AxisTitle struct {
    Text string `json:"text"`
    Style TextStyle `json:"style"`
}
```

#### BarChartBuilder

BarChartBuilder provides a fluent interface for building bar charts


##### Methods

- `func (BarChartBuilder) AsHorizontal() *BarChartBuilder`
  AsHorizontal makes the bar chart horizontal
  

- `func (BarChartBuilder) AsStacked() *BarChartBuilder`
  AsStacked makes the bar chart stacked
  

- `func (BarChartBuilder) Build() ChartOptions`
  Build converts the builder to standard ChartOptions
  

- `func (BarChartBuilder) WithBorderRadius(radius int) *BarChartBuilder`
  WithBorderRadius sets the border radius for bars
  

- `func (BarChartBuilder) WithCategories(categories []string) *BarChartBuilder`
  WithCategories sets the X-axis categories
  

- `func (BarChartBuilder) WithColors(colors ...string) *BarChartBuilder`
  WithColors sets the chart colors
  

- `func (BarChartBuilder) WithHeight(height string) *BarChartBuilder`
  WithHeight sets the chart height
  

- `func (BarChartBuilder) WithSeries(name string, data []float64) *BarChartBuilder`
  WithSeries adds a data series to the bar chart
  

#### BarChartOptions

BarChartOptions represents type-safe options for bar charts


```go
type BarChartOptions struct {
    Height string
    Series []NamedSeries
    Categories []string
    Colors []string
    DataLabels bool
    Horizontal bool
    Stacked bool
    BorderRadius int
    ColumnWidth string
    Title string
}
```

#### BarConfig

```go
type BarConfig struct {
    BorderRadius int `json:"borderRadius,omitempty"`
    ColumnWidth string `json:"columnWidth,omitempty"`
    DataLabels BarLabels `json:"dataLabels,omitempty"`
    Horizontal *bool `json:"horizontal,omitempty"`
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
    Events *ChartEvents `json:"events,omitempty"`
}
```

#### ChartEvents

```go
type ChartEvents struct {
    AnimationEnd templ.JSExpression `json:"animationEnd,omitempty"`
    BeforeMount templ.JSExpression `json:"beforeMount,omitempty"`
    Mounted templ.JSExpression `json:"mounted,omitempty"`
    Updated templ.JSExpression `json:"updated,omitempty"`
    MouseMove templ.JSExpression `json:"mouseMove,omitempty"`
    MouseLeave templ.JSExpression `json:"mouseLeave,omitempty"`
    Click templ.JSExpression `json:"click,omitempty"`
    LegendClick templ.JSExpression `json:"legendClick,omitempty"`
    MarkerClick templ.JSExpression `json:"markerClick,omitempty"`
    XAxisLabelClick templ.JSExpression `json:"xAxisLabelClick,omitempty"`
    Selection templ.JSExpression `json:"selection,omitempty"`
    DataPointSelection templ.JSExpression `json:"dataPointSelection,omitempty"`
    DataPointMouseEnter templ.JSExpression `json:"dataPointMouseEnter,omitempty"`
    DataPointMouseLeave templ.JSExpression `json:"dataPointMouseLeave,omitempty"`
    BeforeZoom templ.JSExpression `json:"beforeZoom,omitempty"`
    BeforeResetZoom templ.JSExpression `json:"beforeResetZoom,omitempty"`
    Zoomed templ.JSExpression `json:"zoomed,omitempty"`
    Scrolled templ.JSExpression `json:"scrolled,omitempty"`
}
```

#### ChartOption

ChartOption is a function that modifies chart options


#### ChartOptions

```go
type ChartOptions struct {
    Chart ChartConfig `json:"chart"`
    Series interface{} `json:"series"`
    Labels []string `json:"labels,omitempty"`
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

#### LineChartBuilder

LineChartBuilder provides a fluent interface for building line charts


##### Methods

- `func (LineChartBuilder) Build() ChartOptions`
  Build converts the builder to standard ChartOptions
  

- `func (LineChartBuilder) WithCategories(categories []string) *LineChartBuilder`
  WithCategories sets the X-axis categories
  

- `func (LineChartBuilder) WithColors(colors ...string) *LineChartBuilder`
  WithColors sets the chart colors
  

- `func (LineChartBuilder) WithDataLabels(enabled bool) *LineChartBuilder`
  WithDataLabels enables or disables data labels
  

- `func (LineChartBuilder) WithHeight(height string) *LineChartBuilder`
  WithHeight sets the chart height
  

- `func (LineChartBuilder) WithSeries(name string, data []float64) *LineChartBuilder`
  WithSeries adds a data series to the line chart
  

- `func (LineChartBuilder) WithTitle(title string) *LineChartBuilder`
  WithTitle sets the chart title
  

#### LineChartOptions

LineChartOptions represents type-safe options for line charts


```go
type LineChartOptions struct {
    Height string
    Series []NamedSeries
    Categories []string
    YAxis *YAxisConfig
    Colors []string
    DataLabels bool
    ShowGrid bool
    Stroke *StrokeConfig
    Markers *MarkersConfig
    Title string
}
```

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

#### NamedSeries

NamedSeries represents a data series with a name


```go
type NamedSeries struct {
    Name string
    Data []float64
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

#### PieChartBuilder

PieChartBuilder provides a fluent interface for building pie charts


##### Methods

- `func (PieChartBuilder) AsDonut(size string) *PieChartBuilder`
  AsDonut converts the pie chart to a donut chart with the specified size
  

- `func (PieChartBuilder) Build() ChartOptions`
  Build converts the builder to standard ChartOptions
  

- `func (PieChartBuilder) WithColors(colors ...string) *PieChartBuilder`
  WithColors sets the chart colors
  

- `func (PieChartBuilder) WithDataLabels(enabled bool) *PieChartBuilder`
  WithDataLabels enables or disables data labels
  

- `func (PieChartBuilder) WithHeight(height string) *PieChartBuilder`
  WithHeight sets the chart height
  

- `func (PieChartBuilder) WithLegend(enabled bool) *PieChartBuilder`
  WithLegend enables or disables the legend
  

#### PieChartOptions

PieChartOptions represents type-safe options for pie charts


```go
type PieChartOptions struct {
    Height string
    Colors []string
    DataLabels bool
    ShowLegend bool
    Title string
    DonutSize string
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

#### RadialBarChartBuilder

RadialBarChartBuilder provides a fluent interface for building radial bar charts


##### Methods

- `func (RadialBarChartBuilder) Build() ChartOptions`
  Build converts the builder to standard ChartOptions
  

- `func (RadialBarChartBuilder) WithAngles(start, end int) *RadialBarChartBuilder`
  WithAngles sets the start and end angles
  

- `func (RadialBarChartBuilder) WithColors(colors ...string) *RadialBarChartBuilder`
  WithColors sets the chart colors
  

- `func (RadialBarChartBuilder) WithHeight(height string) *RadialBarChartBuilder`
  WithHeight sets the chart height
  

- `func (RadialBarChartBuilder) WithSize(size string) *RadialBarChartBuilder`
  WithSize sets the radial bar size (thickness)
  

- `func (RadialBarChartBuilder) WithTrackStrokeWidth(width string) *RadialBarChartBuilder`
  WithTrackStrokeWidth sets the track stroke width (bar thickness)
  

#### RadialBarChartOptions

RadialBarChartOptions represents type-safe options for radial bar charts


```go
type RadialBarChartOptions struct {
    Height string
    Colors []string
    StartAngle int
    EndAngle int
    Hollow string
    Track bool
    Size string
    TrackStrokeWidth string
    DataLabels bool
    Title string
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

## Package `export` (components/export)

templ: version: v0.3.857


### Types

#### ExportDropdownProps

```go
type ExportDropdownProps struct {
    Formats []ExportFormat
    ExportURL string
    Label string
    Size button.Size
    Class string
    Attrs templ.Attributes
}
```

#### ExportFormat

### Functions

#### `func ExportDropdown(props ExportDropdownProps) templ.Component`

#### `func GetExportFormatString(format ExportFormat) string`

GetExportFormatString returns the string representation of an ExportFormat


#### `func IsValidExportFormat(formatStr string) bool`

IsValidExportFormat checks if a string is a valid export format


### Variables and Constants

- Const: `[ExportFormatExcel ExportFormatCSV ExportFormatJSON ExportFormatTXT]`

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

## Package `actions` (components/scaffold/actions)

templ: version: v0.3.857


### Types

#### ActionOption

ActionOption is a function that modifies ActionProps


#### ActionProps

ActionProps defines properties for an action


```go
type ActionProps struct {
    Type ActionType
    Label string
    Href string
    Icon templ.Component
    Variant button.Variant
    Size button.Size
    Attrs templ.Attributes
    OnClick string
}
```

#### ActionType

ActionType defines the type of action


### Functions

#### `func Action(props ActionProps) templ.Component`

Action renders a single action button based on ActionProps


#### `func Actions(actions ...ActionProps) templ.Component`

Actions renders a group of action buttons


#### `func RenderAction(props ActionProps) templ.Component`

RenderAction creates a templ.Component from ActionProps


#### `func RenderRowActions(actions ...ActionProps) templ.Component`

RenderRowActions creates a templ.Component that renders multiple actions in a row


#### `func RowActions(actions ...ActionProps) templ.Component`

RowActions renders action buttons for table rows


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

#### ComboboxField

ComboboxField for multi-select dropdowns


##### Interface Methods

- `<?>`
- `Endpoint() string`
- `Multiple() bool`
- `Searchable() bool`
- `Placeholder() string`

#### ComboboxFieldBuilder

ComboboxFieldBuilder for multi-select fields


##### Methods

- `func (ComboboxFieldBuilder) Attrs(a templ.Attributes) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Build() ComboboxField`

- `func (ComboboxFieldBuilder) Default(val string) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Endpoint(endpoint string) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Key(key string) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Label(label string) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Multiple(multiple bool) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Placeholder(placeholder string) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Required(required bool) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Searchable(searchable bool) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) Validators(v []Validator) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) WithMultiple(multiple bool) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) WithRequired(required bool) *ComboboxFieldBuilder`

- `func (ComboboxFieldBuilder) WithValue(value string) *ComboboxFieldBuilder`

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

#### SearchSelectField

SearchSelectField for async search dropdowns


##### Interface Methods

- `<?>`
- `Endpoint() string`
- `Placeholder() string`

#### SearchSelectFieldBuilder

SearchSelectFieldBuilder for async search select fields


##### Methods

- `func (SearchSelectFieldBuilder) Attrs(a templ.Attributes) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) Build() SearchSelectField`

- `func (SearchSelectFieldBuilder) Default(val string) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) Endpoint(endpoint string) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) Key(key string) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) Label(label string) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) Placeholder(placeholder string) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) Required(required bool) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) Validators(v []Validator) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) WithRequired(required bool) *SearchSelectFieldBuilder`

- `func (SearchSelectFieldBuilder) WithValue(value string) *SearchSelectFieldBuilder`

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

#### DetailAction

```go
type DetailAction struct {
    Label string
    URL string
    Method string
    Class string
    Confirm string
}
```

#### DetailFieldType

DetailFieldType represents the type of field for display purposes


#### DetailFieldValue

DetailFieldValue represents a field value to display in the details drawer


```go
type DetailFieldValue struct {
    Name string
    Label string
    Value string
    Type DetailFieldType
}
```

#### DetailsDrawerProps

```go
type DetailsDrawerProps struct {
    ID string
    Title string
    CallbackURL string
    Fields []DetailFieldValue
    Actions []DetailAction
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
    Actions []templ.Component
    Columns []TableColumn
    Rows []TableRow
    Infinite *InfiniteScrollConfig
    SideFilter templ.Component
}
```

##### Methods

- `func (TableConfig) AddActions(actions ...templ.Component) *TableConfig`

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

#### `func DetailsDrawer(props DetailsDrawerProps) templ.Component`

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
    TabGroups TabGroupCollection
    Footer templ.Component
}
```

#### TabGroup

TabGroup represents a group of sidebar items organized under a tab


```go
type TabGroup struct {
    Label string
    Value string
    Items []Item
}
```

#### TabGroupCollection

TabGroupCollection holds multiple tab groups for the sidebar


```go
type TabGroupCollection struct {
    Groups []TabGroup
    DefaultValue string
}
```

### Functions

#### `func AccordionGroup(group Group) templ.Component`

#### `func AccordionLink(link Link) templ.Component`

#### `func Sidebar(props Props) templ.Component`

#### `func SidebarContent(props Props) templ.Component`

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

templ: version: v0.3.857


### Types

#### LogoProps

```go
type LogoProps struct {
    LogoUpload *viewmodels.Upload
    LogoCompactUpload *viewmodels.Upload
}
```

### Functions

#### `func DefaultLogo() templ.Component`

#### `func DynamicLogo(props *LogoProps) templ.Component`

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

## Package `modules` (modules)

### Functions

#### `func Load(app application.Application, externalModules ...application.Module) error`

### Variables and Constants

- Var: `[BuiltInModules NavLinks]`

---

## Package `bichat` (modules/bichat)

### Types

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

- Var: `[BiChatLink]`

- Var: `[LocaleFiles]`

- Var: `[MigrationFiles]`

- Var: `[NavItems]`

---

## Package `dialogue` (modules/bichat/domain/entities/dialogue)

### Types

#### CreatedEvent

```go
type CreatedEvent struct {
    Data Dialogue
    Result Dialogue
    Sender user.User
    Session session.Session
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Result Dialogue
    Sender user.User
    Session session.Session
}
```

#### Dialogue

##### Interface Methods

- `ID() uint`
- `TenantID() uuid.UUID`
- `UserID() uint`
- `Label() string`
- `Messages() Messages`
- `LastMessage() llm.ChatCompletionMessage`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `AddMessages(messages ...llm.ChatCompletionMessage) Dialogue`
- `SetMessages(messages Messages) Dialogue`
- `SetLastMessage(msg llm.ChatCompletionMessage) Dialogue`

#### FindParams

```go
type FindParams struct {
    Query string
    Field string
    Limit int
    Offset int
}
```

#### Messages

#### Reply

```go
type Reply struct {
    Message string
    Model *string
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]Dialogue, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Dialogue, error)`
- `GetByID(ctx context.Context, id uint) (Dialogue, error)`
- `GetByUserID(ctx context.Context, userID uint) ([]Dialogue, error)`
- `Create(ctx context.Context, data Dialogue) (Dialogue, error)`
- `Update(ctx context.Context, data Dialogue) error`
- `Delete(ctx context.Context, id uint) error`

#### Start

```go
type Start struct {
    Message string
    Model string
}
```

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Data Dialogue
    Result Dialogue
    Sender user.User
    Session session.Session
}
```

---

## Package `embedding` (modules/bichat/domain/entities/embedding)

### Types

#### SearchResult

```go
type SearchResult struct {
    UUID string `json:"uuid"`
    Text string `json:"text"`
    ReferenceID string `json:"reference_id"`
    Score float64 `json:"score"`
}
```

---

## Package `llm` (modules/bichat/domain/entities/llm)

### Types

#### ChatCompletionMessage

```go
type ChatCompletionMessage struct {
    Role string `json:"role"`
    Content string `json:"content"`
    Refusal string `json:"refusal,omitempty"`
    MultiContent []ChatMessagePart `json:"multi_content,omitempty"`
    Name string `json:"name,omitempty"`
    FunctionCall *FunctionCall `json:"function_call,omitempty"`
    ToolCalls []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string `json:"tool_call_id,omitempty"`
}
```

#### ChatCompletionRequest

ChatCompletionRequest represents a request structure for chat completion API.


```go
type ChatCompletionRequest struct {
    Model string `json:"model"`
    Messages []ChatCompletionMessage `json:"messages"`
    MaxTokens int `json:"max_tokens,omitempty"`
    MaxCompletionTokens int `json:"max_completion_tokens,omitempty"`
    Temperature float32 `json:"temperature,omitempty"`
    TopP float32 `json:"top_p,omitempty"`
    N int `json:"n,omitempty"`
    Stream bool `json:"stream,omitempty"`
    Stop []string `json:"stop,omitempty"`
    PresencePenalty float32 `json:"presence_penalty,omitempty"`
    ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`
    Seed *int `json:"seed,omitempty"`
    FrequencyPenalty float32 `json:"frequency_penalty,omitempty"`
    LogitBias map[string]int `json:"logit_bias,omitempty"`
    LogProbs bool `json:"logprobs,omitempty"`
    TopLogProbs int `json:"top_logprobs,omitempty"`
    User string `json:"user,omitempty"`
    Functions []FunctionDefinition `json:"functions,omitempty"`
    FunctionCall any `json:"function_call,omitempty"`
    Tools []Tool `json:"tools,omitempty"`
    ToolChoice any `json:"tool_choice,omitempty"`
    StreamOptions *StreamOptions `json:"stream_options,omitempty"`
    ParallelToolCalls any `json:"parallel_tool_calls,omitempty"`
    Store bool `json:"store,omitempty"`
    Metadata map[string]string `json:"metadata,omitempty"`
}
```

#### ChatCompletionResponseFormat

```go
type ChatCompletionResponseFormat struct {
    Type ChatCompletionResponseFormatType `json:"type,omitempty"`
    JSONSchema *ChatCompletionResponseFormatJSONSchema `json:"json_schema,omitempty"`
}
```

#### ChatCompletionResponseFormatJSONSchema

```go
type ChatCompletionResponseFormatJSONSchema struct {
    Name string `json:"name"`
    Description string `json:"description,omitempty"`
    Schema json.Marshaler `json:"schema"`
    Strict bool `json:"strict"`
}
```

#### ChatCompletionResponseFormatType

#### ChatMessageImageURL

```go
type ChatMessageImageURL struct {
    URL string `json:"url,omitempty"`
    Detail ImageURLDetail `json:"detail,omitempty"`
}
```

#### ChatMessagePart

```go
type ChatMessagePart struct {
    Type ChatMessagePartType `json:"type,omitempty"`
    Text string `json:"text,omitempty"`
    ImageURL *ChatMessageImageURL `json:"image_url,omitempty"`
}
```

#### ChatMessagePartType

#### FunctionCall

```go
type FunctionCall struct {
    Name string `json:"name,omitempty"`
    Arguments string `json:"arguments,omitempty"`
}
```

#### FunctionDefinition

```go
type FunctionDefinition struct {
    Name string `json:"name"`
    Description string `json:"description,omitempty"`
    Strict bool `json:"strict,omitempty"`
    Parameters any `json:"parameters"`
}
```

#### ImageURLDetail

#### StreamOptions

```go
type StreamOptions struct {
    IncludeUsage bool `json:"include_usage,omitempty"`
}
```

#### Tool

```go
type Tool struct {
    Type ToolType `json:"type"`
    Function *FunctionDefinition `json:"function,omitempty"`
}
```

#### ToolCall

```go
type ToolCall struct {
    Index *int `json:"index,omitempty"`
    ID string `json:"id"`
    Type ToolType `json:"type"`
    Function FunctionCall `json:"function"`
}
```

#### ToolChoice

```go
type ToolChoice struct {
    Type ToolType `json:"type"`
    Function ToolFunction `json:"function,omitempty"`
}
```

#### ToolFunction

```go
type ToolFunction struct {
    Name string `json:"name"`
}
```

#### ToolType

---

## Package `prompt` (modules/bichat/domain/entities/prompt)

### Types

#### Prompt

```go
type Prompt struct {
    ID string
    Title string
    Description string
    Prompt string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]*Prompt, error)`
- `GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Prompt, error)`
- `GetByID(ctx context.Context, id string) (*Prompt, error)`
- `Create(ctx context.Context, upload *Prompt) error`
- `Update(ctx context.Context, upload *Prompt) error`
- `Delete(ctx context.Context, id int64) error`

---

## Package `llmproviders` (modules/bichat/infrastructure/llmproviders)

### Types

#### OpenAIProvider

##### Methods

- `func (OpenAIProvider) CreateChatCompletionStream(ctx context.Context, request llm.ChatCompletionRequest) (*openai.ChatCompletionStream, error)`

### Functions

#### `func DomainChatCompletionMessageToOpenAI(message llm.ChatCompletionMessage) openai.ChatCompletionMessage`

#### `func DomainFuncCallToOpenAI(fc llm.FunctionCall) openai.FunctionCall`

#### `func DomainFuncDefinitionToOpenAI(f llm.FunctionDefinition) openai.FunctionDefinition`

#### `func DomainImageURLToOpenAI(i llm.ChatMessageImageURL) openai.ChatMessageImageURL`

#### `func DomainMessagePartToOpenAI(m llm.ChatMessagePart) openai.ChatMessagePart`

#### `func DomainToOpenAIChatCompletionRequest(d llm.ChatCompletionRequest) openai.ChatCompletionRequest`

#### `func DomainToolCallToOpenAI(toolCalls []llm.ToolCall) []openai.ToolCall`

#### `func DomainToolToOpenAI(t llm.Tool) openai.Tool`

#### `func OpenAIChatCompletionMessageToDomain(message openai.ChatCompletionMessage) llm.ChatCompletionMessage`

#### `func OpenAIToDomainFuncCall(fc openai.FunctionCall) llm.FunctionCall`

#### `func OpenAIToDomainImageURL(i openai.ChatMessageImageURL) llm.ChatMessageImageURL`

#### `func OpenAIToDomainMessagePart(m openai.ChatMessagePart) llm.ChatMessagePart`

#### `func OpenAIToDomainToolCall(toolCalls []openai.ToolCall) []llm.ToolCall`

---

## Package `persistence` (modules/bichat/infrastructure/persistence)

### Types

#### GormDialogueRepository

##### Methods

- `func (GormDialogueRepository) Count(ctx context.Context) (int64, error)`

- `func (GormDialogueRepository) Create(ctx context.Context, d dialogue.Dialogue) (dialogue.Dialogue, error)`

- `func (GormDialogueRepository) Delete(ctx context.Context, id uint) error`

- `func (GormDialogueRepository) GetAll(ctx context.Context) ([]dialogue.Dialogue, error)`

- `func (GormDialogueRepository) GetByID(ctx context.Context, id uint) (dialogue.Dialogue, error)`

- `func (GormDialogueRepository) GetByUserID(ctx context.Context, userID uint) ([]dialogue.Dialogue, error)`

- `func (GormDialogueRepository) GetPaginated(ctx context.Context, params *dialogue.FindParams) ([]dialogue.Dialogue, error)`

- `func (GormDialogueRepository) Update(ctx context.Context, d dialogue.Dialogue) error`

### Functions

#### `func NewDialogueRepository() dialogue.Repository`

### Variables and Constants

- Var: `[ErrDialogueNotFound]`

---

## Package `models` (modules/bichat/infrastructure/persistence/models)

### Types

#### Dialogue

```go
type Dialogue struct {
    ID uint
    TenantID string
    UserID uint
    Label string
    Messages string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Prompt

```go
type Prompt struct {
    ID string
    Title string
    Description string
    Prompt string
    CreatedAt time.Time
}
```

---

## Package `controllers` (modules/bichat/presentation/controllers)

### Types

#### BiChatController

##### Methods

- `func (BiChatController) Create(w http.ResponseWriter, r *http.Request)`

- `func (BiChatController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (BiChatController) Index(w http.ResponseWriter, r *http.Request)`

- `func (BiChatController) Key() string`

- `func (BiChatController) Register(r *mux.Router)`

### Functions

#### `func NewBiChatController(app application.Application) application.Controller`

---

## Package `dtos` (modules/bichat/presentation/controllers/dtos)

### Types

#### MessageDTO

```go
type MessageDTO struct {
    Message string
}
```

---

## Package `bichat` (modules/bichat/presentation/templates/pages/bichat)

templ: version: v0.3.857


### Types

#### ChatPageProps

```go
type ChatPageProps struct {
    History []*HistoryItem
    Suggestions []string
}
```

#### HistoryItem

```go
type HistoryItem struct {
    Title string
    Link string
}
```

### Functions

#### `func BiChatPage(props *ChatPageProps) templ.Component`

#### `func ChatSideBar(props *ChatPageProps) templ.Component`

#### `func Index(props *ChatPageProps) templ.Component`

#### `func ModelSelect() templ.Component`

### Variables and Constants

---

## Package `services` (modules/bichat/services)

### Types

#### DialogueService

##### Methods

- `func (DialogueService) ChatComplete(ctx context.Context, data dialogue.Dialogue, model string) error`

- `func (DialogueService) Count(ctx context.Context) (int64, error)`

- `func (DialogueService) Delete(ctx context.Context, id uint) (dialogue.Dialogue, error)`

- `func (DialogueService) GetAll(ctx context.Context) ([]dialogue.Dialogue, error)`

- `func (DialogueService) GetByID(ctx context.Context, id uint) (dialogue.Dialogue, error)`

- `func (DialogueService) GetPaginated(ctx context.Context, params *dialogue.FindParams) ([]dialogue.Dialogue, error)`

- `func (DialogueService) GetUserDialogues(ctx context.Context, userID uint) ([]dialogue.Dialogue, error)`

- `func (DialogueService) ReplyToDialogue(ctx context.Context, dialogueID uint, message, model string) (dialogue.Dialogue, error)`

- `func (DialogueService) StartDialogue(ctx context.Context, message string, model string) (dialogue.Dialogue, error)`

- `func (DialogueService) Update(ctx context.Context, data dialogue.Dialogue) error`

#### EmbeddingService

##### Methods

- `func (EmbeddingService) Search(ctx context.Context, query string) ([]*embedding.SearchResult, error)`

#### PromptService

##### Methods

- `func (PromptService) Count(ctx context.Context) (int64, error)`

- `func (PromptService) Create(ctx context.Context, data *prompt2.Prompt) error`

- `func (PromptService) Delete(ctx context.Context, id int64) error`

- `func (PromptService) GetAll(ctx context.Context) ([]*prompt2.Prompt, error)`

- `func (PromptService) GetByID(ctx context.Context, id string) (*prompt2.Prompt, error)`

- `func (PromptService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*prompt2.Prompt, error)`

- `func (PromptService) Update(ctx context.Context, data *prompt2.Prompt) error`

### Functions

#### `func NewSearchKnowledgeBase(service *EmbeddingService) functions.ChatFunctionDefinition`

### Variables and Constants

- Var: `[ErrMessageTooLong ErrModelRequired]`

---

## Package `chatfuncs` (modules/bichat/services/chatfuncs)

### Types

### Functions

#### `func GetExchangeRate(from string, to string) (float64, error)`

#### `func NewCurrencyConvert() functions.ChatFunctionDefinition`

#### `func NewDoSQLQuery(db *gorm.DB) functions.ChatFunctionDefinition`

#### `func NewUnitConversion(db *gorm.DB) functions.ChatFunctionDefinition`

#### `func UnitConversion(amount float64, from string, to string) (float64, error)`

### Variables and Constants

- Var: `[SupportedCurrencies]`

- Var: `[SupportedUnits]`

---

## Package `billing` (modules/billing)

### Types

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

---

## Package `billing` (modules/billing/domain/aggregates/billing)

### Types

#### Amount

##### Interface Methods

- `Quantity() float64`
- `Currency() Currency`

#### AmountChangedEvent

```go
type AmountChangedEvent struct {
    TransactionID uuid.UUID
    Data Amount
    Result Amount
}
```

#### ComparisonOperator

#### CreatedEvent

```go
type CreatedEvent struct {
    Result Transaction
}
```

#### Currency

#### DeletedEvent

```go
type DeletedEvent struct {
    Result Transaction
}
```

#### DetailsChangedEvent

```go
type DetailsChangedEvent struct {
    TransactionID uuid.UUID
    Data details.Details
    Result details.Details
}
```

#### DetailsFieldFilter

```go
type DetailsFieldFilter struct {
    Path []string
    Operator ComparisonOperator
    Value any
}
```

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy SortBy
    Search string
    Filters []Filter
}
```

##### Methods

- `func (FindParams) FilterBy(field Field, filter repo.Filter) *FindParams`

#### Gateway

#### Option

#### Provider

##### Interface Methods

- `Gateway() Gateway`
- `Create(ctx context.Context, t Transaction) (Transaction, error)`
- `Cancel(ctx context.Context, t Transaction) (Transaction, error)`
- `Refund(ctx context.Context, t Transaction, quantity float64) (Transaction, error)`

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Transaction, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)`
- `GetByDetailsFields(ctx context.Context, gateway Gateway, filters []DetailsFieldFilter) ([]Transaction, error)`
- `GetAll(ctx context.Context) ([]Transaction, error)`
- `Save(ctx context.Context, data Transaction) (Transaction, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### SortBy

#### SortByField

#### Status

#### StatusChangedEvent

```go
type StatusChangedEvent struct {
    TransactionID uuid.UUID
    Data Status
    Result Status
}
```

#### Transaction

##### Interface Methods

- `ID() uuid.UUID`
- `TenantID() uuid.UUID`
- `Status() Status`
- `Amount() Amount`
- `Gateway() Gateway`
- `Details() details.Details`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `Events() []interface{}`
- `SetTenantID(tenantID uuid.UUID) Transaction`
- `SetStatus(status Status) Transaction`
- `SetAmount(quantity float64, currency Currency) Transaction`
- `SetDetails(details details.Details) Transaction`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Data Transaction
    Result Transaction
}
```

---

## Package `details` (modules/billing/domain/aggregates/details)

### Types

#### ClickDetails

##### Interface Methods

- `Details`
- `ServiceID() int64`
- `MerchantID() int64`
- `MerchantUserID() int64`
- `MerchantTransID() string`
- `MerchantPrepareID() int64`
- `MerchantConfirmID() int64`
- `PayDocId() int64`
- `PaymentID() int64`
- `PaymentStatus() int32`
- `SignTime() string`
- `SignString() string`
- `ErrorCode() int32`
- `ErrorNote() string`
- `Link() string`
- `Params() map[string]any`
- `SetServiceID(serviceID int64) ClickDetails`
- `SetMerchantID(merchantID int64) ClickDetails`
- `SetMerchantUserID(merchantUserID int64) ClickDetails`
- `SetMerchantPrepareID(merchantPrepareID int64) ClickDetails`
- `SetMerchantConfirmID(merchantConfirmID int64) ClickDetails`
- `SetPayDocId(payDocId int64) ClickDetails`
- `SetPaymentID(paymentID int64) ClickDetails`
- `SetPaymentStatus(paymentStatus int32) ClickDetails`
- `SetSignTime(signTime string) ClickDetails`
- `SetSignString(signString string) ClickDetails`
- `SetErrorCode(errorCode int32) ClickDetails`
- `SetErrorNote(errorNote string) ClickDetails`
- `SetLink(link string) ClickDetails`
- `SetParams(params map[string]any) ClickDetails`

#### ClickOption

#### Details

#### OctoDetails

##### Interface Methods

- `Details`
- `OctoShopId() int32`
- `ShopTransactionId() string`
- `OctoPaymentUUID() string`
- `InitTime() string`
- `AutoCapture() bool`
- `Test() bool`
- `Status() string`
- `Description() string`
- `CardType() string`
- `CardCountry() string`
- `CardIsPhysical() bool`
- `CardMaskedPan() string`
- `Rrn() string`
- `RiskLevel() int32`
- `RefundedSum() float64`
- `TransferSum() float64`
- `ReturnUrl() string`
- `NotifyUrl() string`
- `OctoPayUrl() string`
- `Signature() string`
- `HashKey() string`
- `PayedTime() string`
- `Error() int32`
- `ErrMessage() string`
- `SetOctoShopId(octoShopId int32) OctoDetails`
- `SetShopTransactionId(shopTransactionId string) OctoDetails`
- `SetOctoPaymentUUID(octoPaymentUUID string) OctoDetails`
- `SetInitTime(initTime string) OctoDetails`
- `SetAutoCapture(autoCapture bool) OctoDetails`
- `SetTest(test bool) OctoDetails`
- `SetStatus(status string) OctoDetails`
- `SetDescription(description string) OctoDetails`
- `SetCardType(cardType string) OctoDetails`
- `SetCardCountry(cardCountry string) OctoDetails`
- `SetCardIsPhysical(cardIsPhysical bool) OctoDetails`
- `SetCardMaskedPan(cardMaskedPan string) OctoDetails`
- `SetRrn(rrn string) OctoDetails`
- `SetRiskLevel(riskLevel int32) OctoDetails`
- `SetRefundedSum(refundedSum float64) OctoDetails`
- `SetTransferSum(transferSum float64) OctoDetails`
- `SetReturnUrl(returnUrl string) OctoDetails`
- `SetNotifyUrl(notifyUrl string) OctoDetails`
- `SetOctoPayUrl(octoPayUrl string) OctoDetails`
- `SetSignature(signature string) OctoDetails`
- `SetHashKey(hashKey string) OctoDetails`
- `SetPayedTime(payedTime string) OctoDetails`
- `SetError(errCode int32) OctoDetails`
- `SetErrMessage(errMessage string) OctoDetails`

#### OctoOption

#### PaymeDetails

##### Interface Methods

- `Details`
- `MerchantID() string`
- `ID() string`
- `Transaction() string`
- `State() int32`
- `Time() int64`
- `CreatedTime() int64`
- `PerformTime() int64`
- `CancelTime() int64`
- `Account() map[string]any`
- `Receivers() []PaymeReceiver`
- `Additional() map[string]any`
- `Reason() int32`
- `ErrorCode() int32`
- `Link() string`
- `Params() map[string]any`
- `SetMerchantID(merchantID string) PaymeDetails`
- `SetID(id string) PaymeDetails`
- `SetTransaction(transaction string) PaymeDetails`
- `SetState(state int32) PaymeDetails`
- `SetTime(time int64) PaymeDetails`
- `SetCreatedTime(createdTime int64) PaymeDetails`
- `SetPerformTime(performTime int64) PaymeDetails`
- `SetCancelTime(cancelTime int64) PaymeDetails`
- `SetAccount(account map[string]any) PaymeDetails`
- `SetReceivers(receivers []PaymeReceiver) PaymeDetails`
- `SetAdditional(additional map[string]any) PaymeDetails`
- `SetReason(reason int32) PaymeDetails`
- `SetErrorCode(errorCode int32) PaymeDetails`
- `SetLink(link string) PaymeDetails`
- `SetParams(params map[string]any) PaymeDetails`

#### PaymeOption

#### PaymeReceiver

##### Interface Methods

- `ID() string`
- `Amount() float64`

#### StripeDetails

##### Interface Methods

- `Details`
- `Mode() string`
- `BillingReason() string`
- `SessionID() string`
- `ClientReferenceID() string`
- `InvoiceID() string`
- `SubscriptionID() string`
- `CustomerID() string`
- `Items() []StripeItem`
- `SubscriptionData() StripeSubscriptionData`
- `SuccessURL() string`
- `CancelURL() string`
- `URL() string`
- `SetMode(mode string) StripeDetails`
- `SetBillingReason(billingReason string) StripeDetails`
- `SetSessionID(sessionID string) StripeDetails`
- `SetClientReferenceID(clientReferenceID string) StripeDetails`
- `SetInvoiceID(invoiceID string) StripeDetails`
- `SetSubscriptionID(subscriptionID string) StripeDetails`
- `SetCustomerID(customerID string) StripeDetails`
- `SetItems(items []StripeItem) StripeDetails`
- `SetSuccessURL(successURL string) StripeDetails`
- `SetCancelURL(cancelURL string) StripeDetails`
- `SetURL(url string) StripeDetails`

#### StripeItem

##### Interface Methods

- `PriceID() string`
- `Quantity() int64`
- `AdjustableQuantity() StripeItemAdjustableQuantity`

#### StripeItemAdjustableQuantity

##### Interface Methods

- `Enabled() bool`
- `Maximum() int64`
- `Minimum() int64`

#### StripeItemOption

#### StripeOption

#### StripeSubscriptionData

##### Interface Methods

- `Description() string`
- `TrialPeriodDays() int64`

#### StripeSubscriptionDataOption

---

## Package `persistence` (modules/billing/infrastructure/persistence)

### Types

#### BillingRepository

##### Methods

- `func (BillingRepository) Count(ctx context.Context, params *billing.FindParams) (int64, error)`

- `func (BillingRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (BillingRepository) GetAll(ctx context.Context) ([]billing.Transaction, error)`

- `func (BillingRepository) GetByDetailsFields(ctx context.Context, gateway billing.Gateway, filters []billing.DetailsFieldFilter) ([]billing.Transaction, error)`

- `func (BillingRepository) GetByID(ctx context.Context, id uuid.UUID) (billing.Transaction, error)`

- `func (BillingRepository) GetPaginated(ctx context.Context, params *billing.FindParams) ([]billing.Transaction, error)`

- `func (BillingRepository) Save(ctx context.Context, data billing.Transaction) (billing.Transaction, error)`

### Functions

#### `func ToDBTransaction(entity billing.Transaction) (*models.Transaction, error)`

#### `func ToDbDetails(data details.Details) (json.RawMessage, error)`

#### `func ToDomainDetails(gateway billing.Gateway, data json.RawMessage) (details.Details, error)`

#### `func ToDomainTransaction(dbRow *models.Transaction) (billing.Transaction, error)`

### Variables and Constants

- Var: `[ErrTransactionNotFound]`

---

## Package `models` (modules/billing/infrastructure/persistence/models)

### Types

#### ClickDetails

```go
type ClickDetails struct {
    ServiceID int64 `json:"service_id"`
    MerchantID int64 `json:"merchant_id"`
    MerchantUserID int64 `json:"merchant_user_id"`
    MerchantTransID string `json:"merchant_trans_id"`
    MerchantPrepareID int64 `json:"merchant_prepare_id"`
    MerchantConfirmID int64 `json:"merchant_confirm_id"`
    PayDocId int64 `json:"pay_doc_id"`
    PaymentID int64 `json:"payment_id"`
    PaymentStatus int32 `json:"payment_status"`
    SignTime string `json:"sign_time"`
    SignString string `json:"sign_string"`
    ErrorCode int32 `json:"error_code"`
    ErrorNote string `json:"error_note"`
    Link string `json:"link"`
    Params map[string]any `json:"params"`
}
```

#### OctoDetails

```go
type OctoDetails struct {
    OctoShopID int32 `json:"octo_shop_id"`
    ShopTransactionId string `json:"shop_transaction_id"`
    OctoPaymentUUID string `json:"octo_payment_uuid"`
    InitTime string `json:"init_time"`
    AutoCapture bool `json:"auto_capture"`
    Test bool `json:"test"`
    Status string `json:"status"`
    Description string `json:"description"`
    CardType string `json:"card_type"`
    CardCountry string `json:"card_country"`
    CardIsPhysical bool `json:"card_is_physical"`
    CardMaskedPan string `json:"card_masked_pan"`
    Rrn string `json:"rrn"`
    RiskLevel int32 `json:"risk_level"`
    RefundedSum float64 `json:"refunded_sum"`
    TransferSum float64 `json:"transfer_sum"`
    ReturnUrl string `json:"return_url"`
    NotifyUrl string `json:"notify_url"`
    OctoPayUrl string `json:"octo_pay_url"`
    Signature string `json:"signature"`
    HashKey string `json:"hash_key"`
    PayedTime string `json:"payed_time"`
    Error int32 `json:"error"`
    ErrMessage string `json:"err_message"`
}
```

#### PaymeDetails

```go
type PaymeDetails struct {
    MerchantID string `json:"merchant_id"`
    ID string `json:"id"`
    Transaction string `json:"transaction"`
    State int32 `json:"state"`
    Time int64 `json:"time"`
    CreatedTime int64 `json:"created_time"`
    PerformTime int64 `json:"perform_time"`
    CancelTime int64 `json:"cancel_time"`
    Account map[string]any `json:"account"`
    Receivers []PaymeReceiver `json:"receivers"`
    Additional map[string]any `json:"additional"`
    Reason int32 `json:"reason"`
    ErrorCode int32 `json:"error_code"`
    Link string `json:"link"`
    Params map[string]any `json:"params"`
}
```

#### PaymeReceiver

```go
type PaymeReceiver struct {
    ID string `json:"id"`
    Amount float64 `json:"amount"`
}
```

#### StripeDetails

```go
type StripeDetails struct {
    Mode string `json:"mode"`
    BillingReason string `json:"billing_reason"`
    SessionID string `json:"session_id"`
    ClientReferenceID string `json:"client_reference_id"`
    InvoiceID string `json:"invoice_id"`
    SubscriptionID string `json:"subscription_id"`
    CustomerID string `json:"customer_id"`
    SubscriptionData *StripeSubscriptionData `json:"subscription_data"`
    Items []StripeItem `json:"items"`
    SuccessURL string `json:"success_url"`
    CancelURL string `json:"cancel_url"`
    URL string `json:"url"`
}
```

#### StripeItem

```go
type StripeItem struct {
    PriceID string `json:"price_id"`
    Quantity int64 `json:"quantity"`
    AdjustableQuantity *StripeItemAdjustableQuantity `json:"adjustable_quantity"`
}
```

#### StripeItemAdjustableQuantity

```go
type StripeItemAdjustableQuantity struct {
    Enabled bool `json:"enabled"`
    Maximum int64 `json:"maximum"`
    Minimum int64 `json:"minimum"`
}
```

#### StripeSubscriptionData

```go
type StripeSubscriptionData struct {
    Description string `json:"description"`
    TrialPeriodDays int64 `json:"trial_period_days"`
}
```

#### Transaction

```go
type Transaction struct {
    ID string
    TenantID string
    Status string
    Quantity float64
    Currency string
    Gateway string
    Details json.RawMessage
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## Package `providers` (modules/billing/infrastructure/providers)

### Types

#### ClickConfig

```go
type ClickConfig struct {
    URL string
    ServiceID int64
    SecretKey string
    MerchantID int64
    MerchantUserID int64
}
```

#### OctoConfig

```go
type OctoConfig struct {
    OctoShopID int32
    OctoSecret string
    NotifyURL string
}
```

#### PaymeConfig

```go
type PaymeConfig struct {
    URL string
    SecretKey string
    MerchantID string
    User string
}
```

#### StripeConfig

```go
type StripeConfig struct {
    SecretKey string
}
```

### Functions

#### `func NewClickProvider(config ClickConfig) billing.Provider`

#### `func NewOctoProvider(config OctoConfig, logTransport *middleware.LogTransport) billing.Provider`

#### `func NewPaymeProvider(config PaymeConfig) billing.Provider`

#### `func NewStripeProvider(config StripeConfig) billing.Provider`

---

## Package `permissions` (modules/billing/permissions)

### Variables and Constants

- Var: `[Permissions]`

---

## Package `controllers` (modules/billing/presentation/controllers)

### Types

#### ClickController

##### Methods

- `func (ClickController) Complete(w http.ResponseWriter, r *http.Request)`

- `func (ClickController) Key() string`

- `func (ClickController) Prepare(w http.ResponseWriter, r *http.Request)`

- `func (ClickController) Register(r *mux.Router)`

#### OctoController

##### Methods

- `func (OctoController) Handle(w http.ResponseWriter, r *http.Request)`

- `func (OctoController) Key() string`

- `func (OctoController) Register(r *mux.Router)`

#### PaymeController

##### Methods

- `func (PaymeController) Handle(w http.ResponseWriter, r *http.Request)`

- `func (PaymeController) Key() string`

- `func (PaymeController) Register(r *mux.Router)`

#### StripeController

##### Methods

- `func (StripeController) Handle(w http.ResponseWriter, r *http.Request)`

- `func (StripeController) Key() string`

- `func (StripeController) Register(r *mux.Router)`

### Functions

#### `func NewClickController(app application.Application, click configuration.ClickOptions, basePath string) application.Controller`

#### `func NewOctoController(app application.Application, octo configuration.OctoOptions, basePath string, logger *middleware.LogTransport) application.Controller`

#### `func NewPaymeController(app application.Application, payme configuration.PaymeOptions, basePath string) application.Controller`

#### `func NewStripeController(app application.Application, stripe configuration.StripeOptions, basePath string) application.Controller`

---

## Package `services` (modules/billing/services)

### Types

#### BillingService

##### Methods

- `func (BillingService) Cancel(ctx context.Context, cmd *CancelTransactionCommand) (billing.Transaction, error)`

- `func (BillingService) Count(ctx context.Context, params *billing.FindParams) (int64, error)`

- `func (BillingService) Create(ctx context.Context, cmd *CreateTransactionCommand) (billing.Transaction, error)`

- `func (BillingService) Delete(ctx context.Context, id uuid.UUID) (billing.Transaction, error)`

- `func (BillingService) GetByDetailsFields(ctx context.Context, gateway billing.Gateway, filters []billing.DetailsFieldFilter) ([]billing.Transaction, error)`

- `func (BillingService) GetByID(ctx context.Context, id uuid.UUID) (billing.Transaction, error)`

- `func (BillingService) GetPaginated(ctx context.Context, params *billing.FindParams) ([]billing.Transaction, error)`

- `func (BillingService) Refund(ctx context.Context, cmd *RefundTransactionCommand) (billing.Transaction, error)`

- `func (BillingService) Save(ctx context.Context, entity billing.Transaction) (billing.Transaction, error)`

#### CancelTransactionCommand

```go
type CancelTransactionCommand struct {
    TransactionID uuid.UUID
}
```

#### CreateTransactionCommand

```go
type CreateTransactionCommand struct {
    TenantID uuid.UUID
    Quantity float64
    Currency billing.Currency
    Gateway billing.Gateway
    Details details.Details
}
```

#### RefundTransactionCommand

```go
type RefundTransactionCommand struct {
    TransactionID uuid.UUID
    Quantity float64
}
```

---

## Package `core` (modules/core)

### Types

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

- Var: `[AdministrationLink]`

- Var: `[DashboardLink]`

- Var: `[GroupsLink]`

- Var: `[LocaleFiles]`

- Var: `[MigrationFiles]`

- Var: `[NavItems]`

- Var: `[RolesLink]`

- Var: `[SettingsLink]`

- Var: `[UsersLink]`

---

## Package `group` (modules/core/domain/aggregates/group)

### Types

#### CreatedEvent

```go
type CreatedEvent struct {
    Group Group
    Timestamp time.Time
    Actor user.User
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Group Group
    Timestamp time.Time
    Actor user.User
}
```

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy SortBy
    Search string
    Filters []Filter
}
```

##### Methods

- `func (FindParams) FilterBy(field Field, filter repo.Filter) *FindParams`

#### Group

##### Interface Methods

- `ID() uuid.UUID`
- `Type() Type`
- `TenantID() uuid.UUID`
- `Name() string`
- `Description() string`
- `Users() []user.User`
- `Roles() []role.Role`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `CanUpdate() bool`
- `CanDelete() bool`
- `AddUser(u user.User) Group`
- `RemoveUser(u user.User) Group`
- `AssignRole(r role.Role) Group`
- `RemoveRole(r role.Role) Group`
- `SetRoles(roles []role.Role) Group`
- `SetName(name string) Group`
- `SetDescription(desc string) Group`
- `SetTenantID(tenantID uuid.UUID) Group`

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Group, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (Group, error)`
- `Save(ctx context.Context, group Group) (Group, error)`
- `Exists(ctx context.Context, id uuid.UUID) (bool, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### SortBy

#### Type

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Group Group
    OldGroup Group
    Timestamp time.Time
    Actor user.User
}
```

#### UserAddedEvent

```go
type UserAddedEvent struct {
    Group Group
    AddedUser user.User
    Timestamp time.Time
    Actor user.User
}
```

#### UserRemovedEvent

```go
type UserRemovedEvent struct {
    Group Group
    RemovedUser user.User
    Timestamp time.Time
    Actor user.User
}
```

---

## Package `project` (modules/core/domain/aggregates/project)

### Types

#### CreateDTO

```go
type CreateDTO struct {
    Name string `validate:"required"`
    Description string
}
```

##### Methods

- `func (CreateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() *Project`

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data CreateDTO
    Result Project
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Project
}
```

#### Project

```go
type Project struct {
    ID uint
    Name string
    Description string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (uint, error)`
- `GetAll(ctx context.Context) ([]*Project, error)`
- `GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Project, error)`
- `GetByID(ctx context.Context, id uint) (*Project, error)`
- `Create(ctx context.Context, project *Project) error`
- `Update(ctx context.Context, project *Project) error`
- `Delete(ctx context.Context, id uint) error`

#### UpdateDTO

```go
type UpdateDTO struct {
    Name string
    Description string
}
```

##### Methods

- `func (UpdateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (UpdateDTO) ToEntity(id uint) *Project`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data UpdateDTO
    Result Project
}
```

---

## Package `role` (modules/core/domain/aggregates/role)

### Types

#### CreatedEvent

```go
type CreatedEvent struct {
    Session session.Session
    Data Role
    Result Role
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Session session.Session
    Result Role
}
```

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    Search string
    AttachPermissions bool
    Limit int
    Offset int
    SortBy SortBy
    Filters []Filter
}
```

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetAll(ctx context.Context) ([]Role, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Role, error)`
- `GetByID(ctx context.Context, id uint) (Role, error)`
- `Create(ctx context.Context, role Role) (Role, error)`
- `Update(ctx context.Context, role Role) (Role, error)`
- `Delete(ctx context.Context, id uint) error`

#### Role

##### Interface Methods

- `ID() uint`
- `TenantID() uuid.UUID`
- `Type() Type`
- `Name() string`
- `Description() string`
- `Permissions() []*permission.Permission`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `Can(perm *permission.Permission) bool`
- `CanUpdate() bool`
- `CanDelete() bool`
- `SetName(name string) Role`
- `SetDescription(description string) Role`
- `SetTenantID(tenantID uuid.UUID) Role`
- `AddPermission(p *permission.Permission) Role`
- `SetPermissions(permissions []*permission.Permission) Role`

#### SortBy

#### Type

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Session session.Session
    Data Role
    Result Role
}
```

---

## Package `user` (modules/core/domain/aggregates/user)

### Types

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender User
    Session *session.Session
    Data User
    Result User
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender User
    Session *session.Session
    Result User
}
```

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy SortBy
    Search string
    Filters []Filter
}
```

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetAll(ctx context.Context) ([]User, error)`
- `GetByEmail(ctx context.Context, email string) (User, error)`
- `GetByPhone(ctx context.Context, phone string) (User, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]User, error)`
- `GetByID(ctx context.Context, id uint) (User, error)`
- `PhoneExists(ctx context.Context, phone string) (bool, error)`
- `EmailExists(ctx context.Context, email string) (bool, error)`
- `Create(ctx context.Context, user User) (User, error)`
- `Update(ctx context.Context, user User) error`
- `UpdateLastAction(ctx context.Context, id uint) error`
- `UpdateLastLogin(ctx context.Context, id uint) error`
- `Delete(ctx context.Context, id uint) error`

#### SortBy

#### Type

#### UILanguage

##### Methods

- `func (UILanguage) IsValid() bool`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender User
    Session *session.Session
    Data User
    Result User
}
```

#### UpdatedPasswordEvent

```go
type UpdatedPasswordEvent struct {
    UserID uint
}
```

#### User

##### Interface Methods

- `ID() uint`
- `Type() Type`
- `TenantID() uuid.UUID`
- `FirstName() string`
- `LastName() string`
- `MiddleName() string`
- `Password() string`
- `Email() internet.Email`
- `Phone() phone.Phone`
- `AvatarID() uint`
- `Avatar() upload.Upload`
- `LastIP() string`
- `UILanguage() UILanguage`
- `Roles() []role.Role`
- `GroupIDs() []uuid.UUID`
- `Permissions() []*permission.Permission`
- `LastLogin() time.Time`
- `LastAction() time.Time`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `Events() []interface{}`
- `Can(perm *permission.Permission) bool`
- `CanUpdate() bool`
- `CanDelete() bool`
- `CheckPassword(password string) bool`
- `AddRole(r role.Role) User`
- `RemoveRole(r role.Role) User`
- `SetRoles(roles []role.Role) User`
- `AddGroupID(groupID uuid.UUID) User`
- `RemoveGroupID(groupID uuid.UUID) User`
- `SetGroupIDs(groupIDs []uuid.UUID) User`
- `AddPermission(perm *permission.Permission) User`
- `RemovePermission(permID uuid.UUID) User`
- `SetPermissions(perms []*permission.Permission) User`
- `SetName(firstName, lastName, middleName string) User`
- `SetUILanguage(lang UILanguage) User`
- `SetAvatarID(id uint) User`
- `SetLastIP(ip string) User`
- `SetPassword(password string) (User, error)`
- `SetPasswordUnsafe(password string) User`
- `SetEmail(email internet.Email) User`
- `SetPhone(p phone.Phone) User`

#### Validator

##### Interface Methods

- `ValidateCreate(ctx context.Context, u User) error`
- `ValidateUpdate(ctx context.Context, u User) error`

---

## Package `authlog` (modules/core/domain/entities/authlog)

### Types

#### AuthenticationLog

```go
type AuthenticationLog struct {
    ID uint
    TenantID uuid.UUID
    UserID uint
    IP string
    UserAgent string
    CreatedAt time.Time
}
```

#### FindParams

```go
type FindParams struct {
    ID uint
    UserID uint
    Limit int
    Offset int
    SortBy []string
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]*AuthenticationLog, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]*AuthenticationLog, error)`
- `GetByID(ctx context.Context, id uint) (*AuthenticationLog, error)`
- `Create(ctx context.Context, upload *AuthenticationLog) error`
- `Update(ctx context.Context, upload *AuthenticationLog) error`
- `Delete(ctx context.Context, id uint) error`

---

## Package `costcomponent` (modules/core/domain/entities/costcomponent)

### Types

#### BillableHourEntity

```go
type BillableHourEntity struct {
    Name string
}
```

#### CostComponent

```go
type CostComponent struct {
    Purpose string
    Monthly float64
    Hourly float64
}
```

#### ExpenseComponent

```go
type ExpenseComponent struct {
    Purpose string
    Value float64
}
```

#### UnifiedHourlyRateResult

```go
type UnifiedHourlyRateResult struct {
    Entity BillableHourEntity
    Components []CostComponent
}
```

### Variables and Constants

- Var: `[HoursInMonth]`

---

## Package `currency` (modules/core/domain/entities/currency)

### Types

#### Code

TODO: make this private


##### Methods

- `func (Code) IsValid() bool`

#### CreateDTO

```go
type CreateDTO struct {
    Code string `validate:"required"`
    Name string `validate:"required"`
    Symbol string `validate:"required"`
}
```

##### Methods

- `func (CreateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() (*Currency, error)`

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data CreateDTO
    Result Currency
}
```

#### Currency

```go
type Currency struct {
    Code Code
    Name string
    Symbol Symbol
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

##### Methods

- `func (Currency) Ok(l ut.Translator) (map[string]string, bool)`

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Currency
}
```

#### Field

#### FindParams

```go
type FindParams struct {
    Code string
    Limit int
    Offset int
    SortBy SortBy
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (uint, error)`
- `GetAll(ctx context.Context) ([]*Currency, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]*Currency, error)`
- `GetByCode(ctx context.Context, code string) (*Currency, error)`
- `CreateOrUpdate(ctx context.Context, currency *Currency) error`
- `Create(ctx context.Context, currency *Currency) error`
- `Update(ctx context.Context, payment *Currency) error`
- `Delete(ctx context.Context, code string) error`

#### SortBy

#### SortByField

#### Symbol

TODO: make this private


##### Methods

- `func (Symbol) IsValid() bool`

#### UpdateDTO

```go
type UpdateDTO struct {
    Code string `validate:"len=3"`
    Name string
    Symbol string
}
```

##### Methods

- `func (UpdateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (UpdateDTO) ToEntity() (*Currency, error)`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data UpdateDTO
    Result Currency
}
```

### Functions

#### `func NewMapper(fields crud.Fields) <?>`

### Variables and Constants

- Var: `[USD EUR TRY GBP RUB JPY CNY UZS AUD CAD CHF]`

- Var: `[ValidCodes ValidSymbols Currencies]`

---

## Package `exportconfig` (modules/core/domain/entities/exportconfig)

### Types

#### ExportConfig

ExportConfig represents export configuration


##### Interface Methods

- `Filename() string`
- `ExportOptions() *excel.ExportOptions`
- `StyleOptions() *excel.StyleOptions`

#### Option

#### Query

Query represents an SQL query with its arguments


##### Interface Methods

- `SQL() string`
- `Args() []interface{}`

---

## Package `passport` (modules/core/domain/entities/passport)

### Types

#### Option

Option is a function type that configures a passport


#### Passport

##### Interface Methods

- `ID() uuid.UUID`
- `TenantID() uuid.UUID`
- `Series() string`
- `Number() string`
- `Identifier() string`
- `FirstName() string`
- `LastName() string`
- `MiddleName() string`
- `Gender() general.Gender`
- `BirthDate() time.Time`
- `BirthPlace() string`
- `Nationality() string`
- `PassportType() string`
- `IssuedAt() time.Time`
- `IssuedBy() string`
- `IssuingCountry() string`
- `ExpiresAt() time.Time`
- `MachineReadableZone() string`
- `BiometricData() map[string]interface{}`
- `SignatureImage() []byte`
- `Remarks() string`

#### Repository

##### Interface Methods

- `GetByID(ctx context.Context, id uuid.UUID) (Passport, error)`
- `GetByPassportNumber(ctx context.Context, series, number string) (Passport, error)`
- `Exists(ctx context.Context, id uuid.UUID) (bool, error)`
- `Save(ctx context.Context, data Passport) (Passport, error)`
- `Create(ctx context.Context, data Passport) (Passport, error)`
- `Update(ctx context.Context, id uuid.UUID, data Passport) (Passport, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

---

## Package `permission` (modules/core/domain/entities/permission)

### Types

#### Action

#### Field

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    RoleID uint
    SortBy SortBy
}
```

#### Modifier

#### Permission

```go
type Permission struct {
    ID uuid.UUID
    TenantID uuid.UUID
    Name string
    Resource Resource
    Action Action
    Modifier Modifier
}
```

##### Methods

- `func (Permission) Equals(p2 Permission) bool`

#### Repository

##### Interface Methods

- `GetPaginated(ctx context.Context, params *FindParams) ([]*Permission, error)`
- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]*Permission, error)`
- `GetByID(ctx context.Context, id string) (*Permission, error)`
- `Save(ctx context.Context, p *Permission) error`
- `Delete(ctx context.Context, id string) error`

#### Resource

#### SortBy

#### SortByField

---

## Package `session` (modules/core/domain/entities/session)

### Types

#### CreateDTO

```go
type CreateDTO struct {
    Token string
    UserID uint
    TenantID uuid.UUID
    IP string
    UserAgent string
}
```

##### Methods

- `func (CreateDTO) ToEntity() *Session`

#### CreatedEvent

```go
type CreatedEvent struct {
    Result Session
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Result Session
}
```

#### Field

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy SortBy
    Token string
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]*Session, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]*Session, error)`
- `GetByToken(ctx context.Context, token string) (*Session, error)`
- `Create(ctx context.Context, user *Session) error`
- `Update(ctx context.Context, user *Session) error`
- `Delete(ctx context.Context, token string) error`
- `DeleteByUserId(ctx context.Context, userId uint) ([]*Session, error)`

#### Session

```go
type Session struct {
    Token string
    UserID uint
    TenantID uuid.UUID
    IP string
    UserAgent string
    ExpiresAt time.Time
    CreatedAt time.Time
}
```

##### Methods

- `func (Session) IsExpired() bool`

#### SortBy

#### SortByField

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Data Session
    Result Session
}
```

---

## Package `tab` (modules/core/domain/entities/tab)

### Types

#### CreateDTO

```go
type CreateDTO struct {
    Href string `validate:"required"`
    UserID uint
    Position uint
}
```

##### Methods

- `func (CreateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() (*Tab, error)`

#### FindParams

```go
type FindParams struct {
    SortBy []string
    UserID uint
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context, params *FindParams) ([]*Tab, error)`
- `GetUserTabs(ctx context.Context, userID uint) ([]*Tab, error)`
- `GetByID(ctx context.Context, id uint) (*Tab, error)`
- `Create(ctx context.Context, data *Tab) error`
- `CreateMany(ctx context.Context, data []*Tab) error`
- `CreateOrUpdate(ctx context.Context, data *Tab) error`
- `Update(ctx context.Context, data *Tab) error`
- `Delete(ctx context.Context, id uint) error`
- `DeleteUserTabs(ctx context.Context, userID uint) error`

#### Tab

```go
type Tab struct {
    ID uint
    Href string
    UserID uint
    Position uint
    TenantID uuid.UUID
}
```

#### UpdateDTO

```go
type UpdateDTO struct {
    Href string `validate:"required"`
    Position uint
}
```

##### Methods

- `func (UpdateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (UpdateDTO) ToEntity(id uint) (*Tab, error)`

---

## Package `telegramsession` (modules/core/domain/entities/telegramsession)

### Types

#### TelegramSession

```go
type TelegramSession struct {
    UserID int `db:"user_id"`
    Session []byte `db:"session"`
    CreatedAt time.Time `db:"created_at"`
}
```

##### Methods

- `func (TelegramSession) ToGraph()`

---

## Package `tenant` (modules/core/domain/entities/tenant)

### Types

#### Option

#### Repository

##### Interface Methods

- `GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error)`
- `GetByDomain(ctx context.Context, domain string) (*Tenant, error)`
- `Create(ctx context.Context, tenant *Tenant) (*Tenant, error)`
- `Update(ctx context.Context, tenant *Tenant) (*Tenant, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`
- `List(ctx context.Context) ([]*Tenant, error)`

#### Tenant

##### Methods

- `func (Tenant) CreatedAt() time.Time`

- `func (Tenant) Domain() string`

- `func (Tenant) ID() uuid.UUID`

- `func (Tenant) IsActive() bool`

- `func (Tenant) LogoCompactID() *int`

- `func (Tenant) LogoID() *int`

- `func (Tenant) Name() string`

- `func (Tenant) SetLogoCompactID(logoCompactID *int)`

- `func (Tenant) SetLogoID(logoID *int)`

- `func (Tenant) UpdatedAt() time.Time`

---

## Package `upload` (modules/core/domain/entities/upload)

Package upload README: Commented out everything until I find a way to solve import cycles.


### Types

#### CreateDTO

```go
type CreateDTO struct {
    File io.ReadSeeker `validate:"required"`
    Name string `validate:"required"`
    Size int `validate:"required"`
}
```

##### Methods

- `func (CreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() (Upload, []byte, error)`

#### CreatedEvent

```go
type CreatedEvent struct {
    Session session.Session
    Data CreateDTO
    Result Upload
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Session session.Session
    Result Upload
}
```

#### Field

#### FindParams

```go
type FindParams struct {
    ID uint
    Hash string
    Limit int
    Offset int
    SortBy SortBy
    Search string
    Type UploadType
    Mimetype *mimetype.MIME
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]Upload, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Upload, error)`
- `GetByID(ctx context.Context, id uint) (Upload, error)`
- `GetByHash(ctx context.Context, hash string) (Upload, error)`
- `Exists(ctx context.Context, id uint) (bool, error)`
- `Create(ctx context.Context, data Upload) (Upload, error)`
- `Update(ctx context.Context, data Upload) error`
- `Delete(ctx context.Context, id uint) error`

#### Size

##### Interface Methods

- `String() string`
- `Bytes() int`
- `Kilobytes() int`
- `Megabytes() int`
- `Gigabytes() int`

#### SortBy

#### SortByField

#### Storage

##### Interface Methods

- `Open(ctx context.Context, fileName string) ([]byte, error)`
- `Save(ctx context.Context, fileName string, bytes []byte) error`

#### Upload

##### Interface Methods

- `ID() uint`
- `TenantID() uuid.UUID`
- `Type() UploadType`
- `Hash() string`
- `Path() string`
- `Name() string`
- `Size() Size`
- `IsImage() bool`
- `PreviewURL() string`
- `URL() *url.URL`
- `Mimetype() *mimetype.MIME`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`

#### UploadType

##### Methods

- `func (UploadType) String() string`

---

## Package `country` (modules/core/domain/value_objects/country)

### Types

#### Country

##### Interface Methods

- `String() string`

### Functions

#### `func IsValid(c string) bool`

IsValid checks if a given country code is valid.


### Variables and Constants

- Var: `[ErrInvalidCountry NilCountry]`

- Var: `[AllCountries]`

---

## Package `general` (modules/core/domain/value_objects/general)

### Types

#### Gender

##### Interface Methods

- `String() string`

#### GenderEnum

##### Methods

- `func (GenderEnum) String() string`

### Functions

#### `func IsValid(c string) bool`

IsValid checks if a given country code is valid.


### Variables and Constants

- Var: `[ErrInvalidGender NilGender]`

---

## Package `internet` (modules/core/domain/value_objects/internet)

### Types

#### Email

##### Interface Methods

- `Value() string`
- `Domain() string`
- `Username() string`

#### IP

##### Interface Methods

- `Value() string`
- `Version() IpVersion`

#### IpVersion

### Functions

#### `func IsValidEmail(v string) bool`

#### `func IsValidIP(value string, version IpVersion) bool`

### Variables and Constants

- Var: `[ErrInvalidEmail]`

- Var: `[ErrInvalidIP]`

---

## Package `phone` (modules/core/domain/value_objects/phone)

### Types

#### AreaCode

AreaCode represents the mapping between area codes and countries


```go
type AreaCode struct {
    Country country.Country
    AreaCodes []string
    CodeLength int
}
```

#### Phone

##### Interface Methods

- `Value() string`
- `E164() string`

### Functions

#### `func IsValidGlobalPhoneNumber(v string) bool`

#### `func IsValidPhoneNumber(v string, c country.Country) bool`

#### `func IsValidUSPhoneNumber(v string) bool`

#### `func IsValidUZPhoneNumber(v string) bool`

#### `func ParseCountry(phoneNumber string) (country.Country, error)`

ParseCountry attempts to determine the country from a phone number


#### `func Strip(v string) string`

#### `func ToE164Format(phone string, c country.Country) string`

ToE164Format converts a phone number to E.164 format based on the country


### Variables and Constants

- Var: `[ErrInvalidPhoneNumber ErrUnknownCountry]`

- Var: `[PhoneCodeToCountry]`
  PhoneCodeToCountry maps phone number prefixes to their respective countries
  

---

## Package `tax` (modules/core/domain/value_objects/tax)

### Types

#### Pin

Pin - Personal Identification Number (ПИНФЛ - Персональный идентификационный номер физического лица)


##### Interface Methods

- `Value() string`
- `Country() country.Country`

#### Tin

Tin - Taxpayer Identification Number (ИНН - Идентификационный номер налогоплательщика)


##### Interface Methods

- `Value() string`
- `Country() country.Country`

### Functions

#### `func IsValidPin(v string, c country.Country) bool`

#### `func ValidateTin(t string, c country2.Country) error`

### Variables and Constants

- Var: `[ErrInvalidPin]`

- Var: `[ErrInvalidTin]`

---

## Package `handlers` (modules/core/handlers)

### Types

#### ActionLogEventHandler

##### Methods

#### SessionEventsHandler

##### Methods

#### TabHandler

##### Methods

- `func (TabHandler) HandleUserCreated(event *user.CreatedEvent)`

- `func (TabHandler) Register(publisher eventbus.EventBus)`

#### UserHandler

##### Methods

---

## Package `persistence` (modules/core/infrastructure/persistence)

### Types

#### FSStorage

##### Methods

- `func (FSStorage) Open(ctx context.Context, fileName string) ([]byte, error)`

- `func (FSStorage) Save(ctx context.Context, fileName string, bytes []byte) error`

#### GormAuthLogRepository

##### Methods

- `func (GormAuthLogRepository) Count(ctx context.Context) (int64, error)`

- `func (GormAuthLogRepository) Create(ctx context.Context, data *authlog.AuthenticationLog) error`

- `func (GormAuthLogRepository) Delete(ctx context.Context, id uint) error`

- `func (GormAuthLogRepository) GetAll(ctx context.Context) ([]*authlog.AuthenticationLog, error)`

- `func (GormAuthLogRepository) GetByID(ctx context.Context, id uint) (*authlog.AuthenticationLog, error)`

- `func (GormAuthLogRepository) GetPaginated(ctx context.Context, params *authlog.FindParams) ([]*authlog.AuthenticationLog, error)`

- `func (GormAuthLogRepository) Update(ctx context.Context, data *authlog.AuthenticationLog) error`

#### GormCurrencyRepository

##### Methods

- `func (GormCurrencyRepository) Count(ctx context.Context) (uint, error)`

- `func (GormCurrencyRepository) Create(ctx context.Context, entity *currency.Currency) error`

- `func (GormCurrencyRepository) CreateOrUpdate(ctx context.Context, currency *currency.Currency) error`

- `func (GormCurrencyRepository) Delete(ctx context.Context, code string) error`

- `func (GormCurrencyRepository) GetAll(ctx context.Context) ([]*currency.Currency, error)`

- `func (GormCurrencyRepository) GetByCode(ctx context.Context, code string) (*currency.Currency, error)`

- `func (GormCurrencyRepository) GetPaginated(ctx context.Context, params *currency.FindParams) ([]*currency.Currency, error)`

- `func (GormCurrencyRepository) Update(ctx context.Context, entity *currency.Currency) error`

#### GormRoleRepository

##### Methods

- `func (GormRoleRepository) Count(ctx context.Context, params *role.FindParams) (int64, error)`

- `func (GormRoleRepository) Create(ctx context.Context, data role.Role) (role.Role, error)`

- `func (GormRoleRepository) Delete(ctx context.Context, id uint) error`

- `func (GormRoleRepository) GetAll(ctx context.Context) ([]role.Role, error)`

- `func (GormRoleRepository) GetByID(ctx context.Context, id uint) (role.Role, error)`

- `func (GormRoleRepository) GetPaginated(ctx context.Context, params *role.FindParams) ([]role.Role, error)`

- `func (GormRoleRepository) Update(ctx context.Context, data role.Role) (role.Role, error)`

#### GormUploadRepository

##### Methods

- `func (GormUploadRepository) Count(ctx context.Context) (int64, error)`

- `func (GormUploadRepository) Create(ctx context.Context, data upload.Upload) (upload.Upload, error)`

- `func (GormUploadRepository) Delete(ctx context.Context, id uint) error`

- `func (GormUploadRepository) Exists(ctx context.Context, id uint) (bool, error)`

- `func (GormUploadRepository) GetAll(ctx context.Context) ([]upload.Upload, error)`

- `func (GormUploadRepository) GetByHash(ctx context.Context, hash string) (upload.Upload, error)`

- `func (GormUploadRepository) GetByID(ctx context.Context, id uint) (upload.Upload, error)`

- `func (GormUploadRepository) GetPaginated(ctx context.Context, params *upload.FindParams) ([]upload.Upload, error)`

- `func (GormUploadRepository) Update(ctx context.Context, data upload.Upload) error`

#### PassportRepository

##### Methods

- `func (PassportRepository) Create(ctx context.Context, data passport.Passport) (passport.Passport, error)`

- `func (PassportRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (PassportRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error)`

- `func (PassportRepository) GetByID(ctx context.Context, id uuid.UUID) (passport.Passport, error)`

- `func (PassportRepository) GetByPassportNumber(ctx context.Context, series, number string) (passport.Passport, error)`

- `func (PassportRepository) Save(ctx context.Context, data passport.Passport) (passport.Passport, error)`

- `func (PassportRepository) Update(ctx context.Context, id uuid.UUID, data passport.Passport) (passport.Passport, error)`

#### PgGroupRepository

##### Methods

- `func (PgGroupRepository) Count(ctx context.Context, params *group.FindParams) (int64, error)`

- `func (PgGroupRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (PgGroupRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error)`

- `func (PgGroupRepository) GetByID(ctx context.Context, id uuid.UUID) (group.Group, error)`

- `func (PgGroupRepository) GetPaginated(ctx context.Context, params *group.FindParams) ([]group.Group, error)`

- `func (PgGroupRepository) Save(ctx context.Context, group group.Group) (group.Group, error)`

#### PgPermissionRepository

##### Methods

- `func (PgPermissionRepository) Count(ctx context.Context) (int64, error)`

- `func (PgPermissionRepository) Delete(ctx context.Context, id string) error`

- `func (PgPermissionRepository) GetAll(ctx context.Context) ([]*permission.Permission, error)`

- `func (PgPermissionRepository) GetByID(ctx context.Context, id string) (*permission.Permission, error)`

- `func (PgPermissionRepository) GetPaginated(ctx context.Context, params *permission.FindParams) ([]*permission.Permission, error)`

- `func (PgPermissionRepository) Save(ctx context.Context, data *permission.Permission) error`

#### PgUserRepository

##### Methods

- `func (PgUserRepository) Count(ctx context.Context, params *user.FindParams) (int64, error)`

- `func (PgUserRepository) Create(ctx context.Context, data user.User) (user.User, error)`

- `func (PgUserRepository) Delete(ctx context.Context, id uint) error`

- `func (PgUserRepository) EmailExists(ctx context.Context, email string) (bool, error)`

- `func (PgUserRepository) GetAll(ctx context.Context) ([]user.User, error)`

- `func (PgUserRepository) GetByEmail(ctx context.Context, email string) (user.User, error)`

- `func (PgUserRepository) GetByID(ctx context.Context, id uint) (user.User, error)`

- `func (PgUserRepository) GetByPhone(ctx context.Context, phone string) (user.User, error)`

- `func (PgUserRepository) GetPaginated(ctx context.Context, params *user.FindParams) ([]user.User, error)`

- `func (PgUserRepository) PhoneExists(ctx context.Context, phone string) (bool, error)`

- `func (PgUserRepository) Update(ctx context.Context, data user.User) error`

- `func (PgUserRepository) UpdateLastAction(ctx context.Context, id uint) error`

- `func (PgUserRepository) UpdateLastLogin(ctx context.Context, id uint) error`

#### SessionRepository

##### Methods

- `func (SessionRepository) Count(ctx context.Context) (int64, error)`

- `func (SessionRepository) Create(ctx context.Context, data *session.Session) error`

- `func (SessionRepository) Delete(ctx context.Context, token string) error`

- `func (SessionRepository) DeleteByUserId(ctx context.Context, userId uint) ([]*session.Session, error)`

- `func (SessionRepository) GetAll(ctx context.Context) ([]*session.Session, error)`

- `func (SessionRepository) GetByToken(ctx context.Context, token string) (*session.Session, error)`

- `func (SessionRepository) GetPaginated(ctx context.Context, params *session.FindParams) ([]*session.Session, error)`

- `func (SessionRepository) Update(ctx context.Context, data *session.Session) error`

#### TenantRepository

##### Methods

- `func (TenantRepository) Create(ctx context.Context, t *tenant.Tenant) (*tenant.Tenant, error)`

- `func (TenantRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (TenantRepository) GetByDomain(ctx context.Context, domain string) (*tenant.Tenant, error)`

- `func (TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*tenant.Tenant, error)`

- `func (TenantRepository) List(ctx context.Context) ([]*tenant.Tenant, error)`

- `func (TenantRepository) Update(ctx context.Context, t *tenant.Tenant) (*tenant.Tenant, error)`

### Functions

#### `func NewAuthLogRepository() authlog.Repository`

#### `func NewCurrencyRepository() currency.Repository`

#### `func NewGroupRepository(userRepo user.Repository, roleRepo role.Repository) group.Repository`

#### `func NewPassportRepository() passport.Repository`

#### `func NewPermissionRepository() permission.Repository`

#### `func NewRoleRepository() role.Repository`

#### `func NewSessionRepository() session.Repository`

#### `func NewTabRepository() tab.Repository`

#### `func NewTenantRepository() tenant.Repository`

#### `func NewUploadRepository() upload.Repository`

#### `func NewUserRepository(uploadRepo upload.Repository) user.Repository`

#### `func ToDBCurrency(entity *currency.Currency) *models.Currency`

#### `func ToDBGroup(g group.Group) *models.Group`

#### `func ToDBPassport(passportEntity passport.Passport) (*models.Passport, error)`

#### `func ToDBSession(session *session.Session) *models.Session`

#### `func ToDBTab(tab *tab.Tab) *models.Tab`

#### `func ToDBUpload(upload upload.Upload) *models.Upload`

#### `func ToDomainCurrency(dbCurrency *models.Currency) (*currency.Currency, error)`

#### `func ToDomainGroup(dbGroup *models.Group, users []user.User, roles []role.Role) (group.Group, error)`

#### `func ToDomainPassport(dbPassport *models.Passport) (passport.Passport, error)`

Passport mappers


#### `func ToDomainPin(s sql.NullString, c country.Country) (tax.Pin, error)`

#### `func ToDomainSession(dbSession *models.Session) *session.Session`

#### `func ToDomainTab(dbTab *models.Tab) (*tab.Tab, error)`

#### `func ToDomainTin(s sql.NullString, c country.Country) (tax.Tin, error)`

#### `func ToDomainUpload(dbUpload *models.Upload) (upload.Upload, error)`

#### `func ToDomainUser(dbUser *models.User, dbUpload *models.Upload, roles []role.Role, groupIDs []uuid.UUID, permissions []*permission.Permission) (user.User, error)`

### Variables and Constants

- Var: `[ErrAuthlogNotFound]`

- Var: `[ErrCurrencyNotFound]`

- Var: `[ErrGroupNotFound]`

- Var: `[ErrPassportNotFound]`

- Var: `[ErrPermissionNotFound]`

- Var: `[ErrRoleNotFound]`

- Var: `[ErrSessionNotFound]`

- Var: `[ErrTabNotFound]`

- Var: `[ErrTenantNotFound]`

- Var: `[ErrUploadNotFound]`

- Var: `[ErrUserNotFound]`

---

## Package `models` (modules/core/infrastructure/persistence/models)

### Types

#### AuthenticationLog

```go
type AuthenticationLog struct {
    ID uint
    TenantID string
    UserID uint
    IP string
    UserAgent string
    CreatedAt time.Time
}
```

#### Company

```go
type Company struct {
    ID uint
    TenantID string
    Name string
    About string
    Address string
    Phone string
    LogoID *uint
    CreatedAt time.Time
    UpdatedAt time.Time
    Logo Upload
}
```

#### Currency

```go
type Currency struct {
    Code string
    Name string
    Symbol string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Group

```go
type Group struct {
    ID string
    Type string
    TenantID string
    Name string
    Description sql.NullString
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### GroupRole

```go
type GroupRole struct {
    GroupID string
    RoleID uint
    CreatedAt time.Time
}
```

#### GroupUser

```go
type GroupUser struct {
    GroupID string
    UserID uint
    CreatedAt time.Time
}
```

#### Passport

```go
type Passport struct {
    ID string
    TenantID string
    FirstName sql.NullString
    LastName sql.NullString
    MiddleName sql.NullString
    Gender sql.NullString
    BirthDate sql.NullTime
    BirthPlace sql.NullString
    Nationality sql.NullString
    PassportType sql.NullString
    PassportNumber sql.NullString
    Series sql.NullString
    IssuingCountry sql.NullString
    IssuedAt sql.NullTime
    IssuedBy sql.NullString
    ExpiresAt sql.NullTime
    MachineReadableZone sql.NullString
    BiometricData []byte
    SignatureImage []byte
    Remarks sql.NullString
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Permission

```go
type Permission struct {
    ID string
    TenantID string
    Name string
    Resource string
    Action string
    Modifier string
    Description sql.NullString
}
```

#### Role

```go
type Role struct {
    ID uint
    Type string
    TenantID string
    Name string
    Description sql.NullString
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### RolePermission

```go
type RolePermission struct {
    RoleID uint
    PermissionID uint
}
```

#### Session

```go
type Session struct {
    Token string
    TenantID string
    UserID uint
    ExpiresAt time.Time
    IP string
    UserAgent string
    CreatedAt time.Time
}
```

#### Tab

```go
type Tab struct {
    ID uint
    TenantID string
    Href string
    Position uint
    UserID uint
}
```

#### Tenant

```go
type Tenant struct {
    ID string
    Name string
    Domain sql.NullString
    IsActive bool
    LogoID sql.NullInt32
    LogoCompactID sql.NullInt32
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Upload

```go
type Upload struct {
    ID uint
    TenantID string
    Hash string
    Path string
    Name string
    Size int
    Mimetype string
    Type string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### UploadedImage

```go
type UploadedImage struct {
    ID uint
    UploadID uint
    Type string
    Size float64
    Width int
    Height int
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### User

```go
type User struct {
    ID uint
    TenantID string
    Type string
    FirstName string
    LastName string
    MiddleName sql.NullString
    Email string
    Phone sql.NullString
    Password sql.NullString
    AvatarID sql.NullInt32
    LastLogin sql.NullTime
    LastIP sql.NullString
    UILanguage string
    LastAction sql.NullTime
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### UserRole

```go
type UserRole struct {
    UserID uint
    RoleID uint
    CreatedAt time.Time
}
```

---

## Package `query` (modules/core/infrastructure/query)

### Types

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy SortBy
    Search string
    Filters []Filter
}
```

#### GroupFilter

#### GroupFindParams

```go
type GroupFindParams struct {
    Limit int
    Offset int
    SortBy SortBy
    Search string
    Filters []GroupFilter
}
```

#### GroupQueryRepository

##### Interface Methods

- `FindGroups(ctx context.Context, params *GroupFindParams) ([]*viewmodels.Group, int, error)`
- `FindGroupByID(ctx context.Context, groupID string) (*viewmodels.Group, error)`
- `SearchGroups(ctx context.Context, params *GroupFindParams) ([]*viewmodels.Group, int, error)`

#### SortBy

#### UserQueryRepository

##### Interface Methods

- `FindUsers(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error)`
- `FindUserByID(ctx context.Context, userID int) (*viewmodels.User, error)`
- `SearchUsers(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error)`
- `FindUsersWithRoles(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error)`

### Functions

### Variables and Constants

---

## Package `graph` (modules/core/interfaces/graph)

### Types

#### ComplexityRoot

```go
type ComplexityRoot struct {
    Mutation struct{...}
    PaginatedUsers struct{...}
    Query struct{...}
    Session struct{...}
    Subscription struct{...}
    Upload struct{...}
    User struct{...}
}
```

#### Config

```go
type Config struct {
    Schema *ast.Schema
    Resolvers ResolverRoot
    Directives DirectiveRoot
    Complexity ComplexityRoot
}
```

#### DirectiveRoot

#### MutationResolver

##### Interface Methods

- `Add(ctx context.Context, a int, b int) (int, error)`
- `Authenticate(ctx context.Context, email string, password string) (*model.Session, error)`
- `GoogleAuthenticate(ctx context.Context) (string, error)`
- `DeleteSession(ctx context.Context, token string) (bool, error)`
- `UploadFile(ctx context.Context, file *graphql.Upload) (*model.Upload, error)`

#### QueryResolver

##### Interface Methods

- `Hello(ctx context.Context, name *string) (*string, error)`
- `Uploads(ctx context.Context, filter model.UploadFilter) ([]*model.Upload, error)`
- `User(ctx context.Context, id int64) (*model.User, error)`
- `Users(ctx context.Context, offset int, limit int, sortBy []int, ascending bool) (*model.PaginatedUsers, error)`

#### Resolver

##### Methods

- `func (Resolver) Mutation() MutationResolver`
  Mutation returns MutationResolver implementation.
  

- `func (Resolver) Query() QueryResolver`
  Query returns QueryResolver implementation.
  

- `func (Resolver) Subscription() SubscriptionResolver`
  Subscription returns SubscriptionResolver implementation.
  

#### ResolverRoot

##### Interface Methods

- `Mutation() MutationResolver`
- `Query() QueryResolver`
- `Subscription() SubscriptionResolver`

#### SubscriptionResolver

##### Interface Methods

- `Counter(ctx context.Context) (chan int, error)`
- `SessionDeleted(ctx context.Context) (chan int64, error)`

### Functions

#### `func NewExecutableSchema(cfg Config) graphql.ExecutableSchema`

NewExecutableSchema creates an ExecutableSchema from the ResolverRoot interface.


### Variables and Constants

---

## Package `model` (modules/core/interfaces/graph/gqlmodels)

### Types

#### Mutation

#### PaginatedUsers

```go
type PaginatedUsers struct {
    Data []*User `json:"data"`
    Total int64 `json:"total"`
}
```

#### Query

#### Session

```go
type Session struct {
    Token string `json:"token"`
    UserID int64 `json:"userId"`
    IP string `json:"ip"`
    UserAgent string `json:"userAgent"`
    ExpiresAt time.Time `json:"expiresAt"`
    CreatedAt time.Time `json:"createdAt"`
}
```

#### Subscription

#### Upload

```go
type Upload struct {
    ID int64 `json:"id"`
    URL string `json:"url"`
    Hash string `json:"hash"`
    Path string `json:"path"`
    Name string `json:"name"`
    Mimetype string `json:"mimetype"`
    Type upload.UploadType `json:"type"`
    Size int `json:"size"`
}
```

#### UploadFilter

```go
type UploadFilter struct {
    MimeType *string `json:"mimeType,omitempty"`
    MimeTypePrefix *string `json:"mimeTypePrefix,omitempty"`
    Type *upload.UploadType `json:"type,omitempty"`
    Sort *UploadSort `json:"sort,omitempty"`
}
```

#### UploadSort

```go
type UploadSort struct {
    Field UploadSortField `json:"field"`
    Ascending bool `json:"ascending"`
}
```

#### UploadSortField

##### Methods

- `func (UploadSortField) IsValid() bool`

- `func (UploadSortField) MarshalGQL(w io.Writer)`

- `func (UploadSortField) String() string`

- `func (UploadSortField) UnmarshalGQL(v interface{}) error`

#### User

```go
type User struct {
    ID int64 `json:"id"`
    FirstName string `json:"firstName"`
    LastName string `json:"lastName"`
    Email string `json:"email"`
    UILanguage string `json:"uiLanguage"`
    UpdatedAt time.Time `json:"updatedAt"`
    CreatedAt time.Time `json:"createdAt"`
}
```

### Variables and Constants

- Var: `[AllUploadSortField]`

---

## Package `mappers` (modules/core/interfaces/graph/mappers)

### Functions

#### `func SessionToGraphModel(s *session.Session) *model.Session`

#### `func UploadToGraphModel(u upload.Upload) *model.Upload`

#### `func UserToGraphModel(u user.User) *model.User`

---

## Package `permissions` (modules/core/permissions)

### Variables and Constants

- Var: `[UserCreate UserRead UserUpdate UserDelete RoleCreate RoleRead RoleUpdate RoleDelete]`

- Var: `[Permissions]`

- Const: `[ResourceUser ResourceRole ResourceUpload]`

---

## Package `assets` (modules/core/presentation/assets)

### Variables and Constants

- Var: `[FS]`

- Var: `[HashFS]`

---

## Package `controllers` (modules/core/presentation/controllers)

### Types

#### AccountController

##### Methods

- `func (AccountController) Get(w http.ResponseWriter, r *http.Request)`

- `func (AccountController) GetSettings(w http.ResponseWriter, r *http.Request)`

- `func (AccountController) Key() string`

- `func (AccountController) PostSettings(w http.ResponseWriter, r *http.Request)`

- `func (AccountController) Register(r *mux.Router)`

- `func (AccountController) Update(w http.ResponseWriter, r *http.Request)`

#### ChartEventRequest

ChartEventRequest represents the request payload for chart events


```go
type ChartEventRequest struct {
    PanelID string `json:"panelId"`
    EventType string `json:"eventType"`
    ChartType string `json:"chartType"`
    ActionConfig lens.ActionConfig `json:"actionConfig"`
    DataPoint *lens.DataPointContext `json:"dataPoint,omitempty"`
    SeriesIndex *int `json:"seriesIndex,omitempty"`
    DataIndex *int `json:"dataIndex,omitempty"`
    Label string `json:"label,omitempty"`
    Value interface{} `json:"value,omitempty"`
    SeriesName string `json:"seriesName,omitempty"`
    CategoryName string `json:"categoryName,omitempty"`
    Variables map[string]interface{} `json:"variables,omitempty"`
    CustomData map[string]interface{} `json:"customData,omitempty"`
}
```

#### ChartEventResponse

ChartEventResponse represents the response for chart events


```go
type ChartEventResponse struct {
    Success bool `json:"success"`
    Result *lens.EventResult `json:"result,omitempty"`
    Error string `json:"error,omitempty"`
    Headers map[string]string `json:"headers,omitempty"`
}
```

#### CrudController

##### Methods

- `func (CrudController) Create(w http.ResponseWriter, r *http.Request)`

- `func (CrudController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (CrudController) Details(w http.ResponseWriter, r *http.Request)`

- `func (CrudController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (CrudController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (CrudController) Key() string`

- `func (CrudController) List(w http.ResponseWriter, r *http.Request)`

- `func (CrudController) Register(r *mux.Router)`

- `func (CrudController) Update(w http.ResponseWriter, r *http.Request)`

#### CrudOption

CrudOption defines options for CrudController


#### DashboardController

##### Methods

- `func (DashboardController) Get(w http.ResponseWriter, r *http.Request)`

- `func (DashboardController) Key() string`

- `func (DashboardController) Register(r *mux.Router)`

#### GraphQLController

##### Methods

- `func (GraphQLController) Key() string`

- `func (GraphQLController) Register(r *mux.Router)`

#### GroupRealtimeUpdates

##### Methods

- `func (GroupRealtimeUpdates) Register()`

#### GroupsController

##### Methods

- `func (GroupsController) Create(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, groupService *services.GroupService, roleService *services.RoleService)`

- `func (GroupsController) Delete(r *http.Request, w http.ResponseWriter, groupService *services.GroupService)`

- `func (GroupsController) GetEdit(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, groupQueryService *services.GroupQueryService, roleService *services.RoleService)`

- `func (GroupsController) GetNew(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, roleService *services.RoleService)`

- `func (GroupsController) Groups(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, groupQueryService *services.GroupQueryService)`

- `func (GroupsController) Key() string`

- `func (GroupsController) Register(r *mux.Router)`

- `func (GroupsController) Update(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, groupService *services.GroupService, roleService *services.RoleService)`

#### HealthController

##### Methods

- `func (HealthController) Get(w http.ResponseWriter, r *http.Request)`

- `func (HealthController) Key() string`

- `func (HealthController) Register(r *mux.Router)`

#### LensEventsController

LensEventsController handles chart event requests from the UI


##### Methods

- `func (LensEventsController) HandleChartEvent(w http.ResponseWriter, r *http.Request)`
  HandleChartEvent processes chart click events via HTMX
  

- `func (LensEventsController) Key() string`

- `func (LensEventsController) Register(r *mux.Router)`

#### LoginController

##### Methods

- `func (LoginController) Get(w http.ResponseWriter, r *http.Request)`

- `func (LoginController) GoogleCallback(w http.ResponseWriter, r *http.Request)`

- `func (LoginController) Key() string`

- `func (LoginController) Post(w http.ResponseWriter, r *http.Request)`

- `func (LoginController) Register(r *mux.Router)`

#### LoginDTO

```go
type LoginDTO struct {
    Email string `validate:"required"`
    Password string `validate:"required"`
}
```

##### Methods

- `func (LoginDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### LogoutController

##### Methods

- `func (LogoutController) Key() string`

- `func (LogoutController) Logout(w http.ResponseWriter, r *http.Request)`

- `func (LogoutController) Register(r *mux.Router)`

#### RolesController

##### Methods

- `func (RolesController) Create(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, roleService *services.RoleService)`

- `func (RolesController) Delete(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, roleService *services.RoleService)`

- `func (RolesController) GetEdit(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, roleService *services.RoleService)`

- `func (RolesController) GetNew(r *http.Request, w http.ResponseWriter, logger *logrus.Entry)`

- `func (RolesController) Key() string`

- `func (RolesController) List(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, roleService *services.RoleService)`

- `func (RolesController) Register(r *mux.Router)`

- `func (RolesController) Update(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, roleService *services.RoleService)`

#### SettingsController

##### Methods

- `func (SettingsController) GetLogo(w http.ResponseWriter, r *http.Request)`

- `func (SettingsController) Key() string`

- `func (SettingsController) PostLogo(w http.ResponseWriter, r *http.Request)`

- `func (SettingsController) Register(r *mux.Router)`

#### ShowcaseController

##### Methods

- `func (ShowcaseController) Charts(r *http.Request, w http.ResponseWriter, logger *logrus.Entry)`

- `func (ShowcaseController) Form(r *http.Request, w http.ResponseWriter, logger *logrus.Entry)`

- `func (ShowcaseController) Key() string`

- `func (ShowcaseController) Lens(r *http.Request, w http.ResponseWriter, logger *logrus.Entry)`

- `func (ShowcaseController) Loaders(r *http.Request, w http.ResponseWriter, logger *logrus.Entry)`

- `func (ShowcaseController) Other(r *http.Request, w http.ResponseWriter, logger *logrus.Entry)`

- `func (ShowcaseController) Overview(r *http.Request, w http.ResponseWriter, logger *logrus.Entry)`

- `func (ShowcaseController) Register(r *mux.Router)`

#### SpotlightController

##### Methods

- `func (SpotlightController) Get(w http.ResponseWriter, r *http.Request)`

- `func (SpotlightController) Key() string`

- `func (SpotlightController) Register(r *mux.Router)`

#### StaticFilesController

##### Methods

- `func (StaticFilesController) Key() string`

- `func (StaticFilesController) Register(r *mux.Router)`

#### UploadController

##### Methods

- `func (UploadController) Create(w http.ResponseWriter, r *http.Request)`

- `func (UploadController) Key() string`

- `func (UploadController) Register(r *mux.Router)`

#### UserRealtimeUpdates

##### Methods

- `func (UserRealtimeUpdates) Register()`

#### UsersController

##### Methods

- `func (UsersController) Create(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, userService *services.UserService, roleService *services.RoleService, groupQueryService *services.GroupQueryService)`

- `func (UsersController) Delete(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, userService *services.UserService)`

- `func (UsersController) GetEdit(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, userService *services.UserService, roleService *services.RoleService, groupQueryService *services.GroupQueryService)`

- `func (UsersController) GetNew(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, roleService *services.RoleService, groupQueryService *services.GroupQueryService)`

- `func (UsersController) Key() string`

- `func (UsersController) Register(r *mux.Router)`

- `func (UsersController) Update(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, userService *services.UserService, roleService *services.RoleService, groupQueryService *services.GroupQueryService, permissionService *services.PermissionService)`

- `func (UsersController) Users(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, userService *services.UserService, userQueryService *services.UserQueryService, groupQueryService *services.GroupQueryService)`

#### WebSocketController

##### Methods

- `func (WebSocketController) Key() string`

- `func (WebSocketController) Register(r *mux.Router)`

### Functions

#### `func MethodNotAllowed() http.HandlerFunc`

#### `func NewAccountController(app application.Application) application.Controller`

#### `func NewCrudController(basePath string, app application.Application, builder <?>, opts ...<?>) application.Controller`

#### `func NewDashboardController(app application.Application) application.Controller`

#### `func NewGraphQLController(app application.Application) application.Controller`

#### `func NewGroupsController(app application.Application) application.Controller`

#### `func NewHealthController(app application.Application) application.Controller`

#### `func NewLensEventsController(app application.Application) application.Controller`

NewLensEventsController creates a new lens events controller


#### `func NewLoginController(app application.Application) application.Controller`

#### `func NewLogoutController(app application.Application) application.Controller`

#### `func NewRolesController(app application.Application) application.Controller`

#### `func NewSettingsController(app application.Application) application.Controller`

#### `func NewShowcaseController(app application.Application) application.Controller`

#### `func NewSpotlightController(app application.Application) application.Controller`

#### `func NewStaticFilesController(fsInstances []*hashfs.FS) application.Controller`

#### `func NewUploadController(app application.Application) application.Controller`

#### `func NewUsersController(app application.Application) application.Controller`

#### `func NewWebSocketController(app application.Application) application.Controller`

#### `func NotFound(app application.Application) http.HandlerFunc`

### Variables and Constants

---

## Package `dtos` (modules/core/presentation/controllers/dtos)

### Types

#### CreateGroupDTO

```go
type CreateGroupDTO struct {
    Name string `validate:"required"`
    Description string `validate:"omitempty" label:"_Description"`
    RoleIDs []string `validate:"omitempty,dive,required"`
}
```

##### Methods

- `func (CreateGroupDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CreateGroupDTO) ToEntity() (group.Group, error)`

#### CreateRoleDTO

```go
type CreateRoleDTO struct {
    Name string `validate:"required"`
    Description string `validate:"required" label:"_Description"`
    Permissions map[string]string `validate:"omitempty,dive,required"`
}
```

##### Methods

- `func (CreateRoleDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CreateRoleDTO) ToEntity(rbac rbac.RBAC) (role.Role, error)`

#### CreateUserDTO

```go
type CreateUserDTO struct {
    FirstName string `validate:"required"`
    LastName string `validate:"required"`
    MiddleName string `validate:"omitempty"`
    Email string `validate:"required,email"`
    Phone string `validate:"omitempty"`
    Password string `validate:"omitempty"`
    RoleIDs []uint `validate:"omitempty,dive,required"`
    GroupIDs []string `validate:"omitempty,dive,required"`
    AvatarID uint `validate:"omitempty,gt=0"`
    Language string `validate:"required"`
}
```

##### Methods

- `func (CreateUserDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CreateUserDTO) ToEntity(tenantID uuid.UUID) (user.User, error)`

#### SaveAccountDTO

```go
type SaveAccountDTO struct {
    FirstName string `validate:"required"`
    LastName string `validate:"required"`
    Phone string
    MiddleName string
    Language string `validate:"required"`
    AvatarID uint
}
```

##### Methods

- `func (SaveAccountDTO) Apply(u user.User) (user.User, error)`

- `func (SaveAccountDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### SaveLogosDTO

```go
type SaveLogosDTO struct {
    LogoID int `validate:"omitempty,min=1"`
    LogoCompactID int `validate:"omitempty,min=1"`
}
```

##### Methods

- `func (SaveLogosDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### UpdateGroupDTO

```go
type UpdateGroupDTO struct {
    Name string `validate:"required"`
    Description string `validate:"omitempty" label:"_Description"`
    RoleIDs []string `validate:"omitempty,dive,required"`
}
```

##### Methods

- `func (UpdateGroupDTO) Apply(g group.Group, roles []role.Role) (group.Group, error)`

- `func (UpdateGroupDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### UpdateRoleDTO

```go
type UpdateRoleDTO struct {
    Name string `validate:"required"`
    Description string `validate:"required" label:"_Description"`
    Permissions map[string]string `validate:"omitempty,dive,required"`
}
```

##### Methods

- `func (UpdateRoleDTO) Apply(roleEntity role.Role, rbac rbac.RBAC) (role.Role, error)`

- `func (UpdateRoleDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### UpdateUserDTO

```go
type UpdateUserDTO struct {
    FirstName string `validate:"required"`
    LastName string `validate:"required"`
    MiddleName string `validate:"omitempty"`
    Email string `validate:"required,email"`
    Phone string `validate:"omitempty"`
    Password string `validate:"omitempty"`
    RoleIDs []uint `validate:"omitempty,dive,required"`
    GroupIDs []string `validate:"omitempty,dive,required"`
    AvatarID uint `validate:"omitempty,gt=0"`
    Language string `validate:"required"`
}
```

##### Methods

- `func (UpdateUserDTO) Apply(u user.User, roles []role.Role, permissions []*permission.Permission) (user.User, error)`

- `func (UpdateUserDTO) Ok(ctx context.Context) (map[string]string, bool)`

---

## Package `mappers` (modules/core/presentation/mappers)

### Functions

#### `func CurrencyToViewModel(entity *currency.Currency) *viewmodels.Currency`

#### `func GroupToViewModel(entity group.Group) *viewmodels.Group`

#### `func PermissionToViewModel(entity *permission.Permission) *viewmodels.Permission`

#### `func RoleToViewModel(entity role.Role) *viewmodels.Role`

#### `func TabToViewModel(entity *tab.Tab) *viewmodels.Tab`

#### `func UploadToViewModel(entity upload.Upload) *viewmodels.Upload`

#### `func UserToViewModel(entity user.User) *viewmodels.User`

---

## Package `components` (modules/core/presentation/templates/components)

templ: version: v0.3.857


### Types

#### CurrencySelectProps

```go
type CurrencySelectProps struct {
    Label string
    Placeholder string
    Value string
    Error string
    Currencies []*viewmodels.Currency
    Attrs templ.Attributes
}
```

### Functions

#### `func CurrencySelect(props *CurrencySelectProps) templ.Component`

### Variables and Constants

---

## Package `layouts` (modules/core/presentation/templates/layouts)

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### AuthenticatedProps

#### BaseProps

```go
type BaseProps struct {
    Title string
    WebsocketURL string
}
```

### Functions

#### `func Authenticated(props AuthenticatedProps) templ.Component`

#### `func Avatar() templ.Component`

#### `func Base(props *BaseProps) templ.Component`

#### `func DefaultHead() templ.Component`

#### `func DefaultSidebarFooter() templ.Component`

#### `func DefaultSidebarHeader() templ.Component`

#### `func MapNavItemToSidebar(navItem types.NavigationItem) sidebar.Item`

#### `func MapNavItemsToSidebar(navItems []types.NavigationItem) []sidebar.Item`

#### `func MobileSidebar(props sidebar.Props) templ.Component`

#### `func MustUseHead(ctx context.Context) templ.Component`

MustUseHead returns the head component from the context or panics


#### `func MustUseLogo(ctx context.Context) templ.Component`

MustUseLogo returns the logo component from the context or panics


#### `func MustUseSidebarProps(ctx context.Context) sidebar.Props`

MustUseSidebarProps returns the sidebar props from the context or panics


#### `func Navbar(pageCtx *types.PageContext) templ.Component`

#### `func SidebarFooter(pageCtx *types.PageContext) templ.Component`

#### `func SidebarTrigger(class string) templ.Component`

#### `func ThemeSwitcher() templ.Component`

#### `func UseHead(ctx context.Context) (templ.Component, error)`

UseHead returns the head component from the context


#### `func UseLogo(ctx context.Context) (templ.Component, error)`

UseLogo returns the logo component from the context


#### `func UseSidebarProps(ctx context.Context) (sidebar.Props, error)`

UseSidebarProps returns the sidebar props from the context


### Variables and Constants

- Var: `[ErrNoLogoFound ErrNoHeadFound ErrNoSidebarPropsFound]`

---

## Package `account` (modules/core/presentation/templates/pages/account)

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### ProfilePageProps

```go
type ProfilePageProps struct {
    User *viewmodels.User
    Errors map[string]string
    PostPath string
}
```

#### SettingsPageProps

```go
type SettingsPageProps struct {
    Tabs []*viewmodels.Tab
    AllNavItems []types.NavigationItem
}
```

### Functions

#### `func Index(props *ProfilePageProps) templ.Component`

#### `func NavItems(items []types.NavigationItem, tabs []*viewmodels.Tab, depth int, class string) templ.Component`

#### `func ProfileForm(props *ProfilePageProps) templ.Component`

#### `func SidebarSettings(props *SettingsPageProps) templ.Component`

#### `func SidebarSettingsForm(props *SettingsPageProps) templ.Component`

### Variables and Constants

---

## Package `dashboard` (modules/core/presentation/templates/pages/dashboard)

templ: version: v0.3.857


### Types

#### IndexPageProps

```go
type IndexPageProps struct {
    Dashboard lens.DashboardConfig
    DashboardResult *executor.DashboardResult
}
```

### Functions

#### `func DashboardContent(props *IndexPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

### Variables and Constants

---

## Package `error_pages` (modules/core/presentation/templates/pages/error_pages)

templ: version: v0.3.857


### Functions

#### `func NotFoundContent() templ.Component`

### Variables and Constants

---

## Package `groups` (modules/core/presentation/templates/pages/groups)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreateFormProps

```go
type CreateFormProps struct {
    Group *GroupFormData
    Roles []*viewmodels.Role
    Errors map[string]string
}
```

#### EditFormProps

```go
type EditFormProps struct {
    Group *viewmodels.Group
    Roles []*viewmodels.Role
    Errors map[string]string
}
```

#### GroupFormData

```go
type GroupFormData struct {
    Name string
    Description string
    RoleIDs []string
}
```

#### IndexPageProps

IndexPageProps holds the data for rendering the groups index page


```go
type IndexPageProps struct {
    Groups []*viewmodels.Group
    Page int
    PerPage int
    Search string
    HasMore bool
}
```

#### RoleSelectProps

```go
type RoleSelectProps struct {
    Roles []*viewmodels.Role
    Selected []*viewmodels.Role
    Name string
    Form string
    Error string
}
```

#### SharedProps

```go
type SharedProps struct {
    Value string
    Form string
    Error string
}
```

### Functions

#### `func CreateForm(props *CreateFormProps) templ.Component`

#### `func EditForm(props *EditFormProps) templ.Component`

#### `func EditGroupDrawer(props *EditFormProps) templ.Component`

#### `func GroupCreatedEvent(group *viewmodels.Group, rowProps *base.TableRowProps) templ.Component`

#### `func GroupRow(group *viewmodels.Group, rowProps *base.TableRowProps) templ.Component`

#### `func GroupRows(props *IndexPageProps) templ.Component`

#### `func GroupsContent(props *IndexPageProps) templ.Component`

#### `func GroupsTable(props *IndexPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func ModernRoleSelect(props *RoleSelectProps) templ.Component`

Modern styled version of role select with checkboxes


#### `func NewGroupDrawer() templ.Component`

### Variables and Constants

---

## Package `login` (modules/core/presentation/templates/pages/login)

templ: version: v0.3.857


### Types

#### LoginProps

```go
type LoginProps struct {
    ErrorsMap map[string]string
    ErrorMessage string
    Email string
    GoogleOAuthCodeURL string
}
```

### Functions

#### `func GoogleIcon() templ.Component`

#### `func Header() templ.Component`

#### `func Index(p *LoginProps) templ.Component`

### Variables and Constants

---

## Package `roles` (modules/core/presentation/templates/pages/roles)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreateFormProps

```go
type CreateFormProps struct {
    Role *viewmodels.Role
    PermissionGroups []*viewmodels.PermissionGroup
    Errors map[string]string
}
```

#### EditFormProps

```go
type EditFormProps struct {
    Role *viewmodels.Role
    PermissionGroups []*viewmodels.PermissionGroup
    Errors map[string]string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Roles []*viewmodels.Role
    Search string
}
```

#### SharedProps

```go
type SharedProps struct {
    Label string
    Attrs templ.Attributes
    Error string
    Checked bool
}
```

### Functions

#### `func CreateForm(props *CreateFormProps) templ.Component`

#### `func Edit(props *EditFormProps) templ.Component`

#### `func EditForm(props *EditFormProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreateFormProps) templ.Component`

#### `func Permission(props SharedProps) templ.Component`

#### `func RoleRow(role *viewmodels.Role) templ.Component`

#### `func RoleRows(props *IndexPageProps) templ.Component`

#### `func RolesContent(props *IndexPageProps) templ.Component`

#### `func RolesTable(props *IndexPageProps) templ.Component`

### Variables and Constants

---

## Package `settings` (modules/core/presentation/templates/pages/settings)

templ: version: v0.3.857


### Types

#### LogoPageProps

```go
type LogoPageProps struct {
    LogoUpload *viewmodels.Upload
    LogoCompactUpload *viewmodels.Upload
    Errors map[string]string
    PostPath string
}
```

### Functions

#### `func Logo(props *LogoPageProps) templ.Component`

#### `func LogoForm(props *LogoPageProps) templ.Component`

### Variables and Constants

---

## Package `showcase` (modules/core/presentation/templates/pages/showcase)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### IndexPageProps

```go
type IndexPageProps struct {
    SidebarProps sidebar.Props
}
```

#### LayoutProps

```go
type LayoutProps struct {
    Title string
    SidebarProps sidebar.Props
}
```

#### LensPageProps

```go
type LensPageProps struct {
    SidebarProps sidebar.Props
    Dashboard lens.DashboardConfig
    DashboardResult *executor.DashboardResult
}
```

#### ShowcaseProps

```go
type ShowcaseProps struct {
    Title string
    Code string
}
```

### Functions

#### `func ChartsContent() templ.Component`

#### `func ChartsPage(props IndexPageProps) templ.Component`

#### `func ComponentShowcase(props ShowcaseProps) templ.Component`

#### `func Content() templ.Component`

#### `func FormContent() templ.Component`

#### `func FormPage(props IndexPageProps) templ.Component`

#### `func Index(props IndexPageProps) templ.Component`

#### `func Layout(props LayoutProps) templ.Component`

#### `func LensContent(props LensPageProps) templ.Component`

#### `func LensPage(props LensPageProps) templ.Component`

#### `func LoadersContent() templ.Component`

#### `func LoadersPage(props IndexPageProps) templ.Component`

#### `func OtherContent() templ.Component`

#### `func OtherPage(props IndexPageProps) templ.Component`

#### `func OverviewContent() templ.Component`

#### `func OverviewPage(props IndexPageProps) templ.Component`

### Variables and Constants

- Var: `[AdvancedDatepickerComponentSource]`

- Var: `[AreaChartSource]`

- Var: `[AvatarComponentSource]`

- Var: `[BarChartSource]`
  Chart components
  

- Var: `[ButtonsComponentSource]`

- Var: `[CardComponentSource]`

- Var: `[ComboboxComponentSource]`

- Var: `[DatepickerComponentSource]`

- Var: `[DonutChartSource]`

- Var: `[HeatmapChartSource]`

- Var: `[InputComponentSource]`

- Var: `[LineChartSource]`

- Var: `[NavTabsComponentSource]`

- Var: `[NumberComponentSource]`

- Var: `[PieChartSource]`

- Var: `[PolarAreaChartSource]`

- Var: `[RadarChartSource]`

- Var: `[RadialBarChartSource]`

- Var: `[RadioComponentSource]`

- Var: `[ScatterChartSource]`

- Var: `[SelectComponentSource]`

- Var: `[SkeletonsComponentSource]`

- Var: `[SliderComponentSource]`

- Var: `[SpinnersComponentSource]`

- Var: `[TableComponentSource]`

- Var: `[TextareaComponentSource]`

---

## Package `components` (modules/core/presentation/templates/pages/showcase/components)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Functions

#### `func AdvancedDatePicker() templ.Component`

#### `func Avatars() templ.Component`

#### `func BasicSelect() templ.Component`

#### `func Buttons() templ.Component`

#### `func Card() templ.Component`

#### `func DateInput() templ.Component`

#### `func NavTabs() templ.Component`

#### `func NumberInput() templ.Component`

#### `func RadioGroup() templ.Component`

#### `func SearchableSelect() templ.Component`

#### `func Skeletons() templ.Component`

#### `func SliderInput() templ.Component`

#### `func Spinners() templ.Component`

#### `func Table() templ.Component`

#### `func TextArea() templ.Component`

#### `func TextInput() templ.Component`

### Variables and Constants

---

## Package `components` (modules/core/presentation/templates/pages/showcase/components/charts)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Functions

#### `func AreaChart() templ.Component`

#### `func BarChart() templ.Component`

#### `func DonutChart() templ.Component`

#### `func HeatmapChart() templ.Component`

#### `func LineChart() templ.Component`

#### `func PieChart() templ.Component`

#### `func PolarAreaChart() templ.Component`

#### `func RadarChart() templ.Component`

#### `func RadialBarChart() templ.Component`

#### `func ScatterChart() templ.Component`

### Variables and Constants

---

## Package `users` (modules/core/presentation/templates/pages/users)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreateFormProps

```go
type CreateFormProps struct {
    User viewmodels.User
    Roles []*viewmodels.Role
    Groups []*viewmodels.Group
    PermissionGroups []*viewmodels.PermissionGroup
    Errors map[string]string
}
```

#### EditFormProps

```go
type EditFormProps struct {
    User *viewmodels.User
    Roles []*viewmodels.Role
    Groups []*viewmodels.Group
    PermissionGroups []*viewmodels.PermissionGroup
    Errors map[string]string
}
```

#### GroupSelectProps

```go
type GroupSelectProps struct {
    Groups []*viewmodels.Group
    Selected []string
    Name string
    Form string
    Error string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Users []*viewmodels.User
    Groups []*viewmodels.Group
    Page int
    PerPage int
    HasMore bool
}
```

#### RoleSelectProps

```go
type RoleSelectProps struct {
    Roles []*viewmodels.Role
    Selected []*viewmodels.Role
    Name string
    Form string
    Error string
}
```

#### SharedProps

```go
type SharedProps struct {
    Value string
    Form string
    Error string
}
```

### Functions

#### `func CreateForm(props *CreateFormProps) templ.Component`

#### `func Edit(props *EditFormProps) templ.Component`

#### `func EditForm(props *EditFormProps) templ.Component`

#### `func EmailInput(props SharedProps) templ.Component`

#### `func GroupFilter(props *IndexPageProps) templ.Component`

#### `func GroupSelect(props *GroupSelectProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreateFormProps) templ.Component`

#### `func RoleSelect(props *RoleSelectProps) templ.Component`

#### `func UserCreatedEvent(user *viewmodels.User, rowProps *base.TableRowProps) templ.Component`

#### `func UserRow(user *viewmodels.User, rowProps *base.TableRowProps) templ.Component`

#### `func UserRows(props *IndexPageProps) templ.Component`

#### `func UsersContent(props *IndexPageProps) templ.Component`

#### `func UsersTable(props *IndexPageProps) templ.Component`

### Variables and Constants

---

## Package `viewmodels` (modules/core/presentation/viewmodels)

### Types

#### Currency

```go
type Currency struct {
    Code string
    Name string
    Symbol string
}
```

#### Group

```go
type Group struct {
    ID string
    Type string
    Name string
    Description string
    Roles []*Role
    Users []*User
    CreatedAt string
    UpdatedAt string
    CanUpdate bool
    CanDelete bool
}
```

##### Methods

- `func (Group) GetInitials() string`
  GetInitials returns the first letters of each word in the group name
  

- `func (Group) UsersCount() int`

#### Permission

```go
type Permission struct {
    ID string
    Name string
    Resource string
    Action string
    Modifier string
}
```

##### Methods

- `func (Permission) DisplayName() string`

#### PermissionGroup

PermissionGroup represents a group of permissions categorized by resource
Used for UI rendering in forms


```go
type PermissionGroup struct {
    Resource string
    Permissions []*PermissionItem
}
```

#### PermissionItem

PermissionItem represents a single permission with its selection state
Used for UI rendering in forms


```go
type PermissionItem struct {
    ID string
    Name string
    Checked bool
}
```

#### Role

```go
type Role struct {
    ID string
    Type string
    Name string
    Description string
    CreatedAt string
    UpdatedAt string
    CanUpdate bool
    CanDelete bool
}
```

#### Tab

```go
type Tab struct {
    ID string
    Href string
}
```

#### Upload

```go
type Upload struct {
    ID string
    Hash string
    URL string
    Mimetype string
    Size string
    CreatedAt string
    UpdatedAt string
}
```

#### User

```go
type User struct {
    ID string
    Type string
    FirstName string
    LastName string
    MiddleName string
    Email string
    Phone string
    Language string
    LastAction string
    CreatedAt string
    UpdatedAt string
    AvatarID string
    Roles []*Role
    GroupIDs []string
    Permissions []*Permission
    Avatar *Upload
    CanUpdate bool
    CanDelete bool
}
```

##### Methods

- `func (User) FullName() string`

- `func (User) Initials() string`

- `func (User) RolesVerbose() string`

---

## Package `seed` (modules/core/seed)

### Types

### Functions

#### `func CreateCurrencies(ctx context.Context, app application.Application) error`

#### `func CreateDefaultTenant(ctx context.Context, app application.Application) error`

#### `func CreatePermissions(ctx context.Context, app application.Application) error`

#### `func GroupsSeedFunc(groups ...group.Group) application.SeedFunc`

#### `func UserSeedFunc(usr user.User) application.SeedFunc`

### Variables and Constants

---

## Package `services` (modules/core/services)

### Types

#### AuthLogService

##### Methods

- `func (AuthLogService) Count(ctx context.Context) (int64, error)`

- `func (AuthLogService) Create(ctx context.Context, data *authlog.AuthenticationLog) error`

- `func (AuthLogService) Delete(ctx context.Context, id uint) error`

- `func (AuthLogService) GetAll(ctx context.Context) ([]*authlog.AuthenticationLog, error)`

- `func (AuthLogService) GetByID(ctx context.Context, id uint) (*authlog.AuthenticationLog, error)`

- `func (AuthLogService) GetPaginated(ctx context.Context, params *authlog.FindParams) ([]*authlog.AuthenticationLog, error)`

- `func (AuthLogService) Update(ctx context.Context, data *authlog.AuthenticationLog) error`

#### AuthService

##### Methods

- `func (AuthService) Authenticate(ctx context.Context, email, password string) (user.User, *session.Session, error)`

- `func (AuthService) AuthenticateGoogle(ctx context.Context, code string) (user.User, *session.Session, error)`

- `func (AuthService) AuthenticateWithUserID(ctx context.Context, id uint, password string) (user.User, *session.Session, error)`

- `func (AuthService) Authorize(ctx context.Context, token string) (*session.Session, error)`

- `func (AuthService) CookieAuthenticate(ctx context.Context, email, password string) (*http.Cookie, error)`

- `func (AuthService) CookieAuthenticateWithUserID(ctx context.Context, id uint, password string) (*http.Cookie, error)`

- `func (AuthService) CookieGoogleAuthenticate(ctx context.Context, code string) (*http.Cookie, error)`

- `func (AuthService) GoogleAuthenticate(w http.ResponseWriter) (string, error)`

- `func (AuthService) Logout(ctx context.Context, token string) error`

#### CurrencyService

```go
type CurrencyService struct {
    Repo currency.Repository
    Publisher eventbus.EventBus
}
```

##### Methods

- `func (CurrencyService) Create(ctx context.Context, data *currency.CreateDTO) error`

- `func (CurrencyService) Delete(ctx context.Context, code string) (*currency.Currency, error)`

- `func (CurrencyService) GetAll(ctx context.Context) ([]*currency.Currency, error)`

- `func (CurrencyService) GetByCode(ctx context.Context, id string) (*currency.Currency, error)`

- `func (CurrencyService) GetPaginated(ctx context.Context, params *currency.FindParams) ([]*currency.Currency, error)`

- `func (CurrencyService) Update(ctx context.Context, data *currency.UpdateDTO) error`

#### ExcelExportService

ExcelExportService handles Excel export operations


##### Methods

- `func (ExcelExportService) ExportFromDataSource(ctx context.Context, datasource excel.DataSource, config exportconfig.ExportConfig) (upload.Upload, error)`
  ExportFromDataSource exports from a custom data source to Excel
  

- `func (ExcelExportService) ExportFromQuery(ctx context.Context, query exportconfig.Query, config exportconfig.ExportConfig) (upload.Upload, error)`
  ExportFromQuery exports SQL query results to Excel and saves as upload
  

#### GroupQueryService

##### Methods

- `func (GroupQueryService) FindGroupByID(ctx context.Context, groupID string) (*viewmodels.Group, error)`

- `func (GroupQueryService) FindGroups(ctx context.Context, params *query.GroupFindParams) ([]*viewmodels.Group, int, error)`

- `func (GroupQueryService) SearchGroups(ctx context.Context, params *query.GroupFindParams) ([]*viewmodels.Group, int, error)`

#### GroupService

GroupService TODO: refactor it
GroupService provides operations for managing groups


##### Methods

- `func (GroupService) AddUser(ctx context.Context, groupID uuid.UUID, userToAdd user.User) (group.Group, error)`
  AddUser adds a user to a group
  

- `func (GroupService) AssignRole(ctx context.Context, groupID uuid.UUID, roleToAssign role.Role) (group.Group, error)`
  AssignRole assigns a role to a group
  

- `func (GroupService) Count(ctx context.Context, params *group.FindParams) (int64, error)`
  Count returns the total number of groups
  

- `func (GroupService) Create(ctx context.Context, g group.Group) (group.Group, error)`
  Create creates a new group
  

- `func (GroupService) Delete(ctx context.Context, id uuid.UUID) error`
  Delete removes a group by its ID
  

- `func (GroupService) GetAll(ctx context.Context) ([]group.Group, error)`
  GetAll returns all groups
  

- `func (GroupService) GetByID(ctx context.Context, id uuid.UUID) (group.Group, error)`
  GetByID returns a group by its ID
  

- `func (GroupService) GetPaginated(ctx context.Context, params *group.FindParams) ([]group.Group, error)`
  GetPaginated returns a paginated list of groups
  

- `func (GroupService) GetPaginatedWithTotal(ctx context.Context, params *group.FindParams) ([]group.Group, int64, error)`
  GetPaginatedWithTotal returns a paginated list of groups with total count
  

- `func (GroupService) RemoveRole(ctx context.Context, groupID uuid.UUID, roleToRemove role.Role) (group.Group, error)`
  RemoveRole removes a role from a group
  

- `func (GroupService) RemoveUser(ctx context.Context, groupID uuid.UUID, userToRemove user.User) (group.Group, error)`
  RemoveUser removes a user from a group
  

- `func (GroupService) Update(ctx context.Context, g group.Group) (group.Group, error)`
  Update updates an existing group
  

#### PermissionService

##### Methods

- `func (PermissionService) Count(ctx context.Context) (int64, error)`

- `func (PermissionService) Delete(ctx context.Context, id string) error`

- `func (PermissionService) GetAll(ctx context.Context) ([]*permission.Permission, error)`

- `func (PermissionService) GetByID(ctx context.Context, id string) (*permission.Permission, error)`

- `func (PermissionService) GetPaginated(ctx context.Context, params *permission.FindParams) ([]*permission.Permission, error)`

- `func (PermissionService) Save(ctx context.Context, data *permission.Permission) error`

#### ProjectService

##### Methods

- `func (ProjectService) Create(ctx context.Context, data *project.CreateDTO) error`

- `func (ProjectService) Delete(ctx context.Context, id uint) (*project.Project, error)`

- `func (ProjectService) GetAll(ctx context.Context) ([]*project.Project, error)`

- `func (ProjectService) GetByID(ctx context.Context, id uint) (*project.Project, error)`

- `func (ProjectService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*project.Project, error)`

- `func (ProjectService) Update(ctx context.Context, id uint, data *project.UpdateDTO) error`

#### RoleService

##### Methods

- `func (RoleService) Count(ctx context.Context, params *role.FindParams) (int64, error)`

- `func (RoleService) Create(ctx context.Context, data role.Role) error`

- `func (RoleService) Delete(ctx context.Context, id uint) error`

- `func (RoleService) GetAll(ctx context.Context) ([]role.Role, error)`

- `func (RoleService) GetByID(ctx context.Context, id uint) (role.Role, error)`

- `func (RoleService) GetPaginated(ctx context.Context, params *role.FindParams) ([]role.Role, error)`

- `func (RoleService) Update(ctx context.Context, data role.Role) error`

#### SessionService

##### Methods

- `func (SessionService) Create(ctx context.Context, data *session.CreateDTO) error`

- `func (SessionService) Delete(ctx context.Context, token string) error`

- `func (SessionService) DeleteByUserId(ctx context.Context, userId uint) ([]*session.Session, error)`

- `func (SessionService) GetAll(ctx context.Context) ([]*session.Session, error)`

- `func (SessionService) GetByToken(ctx context.Context, id string) (*session.Session, error)`

- `func (SessionService) GetCount(ctx context.Context) (int64, error)`

- `func (SessionService) GetPaginated(ctx context.Context, params *session.FindParams) ([]*session.Session, error)`

- `func (SessionService) Update(ctx context.Context, data *session.Session) error`

#### TabService

##### Methods

- `func (TabService) Create(ctx context.Context, data *tab.CreateDTO) (*tab.Tab, error)`

- `func (TabService) CreateManyUserTabs(ctx context.Context, userID uint, data []*tab.Tab) error`

- `func (TabService) Delete(ctx context.Context, id uint) error`

- `func (TabService) GetAll(ctx context.Context, params *tab.FindParams) ([]*tab.Tab, error)`

- `func (TabService) GetByID(ctx context.Context, id uint) (*tab.Tab, error)`

- `func (TabService) GetUserTabs(ctx context.Context, userID uint) ([]*tab.Tab, error)`

- `func (TabService) Update(ctx context.Context, id uint, data *tab.UpdateDTO) error`

#### TenantService

##### Methods

- `func (TenantService) Create(ctx context.Context, name, domain string) (*tenant.Tenant, error)`

- `func (TenantService) Delete(ctx context.Context, id uuid.UUID) error`

- `func (TenantService) GetByDomain(ctx context.Context, domain string) (*tenant.Tenant, error)`

- `func (TenantService) GetByID(ctx context.Context, id uuid.UUID) (*tenant.Tenant, error)`

- `func (TenantService) List(ctx context.Context) ([]*tenant.Tenant, error)`

- `func (TenantService) Update(ctx context.Context, t *tenant.Tenant) (*tenant.Tenant, error)`

#### UhrProps

```go
type UhrProps struct {
    Entities []costcomponent.BillableHourEntity
    ExpenseComponents []costcomponent.ExpenseComponent
}
```

#### UhrService

##### Methods

- `func (UhrService) Calculate(props *UhrProps) []costcomponent.UnifiedHourlyRateResult`

#### UploadService

##### Methods

- `func (UploadService) Create(ctx context.Context, data *upload.CreateDTO) (upload.Upload, error)`

- `func (UploadService) CreateMany(ctx context.Context, data []*upload.CreateDTO) ([]upload.Upload, error)`

- `func (UploadService) Delete(ctx context.Context, id uint) (upload.Upload, error)`

- `func (UploadService) Exists(ctx context.Context, id uint) (bool, error)`

- `func (UploadService) GetAll(ctx context.Context) ([]upload.Upload, error)`

- `func (UploadService) GetByHash(ctx context.Context, hash string) (upload.Upload, error)`

- `func (UploadService) GetByID(ctx context.Context, id uint) (upload.Upload, error)`

- `func (UploadService) GetPaginated(ctx context.Context, params *upload.FindParams) ([]upload.Upload, error)`

#### UserQueryService

##### Methods

- `func (UserQueryService) FindUserByID(ctx context.Context, userID int) (*viewmodels.User, error)`

- `func (UserQueryService) FindUsers(ctx context.Context, params *query.FindParams) ([]*viewmodels.User, int, error)`

- `func (UserQueryService) FindUsersWithRoles(ctx context.Context, params *query.FindParams) ([]*viewmodels.User, int, error)`

- `func (UserQueryService) SearchUsers(ctx context.Context, params *query.FindParams) ([]*viewmodels.User, int, error)`

#### UserService

##### Methods

- `func (UserService) Count(ctx context.Context, params *user.FindParams) (int64, error)`

- `func (UserService) Create(ctx context.Context, data user.User) (user.User, error)`

- `func (UserService) Delete(ctx context.Context, id uint) (user.User, error)`

- `func (UserService) GetAll(ctx context.Context) ([]user.User, error)`

- `func (UserService) GetByEmail(ctx context.Context, email string) (user.User, error)`

- `func (UserService) GetByID(ctx context.Context, id uint) (user.User, error)`

- `func (UserService) GetPaginated(ctx context.Context, params *user.FindParams) ([]user.User, error)`

- `func (UserService) GetPaginatedWithTotal(ctx context.Context, params *user.FindParams) ([]user.User, int64, error)`

- `func (UserService) Update(ctx context.Context, data user.User) (user.User, error)`

- `func (UserService) UpdateLastAction(ctx context.Context, id uint) error`

- `func (UserService) UpdateLastLogin(ctx context.Context, id uint) error`

### Functions

---

## Package `validators` (modules/core/validators)

### Types

#### UserValidator

##### Methods

- `func (UserValidator) ValidateCreate(ctx context.Context, u user.User) error`

- `func (UserValidator) ValidateUpdate(ctx context.Context, u user.User) error`

---

## Package `crm` (modules/crm)

### Types

#### ClientDataSource

##### Methods

- `func (ClientDataSource) Find(ctx context.Context, q string) []spotlight.Item`

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

- Var: `[CRMLink]`

- Var: `[ChatsLink]`

- Var: `[ClientsLink]`

- Var: `[LocaleFiles]`

- Var: `[MigrationFiles]`

- Var: `[NavItems]`

---

## Package `chat` (modules/crm/domain/aggregates/chat)

### Types

#### Chat

##### Interface Methods

- `ID() uint`
- `WithID(id uint) Chat`
- `ClientID() uint`
- `TenantID() uuid.UUID`
- `Messages() []Message`
- `AddMessage(msg Message) Chat`
- `UnreadMessages() int`
- `MarkAllAsRead()`
- `Members() []Member`
- `AddMember(member Member) Chat`
- `RemoveMember(memberID uuid.UUID) Chat`
- `LastMessage() (Message, error)`
- `LastMessageAt() *time.Time`
- `CreatedAt() time.Time`

#### ChatOption

#### ClientSender

##### Interface Methods

- `Sender`
- `ClientID() uint`
- `ContactID() uint`
- `FirstName() string`
- `LastName() string`

#### CreateDTO

```go
type CreateDTO struct {
    ClientID uint
}
```

##### Methods

- `func (CreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() (Chat, error)`

#### CreatedEvent

```go
type CreatedEvent struct {
    User user.User
    Result Chat
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    User user.User
    Result Chat
}
```

#### EmailSender

##### Interface Methods

- `Sender`
- `Email() internet.Email`

#### Field

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    Search string
    SortBy SortBy
}
```

#### InstagramSender

##### Interface Methods

- `Sender`
- `Username() string`

#### Member

##### Interface Methods

- `ID() uuid.UUID`
- `TenantID() uuid.UUID`
- `Transport() Transport`
- `Sender() Sender`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`

#### MemberOption

#### Message

##### Interface Methods

- `ID() uint`
- `ChatID() uint`
- `Sender() Member`
- `Message() string`
- `IsRead() bool`
- `MarkAsRead()`
- `ReadAt() *time.Time`
- `SentAt() *time.Time`
- `Attachments() []upload.Upload`
- `CreatedAt() time.Time`

#### MessageOption

#### MessagedAddedEvent

```go
type MessagedAddedEvent struct {
    User user.User
    Result Chat
}
```

#### OtherSender

##### Interface Methods

- `Sender`

#### PhoneSender

##### Interface Methods

- `Sender`
- `Phone() phone.Phone`

#### Provider

##### Interface Methods

- `Transport() Transport`
- `Send(ctx context.Context, msg Message) error`
- `OnReceived(callback func(msg Message) error)`

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Chat, error)`
- `GetByID(ctx context.Context, id uint) (Chat, error)`
- `GetByClientID(ctx context.Context, clientID uint) (Chat, error)`
- `GetMemberByContact(ctx context.Context, contactType string, contactValue string) (Member, error)`
- `Save(ctx context.Context, data Chat) (Chat, error)`
- `Delete(ctx context.Context, id uint) error`

#### SMSSender

##### Interface Methods

- `Sender`
- `Phone() phone.Phone`

#### Sender

##### Interface Methods

- `Type() SenderType`

#### SenderType

#### SortBy

#### SortByField

#### TelegramSender

##### Interface Methods

- `Sender`
- `ChatID() int64`
- `Username() string`
- `Phone() phone.Phone`

#### Transport

#### UserSender

##### Interface Methods

- `Sender`
- `UserID() uint`
- `FirstName() string`
- `LastName() string`

#### WebsiteSender

##### Interface Methods

- `Sender`
- `Phone() phone.Phone`
- `Email() internet.Email`

#### WhatsAppSender

##### Interface Methods

- `Sender`
- `Phone() phone.Phone`

### Variables and Constants

- Var: `[ErrEmptyMessage ErrNoMessages ErrSenderNotMember ErrMemberNotFound]`

---

## Package `client` (modules/crm/domain/aggregates/client)

### Types

#### Client

##### Interface Methods

- `ID() uint`
- `TenantID() uuid.UUID`
- `FirstName() string`
- `LastName() string`
- `MiddleName() string`
- `Phone() phone.Phone`
- `Address() string`
- `Email() internet.Email`
- `DateOfBirth() *time.Time`
- `Gender() general.Gender`
- `Passport() passport.Passport`
- `Pin() tax.Pin`
- `Comments() string`
- `Contacts() []Contact`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `SetContacts(contacts []Contact) Client`
- `AddContact(contact Contact) Client`
- `RemoveContact(contactID uint) Client`
- `SetPhone(number phone.Phone) Client`
- `SetName(firstName, lastName, middleName string) Client`
- `SetAddress(address string) Client`
- `SetEmail(email internet.Email) Client`
- `SetDateOfBirth(dob *time.Time) Client`
- `SetGender(gender general.Gender) Client`
- `SetPassport(p passport.Passport) Client`
- `SetPIN(pin tax.Pin) Client`
- `SetComments(comments string) Client`

#### Contact

##### Interface Methods

- `ID() uint`
- `Type() ContactType`
- `Value() string`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`

#### ContactOption

ContactOption is a function that configures a contact


#### ContactType

#### CreatedEvent

CreatedEvent represents the event of a client being created.


```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data Client
    Result Client
}
```

#### DeletedEvent

DeletedEvent represents the event of a client being deleted.


```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Data Client
    Result Client
}
```

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    Search string
    SortBy SortBy
    Filters []Filter
}
```

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Client, error)`
- `GetByID(ctx context.Context, id uint) (Client, error)`
- `GetByPhone(ctx context.Context, phoneNumber string) (Client, error)`
- `GetByContactValue(ctx context.Context, contactType ContactType, value string) (Client, error)`
- `Save(ctx context.Context, data Client) (Client, error)`
- `Delete(ctx context.Context, id uint) error`

#### SortBy

#### SortByField

#### UpdatedEvent

UpdatedEvent represents the event of a client being updated.


```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data Client
    Result Client
}
```

---

## Package `messagetemplate` (modules/crm/domain/entities/message-template)

### Types

#### CreateDTO

```go
type CreateDTO struct {
    Template string `validate:"required"`
}
```

##### Methods

- `func (CreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() MessageTemplate`

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
}
```

#### MessageTemplate

##### Interface Methods

- `ID() uint`
- `Template() string`
- `UpdateTemplate(template string) MessageTemplate`
- `CreatedAt() time.Time`

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]MessageTemplate, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]MessageTemplate, error)`
- `GetByID(ctx context.Context, id uint) (MessageTemplate, error)`
- `Create(ctx context.Context, data MessageTemplate) (MessageTemplate, error)`
- `Update(ctx context.Context, data MessageTemplate) (MessageTemplate, error)`
- `Delete(ctx context.Context, id uint) error`

#### UpdateDTO

```go
type UpdateDTO struct {
    Template string `validate:"required"`
}
```

##### Methods

- `func (UpdateDTO) Apply(entity MessageTemplate) MessageTemplate`

- `func (UpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

---

## Package `handlers` (modules/crm/handlers)

### Types

#### ClientHandler

##### Methods

#### NotificationHandler

##### Methods

#### SMSHandler

##### Methods

---

## Package `cpassproviders` (modules/crm/infrastructure/cpass-providers)

### Types

#### Config

Config holds the Twilio service configuration


```go
type Config struct {
    Params twilio.ClientParams
    From string
    WebhookURL string
}
```

#### DownloadMediaDTO

DownloadMediaDTO represents the data needed to download media


```go
type DownloadMediaDTO struct {
    URL string
    MimeType string
    Filename string
}
```

#### DownloadMediaResultDTO

DownloadMediaResultDTO represents the result of a media download


```go
type DownloadMediaResultDTO struct {
    Filename string
    MimeType string
    Path string
}
```

#### InboundTwilioMessageDTO

```go
type InboundTwilioMessageDTO struct {
    MessageSid string `json:"MessageSid"`
    SmsSid string `json:"SmsSid"`
    SmsMessageSid string `json:"SmsMessageSid"`
    AccountSid string `json:"AccountSid"`
    MessagingServiceSid string `json:"MessagingServiceSid"`
    From string `json:"From"`
    To string `json:"To"`
    Body string `json:"Body"`
    NumMedia int `json:"NumMedia,string"`
    NumSegments int `json:"NumSegments,string"`
    MediaContentTypes map[string]string `json:"MediaContentTypes"`
    MediaUrls map[string]string `json:"MediaUrls"`
    FromCity string `json:"FromCity"`
    FromState string `json:"FromState"`
    FromZip string `json:"FromZip"`
    FromCountry string `json:"FromCountry"`
    ToCity string `json:"ToCity"`
    ToState string `json:"ToState"`
    ToZip string `json:"ToZip"`
    ToCountry string `json:"ToCountry"`
}
```

#### Provider

##### Interface Methods

- `SendMessage(ctx context.Context, dto SendMessageDTO) error`
- `WebhookHandler(evb eventbus.EventBus) http.HandlerFunc`

#### ReceivedMessageEvent

```go
type ReceivedMessageEvent struct {
    From string `json:"From"`
    To string `json:"To"`
    Body string `json:"Body"`
}
```

#### SendMessageDTO

SendMessageDTO represents the data needed to send a message


```go
type SendMessageDTO struct {
    Message string
    To string
    From string
    MediaURL string
}
```

#### TwilioProvider

TwilioProvider handles Twilio-related operations


##### Methods

- `func (TwilioProvider) OnReceived(callback func(msg chat.Message) error)`

- `func (TwilioProvider) Send(ctx context.Context, msg chat.Message) error`
  SendMessage sends a message using Twilio
  

- `func (TwilioProvider) Transport() chat.Transport`

- `func (TwilioProvider) WebhookHandler(eventBus eventbus.EventBus) http.HandlerFunc`

#### UploadResult

UploadResult represents the result of a file upload


```go
type UploadResult struct {
    Path string
}
```

#### UploadsParams

UploadsParams represents parameters for uploading a file


```go
type UploadsParams struct {
    BucketName string
    File io.Reader
    ObjectName string
    MimeType string
}
```

### Variables and Constants

---

## Package `persistence` (modules/crm/infrastructure/persistence)

### Types

#### ChatRepository

##### Methods

- `func (ChatRepository) Count(ctx context.Context) (int64, error)`

- `func (ChatRepository) Delete(ctx context.Context, id uint) error`

- `func (ChatRepository) GetAll(ctx context.Context) ([]chat.Chat, error)`

- `func (ChatRepository) GetByClientID(ctx context.Context, clientID uint) (chat.Chat, error)`

- `func (ChatRepository) GetByID(ctx context.Context, id uint) (chat.Chat, error)`

- `func (ChatRepository) GetMemberByContact(ctx context.Context, contactType string, contactValue string) (chat.Member, error)`

- `func (ChatRepository) GetPaginated(ctx context.Context, params *chat.FindParams) ([]chat.Chat, error)`

- `func (ChatRepository) Save(ctx context.Context, data chat.Chat) (chat.Chat, error)`

#### ClientRepository

##### Methods

- `func (ClientRepository) Count(ctx context.Context, params *client.FindParams) (int64, error)`

- `func (ClientRepository) Delete(ctx context.Context, id uint) error`

- `func (ClientRepository) GetByContactValue(ctx context.Context, contactType client.ContactType, value string) (client.Client, error)`

- `func (ClientRepository) GetByID(ctx context.Context, id uint) (client.Client, error)`

- `func (ClientRepository) GetByPhone(ctx context.Context, phoneNumber string) (client.Client, error)`

- `func (ClientRepository) GetPaginated(ctx context.Context, params *client.FindParams) ([]client.Client, error)`

- `func (ClientRepository) Save(ctx context.Context, data client.Client) (client.Client, error)`

#### MessageTemplateRepository

##### Methods

- `func (MessageTemplateRepository) Count(ctx context.Context) (int64, error)`

- `func (MessageTemplateRepository) Create(ctx context.Context, data messagetemplate.MessageTemplate) (messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateRepository) Delete(ctx context.Context, id uint) error`

- `func (MessageTemplateRepository) GetAll(ctx context.Context) ([]messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateRepository) GetByID(ctx context.Context, id uint) (messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateRepository) GetPaginated(ctx context.Context, params *messagetemplate.FindParams) ([]messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateRepository) Update(ctx context.Context, data messagetemplate.MessageTemplate) (messagetemplate.MessageTemplate, error)`

### Functions

#### `func EmailMetaToSender(baseSender chat.Sender, meta *models.EmailMeta) (chat.Sender, error)`

#### `func InstagramMetaToSender(baseSender chat.Sender, meta *models.InstagramMeta) (chat.Sender, error)`

#### `func NewChatRepository() chat.Repository`

#### `func NewClientRepository(passportRepo passport.Repository) client.Repository`

#### `func NewMessageTemplateRepository() messagetemplate.Repository`

#### `func PhoneMetaToSender(baseSender chat.Sender, meta *models.PhoneMeta) (chat.Sender, error)`

#### `func SMSMetaToSender(baseSender chat.Sender, meta *models.SMSMeta) (chat.Sender, error)`

#### `func TelegramMetaToSender(baseSender chat.Sender, meta *models.TelegramMeta) (chat.Sender, error)`

#### `func ToDBChat(domainEntity chat.Chat) (*models.Chat, []*models.Message)`

#### `func ToDBChatMember(chatID uint, entity chat.Member) *models.ChatMember`

#### `func ToDBClient(domainEntity client.Client) *models.Client`

#### `func ToDBClientContact(clientID uint, domainContact client.Contact) *models.ClientContact`

ToDBClientContact converts a domain client contact entity to a database client contact model


#### `func ToDBMessage(entity chat.Message) *models.Message`

#### `func ToDBMessageTemplate(domainTemplate messagetemplate.MessageTemplate) *models.MessageTemplate`

#### `func ToDomainChat(dbRow *models.Chat, messages []chat.Message, members []chat.Member) (chat.Chat, error)`

#### `func ToDomainChatMember(dbMember *models.ChatMember) (chat.Member, error)`

#### `func ToDomainClient(dbRow *models.Client, passportData passport.Passport) (client.Client, error)`

#### `func ToDomainClientContact(dbContact *models.ClientContact) client.Contact`

ToDomainClientContact converts a database client contact model to a domain client contact entity


#### `func ToDomainMessage(dbRow *models.Message, sender chat.Member, dbUploads []*coremodels.Upload) (chat.Message, error)`

#### `func ToDomainMessageTemplate(dbTemplate *models.MessageTemplate) (messagetemplate.MessageTemplate, error)`

#### `func WebsiteMetaToSender(baseSender chat.Sender, meta *models.WebsiteMeta) (chat.Sender, error)`

#### `func WhatsAppMetaToSender(baseSender chat.Sender, meta *models.WhatsAppMeta) (chat.Sender, error)`

### Variables and Constants

- Var: `[ErrChatNotFound ErrMessageNotFound ErrMemberNotFound]`

- Var: `[ErrClientNotFound]`

- Var: `[ErrMessageTemplateNotFound]`

---

## Package `models` (modules/crm/infrastructure/persistence/models)

### Types

#### Chat

```go
type Chat struct {
    ID uint
    TenantID string
    ClientID uint
    LastMessageAt sql.NullTime
    CreatedAt time.Time
}
```

#### ChatMember

```go
type ChatMember struct {
    ID string
    TenantID string
    ChatID uint
    UserID sql.NullInt32
    ClientID sql.NullInt32
    ClientContactID sql.NullInt32
    Transport string
    TransportMeta *TransportMeta
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Client

```go
type Client struct {
    ID uint
    TenantID string
    FirstName string
    LastName sql.NullString
    MiddleName sql.NullString
    PhoneNumber sql.NullString
    Address sql.NullString
    Email sql.NullString
    DateOfBirth sql.NullTime
    Gender sql.NullString
    PassportID sql.NullString
    Pin sql.NullString
    Comments sql.NullString
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### ClientContact

```go
type ClientContact struct {
    ID uint
    ClientID uint
    ContactType string
    ContactValue string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### EmailMeta

```go
type EmailMeta struct {
    Email string `json:"email"`
}
```

#### InstagramMeta

```go
type InstagramMeta struct {
    Username string `json:"username"`
}
```

#### Message

```go
type Message struct {
    ID uint
    ChatID uint
    Message string
    ReadAt sql.NullTime
    SenderID string
    SentAt sql.NullTime
    CreatedAt time.Time
}
```

#### MessageTemplate

```go
type MessageTemplate struct {
    ID uint
    TenantID string
    Template string
    CreatedAt time.Time
}
```

#### PhoneMeta

```go
type PhoneMeta struct {
    Phone string `json:"phone"`
}
```

#### SMSMeta

```go
type SMSMeta struct {
    Phone string `json:"phone"`
}
```

#### TelegramMeta

```go
type TelegramMeta struct {
    ChatID int64 `json:"chat_id"`
    Username string `json:"username"`
    Phone string `json:"phone"`
}
```

#### TransportMeta

##### Methods

- `func (TransportMeta) Interface() any`

- `func (TransportMeta) Scan(value any) error`

- `func (TransportMeta) Value() (driver.Value, error)`

#### WebsiteMeta

TODO: store IP address & user agent


```go
type WebsiteMeta struct {
    Phone string `json:"phone"`
    Email string `json:"email"`
}
```

#### WhatsAppMeta

```go
type WhatsAppMeta struct {
    Phone string `json:"phone"`
}
```

### Variables and Constants

---

## Package `telegram` (modules/crm/infrastructure/telegram)

### Types

#### Bot

##### Methods

- `func (Bot) SendMessage(ctx context.Context, chatID int64, text string, options *SendMessageOpts) error`

#### SendMessageOpts

---

## Package `permissions` (modules/crm/permissions)

### Variables and Constants

- Var: `[ClientCreate ClientRead ClientUpdate ClientDelete]`

- Var: `[Permissions]`

- Const: `[ResourceClient]`

---

## Package `controllers` (modules/crm/presentation/controllers)

### Types

#### ChatController

##### Methods

- `func (ChatController) Create(w http.ResponseWriter, r *http.Request)`

- `func (ChatController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (ChatController) Key() string`

- `func (ChatController) List(w http.ResponseWriter, r *http.Request)`

- `func (ChatController) Register(r *mux.Router)`

- `func (ChatController) Search(w http.ResponseWriter, r *http.Request)`

- `func (ChatController) SendMessage(w http.ResponseWriter, r *http.Request)`

#### ClientController

##### Methods

- `func (ClientController) Create(r *http.Request, w http.ResponseWriter, user userdomain.User, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) Delete(r *http.Request, w http.ResponseWriter, user userdomain.User, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) GetNotesEdit(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) GetPassportEdit(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) GetPersonalEdit(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) GetTaxEdit(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) Key() string`

- `func (ClientController) List(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, user userdomain.User, clientService *services.ClientService)`

- `func (ClientController) Register(r *mux.Router)`

- `func (ClientController) RegisterTab(tab TabDefinition)`

- `func (ClientController) UpdateNotes(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) UpdatePassport(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) UpdatePersonal(r *http.Request, w http.ResponseWriter, user userdomain.User, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) UpdateTax(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, clientService *services.ClientService)`

- `func (ClientController) View(r *http.Request, w http.ResponseWriter, user userdomain.User, logger *logrus.Entry, clientService *services.ClientService, chatService *services.ChatService)`

#### ClientControllerConfig

```go
type ClientControllerConfig struct {
    BasePath string
    Middleware []mux.MiddlewareFunc
    Tabs []TabDefinition
    RealtimeBus bool
}
```

#### ClientRealtimeUpdates

##### Methods

- `func (ClientRealtimeUpdates) Register()`

#### ClientsPaginatedResponse

```go
type ClientsPaginatedResponse struct {
    Clients []*viewmodels.Client
    Page int
    PerPage int
    HasMore bool
}
```

#### CreateChatDTO

```go
type CreateChatDTO struct {
    Phone string
}
```

#### MessageTemplateController

##### Methods

- `func (MessageTemplateController) Create(w http.ResponseWriter, r *http.Request)`

- `func (MessageTemplateController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (MessageTemplateController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (MessageTemplateController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (MessageTemplateController) Key() string`

- `func (MessageTemplateController) List(w http.ResponseWriter, r *http.Request)`

- `func (MessageTemplateController) Register(r *mux.Router)`

- `func (MessageTemplateController) Update(w http.ResponseWriter, r *http.Request)`

#### SendMessageDTO

```go
type SendMessageDTO struct {
    Message string
}
```

#### TabDefinition

```go
type TabDefinition struct {
    ID string
    NameKey string
    Component func(r *http.Request, clientID uint) (templ.Component, error)
    SortOrder int
    Permissions []*permission.Permission
}
```

#### TwillioController

##### Methods

- `func (TwillioController) Key() string`

- `func (TwillioController) Register(r *mux.Router)`

### Functions

#### `func NewChatController(app application.Application, basePath string) application.Controller`

#### `func NewClientController(app application.Application, config ...ClientControllerConfig) application.Controller`

#### `func NewMessageTemplateController(app application.Application, basePath string) application.Controller`

### Variables and Constants

- Var: `[ProfileTab ChatTab ActionsTab]`
  Default tab definitions - exported for configuration
  

---

## Package `dtos` (modules/crm/presentation/controllers/dtos)

### Types

#### CreateClientDTO

```go
type CreateClientDTO struct {
    FirstName string `validate:"required"`
    LastName string `validate:"required"`
    MiddleName string `validate:"omitempty"`
    Phone string `validate:"required"`
    Email string `validate:"omitempty,email"`
    Address string `validate:"omitempty"`
    PassportSeries string `validate:"omitempty"`
    PassportNumber string `validate:"omitempty"`
    Pin string `validate:"omitempty"`
    CountryCode string `validate:"omitempty"`
}
```

##### Methods

- `func (CreateClientDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CreateClientDTO) ToEntity(tenantID uuid.UUID) (client.Client, error)`

#### UpdateClientNotesDTO

```go
type UpdateClientNotesDTO struct {
    Comments string `validate:"omitempty"`
}
```

##### Methods

- `func (UpdateClientNotesDTO) Apply(entity client.Client) (client.Client, error)`

- `func (UpdateClientNotesDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### UpdateClientPassportDTO

```go
type UpdateClientPassportDTO struct {
    PassportSeries string `validate:"omitempty"`
    PassportNumber string `validate:"omitempty"`
}
```

##### Methods

- `func (UpdateClientPassportDTO) Apply(entity client.Client) (client.Client, error)`

- `func (UpdateClientPassportDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### UpdateClientPersonalDTO

```go
type UpdateClientPersonalDTO struct {
    FirstName string `validate:"required"`
    LastName string `validate:"required"`
    MiddleName string `validate:"omitempty"`
    Phone string `validate:"omitempty"`
    Email string `validate:"omitempty,email"`
    Address string `validate:"omitempty"`
}
```

##### Methods

- `func (UpdateClientPersonalDTO) Apply(entity client.Client) (client.Client, error)`

- `func (UpdateClientPersonalDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### UpdateClientTaxDTO

```go
type UpdateClientTaxDTO struct {
    Pin string `validate:"omitempty"`
    CountryCode string `validate:"omitempty"`
}
```

##### Methods

- `func (UpdateClientTaxDTO) Apply(entity client.Client) (client.Client, error)`

- `func (UpdateClientTaxDTO) Ok(ctx context.Context) (map[string]string, bool)`

---

## Package `mappers` (modules/crm/presentation/mappers)

### Functions

#### `func ChatToViewModel(entity chat.Chat, clientEntity client.Client) *viewmodels.Chat`

#### `func ClientToViewModel(entity client.Client) *viewmodels.Client`

#### `func MemberToViewModel(entity chat.Member) *viewmodels.Member`

#### `func MessageTemplateToViewModel(entity messagetemplate.MessageTemplate) *viewmodels.MessageTemplate`

#### `func MessageToViewModel(entity chat.Message) *viewmodels.Message`

#### `func PassportToViewModel(p passport.Passport) viewmodels.Passport`

#### `func SenderToViewModel(entity chat.Sender) viewmodels.MessageSender`

---

## Package `chatsui` (modules/crm/presentation/templates/pages/chats)

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### ChatInputProps

```go
type ChatInputProps struct {
    SendURL string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    SearchURL string
    NewChatURL string
    Chats []*viewmodels.Chat
    Page int
    PerPage int
    HasMore bool
}
```

#### InstantMessagesDialogProps

```go
type InstantMessagesDialogProps struct {
    OnClick string
    Templates []*viewmodels.MessageTemplate
}
```

#### NewChatProps

```go
type NewChatProps struct {
    BaseURL string
    CreateChatURL string
    Phone string
    Errors map[string]string
}
```

#### SelectedChatProps

```go
type SelectedChatProps struct {
    BaseURL string
    ClientsURL string
    Chat *viewmodels.Chat
    Templates []*viewmodels.MessageTemplate
}
```

### Functions

#### `func ChatCard(chat *viewmodels.Chat) templ.Component`

#### `func ChatInput(props ChatInputProps) templ.Component`

#### `func ChatItems(props *IndexPageProps) templ.Component`

#### `func ChatLayout(props *IndexPageProps) templ.Component`

#### `func ChatList(props *IndexPageProps) templ.Component`

#### `func ChatMessages(chat *viewmodels.Chat) templ.Component`

#### `func ChatNotFound() templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func InstantMessagesDialog(props InstantMessagesDialogProps) templ.Component`

#### `func Message(msg *viewmodels.Message) templ.Component`

#### `func NewChat(props NewChatProps) templ.Component`

#### `func NewChatForm(props NewChatProps) templ.Component`

#### `func NoSelectedChat() templ.Component`

#### `func SelectedChat(props SelectedChatProps) templ.Component`

### Variables and Constants

---

## Package `clients` (modules/crm/presentation/templates/pages/clients)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CardHeaderProps

```go
type CardHeaderProps struct {
    Title string
    EditURL string
    Target string
    FormID string
}
```

#### ClientTab

```go
type ClientTab struct {
    Name string
}
```

#### CreatePageProps

```go
type CreatePageProps struct {
    Client *viewmodels.Client
    Errors map[string]string
    SaveURL string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Clients []*viewmodels.Client
    Page int
    PerPage int
    HasMore bool
    NewURL string
}
```

#### NotesInfoEditProps

```go
type NotesInfoEditProps struct {
    Client *viewmodels.Client
    Errors map[string]string
    Form string
}
```

#### PassportInfoEditProps

PassportInfoEditForm is a dedicated form for editing passport information


```go
type PassportInfoEditProps struct {
    Client *viewmodels.Client
    Errors map[string]string
    Form string
}
```

#### PersonalInfoEditProps

PersonalInfoEditForm is a dedicated form for editing personal information


```go
type PersonalInfoEditProps struct {
    Client *viewmodels.Client
    Errors map[string]string
    Form string
}
```

#### ProfileProps

```go
type ProfileProps struct {
    ClientURL string
    EditURL string
    IsEditing bool
    Client *viewmodels.Client
}
```

#### TaxInfoEditProps

TaxInfoEditForm is a dedicated form for editing tax information


```go
type TaxInfoEditProps struct {
    Client *viewmodels.Client
    Errors map[string]string
    Form string
}
```

#### ViewDrawerProps

```go
type ViewDrawerProps struct {
    SelectedTab string
    CallbackURL string
    Tabs []ClientTab
}
```

### Functions

#### `func ActionsTab(clientID string) templ.Component`

#### `func CardHeader(props CardHeaderProps) templ.Component`

#### `func Chats(props chatsui.SelectedChatProps) templ.Component`

---- Chats -----


#### `func ClientCreatedEvent(client *viewmodels.Client, rowProps *base.TableRowProps) templ.Component`

#### `func ClientRow(client *viewmodels.Client, rowProps *base.TableRowProps) templ.Component`

#### `func ClientRows(props *IndexPageProps) templ.Component`

#### `func ClientsContent(props *IndexPageProps) templ.Component`

#### `func ClientsTable(props *IndexPageProps) templ.Component`

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func NewClientDrawer() templ.Component`

#### `func NotFound() templ.Component`

---- Not Found ----


#### `func NotesInfoCard(client *viewmodels.Client) templ.Component`

#### `func NotesInfoEditForm(props *NotesInfoEditProps) templ.Component`

#### `func PassportInfoCard(client *viewmodels.Client) templ.Component`

PassportInfoCard shows passport information for the client


#### `func PassportInfoEditForm(props *PassportInfoEditProps) templ.Component`

#### `func PersonalInfoCard(client *viewmodels.Client) templ.Component`

PersonalInfoCardProps contains data needed for the personal info card


#### `func PersonalInfoEditForm(props *PersonalInfoEditProps) templ.Component`

#### `func Profile(props ProfileProps) templ.Component`

#### `func TaxInfoCard(client *viewmodels.Client) templ.Component`

TaxInfoCard shows tax information for the client


#### `func TaxInfoEditForm(props *TaxInfoEditProps) templ.Component`

#### `func ViewDrawer(props ViewDrawerProps) templ.Component`

### Variables and Constants

---

## Package `messagetemplatesui` (modules/crm/presentation/templates/pages/message-templates)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Template *viewmodels.MessageTemplate
    Errors map[string]string
    SaveURL string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Template *viewmodels.MessageTemplate
    Errors map[string]string
    SaveURL string
    DeleteURL string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    NewURL string
    BaseURL string
    Templates []*viewmodels.MessageTemplate
}
```

### Functions

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func TemplatesContent(props *IndexPageProps) templ.Component`

#### `func TemplatesTable(props *IndexPageProps) templ.Component`

### Variables and Constants

---

## Package `viewmodels` (modules/crm/presentation/viewmodels)

### Types

#### Chat

```go
type Chat struct {
    ID string
    Client *Client
    CreatedAt string
    Messages []*Message
    UnreadMessages int
}
```

##### Methods

- `func (Chat) HasUnreadMessages() bool`

- `func (Chat) LastMessage() *Message`

- `func (Chat) ReversedMessages() []*Message`

- `func (Chat) UnreadMessagesFormatted() string`

#### Client

```go
type Client struct {
    ID string
    FirstName string
    LastName string
    MiddleName string
    Phone string
    Email string
    Address string
    Passport Passport
    Pin string
    CountryCode string
    DateOfBirth string
    Gender string
    Comments string
    CreatedAt string
    UpdatedAt string
}
```

##### Methods

- `func (Client) FullName() string`

- `func (Client) Initials() string`

#### Member

```go
type Member struct {
    ID string
    Transport string
    Sender MessageSender
    CreatedAt string
}
```

#### Message

```go
type Message struct {
    ID string
    Message string
    Sender MessageSender
    CreatedAt time.Time
}
```

##### Methods

- `func (Message) Date() string`

- `func (Message) Time() string`

#### MessageSender

##### Interface Methods

- `ID() string`
- `IsUser() bool`
- `IsClient() bool`
- `Initials() string`

#### MessageTemplate

```go
type MessageTemplate struct {
    ID string
    Template string
    CreatedAt string
}
```

#### Passport

```go
type Passport struct {
    ID string
    Series string
    Number string
    FirstName string
    LastName string
    MiddleName string
    Gender string
    BirthDate string
    BirthPlace string
    Nationality string
    PassportType string
    IssuedAt string
    IssuedBy string
    IssuingCountry string
    ExpiresAt string
}
```

---

## Package `services` (modules/crm/services)

### Types

#### ChatService

##### Methods

- `func (ChatService) AddMessageToChat(ctx context.Context, chatID uint, msg chat.Message) (chat.Chat, error)`
  AddMessageToChat adds a message to a chat and handles the transaction
  

- `func (ChatService) Count(ctx context.Context) (int64, error)`

- `func (ChatService) CreateOrGetClientByPhone(ctx context.Context, phoneNumber string) (client.Client, error)`
  CreateOrGetClientByPhone creates a new client or gets an existing one by phone number
  

- `func (ChatService) Delete(ctx context.Context, id uint) (chat.Chat, error)`

- `func (ChatService) GetByClientID(ctx context.Context, clientID uint) (chat.Chat, error)`

- `func (ChatService) GetByClientIDOrCreate(ctx context.Context, clientID uint) (chat.Chat, error)`

- `func (ChatService) GetByID(ctx context.Context, id uint) (chat.Chat, error)`

- `func (ChatService) GetMemberByContact(ctx context.Context, contactType client.ContactType, value string) (chat.Member, error)`

- `func (ChatService) GetOrCreateChatByPhone(ctx context.Context, phoneNumber string) (chat.Chat, client.Client, error)`
  GetOrCreateChatByPhone creates a chat for a client based on phone number
  

- `func (ChatService) GetPaginated(ctx context.Context, params *chat.FindParams) ([]chat.Chat, error)`

- `func (ChatService) Save(ctx context.Context, entity chat.Chat) (chat.Chat, error)`
  Create creates a new chat
  

- `func (ChatService) SendMessage(ctx context.Context, cmd SendMessageCommand) (chat.Chat, error)`

#### ClientService

##### Methods

- `func (ClientService) Count(ctx context.Context, params *client.FindParams) (int64, error)`

- `func (ClientService) Create(ctx context.Context, data client.Client) error`

- `func (ClientService) Delete(ctx context.Context, id uint) (client.Client, error)`

- `func (ClientService) GetByID(ctx context.Context, id uint) (client.Client, error)`

- `func (ClientService) GetByPhone(ctx context.Context, phoneNumber string) (client.Client, error)`

- `func (ClientService) GetPaginated(ctx context.Context, params *client.FindParams) ([]client.Client, error)`

- `func (ClientService) Update(ctx context.Context, data client.Client) error`

#### MessageMedia

MessageMedia represents media attached to a message


```go
type MessageMedia struct {
    MinioTempPath string
    Filename string
    MimeType string
}
```

#### MessageTemplateService

##### Methods

- `func (MessageTemplateService) Count(ctx context.Context) (int64, error)`

- `func (MessageTemplateService) Create(ctx context.Context, data *messagetemplate.CreateDTO) (messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateService) Delete(ctx context.Context, id uint) (messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateService) GetAll(ctx context.Context) ([]messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateService) GetByID(ctx context.Context, id uint) (messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateService) GetPaginated(ctx context.Context, params *messagetemplate.FindParams) ([]messagetemplate.MessageTemplate, error)`

- `func (MessageTemplateService) Update(ctx context.Context, id uint, data *messagetemplate.UpdateDTO) (messagetemplate.MessageTemplate, error)`

#### SendMessageCommand

SendMessageCommand represents the data needed to send a message


```go
type SendMessageCommand struct {
    ChatID uint
    Transport chat.Transport
    Message string
    Attachments []*upload.Upload
}
```

---

## Package `finance` (modules/finance)

### Types

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

- Var: `[ExpenseCategoriesItem PaymentCategoriesItem PaymentsItem ExpensesItem AccountsItem CounterpartiesItem InventoryItem]`

- Var: `[FinanceItem]`

- Var: `[NavItems]`

---

## Package `expense` (modules/finance/domain/aggregates/expense)

### Types

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Result Expense
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Expense
}
```

#### Expense

Interface


##### Interface Methods

- `ID() uuid.UUID`
- `Amount() *money.Money`
- `Account() moneyaccount.Account`
- `Category() category.ExpenseCategory`
- `Comment() string`
- `TransactionID() uuid.UUID`
- `AccountingPeriod() time.Time`
- `Date() time.Time`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `TenantID() uuid.UUID`
- `SetAccount(account moneyaccount.Account) Expense`
- `SetCategory(category category.ExpenseCategory) Expense`
- `SetComment(comment string) Expense`
- `SetAmount(amount *money.Money) Expense`
- `SetDate(date time.Time) Expense`
- `SetAccountingPeriod(period time.Time) Expense`

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    ID uuid.UUID
    Limit int
    Offset int
    SortBy SortBy
    Filters []Filter
    Search string
}
```

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (Expense, error)`
- `GetAll(ctx context.Context) ([]Expense, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Expense, error)`
- `Create(ctx context.Context, data Expense) (Expense, error)`
- `Update(ctx context.Context, data Expense) (Expense, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### SortBy

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Result Expense
}
```

---

## Package `category` (modules/finance/domain/aggregates/expense_category)

### Types

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data ExpenseCategory
    Result ExpenseCategory
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result ExpenseCategory
}
```

#### ExpenseCategory

Interface


##### Interface Methods

- `ID() uuid.UUID`
- `TenantID() uuid.UUID`
- `Name() string`
- `Description() string`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `UpdateName(name string) ExpenseCategory`
- `UpdateDescription(description string) ExpenseCategory`

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    ID uuid.UUID
    Limit int
    Offset int
    SortBy SortBy
    Filters []Filter
    Search string
}
```

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetAll(ctx context.Context) ([]ExpenseCategory, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]ExpenseCategory, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (ExpenseCategory, error)`
- `Create(ctx context.Context, category ExpenseCategory) (ExpenseCategory, error)`
- `Update(ctx context.Context, category ExpenseCategory) (ExpenseCategory, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### SortBy

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data ExpenseCategory
    Result ExpenseCategory
}
```

---

## Package `moneyaccount` (modules/finance/domain/aggregates/money_account)

### Types

#### Account

##### Interface Methods

- `ID() uuid.UUID`
- `SetID(id uuid.UUID)`
- `TenantID() uuid.UUID`
- `UpdateTenantID(id uuid.UUID) Account`
- `Name() string`
- `UpdateName(name string) Account`
- `AccountNumber() string`
- `UpdateAccountNumber(accountNumber string) Account`
- `Description() string`
- `UpdateDescription(description string) Account`
- `Balance() *money.Money`
- `UpdateBalance(balance *money.Money) Account`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `InitialTransaction() transaction.Transaction`

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data Account
    Result Account
}
```

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Account
}
```

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    Query string
    Field string
    CreatedAt DateRange
}
```

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]Account, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Account, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (Account, error)`
- `RecalculateBalance(ctx context.Context, id uuid.UUID) error`
- `Create(ctx context.Context, data Account) (Account, error)`
- `Update(ctx context.Context, data Account) (Account, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data Account
    Result Account
}
```

---

## Package `payment` (modules/finance/domain/aggregates/payment)

### Types

#### Created

```go
type Created struct {
    Sender user.User
    Session session.Session
    Data Payment
    Result Payment
}
```

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### Deleted

```go
type Deleted struct {
    Sender user.User
    Session session.Session
    Result Payment
}
```

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    Query string
    Field string
    CreatedAt DateRange
}
```

#### Option

#### Payment

##### Interface Methods

- `ID() uuid.UUID`
- `SetID(id uuid.UUID)`
- `TenantID() uuid.UUID`
- `UpdateTenantID(id uuid.UUID) Payment`
- `Amount() *money.Money`
- `UpdateAmount(amount *money.Money) Payment`
- `TransactionID() uuid.UUID`
- `CounterpartyID() uuid.UUID`
- `UpdateCounterpartyID(partyID uuid.UUID) Payment`
- `Category() paymentcategory.PaymentCategory`
- `UpdateCategory(category paymentcategory.PaymentCategory) Payment`
- `TransactionDate() time.Time`
- `UpdateTransactionDate(t time.Time) Payment`
- `AccountingPeriod() time.Time`
- `UpdateAccountingPeriod(t time.Time) Payment`
- `Comment() string`
- `UpdateComment(comment string) Payment`
- `Account() moneyaccount.Account`
- `User() user.User`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]Payment, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Payment, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (Payment, error)`
- `Create(ctx context.Context, payment Payment) (Payment, error)`
- `Update(ctx context.Context, payment Payment) (Payment, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### Updated

```go
type Updated struct {
    Sender user.User
    Session session.Session
    Data Payment
    Result Payment
}
```

---

## Package `payment_category` (modules/finance/domain/aggregates/payment_category)

### Types

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data PaymentCategory
    Result PaymentCategory
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result PaymentCategory
}
```

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    ID uuid.UUID
    Limit int
    Offset int
    SortBy SortBy
    Filters []Filter
    Search string
}
```

#### Option

#### PaymentCategory

Interface


##### Interface Methods

- `ID() uuid.UUID`
- `TenantID() uuid.UUID`
- `Name() string`
- `Description() string`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `UpdateName(name string) PaymentCategory`
- `UpdateDescription(description string) PaymentCategory`

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetAll(ctx context.Context) ([]PaymentCategory, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]PaymentCategory, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (PaymentCategory, error)`
- `Create(ctx context.Context, category PaymentCategory) (PaymentCategory, error)`
- `Update(ctx context.Context, category PaymentCategory) (PaymentCategory, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### SortBy

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data PaymentCategory
    Result PaymentCategory
}
```

---

## Package `counterparty` (modules/finance/domain/entities/counterparty)

### Types

#### Counterparty

##### Interface Methods

- `ID() uuid.UUID`
- `SetID(uuid.UUID)`
- `TenantID() uuid.UUID`
- `Tin() tax.Tin`
- `SetTin(t tax.Tin)`
- `Name() string`
- `SetName(string)`
- `Type() Type`
- `SetType(Type)`
- `LegalType() LegalType`
- `SetLegalType(LegalType)`
- `LegalAddress() string`
- `SetLegalAddress(string)`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy SortBy
    Search string
    Filters []Filter
}
```

#### LegalType

##### Methods

- `func (LegalType) IsValid() bool`

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetAll(context.Context) ([]Counterparty, error)`
- `GetPaginated(context.Context, *FindParams) ([]Counterparty, error)`
- `GetByID(context.Context, uuid.UUID) (Counterparty, error)`
- `Create(context.Context, Counterparty) (Counterparty, error)`
- `Update(context.Context, Counterparty) (Counterparty, error)`
- `Delete(context.Context, uuid.UUID) error`

#### SortBy

#### Type

##### Methods

- `func (Type) IsValid() bool`

---

## Package `inventory` (modules/finance/domain/entities/inventory)

### Types

#### Field

#### Filter

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy SortBy
    Search string
    Filters []Filter
}
```

##### Methods

- `func (FindParams) FilterBy(field Field, filter repo.Filter) *FindParams`

#### Inventory

##### Interface Methods

- `ID() uuid.UUID`
- `SetID(id uuid.UUID)`
- `TenantID() uuid.UUID`
- `UpdateTenantID(id uuid.UUID) Inventory`
- `Name() string`
- `UpdateName(name string) Inventory`
- `Description() string`
- `UpdateDescription(description string) Inventory`
- `Price() *money.Money`
- `UpdatePrice(price *money.Money) Inventory`
- `Quantity() int`
- `UpdateQuantity(quantity int) Inventory`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `Ok(l ut.Translator) (map[string]string, bool)`

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `GetAll(ctx context.Context) ([]Inventory, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Inventory, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (Inventory, error)`
- `Create(ctx context.Context, data Inventory) (Inventory, error)`
- `Update(ctx context.Context, data Inventory) (Inventory, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### SortBy

---

## Package `transaction` (modules/finance/domain/entities/transaction)

### Types

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    CreatedAt DateRange
}
```

#### Option

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]Transaction, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Transaction, error)`
- `GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)`
- `Create(ctx context.Context, upload Transaction) (Transaction, error)`
- `Update(ctx context.Context, upload Transaction) (Transaction, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`

#### Transaction

##### Interface Methods

- `ID() uuid.UUID`
- `TenantID() uuid.UUID`
- `UpdateTenantID(id uuid.UUID) Transaction`
- `Amount() *money.Money`
- `UpdateAmount(amount *money.Money) Transaction`
- `OriginAccountID() uuid.UUID`
- `UpdateOriginAccountID(accountID uuid.UUID) Transaction`
- `DestinationAccountID() uuid.UUID`
- `UpdateDestinationAccountID(accountID uuid.UUID) Transaction`
- `TransactionDate() time.Time`
- `UpdateTransactionDate(date time.Time) Transaction`
- `AccountingPeriod() time.Time`
- `UpdateAccountingPeriod(period time.Time) Transaction`
- `TransactionType() Type`
- `UpdateTransactionType(transactionType Type) Transaction`
- `Comment() string`
- `UpdateComment(comment string) Transaction`
- `CreatedAt() time.Time`
- `ExchangeRate() *float64`
- `UpdateExchangeRate(rate *float64) Transaction`
- `DestinationAmount() *money.Money`
- `UpdateDestinationAmount(amount *money.Money) Transaction`

#### Type

##### Methods

- `func (Type) IsValid() bool`

---

## Package `persistence` (modules/finance/infrastructure/persistence)

### Types

#### GormCounterpartyRepository

##### Methods

- `func (GormCounterpartyRepository) Count(ctx context.Context, params *counterparty.FindParams) (int64, error)`

- `func (GormCounterpartyRepository) Create(ctx context.Context, data counterparty.Counterparty) (counterparty.Counterparty, error)`

- `func (GormCounterpartyRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (GormCounterpartyRepository) GetAll(ctx context.Context) ([]counterparty.Counterparty, error)`

- `func (GormCounterpartyRepository) GetByID(ctx context.Context, id uuid.UUID) (counterparty.Counterparty, error)`

- `func (GormCounterpartyRepository) GetPaginated(ctx context.Context, params *counterparty.FindParams) ([]counterparty.Counterparty, error)`

- `func (GormCounterpartyRepository) Update(ctx context.Context, data counterparty.Counterparty) (counterparty.Counterparty, error)`

#### GormExpenseRepository

##### Methods

- `func (GormExpenseRepository) Count(ctx context.Context, params *expense.FindParams) (int64, error)`

- `func (GormExpenseRepository) Create(ctx context.Context, data expense.Expense) (expense.Expense, error)`

- `func (GormExpenseRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (GormExpenseRepository) GetAll(ctx context.Context) ([]expense.Expense, error)`

- `func (GormExpenseRepository) GetByID(ctx context.Context, id uuid.UUID) (expense.Expense, error)`

- `func (GormExpenseRepository) GetPaginated(ctx context.Context, params *expense.FindParams) ([]expense.Expense, error)`

- `func (GormExpenseRepository) Update(ctx context.Context, data expense.Expense) (expense.Expense, error)`

#### GormMoneyAccountRepository

##### Methods

- `func (GormMoneyAccountRepository) Count(ctx context.Context) (int64, error)`

- `func (GormMoneyAccountRepository) Create(ctx context.Context, data moneyaccount.Account) (moneyaccount.Account, error)`

- `func (GormMoneyAccountRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (GormMoneyAccountRepository) GetAll(ctx context.Context) ([]moneyaccount.Account, error)`

- `func (GormMoneyAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (moneyaccount.Account, error)`

- `func (GormMoneyAccountRepository) GetPaginated(ctx context.Context, params *moneyaccount.FindParams) ([]moneyaccount.Account, error)`

- `func (GormMoneyAccountRepository) RecalculateBalance(ctx context.Context, id uuid.UUID) error`

- `func (GormMoneyAccountRepository) Update(ctx context.Context, data moneyaccount.Account) (moneyaccount.Account, error)`

#### GormPaymentCategoryRepository

##### Methods

- `func (GormPaymentCategoryRepository) Count(ctx context.Context, params *paymentcategory.FindParams) (int64, error)`

- `func (GormPaymentCategoryRepository) Create(ctx context.Context, data paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error)`

- `func (GormPaymentCategoryRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (GormPaymentCategoryRepository) GetAll(ctx context.Context) ([]paymentcategory.PaymentCategory, error)`

- `func (GormPaymentCategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (paymentcategory.PaymentCategory, error)`

- `func (GormPaymentCategoryRepository) GetPaginated(ctx context.Context, params *paymentcategory.FindParams) ([]paymentcategory.PaymentCategory, error)`

- `func (GormPaymentCategoryRepository) Update(ctx context.Context, data paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error)`

#### GormPaymentRepository

##### Methods

- `func (GormPaymentRepository) Count(ctx context.Context) (int64, error)`

- `func (GormPaymentRepository) Create(ctx context.Context, data payment.Payment) (payment.Payment, error)`

- `func (GormPaymentRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (GormPaymentRepository) GetAll(ctx context.Context) ([]payment.Payment, error)`

- `func (GormPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (payment.Payment, error)`

- `func (GormPaymentRepository) GetPaginated(ctx context.Context, params *payment.FindParams) ([]payment.Payment, error)`

- `func (GormPaymentRepository) Update(ctx context.Context, data payment.Payment) (payment.Payment, error)`

#### InventoryRepository

##### Methods

- `func (InventoryRepository) Count(ctx context.Context, params *inventory.FindParams) (int64, error)`

- `func (InventoryRepository) Create(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error)`

- `func (InventoryRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (InventoryRepository) GetAll(ctx context.Context) ([]inventory.Inventory, error)`

- `func (InventoryRepository) GetByID(ctx context.Context, id uuid.UUID) (inventory.Inventory, error)`

- `func (InventoryRepository) GetPaginated(ctx context.Context, params *inventory.FindParams) ([]inventory.Inventory, error)`

- `func (InventoryRepository) Update(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error)`

#### PgExpenseCategoryRepository

##### Methods

- `func (PgExpenseCategoryRepository) Count(ctx context.Context, params *category.FindParams) (int64, error)`

- `func (PgExpenseCategoryRepository) Create(ctx context.Context, data category.ExpenseCategory) (category.ExpenseCategory, error)`

- `func (PgExpenseCategoryRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (PgExpenseCategoryRepository) GetAll(ctx context.Context) ([]category.ExpenseCategory, error)`

- `func (PgExpenseCategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (category.ExpenseCategory, error)`

- `func (PgExpenseCategoryRepository) GetPaginated(ctx context.Context, params *category.FindParams) ([]category.ExpenseCategory, error)`

- `func (PgExpenseCategoryRepository) Update(ctx context.Context, data category.ExpenseCategory) (category.ExpenseCategory, error)`

#### PgTransactionRepository

##### Methods

- `func (PgTransactionRepository) Count(ctx context.Context) (int64, error)`

- `func (PgTransactionRepository) Create(ctx context.Context, data transaction.Transaction) (transaction.Transaction, error)`

- `func (PgTransactionRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (PgTransactionRepository) GetAll(ctx context.Context) ([]transaction.Transaction, error)`

- `func (PgTransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (transaction.Transaction, error)`

- `func (PgTransactionRepository) GetPaginated(ctx context.Context, params *transaction.FindParams) ([]transaction.Transaction, error)`

- `func (PgTransactionRepository) Update(ctx context.Context, data transaction.Transaction) (transaction.Transaction, error)`

### Functions

#### `func NewCounterpartyRepository() counterparty.Repository`

#### `func NewExpenseCategoryRepository() category.Repository`

#### `func NewExpenseRepository(categoryRepo category.Repository, transactionRepo transaction.Repository) expense.Repository`

#### `func NewInventoryRepository() inventory.Repository`

#### `func NewMoneyAccountRepository() moneyaccount.Repository`

#### `func NewPaymentCategoryRepository() paymentcategory.Repository`

#### `func NewPaymentRepository() payment.Repository`

#### `func NewTransactionRepository() transaction.Repository`

#### `func ToDBCounterparty(entity counterparty.Counterparty) (*models.Counterparty, error)`

#### `func ToDBExpense(entity expense.Expense) (*models.Expense, transaction.Transaction)`

#### `func ToDBExpenseCategory(entity category.ExpenseCategory) *models.ExpenseCategory`

#### `func ToDBInventory(entity inventory.Inventory) *models.Inventory`

#### `func ToDBMoneyAccount(entity moneyaccount.Account) *models.MoneyAccount`

#### `func ToDBPayment(entity payment.Payment) (*models.Payment, *models.Transaction)`

#### `func ToDBPaymentCategory(entity paymentcategory.PaymentCategory) *models.PaymentCategory`

#### `func ToDBTransaction(entity transaction.Transaction) *models.Transaction`

#### `func ToDomainCounterparty(dbRow *models.Counterparty) (counterparty.Counterparty, error)`

#### `func ToDomainExpense(dbExpense *models.Expense, dbTransaction *models.Transaction) (expense.Expense, error)`

#### `func ToDomainExpenseCategory(dbCategory *models.ExpenseCategory) (category.ExpenseCategory, error)`

#### `func ToDomainInventory(dbInventory *models.Inventory) (inventory.Inventory, error)`

#### `func ToDomainMoneyAccount(dbAccount *models.MoneyAccount) (moneyaccount.Account, error)`

#### `func ToDomainPayment(dbPayment *models.Payment, dbTransaction *models.Transaction) (payment.Payment, error)`

TODO: populate user && account


#### `func ToDomainPaymentCategory(dbCategory *models.PaymentCategory) (paymentcategory.PaymentCategory, error)`

#### `func ToDomainTransaction(dbTransaction *models.Transaction) (transaction.Transaction, error)`

### Variables and Constants

- Var: `[ErrAccountNotFound]`

- Var: `[ErrCounterpartyNotFound]`

- Var: `[ErrExpenseCategoryNotFound]`

- Var: `[ErrExpenseNotFound]`

- Var: `[ErrInventoryNotFound]`

- Var: `[ErrPaymentCategoryNotFound]`

- Var: `[ErrPaymentNotFound]`

- Var: `[ErrTransactionNotFound]`

---

## Package `models` (modules/finance/infrastructure/persistence/models)

### Types

#### Counterparty

```go
type Counterparty struct {
    ID string
    TenantID string
    Tin sql.NullString
    Name string
    Type string
    LegalType string
    LegalAddress string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Expense

```go
type Expense struct {
    ID string
    TransactionID string
    CategoryID string
    TenantID string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### ExpenseCategory

```go
type ExpenseCategory struct {
    ID string
    TenantID string
    Name string
    Description sql.NullString
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Inventory

```go
type Inventory struct {
    ID string
    TenantID string
    Name string
    Description sql.NullString
    CurrencyID sql.NullString
    Price int64
    Quantity int
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### MoneyAccount

```go
type MoneyAccount struct {
    ID string
    TenantID string
    Name string
    AccountNumber string
    Description sql.NullString
    Balance int64
    BalanceCurrencyID string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Payment

```go
type Payment struct {
    ID string
    TransactionID string
    CounterpartyID string
    PaymentCategoryID sql.NullString
    TenantID string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### PaymentCategory

```go
type PaymentCategory struct {
    ID string
    TenantID string
    Name string
    Description sql.NullString
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Transaction

```go
type Transaction struct {
    ID string
    TenantID string
    Amount int64
    OriginAccountID sql.NullString
    DestinationAccountID sql.NullString
    TransactionDate time.Time
    AccountingPeriod time.Time
    TransactionType string
    Comment string
    ExchangeRate sql.NullFloat64
    DestinationAmount sql.NullInt64
    CreatedAt time.Time
}
```

---

## Package `permissions` (modules/finance/permissions)

### Variables and Constants

- Var: `[PaymentCreate PaymentRead PaymentUpdate PaymentDelete ExpenseCreate ExpenseRead ExpenseUpdate ExpenseDelete ExpenseCategoryCreate ExpenseCategoryRead ExpenseCategoryUpdate ExpenseCategoryDelete]`

- Var: `[Permissions]`

- Const: `[ResourceExpense ResourcePayment ResourceExpenseCategory]`

---

## Package `controllers` (modules/finance/presentation/controllers)

### Types

#### AccountPaginatedResponse

```go
type AccountPaginatedResponse struct {
    Accounts []*viewmodels.MoneyAccount
    PaginationState *pagination.State
}
```

#### CounterpartiesController

##### Methods

- `func (CounterpartiesController) Create(w http.ResponseWriter, r *http.Request)`

- `func (CounterpartiesController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (CounterpartiesController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (CounterpartiesController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (CounterpartiesController) Key() string`

- `func (CounterpartiesController) List(w http.ResponseWriter, r *http.Request)`

- `func (CounterpartiesController) Register(r *mux.Router)`

- `func (CounterpartiesController) Search(w http.ResponseWriter, r *http.Request)`

- `func (CounterpartiesController) Update(w http.ResponseWriter, r *http.Request)`

#### ExpenseCategoriesController

##### Methods

- `func (ExpenseCategoriesController) Create(w http.ResponseWriter, r *http.Request)`

- `func (ExpenseCategoriesController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (ExpenseCategoriesController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (ExpenseCategoriesController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (ExpenseCategoriesController) Key() string`

- `func (ExpenseCategoriesController) List(w http.ResponseWriter, r *http.Request)`

- `func (ExpenseCategoriesController) Register(r *mux.Router)`

- `func (ExpenseCategoriesController) Update(w http.ResponseWriter, r *http.Request)`

#### ExpenseCategoryPaginatedResponse

```go
type ExpenseCategoryPaginatedResponse struct {
    Categories []*viewmodels2.ExpenseCategory
    PaginationState *pagination.State
}
```

#### ExpenseController

##### Methods

- `func (ExpenseController) Create(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, expenseService *services.ExpenseService, moneyAccountService *services.MoneyAccountService, expenseCategoryService *services.ExpenseCategoryService)`

- `func (ExpenseController) Delete(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, expenseService *services.ExpenseService)`

- `func (ExpenseController) Export(r *http.Request, w http.ResponseWriter, excelService *coreservices.ExcelExportService)`

- `func (ExpenseController) GetEdit(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, expenseService *services.ExpenseService, moneyAccountService *services.MoneyAccountService, expenseCategoryService *services.ExpenseCategoryService)`

- `func (ExpenseController) GetNew(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, moneyAccountService *services.MoneyAccountService, expenseCategoryService *services.ExpenseCategoryService)`

- `func (ExpenseController) Key() string`

- `func (ExpenseController) List(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, expenseService *services.ExpenseService)`

- `func (ExpenseController) Register(r *mux.Router)`

- `func (ExpenseController) Update(r *http.Request, w http.ResponseWriter, logger *logrus.Entry, expenseService *services.ExpenseService, moneyAccountService *services.MoneyAccountService, expenseCategoryService *services.ExpenseCategoryService)`

#### ExpensePaginationResponse

```go
type ExpensePaginationResponse struct {
    Expenses []*viewmodels.Expense
    PaginationState *pagination.State
}
```

#### InventoryController

##### Methods

- `func (InventoryController) Create(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) Key() string`

- `func (InventoryController) List(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) Register(r *mux.Router)`

- `func (InventoryController) Update(w http.ResponseWriter, r *http.Request)`

#### InventoryPaginatedResponse

```go
type InventoryPaginatedResponse struct {
    Items []*viewmodels.Inventory
    PaginationState *pagination.State
}
```

#### MoneyAccountController

##### Methods

- `func (MoneyAccountController) Create(w http.ResponseWriter, r *http.Request)`

- `func (MoneyAccountController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (MoneyAccountController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (MoneyAccountController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (MoneyAccountController) Key() string`

- `func (MoneyAccountController) List(w http.ResponseWriter, r *http.Request)`

- `func (MoneyAccountController) Register(r *mux.Router)`

- `func (MoneyAccountController) Update(w http.ResponseWriter, r *http.Request)`

#### PaymentCategoriesController

##### Methods

- `func (PaymentCategoriesController) Create(w http.ResponseWriter, r *http.Request)`

- `func (PaymentCategoriesController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (PaymentCategoriesController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (PaymentCategoriesController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (PaymentCategoriesController) Key() string`

- `func (PaymentCategoriesController) List(w http.ResponseWriter, r *http.Request)`

- `func (PaymentCategoriesController) Register(r *mux.Router)`

- `func (PaymentCategoriesController) Update(w http.ResponseWriter, r *http.Request)`

#### PaymentCategoryPaginatedResponse

```go
type PaymentCategoryPaginatedResponse struct {
    Categories []*viewmodels2.PaymentCategory
    PaginationState *pagination.State
}
```

#### PaymentPaginatedResponse

```go
type PaymentPaginatedResponse struct {
    Payments []*viewmodels.Payment
    PaginationState *pagination.State
}
```

#### PaymentsController

##### Methods

- `func (PaymentsController) Create(w http.ResponseWriter, r *http.Request)`

- `func (PaymentsController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (PaymentsController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (PaymentsController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (PaymentsController) Key() string`

- `func (PaymentsController) Payments(w http.ResponseWriter, r *http.Request)`

- `func (PaymentsController) Register(r *mux.Router)`

- `func (PaymentsController) Update(w http.ResponseWriter, r *http.Request)`

### Functions

#### `func NewCounterpartiesController(app application.Application) application.Controller`

#### `func NewExpenseCategoriesController(app application.Application) application.Controller`

#### `func NewExpensesController(app application.Application) application.Controller`

#### `func NewInventoryController(app application.Application) application.Controller`

#### `func NewMoneyAccountController(app application.Application) application.Controller`

#### `func NewPaymentCategoriesController(app application.Application) application.Controller`

#### `func NewPaymentsController(app application.Application) application.Controller`

---

## Package `dtos` (modules/finance/presentation/controllers/dtos)

### Types

#### CounterpartyCreateDTO

```go
type CounterpartyCreateDTO struct {
    TIN string
    Name string `validate:"required,min=2,max=255"`
    Type string `validate:"required"`
    LegalType string `validate:"required"`
    LegalAddress string `validate:"max=500"`
}
```

##### Methods

- `func (CounterpartyCreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CounterpartyCreateDTO) ToEntity(tenantID uuid.UUID) (counterparty.Counterparty, error)`

#### CounterpartyUpdateDTO

```go
type CounterpartyUpdateDTO struct {
    TIN string
    Name string `validate:"required,min=2,max=255"`
    Type string `validate:"required"`
    LegalType string `validate:"required"`
    LegalAddress string `validate:"max=500"`
}
```

##### Methods

- `func (CounterpartyUpdateDTO) Apply(existing counterparty.Counterparty) (counterparty.Counterparty, error)`

- `func (CounterpartyUpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### ExpenseCategoryCreateDTO

```go
type ExpenseCategoryCreateDTO struct {
    Name string `validate:"required"`
    Description string
}
```

##### Methods

- `func (ExpenseCategoryCreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (ExpenseCategoryCreateDTO) ToEntity(tenantID uuid.UUID) (category.ExpenseCategory, error)`

#### ExpenseCategoryUpdateDTO

```go
type ExpenseCategoryUpdateDTO struct {
    Name string `validate:"required"`
    Description string
}
```

##### Methods

- `func (ExpenseCategoryUpdateDTO) Apply(existing category.ExpenseCategory) (category.ExpenseCategory, error)`

- `func (ExpenseCategoryUpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (ExpenseCategoryUpdateDTO) ToEntity(id uuid.UUID, tenantID uuid.UUID) (category.ExpenseCategory, error)`

#### ExpenseCreateDTO

```go
type ExpenseCreateDTO struct {
    Amount float64 `validate:"required,gt=0"`
    AccountID string `validate:"required,uuid"`
    CategoryID string `validate:"required,uuid"`
    Comment string
    AccountingPeriod shared.DateOnly `validate:"required"`
    Date shared.DateOnly `validate:"required"`
}
```

##### Methods

- `func (ExpenseCreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (ExpenseCreateDTO) ToEntity(tenantID uuid.UUID) (expense.Expense, error)`

- `func (ExpenseCreateDTO) ToEntityWithReferences(tenantID uuid.UUID, account moneyAccount.Account, cat category.ExpenseCategory) (expense.Expense, error)`

#### ExpenseUpdateDTO

```go
type ExpenseUpdateDTO struct {
    Amount float64 `validate:"gt=0"`
    AccountID string `validate:"omitempty,uuid"`
    CategoryID string `validate:"omitempty,uuid"`
    Comment string
    AccountingPeriod shared.DateOnly
    Date shared.DateOnly
}
```

##### Methods

- `func (ExpenseUpdateDTO) Apply(entity expense.Expense, cat category.ExpenseCategory) (expense.Expense, error)`

- `func (ExpenseUpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### InventoryCreateDTO

```go
type InventoryCreateDTO struct {
    Name string `validate:"required"`
    Description string
    CurrencyCode string
    Price float64 `validate:"gte=0"`
    Quantity int `validate:"gte=0"`
}
```

##### Methods

- `func (InventoryCreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (InventoryCreateDTO) ToEntity() (inventory.Inventory, error)`

#### InventoryUpdateDTO

```go
type InventoryUpdateDTO struct {
    Name string `validate:"required"`
    Description string
    CurrencyCode string
    Price float64 `validate:"gte=0"`
    Quantity int `validate:"gte=0"`
}
```

##### Methods

- `func (InventoryUpdateDTO) Apply(existing inventory.Inventory) (inventory.Inventory, error)`

- `func (InventoryUpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (InventoryUpdateDTO) ToEntity(id uuid.UUID) (inventory.Inventory, error)`

#### MoneyAccountCreateDTO

```go
type MoneyAccountCreateDTO struct {
    Name string `validate:"required"`
    Balance float64 `validate:"gte=0"`
    AccountNumber string
    CurrencyCode string `validate:"required,len=3"`
    Description string
}
```

##### Methods

- `func (MoneyAccountCreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (MoneyAccountCreateDTO) ToEntity(tenantID uuid.UUID) (moneyaccount.Account, error)`

#### MoneyAccountUpdateDTO

```go
type MoneyAccountUpdateDTO struct {
    Name string `validate:"lte=255"`
    Balance float64 `validate:"gte=0"`
    AccountNumber string
    CurrencyCode string `validate:"len=3"`
    Description string
}
```

##### Methods

- `func (MoneyAccountUpdateDTO) Apply(existing moneyaccount.Account) (moneyaccount.Account, error)`

- `func (MoneyAccountUpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (MoneyAccountUpdateDTO) ToEntity(id uuid.UUID, tenantID uuid.UUID) (moneyaccount.Account, error)`

#### PaymentCategoryCreateDTO

```go
type PaymentCategoryCreateDTO struct {
    Name string `validate:"required"`
    Description string
}
```

##### Methods

- `func (PaymentCategoryCreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (PaymentCategoryCreateDTO) ToEntity() (paymentcategory.PaymentCategory, error)`

#### PaymentCategoryUpdateDTO

```go
type PaymentCategoryUpdateDTO struct {
    Name string `validate:"required"`
    Description string
}
```

##### Methods

- `func (PaymentCategoryUpdateDTO) Apply(existing paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error)`

- `func (PaymentCategoryUpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (PaymentCategoryUpdateDTO) ToEntity(id uuid.UUID) (paymentcategory.PaymentCategory, error)`

#### PaymentCreateDTO

```go
type PaymentCreateDTO struct {
    Amount float64 `validate:"required,gt=0"`
    AccountID string `validate:"required,uuid"`
    TransactionDate shared.DateOnly `validate:"required"`
    AccountingPeriod shared.DateOnly `validate:"required"`
    CounterpartyID string `validate:"required,uuid"`
    PaymentCategoryID string `validate:"required,uuid"`
    UserID uint `validate:"required"`
    Comment string
}
```

##### Methods

- `func (PaymentCreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (PaymentCreateDTO) ToEntity(tenantID uuid.UUID, category paymentcategory.PaymentCategory) payment.Payment`

#### PaymentUpdateDTO

```go
type PaymentUpdateDTO struct {
    Amount float64 `validate:"gt=0"`
    AccountID string `validate:"omitempty,uuid"`
    CounterpartyID string `validate:"omitempty,uuid"`
    PaymentCategoryID string `validate:"omitempty,uuid"`
    TransactionDate shared.DateOnly
    AccountingPeriod shared.DateOnly
    Comment string
    UserID uint
}
```

##### Methods

- `func (PaymentUpdateDTO) Apply(existing payment.Payment, category paymentcategory.PaymentCategory, u user.User) (payment.Payment, error)`

- `func (PaymentUpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (PaymentUpdateDTO) ToEntity(id uuid.UUID, tenantID uuid.UUID, category paymentcategory.PaymentCategory) payment.Payment`

### Functions

### Variables and Constants

---

## Package `mappers` (modules/finance/presentation/mappers)

### Functions

#### `func CounterpartyToViewModel(entity counterparty.Counterparty) *viewmodels.Counterparty`

#### `func ExpenseCategoryToViewModel(entity category.ExpenseCategory) *viewmodels.ExpenseCategory`

#### `func ExpenseToViewModel(entity expense.Expense) *viewmodels.Expense`

#### `func InventoryToViewModel(entity inventory.Inventory) *viewmodels.Inventory`

#### `func MoneyAccountToViewModel(entity moneyaccount.Account) *viewmodels.MoneyAccount`

#### `func MoneyAccountToViewUpdateModel(entity moneyaccount.Account) *viewmodels.MoneyAccountUpdateDTO`

#### `func PaymentCategoryToViewModel(entity paymentcategory.PaymentCategory) *viewmodels.PaymentCategory`

#### `func PaymentToViewModel(entity payment.Payment) *viewmodels.Payment`

---

## Package `templates` (modules/finance/presentation/templates)

### Variables and Constants

- Var: `[FS]`

---

## Package `components` (modules/finance/presentation/templates/components)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### AccountSelectProps

```go
type AccountSelectProps struct {
    Label string
    Placeholder string
    Value string
    Accounts []*viewmodels.MoneyAccount
    Error string
    Attrs templ.Attributes
}
```

#### CounterpartyLegalTypeSelectProps

```go
type CounterpartyLegalTypeSelectProps struct {
    Value string
    Attrs templ.Attributes
    Error string
}
```

#### CounterpartySelectProps

```go
type CounterpartySelectProps struct {
    Label string
    Placeholder string
    Value string
    Name string
    NotFoundText string
    Form string
    Counterparties []*viewmodels.Counterparty
}
```

#### CounterpartyTypeSelectProps

```go
type CounterpartyTypeSelectProps struct {
    Value string
    Attrs templ.Attributes
    Error string
}
```

#### PaymentCategorySelectProps

```go
type PaymentCategorySelectProps struct {
    Label string
    Placeholder string
    Value string
    Categories []*viewmodels.PaymentCategory
    Error string
    Attrs templ.Attributes
}
```

### Functions

#### `func AccountSelect(props *AccountSelectProps) templ.Component`

#### `func CounterpartyLegalTypeSelect(props *CounterpartyLegalTypeSelectProps) templ.Component`

#### `func CounterpartySelect(props *CounterpartySelectProps) templ.Component`

#### `func CounterpartyTypeSelect(props *CounterpartyTypeSelectProps) templ.Component`

#### `func PaymentCategorySelect(props *PaymentCategorySelectProps) templ.Component`

### Variables and Constants

---

## Package `counterparties` (modules/finance/presentation/templates/pages/counterparties)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CounterpartyTypeOption

```go
type CounterpartyTypeOption struct {
    Value string
    Label string
}
```

#### CreatePageProps

```go
type CreatePageProps struct {
    Counterparty *viewmodels.Counterparty
    Errors map[string]string
    PostPath string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Counterparty *viewmodels.Counterparty
    Errors map[string]string
    PostPath string
    DeletePath string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Counterparties []*viewmodels.Counterparty
    PaginationState *pagination.State
}
```

### Functions

#### `func CounterpartiesContent(props *IndexPageProps) templ.Component`

#### `func CounterpartiesTable(props *IndexPageProps) templ.Component`

#### `func CounterpartyRows(props *IndexPageProps) templ.Component`

#### `func CounterpartyTableRow(counterparty *viewmodels.Counterparty, rowProps *base.TableRowProps) templ.Component`

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditContent(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func NewContent(props *CreatePageProps) templ.Component`

### Variables and Constants

---

## Package `expense_categories` (modules/finance/presentation/templates/pages/expense_categories)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Category dtos.ExpenseCategoryCreateDTO
    Errors map[string]string
    PostPath string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Category *viewmodels.ExpenseCategory
    Errors map[string]string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Categories []*viewmodels.ExpenseCategory
    PaginationState *pagination.State
}
```

### Functions

#### `func CategoriesContent(props *IndexPageProps) templ.Component`

#### `func CategoriesTable(props *IndexPageProps) templ.Component`

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func SearchFields(props *IndexPageProps) templ.Component`

#### `func SearchFieldsTrigger(trigger *base.TriggerProps) templ.Component`

### Variables and Constants

---

## Package `expenses` (modules/finance/presentation/templates/pages/expenses)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### AccountSelectProps

```go
type AccountSelectProps struct {
    Value string
    Accounts []*viewmodels.MoneyAccount
    Attrs templ.Attributes
}
```

#### CategorySelectProps

```go
type CategorySelectProps struct {
    Value string
    Categories []*viewmodels.ExpenseCategory
    Attrs templ.Attributes
}
```

#### CreatePageProps

```go
type CreatePageProps struct {
    Accounts []*viewmodels.MoneyAccount
    Categories []*viewmodels.ExpenseCategory
    Expense *viewmodels.Expense
    Errors map[string]string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Expense *viewmodels.Expense
    Accounts []*viewmodels.MoneyAccount
    Categories []*viewmodels.ExpenseCategory
    Errors map[string]string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Expenses []*viewmodels.Expense
    PaginationState *pagination.State
}
```

### Functions

#### `func AccountSelect(props *AccountSelectProps) templ.Component`

#### `func CategorySelect(props *CategorySelectProps) templ.Component`

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func ExpenseRow(expense *viewmodels.Expense, attrs *templ.Attributes) templ.Component`

ExpenseRow renders a single expense row


#### `func ExpenseRows(props *IndexPageProps) templ.Component`

#### `func ExpenseTableRow(expense *viewmodels.Expense, rowProps *base.TableRowProps) templ.Component`

#### `func ExpensesContent(props *IndexPageProps) templ.Component`

#### `func ExpensesTable(props *IndexPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

### Variables and Constants

---

## Package `inventory` (modules/finance/presentation/templates/pages/inventory)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Currencies []*coreviewmodels.Currency
    Inventory *viewmodels.Inventory
    Errors map[string]string
    PostPath string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Inventory *viewmodels.Inventory
    Currencies []*coreviewmodels.Currency
    Errors map[string]string
    PostPath string
    DeletePath string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Inventory []*viewmodels.Inventory
    PaginationState *pagination.State
}
```

### Functions

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func InventoryContent(props *IndexPageProps) templ.Component`

#### `func InventoryTable(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

### Variables and Constants

---

## Package `moneyaccounts` (modules/finance/presentation/templates/pages/moneyaccounts)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Currencies []*coreviewmodels.Currency
    Account *viewmodels.MoneyAccount
    Errors map[string]string
    PostPath string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Account *viewmodels.MoneyAccountUpdateDTO
    Currencies []*coreviewmodels.Currency
    Errors map[string]string
    PostPath string
    DeletePath string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Accounts []*viewmodels.MoneyAccount
    PaginationState *pagination.State
}
```

### Functions

#### `func AccountsContent(props *IndexPageProps) templ.Component`

#### `func AccountsTable(props *IndexPageProps) templ.Component`

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

### Variables and Constants

---

## Package `payment_categories` (modules/finance/presentation/templates/pages/payment_categories)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Category dtos.PaymentCategoryCreateDTO
    Errors map[string]string
    PostPath string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Category *viewmodels.PaymentCategory
    Errors map[string]string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Categories []*viewmodels.PaymentCategory
    PaginationState *pagination.State
}
```

### Functions

#### `func CategoriesContent(props *IndexPageProps) templ.Component`

#### `func CategoriesTable(props *IndexPageProps) templ.Component`

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func SearchFields(props *IndexPageProps) templ.Component`

#### `func SearchFieldsTrigger(trigger *base.TriggerProps) templ.Component`

### Variables and Constants

---

## Package `payments` (modules/finance/presentation/templates/pages/payments)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Payment *viewmodels.Payment
    Accounts []*viewmodels.MoneyAccount
    Categories []*viewmodels.PaymentCategory
    Errors map[string]string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Payment *viewmodels.Payment
    Accounts []*viewmodels.MoneyAccount
    Categories []*viewmodels.PaymentCategory
    Errors map[string]string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Payments []*viewmodels.Payment
    PaginationState *pagination.State
}
```

### Functions

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func PaymentsContent(props *IndexPageProps) templ.Component`

#### `func PaymentsTable(props *IndexPageProps) templ.Component`

#### `func SearchFields(props *IndexPageProps) templ.Component`

#### `func SearchFieldsTrigger(trigger *base.TriggerProps) templ.Component`

### Variables and Constants

---

## Package `viewmodels` (modules/finance/presentation/viewmodels)

### Types

#### Counterparty

```go
type Counterparty struct {
    ID string
    TIN string
    Name string
    Type CounterpartyType
    LegalType CounterpartyLegalType
    LegalAddress string
    CreatedAt string
    UpdatedAt string
}
```

#### CounterpartyLegalType

##### Methods

- `func (CounterpartyLegalType) LocalizedString(pageCtx *types.PageContext) string`

- `func (CounterpartyLegalType) String() string`

- `func (CounterpartyLegalType) ToDomain() counterparty.LegalType`

#### CounterpartyType

##### Methods

- `func (CounterpartyType) LocalizedString(pageCtx *types.PageContext) string`

- `func (CounterpartyType) String() string`

- `func (CounterpartyType) ToDomain() counterparty.Type`

#### Expense

```go
type Expense struct {
    ID string
    Amount string
    AccountID string
    AmountWithCurrency string
    CategoryID string
    Category *ExpenseCategory
    Comment string
    TransactionID string
    AccountingPeriod string
    Date string
    CreatedAt string
    UpdatedAt string
}
```

#### ExpenseCategory

```go
type ExpenseCategory struct {
    ID string
    Name string
    Description string
    CreatedAt string
    UpdatedAt string
}
```

#### Inventory

```go
type Inventory struct {
    ID string
    Name string
    Description string
    CurrencyCode string
    Price string
    Quantity string
    TotalValue string
    CreatedAt string
    UpdatedAt string
}
```

#### MoneyAccount

```go
type MoneyAccount struct {
    ID string
    Name string
    AccountNumber string
    Description string
    Balance string
    BalanceWithCurrency string
    CurrencyCode string
    CurrencySymbol string
    CreatedAt string
    UpdatedAt string
}
```

#### MoneyAccountCreateDTO

```go
type MoneyAccountCreateDTO struct {
    Name string
    Description string
    AccountNumber string
    Balance string
    CurrencyCode string
}
```

#### MoneyAccountUpdateDTO

```go
type MoneyAccountUpdateDTO struct {
    Name string
    Description string
    AccountNumber string
    Balance string
    CurrencyCode string
}
```

#### Payment

```go
type Payment struct {
    ID string
    Amount string
    AmountWithCurrency string
    AccountID string
    CounterpartyID string
    CategoryID string
    Category *PaymentCategory
    TransactionID string
    TransactionDate string
    AccountingPeriod string
    Comment string
    CreatedAt string
    UpdatedAt string
}
```

#### PaymentCategory

```go
type PaymentCategory struct {
    ID string
    Name string
    Description string
    CreatedAt string
    UpdatedAt string
}
```

---

## Package `services` (modules/finance/services)

### Types

#### CounterpartyService

##### Methods

- `func (CounterpartyService) Count(ctx context.Context, params *counterparty.FindParams) (int64, error)`

- `func (CounterpartyService) Create(ctx context.Context, entity counterparty.Counterparty) (counterparty.Counterparty, error)`

- `func (CounterpartyService) Delete(ctx context.Context, id uuid.UUID) (counterparty.Counterparty, error)`

- `func (CounterpartyService) GetAll(ctx context.Context) ([]counterparty.Counterparty, error)`

- `func (CounterpartyService) GetByID(ctx context.Context, id uuid.UUID) (counterparty.Counterparty, error)`

- `func (CounterpartyService) GetPaginated(ctx context.Context, params *counterparty.FindParams) ([]counterparty.Counterparty, error)`

- `func (CounterpartyService) Update(ctx context.Context, entity counterparty.Counterparty) (counterparty.Counterparty, error)`

#### ExpenseCategoryService

##### Methods

- `func (ExpenseCategoryService) Count(ctx context.Context, params *category.FindParams) (uint, error)`

- `func (ExpenseCategoryService) Create(ctx context.Context, entity category.ExpenseCategory) (category.ExpenseCategory, error)`

- `func (ExpenseCategoryService) Delete(ctx context.Context, id uuid.UUID) (category.ExpenseCategory, error)`

- `func (ExpenseCategoryService) GetAll(ctx context.Context) ([]category.ExpenseCategory, error)`

- `func (ExpenseCategoryService) GetByID(ctx context.Context, id uuid.UUID) (category.ExpenseCategory, error)`

- `func (ExpenseCategoryService) GetPaginated(ctx context.Context, params *category.FindParams) ([]category.ExpenseCategory, error)`

- `func (ExpenseCategoryService) Update(ctx context.Context, entity category.ExpenseCategory) (category.ExpenseCategory, error)`

#### ExpenseService

##### Methods

- `func (ExpenseService) Count(ctx context.Context, params *expense.FindParams) (uint, error)`

- `func (ExpenseService) Create(ctx context.Context, entity expense.Expense) (expense.Expense, error)`

- `func (ExpenseService) Delete(ctx context.Context, id uuid.UUID) (expense.Expense, error)`

- `func (ExpenseService) GetAll(ctx context.Context) ([]expense.Expense, error)`

- `func (ExpenseService) GetByID(ctx context.Context, id uuid.UUID) (expense.Expense, error)`

- `func (ExpenseService) GetPaginated(ctx context.Context, params *expense.FindParams) ([]expense.Expense, error)`

- `func (ExpenseService) Update(ctx context.Context, entity expense.Expense) (expense.Expense, error)`

#### InventoryService

##### Methods

- `func (InventoryService) Count(ctx context.Context, params *inventory.FindParams) (int64, error)`

- `func (InventoryService) Create(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error)`

- `func (InventoryService) Delete(ctx context.Context, id uuid.UUID) error`

- `func (InventoryService) GetAll(ctx context.Context) ([]inventory.Inventory, error)`

- `func (InventoryService) GetByID(ctx context.Context, id uuid.UUID) (inventory.Inventory, error)`

- `func (InventoryService) GetPaginated(ctx context.Context, params *inventory.FindParams) ([]inventory.Inventory, error)`

- `func (InventoryService) Update(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error)`

#### MoneyAccountService

##### Methods

- `func (MoneyAccountService) Count(ctx context.Context) (int64, error)`

- `func (MoneyAccountService) Create(ctx context.Context, entity moneyaccount.Account) (moneyaccount.Account, error)`

- `func (MoneyAccountService) Delete(ctx context.Context, id uuid.UUID) (moneyaccount.Account, error)`

- `func (MoneyAccountService) GetAll(ctx context.Context) ([]moneyaccount.Account, error)`

- `func (MoneyAccountService) GetByID(ctx context.Context, id uuid.UUID) (moneyaccount.Account, error)`

- `func (MoneyAccountService) GetPaginated(ctx context.Context, params *moneyaccount.FindParams) ([]moneyaccount.Account, error)`

- `func (MoneyAccountService) RecalculateBalance(ctx context.Context, id uuid.UUID) error`

- `func (MoneyAccountService) Update(ctx context.Context, entity moneyaccount.Account) (moneyaccount.Account, error)`

#### PaymentCategoryService

##### Methods

- `func (PaymentCategoryService) Count(ctx context.Context, params *paymentcategory.FindParams) (uint, error)`

- `func (PaymentCategoryService) Create(ctx context.Context, entity paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error)`

- `func (PaymentCategoryService) Delete(ctx context.Context, id uuid.UUID) (paymentcategory.PaymentCategory, error)`

- `func (PaymentCategoryService) GetAll(ctx context.Context) ([]paymentcategory.PaymentCategory, error)`

- `func (PaymentCategoryService) GetByID(ctx context.Context, id uuid.UUID) (paymentcategory.PaymentCategory, error)`

- `func (PaymentCategoryService) GetPaginated(ctx context.Context, params *paymentcategory.FindParams) ([]paymentcategory.PaymentCategory, error)`

- `func (PaymentCategoryService) Update(ctx context.Context, entity paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error)`

#### PaymentService

##### Methods

- `func (PaymentService) Count(ctx context.Context) (int64, error)`

- `func (PaymentService) Create(ctx context.Context, entity payment.Payment) (payment.Payment, error)`

- `func (PaymentService) Delete(ctx context.Context, id uuid.UUID) (payment.Payment, error)`

- `func (PaymentService) GetAll(ctx context.Context) ([]payment.Payment, error)`

- `func (PaymentService) GetByID(ctx context.Context, id uuid.UUID) (payment.Payment, error)`

- `func (PaymentService) GetPaginated(ctx context.Context, params *payment.FindParams) ([]payment.Payment, error)`

- `func (PaymentService) Update(ctx context.Context, entity payment.Payment) (payment.Payment, error)`

#### TransactionService

##### Methods

- `func (TransactionService) Count(ctx context.Context) (int64, error)`

- `func (TransactionService) Create(ctx context.Context, data transaction2.Transaction) (transaction2.Transaction, error)`

- `func (TransactionService) Delete(ctx context.Context, id uuid.UUID) error`

- `func (TransactionService) GetAll(ctx context.Context) ([]transaction2.Transaction, error)`

- `func (TransactionService) GetByID(ctx context.Context, id uuid.UUID) (transaction2.Transaction, error)`

- `func (TransactionService) GetPaginated(ctx context.Context, params *transaction2.FindParams) ([]transaction2.Transaction, error)`

- `func (TransactionService) Update(ctx context.Context, data transaction2.Transaction) (transaction2.Transaction, error)`

---

## Package `hrm` (modules/hrm)

### Types

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

- Var: `[EmployeesLink]`

- Var: `[HRMLink]`

- Var: `[LocaleFiles]`

- Var: `[MigrationFiles]`

- Var: `[NavItems]`

---

## Package `employee` (modules/hrm/domain/aggregates/employee)

### Types

#### CreateDTO

```go
type CreateDTO struct {
    FirstName string `validate:"required"`
    LastName string `validate:"required"`
    MiddleName string
    Email string `validate:"required,email"`
    Phone string `validate:"required"`
    Salary float64 `validate:"required"`
    Tin string
    Pin string
    BirthDate shared.DateOnly
    HireDate shared.DateOnly
    ResignationDate shared.DateOnly
    PrimaryLanguage string
    SecondaryLanguage string
    AvatarID uint
    Notes string
}
```

##### Methods

- `func (CreateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() (Employee, error)`

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data CreateDTO
    Result Employee
}
```

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Employee
}
```

#### Employee

##### Interface Methods

- `ID() uint`
- `TenantID() uuid.UUID`
- `FirstName() string`
- `LastName() string`
- `MiddleName() string`
- `Email() internet.Email`
- `Phone() string`
- `Salary() *money.Money`
- `AvatarID() uint`
- `HireDate() time.Time`
- `BirthDate() time.Time`
- `Language() Language`
- `Passport() passport.Passport`
- `Tin() tax.Tin`
- `Pin() tax.Pin`
- `Notes() string`
- `ResignationDate() *time.Time`
- `UpdateName(firstName, lastName, middleName string)`
- `MarkAsResigned(date time.Time)`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    Query string
    Field string
    CreatedAt DateRange
}
```

#### Language

##### Interface Methods

- `Primary() string`
- `Secondary() string`

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]Employee, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]Employee, error)`
- `GetByID(ctx context.Context, id uint) (Employee, error)`
- `Create(ctx context.Context, data Employee) (Employee, error)`
- `Update(ctx context.Context, data Employee) error`
- `Delete(ctx context.Context, id uint) error`

#### UpdateDTO

```go
type UpdateDTO struct {
    FirstName string
    LastName string
    MiddleName string
    Email string
    Phone string
    Salary float64
    Tin string
    Pin string
    BirthDate shared.DateOnly
    HireDate shared.DateOnly
    ResignationDate shared.DateOnly
    PrimaryLanguage string
    SecondaryLanguage string
    AvatarID uint
    Notes string
}
```

##### Methods

- `func (UpdateDTO) Ok(ctx context.Context) (map[string]string, bool)`

- `func (UpdateDTO) ToEntity(id uint) (Employee, error)`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data UpdateDTO
    Result Employee
}
```

### Functions

---

## Package `position` (modules/hrm/domain/entities/position)

### Types

#### FindParams

```go
type FindParams struct {
    ID int64
    Limit int
    Offset int
    SortBy []string
}
```

#### Position

```go
type Position struct {
    ID uint
    TenantID string
    Name string
    Description string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]*Position, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]*Position, error)`
- `GetByID(ctx context.Context, id int64) (*Position, error)`
- `Create(ctx context.Context, upload *Position) error`
- `Update(ctx context.Context, upload *Position) error`
- `Delete(ctx context.Context, id int64) error`

---

## Package `persistence` (modules/hrm/infrastructure/persistence)

### Types

#### GormEmployeeRepository

##### Methods

- `func (GormEmployeeRepository) Count(ctx context.Context) (int64, error)`

- `func (GormEmployeeRepository) Create(ctx context.Context, data employee.Employee) (employee.Employee, error)`

- `func (GormEmployeeRepository) Delete(ctx context.Context, id uint) error`

- `func (GormEmployeeRepository) GetAll(ctx context.Context) ([]employee.Employee, error)`

- `func (GormEmployeeRepository) GetByID(ctx context.Context, id uint) (employee.Employee, error)`

- `func (GormEmployeeRepository) GetPaginated(ctx context.Context, params *employee.FindParams) ([]employee.Employee, error)`

- `func (GormEmployeeRepository) Update(ctx context.Context, data employee.Employee) error`

#### GormPositionRepository

##### Methods

- `func (GormPositionRepository) Count(ctx context.Context) (int64, error)`

- `func (GormPositionRepository) Create(ctx context.Context, data *position.Position) error`

- `func (GormPositionRepository) Delete(ctx context.Context, id int64) error`

- `func (GormPositionRepository) GetAll(ctx context.Context) ([]*position.Position, error)`

- `func (GormPositionRepository) GetByID(ctx context.Context, id int64) (*position.Position, error)`

- `func (GormPositionRepository) GetPaginated(ctx context.Context, params *position.FindParams) ([]*position.Position, error)`

- `func (GormPositionRepository) Update(ctx context.Context, data *position.Position) error`

### Functions

#### `func NewEmployeeRepository() employee.Repository`

#### `func NewPositionRepository() position.Repository`

### Variables and Constants

- Var: `[ErrEmployeeNotFound]`

- Var: `[ErrPositionNotFound]`

---

## Package `models` (modules/hrm/infrastructure/persistence/models)

### Types

#### Employee

```go
type Employee struct {
    ID uint
    TenantID string
    FirstName string
    LastName string
    MiddleName sql.NullString
    Email string
    Phone sql.NullString
    Salary float64
    SalaryCurrencyID sql.NullString
    HourlyRate float64
    Coefficient float64
    AvatarID *uint
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### EmployeeMeta

```go
type EmployeeMeta struct {
    PrimaryLanguage sql.NullString
    SecondaryLanguage sql.NullString
    Tin sql.NullString
    Pin sql.NullString
    Notes sql.NullString
    BirthDate sql.NullTime
    HireDate sql.NullTime
    ResignationDate sql.NullTime
}
```

#### EmployeePosition

```go
type EmployeePosition struct {
    EmployeeID uint
    PositionID uint
}
```

#### Position

```go
type Position struct {
    ID uint
    TenantID string
    Name string
    Description sql.NullString
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## Package `permissions` (modules/hrm/permissions)

### Variables and Constants

- Var: `[EmployeeCreate EmployeeRead EmployeeUpdate EmployeeDelete]`

- Var: `[Permissions]`

- Const: `[ResourceEmployee]`

---

## Package `controllers` (modules/hrm/presentation/controllers)

### Types

#### EmployeeController

##### Methods

- `func (EmployeeController) Create(w http.ResponseWriter, r *http.Request)`

- `func (EmployeeController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (EmployeeController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (EmployeeController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (EmployeeController) Key() string`

- `func (EmployeeController) List(w http.ResponseWriter, r *http.Request)`

- `func (EmployeeController) Register(r *mux.Router)`

- `func (EmployeeController) Update(w http.ResponseWriter, r *http.Request)`

### Functions

#### `func NewEmployeeController(app application.Application) application.Controller`

---

## Package `mappers` (modules/hrm/presentation/mappers)

### Functions

#### `func EmployeeToViewModel(entity employee.Employee) *viewmodels.Employee`

---

## Package `employees` (modules/hrm/presentation/templates/pages/employees)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Employee *viewmodels.Employee
    PostPath string
    Errors map[string]string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Employee *viewmodels.Employee
    Errors map[string]string
    SaveURL string
    DeleteURL string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Employees []*viewmodels.Employee
    NewURL string
}
```

#### SharedProps

```go
type SharedProps struct {
    Employee *viewmodels.Employee
    Errors map[string]string
}
```

### Functions

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func EmployeesContent(props *IndexPageProps) templ.Component`

#### `func EmployeesTable(props *IndexPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func JoinDateInput(props SharedProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func PassportInput(props SharedProps) templ.Component`

#### `func PinInput(props SharedProps) templ.Component`

#### `func ResignationDateInput(props SharedProps) templ.Component`

#### `func SalaryInput(props SharedProps) templ.Component`

#### `func TinInput(props SharedProps) templ.Component`

### Variables and Constants

---

## Package `viewmodels` (modules/hrm/presentation/viewmodels)

### Types

#### Employee

```go
type Employee struct {
    ID string
    FirstName string
    LastName string
    MiddleName string
    Email string
    Phone string
    Salary string
    BirthDate string
    Tin string
    Pin string
    HireDate string
    ResignationDate string
    Notes string
    CreatedAt string
    UpdatedAt string
}
```

---

## Package `services` (modules/hrm/services)

### Types

#### EmployeeService

##### Methods

- `func (EmployeeService) Count(ctx context.Context) (int64, error)`

- `func (EmployeeService) Create(ctx context.Context, data *employee.CreateDTO) error`

- `func (EmployeeService) Delete(ctx context.Context, id uint) (employee.Employee, error)`

- `func (EmployeeService) GetAll(ctx context.Context) ([]employee.Employee, error)`

- `func (EmployeeService) GetByID(ctx context.Context, id uint) (employee.Employee, error)`

- `func (EmployeeService) GetPaginated(ctx context.Context, params *employee.FindParams) ([]employee.Employee, error)`

- `func (EmployeeService) Update(ctx context.Context, id uint, data *employee.UpdateDTO) error`

#### PositionService

##### Methods

- `func (PositionService) Count(ctx context.Context) (int64, error)`

- `func (PositionService) Create(ctx context.Context, data *position.Position) error`

- `func (PositionService) Delete(ctx context.Context, id int64) error`

- `func (PositionService) GetAll(ctx context.Context) ([]*position.Position, error)`

- `func (PositionService) GetByID(ctx context.Context, id int64) (*position.Position, error)`

- `func (PositionService) GetPaginated(ctx context.Context, params *position.FindParams) ([]*position.Position, error)`

- `func (PositionService) Update(ctx context.Context, data *position.Position) error`

---

## Package `logging` (modules/logging)

### Types

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

---

## Package `persistence` (modules/logging/infrastructure/persistence)

---

## Package `permissions` (modules/logging/permissions)

### Variables and Constants

- Var: `[Permissions]`

- Var: `[ViewLogs]`

- Const: `[ResourceLogs]`

---

## Package `warehouse` (modules/warehouse)

### Types

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

- Var: `[ProductsItem PositionsItem OrdersItem InventoryItem UnitsItem Item]`

- Var: `[NavItems]`

---

## Package `order` (modules/warehouse/domain/aggregates/order)

### Types

#### CreateDTO

```go
type CreateDTO struct {
    Type string
    Status string
    ProductIDs []uint
}
```

##### Methods

- `func (CreateDTO) ToEntity() (Order, error)`

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    Query string
    Field string
    Status string
    Type string
    CreatedAt DateRange
}
```

#### Item

##### Interface Methods

- `Position() *position.Position`
- `Products() []*product.Product`
- `Quantity() int`

#### Order

##### Interface Methods

- `ID() uint`
- `TenantID() uuid.UUID`
- `Type() Type`
- `Status() Status`
- `Items() []Item`
- `CreatedAt() time.Time`
- `SetID(id uint)`
- `SetTenantID(id uuid.UUID)`
- `AddItem(position *position.Position, products ...*product.Product) error`
- `Complete() error`

#### OrderIsCompleteError

```go
type OrderIsCompleteError struct {
    Current Status
}
```

##### Methods

- `func (OrderIsCompleteError) Localize(l *i18n.Localizer) string`

#### ProductIsShippedError

```go
type ProductIsShippedError struct {
    Current product.Status
}
```

##### Methods

- `func (ProductIsShippedError) Localize(l *i18n.Localizer) string`

#### Repository

##### Interface Methods

- `GetPaginated(ctx context.Context, params *FindParams) ([]Order, error)`
- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]Order, error)`
- `GetByID(ctx context.Context, id uint) (Order, error)`
- `Create(ctx context.Context, data Order) error`
- `Update(ctx context.Context, data Order) error`
- `Delete(ctx context.Context, id uint) error`

#### Status

##### Methods

- `func (Status) IsValid() bool`

#### Type

##### Methods

- `func (Type) IsValid() bool`

#### UpdateDTO

```go
type UpdateDTO struct {
    Type string
    Status string
    ProductIDs []uint
}
```

##### Methods

- `func (UpdateDTO) ToEntity(id uint) (Order, error)`

---

## Package `position` (modules/warehouse/domain/aggregates/position)

### Types

#### CreateDTO

```go
type CreateDTO struct {
    Title string `validate:"required"`
    Barcode string `validate:"required"`
    UnitID uint `validate:"required"`
    ImageIDs []uint
}
```

##### Methods

- `func (CreateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() (*Position, error)`

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data CreateDTO
    Result Position
}
```

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Position
}
```

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    Query string
    Field string
    Fields []string
    UnitID string
    CreatedAt DateRange
}
```

#### Position

```go
type Position struct {
    ID uint
    TenantID uuid.UUID
    Title string
    Barcode string
    UnitID uint
    Unit *unit.Unit
    InStock uint
    Images []upload.Upload
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (int64, error)`
- `GetAll(ctx context.Context) ([]*Position, error)`
- `GetAllPositionIds(ctx context.Context) ([]uint, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]*Position, error)`
- `GetByID(ctx context.Context, id uint) (*Position, error)`
- `GetByIDs(ctx context.Context, ids []uint) ([]*Position, error)`
- `GetByBarcode(ctx context.Context, barcode string) (*Position, error)`
- `Create(ctx context.Context, data *Position) error`
- `CreateOrUpdate(ctx context.Context, data *Position) error`
- `Update(ctx context.Context, data *Position) error`
- `Delete(ctx context.Context, id uint) error`

#### UpdateDTO

```go
type UpdateDTO struct {
    Title string
    Barcode string
    UnitID uint
}
```

##### Methods

- `func (UpdateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (UpdateDTO) ToEntity(id uint) (*Position, error)`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data UpdateDTO
    Result Position
}
```

---

## Package `product` (modules/warehouse/domain/aggregates/product)

### Types

#### CountParams

```go
type CountParams struct {
    PositionID uint
    Status Status
}
```

#### CreateDTO

```go
type CreateDTO struct {
    PositionID uint
    Rfid string
    Status string
}
```

##### Methods

- `func (CreateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() (*Product, error)`

#### CreateProductsFromTagsDTO

```go
type CreateProductsFromTagsDTO struct {
    Tags []string
    PositionID uint
}
```

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data CreateDTO
    Result Product
}
```

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Product
}
```

#### FindByPositionParams

```go
type FindByPositionParams struct {
    Limit int
    SortBy []string
    PositionID uint
    Status Status
}
```

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    Query string
    Field string
    Status string
    PositionID uint
    CreatedAt DateRange
    Rfids []string
    OrderID uint
}
```

#### Product

```go
type Product struct {
    ID uint
    TenantID uuid.UUID
    PositionID uint
    Rfid string
    Status Status
    Position *position.Position
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Repository

##### Interface Methods

- `GetPaginated(context.Context, *FindParams) ([]*Product, error)`
- `Count(context.Context, *CountParams) (int64, error)`
- `GetAll(context.Context) ([]*Product, error)`
- `GetByID(context.Context, uint) (*Product, error)`
- `GetByRfid(context.Context, string) (*Product, error)`
- `GetByRfidMany(context.Context, []string) ([]*Product, error)`
- `FindByPositionID(context.Context, *FindByPositionParams) ([]*Product, error)`
- `UpdateStatus(context.Context, []uint, Status) error`
- `Create(context.Context, *Product) error`
- `BulkCreate(context.Context, []*Product) error`
- `CreateOrUpdate(context.Context, *Product) error`
- `Update(context.Context, *Product) error`
- `BulkDelete(context.Context, []uint) error`
- `Delete(context.Context, uint) error`

#### Status

##### Methods

- `func (Status) IsValid() bool`

#### UpdateDTO

```go
type UpdateDTO struct {
    PositionID uint
    Rfid string
    Status string
}
```

##### Methods

- `func (UpdateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (UpdateDTO) ToEntity(id uint) (*Product, error)`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data UpdateDTO
    Result Product
}
```

### Variables and Constants

- Var: `[ErrInvalidStatus]`

---

## Package `inventory` (modules/warehouse/domain/entities/inventory)

### Types

#### Check

```go
type Check struct {
    ID uint
    TenantID uuid.UUID
    Status Status
    Name string
    Results []*CheckResult
    CreatedAt time.Time
    FinishedAt time.Time
    CreatedByID uint
    CreatedBy user.User
    FinishedBy user.User
    FinishedByID uint
}
```

##### Methods

- `func (Check) AddResult(positionID uint, expected, actual int)`

#### CheckResult

```go
type CheckResult struct {
    ID uint
    TenantID uuid.UUID
    PositionID uint
    Position *position.Position
    ExpectedQuantity int
    ActualQuantity int
    Difference int
    CreatedAt time.Time
}
```

#### CreateCheckDTO

```go
type CreateCheckDTO struct {
    Name string `validate:"required"`
    Positions []*PositionCheckDTO
}
```

##### Methods

- `func (CreateCheckDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (CreateCheckDTO) ToEntity(createdBy user.User) (*Check, error)`

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data CreateCheckDTO
    Result Check
}
```

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Check
}
```

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    Query string
    Field string
    Status string
    Type string
    ID uint
    CreatedAt DateRange
    AttachResults bool
    WithDifference bool
}
```

#### Position

```go
type Position struct {
    ID uint
    Title string
    Quantity int
    RfidTags []string
}
```

#### PositionCheckDTO

```go
type PositionCheckDTO struct {
    PositionID uint
    Found uint
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (uint, error)`
- `GetAll(ctx context.Context) ([]*Check, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]*Check, error)`
- `GetByID(ctx context.Context, id uint) (*Check, error)`
- `Positions(ctx context.Context) ([]*Position, error)`
- `GetByIDWithDifference(ctx context.Context, id uint) (*Check, error)`
- `Create(ctx context.Context, upload *Check) error`
- `Update(ctx context.Context, upload *Check) error`
- `Delete(ctx context.Context, id uint) error`

#### Status

##### Methods

- `func (Status) IsValid() bool`

#### Type

#### UpdateCheckDTO

```go
type UpdateCheckDTO struct {
    FinishedAt time.Time
    Name string
}
```

##### Methods

- `func (UpdateCheckDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (UpdateCheckDTO) ToEntity(id uint) (*Check, error)`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data UpdateCheckDTO
    Result Check
}
```

---

## Package `unit` (modules/warehouse/domain/entities/unit)

### Types

#### CreateDTO

```go
type CreateDTO struct {
    Title string `validate:"required"`
    ShortTitle string `validate:"required"`
}
```

##### Methods

- `func (CreateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (CreateDTO) ToEntity() (*Unit, error)`

#### CreatedEvent

```go
type CreatedEvent struct {
    Sender user.User
    Session session.Session
    Data CreateDTO
    Result Unit
}
```

#### DateRange

```go
type DateRange struct {
    From string
    To string
}
```

#### DeletedEvent

```go
type DeletedEvent struct {
    Sender user.User
    Session session.Session
    Result Unit
}
```

#### FindParams

```go
type FindParams struct {
    Limit int
    Offset int
    SortBy []string
    CreatedAt DateRange
}
```

#### Repository

##### Interface Methods

- `Count(ctx context.Context) (uint, error)`
- `GetAll(ctx context.Context) ([]*Unit, error)`
- `GetPaginated(ctx context.Context, params *FindParams) ([]*Unit, error)`
- `GetByID(ctx context.Context, id uint) (*Unit, error)`
- `GetByTitleOrShortTitle(ctx context.Context, name string) (*Unit, error)`
- `Create(ctx context.Context, upload *Unit) error`
- `CreateOrUpdate(ctx context.Context, upload *Unit) error`
- `Update(ctx context.Context, upload *Unit) error`
- `Delete(ctx context.Context, id uint) error`

#### Unit

```go
type Unit struct {
    ID uint
    TenantID uuid.UUID
    Title string
    ShortTitle string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### UpdateDTO

```go
type UpdateDTO struct {
    Title string
    ShortTitle string
}
```

##### Methods

- `func (UpdateDTO) Ok(l ut.Translator) (map[string]string, bool)`

- `func (UpdateDTO) ToEntity(id uint) (*Unit, error)`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Sender user.User
    Session session.Session
    Data UpdateDTO
    Result Unit
}
```

---

## Package `persistence` (modules/warehouse/infrastructure/persistence)

### Types

#### GormInventoryRepository

##### Methods

- `func (GormInventoryRepository) Count(ctx context.Context) (uint, error)`

- `func (GormInventoryRepository) Create(ctx context.Context, data *inventory.Check) error`

- `func (GormInventoryRepository) Delete(ctx context.Context, id uint) error`

- `func (GormInventoryRepository) GetAll(ctx context.Context) ([]*inventory.Check, error)`

- `func (GormInventoryRepository) GetByID(ctx context.Context, id uint) (*inventory.Check, error)`

- `func (GormInventoryRepository) GetByIDWithDifference(ctx context.Context, id uint) (*inventory.Check, error)`

- `func (GormInventoryRepository) GetPaginated(ctx context.Context, params *inventory.FindParams) ([]*inventory.Check, error)`

- `func (GormInventoryRepository) Positions(ctx context.Context) ([]*inventory.Position, error)`

- `func (GormInventoryRepository) Update(ctx context.Context, data *inventory.Check) error`

#### GormOrderRepository

##### Methods

- `func (GormOrderRepository) Count(ctx context.Context) (int64, error)`

- `func (GormOrderRepository) Create(ctx context.Context, data order.Order) error`

- `func (GormOrderRepository) Delete(ctx context.Context, id uint) error`

- `func (GormOrderRepository) GetAll(ctx context.Context) ([]order.Order, error)`

- `func (GormOrderRepository) GetByID(ctx context.Context, id uint) (order.Order, error)`

- `func (GormOrderRepository) GetPaginated(ctx context.Context, params *order.FindParams) ([]order.Order, error)`

- `func (GormOrderRepository) Update(ctx context.Context, data order.Order) error`

#### GormPositionRepository

##### Methods

- `func (GormPositionRepository) Count(ctx context.Context) (int64, error)`

- `func (GormPositionRepository) Create(ctx context.Context, data *position.Position) error`

- `func (GormPositionRepository) CreateOrUpdate(ctx context.Context, data *position.Position) error`

- `func (GormPositionRepository) Delete(ctx context.Context, id uint) error`

- `func (GormPositionRepository) GetAll(ctx context.Context) ([]*position.Position, error)`

- `func (GormPositionRepository) GetAllPositionIds(ctx context.Context) ([]uint, error)`

- `func (GormPositionRepository) GetByBarcode(ctx context.Context, barcode string) (*position.Position, error)`

- `func (GormPositionRepository) GetByID(ctx context.Context, id uint) (*position.Position, error)`

- `func (GormPositionRepository) GetByIDs(ctx context.Context, ids []uint) ([]*position.Position, error)`

- `func (GormPositionRepository) GetPaginated(ctx context.Context, params *position.FindParams) ([]*position.Position, error)`

- `func (GormPositionRepository) Update(ctx context.Context, data *position.Position) error`

#### GormProductRepository

##### Methods

- `func (GormProductRepository) BulkCreate(ctx context.Context, data []*product.Product) error`

- `func (GormProductRepository) BulkDelete(ctx context.Context, ids []uint) error`

- `func (GormProductRepository) Count(ctx context.Context, opts *product.CountParams) (int64, error)`

- `func (GormProductRepository) Create(ctx context.Context, data *product.Product) error`

- `func (GormProductRepository) CreateOrUpdate(ctx context.Context, data *product.Product) error`

- `func (GormProductRepository) Delete(ctx context.Context, id uint) error`

- `func (GormProductRepository) FindByPositionID(ctx context.Context, opts *product.FindByPositionParams) ([]*product.Product, error)`

- `func (GormProductRepository) GetAll(ctx context.Context) ([]*product.Product, error)`

- `func (GormProductRepository) GetByID(ctx context.Context, id uint) (*product.Product, error)`

- `func (GormProductRepository) GetByRfid(ctx context.Context, rfid string) (*product.Product, error)`

- `func (GormProductRepository) GetByRfidMany(ctx context.Context, tags []string) ([]*product.Product, error)`

- `func (GormProductRepository) GetPaginated(ctx context.Context, params *product.FindParams) ([]*product.Product, error)`

- `func (GormProductRepository) Update(ctx context.Context, data *product.Product) error`

- `func (GormProductRepository) UpdateStatus(ctx context.Context, ids []uint, status product.Status) error`

#### GormUnitRepository

##### Methods

- `func (GormUnitRepository) Count(ctx context.Context) (uint, error)`

- `func (GormUnitRepository) Create(ctx context.Context, data *unit.Unit) error`

- `func (GormUnitRepository) CreateOrUpdate(ctx context.Context, data *unit.Unit) error`

- `func (GormUnitRepository) Delete(ctx context.Context, id uint) error`

- `func (GormUnitRepository) GetAll(ctx context.Context) ([]*unit.Unit, error)`

- `func (GormUnitRepository) GetByID(ctx context.Context, id uint) (*unit.Unit, error)`

- `func (GormUnitRepository) GetByTitleOrShortTitle(ctx context.Context, name string) (*unit.Unit, error)`

- `func (GormUnitRepository) GetPaginated(ctx context.Context, params *unit.FindParams) ([]*unit.Unit, error)`

- `func (GormUnitRepository) Update(ctx context.Context, data *unit.Unit) error`

### Functions

#### `func NewInventoryRepository(userRepo user.Repository, positionRepo position.Repository) inventory.Repository`

#### `func NewOrderRepository(productRepo product.Repository) order.Repository`

#### `func NewPositionRepository() position.Repository`

#### `func NewProductRepository() product.Repository`

#### `func NewUnitRepository() unit.Repository`

### Variables and Constants

- Var: `[ErrInventoryCheckNotFound]`

- Var: `[ErrOrderNotFound]`

- Var: `[ErrPositionNotFound]`

- Var: `[ErrProductNotFound]`

- Var: `[ErrUnitNotFound]`

---

## Package `mappers` (modules/warehouse/infrastructure/persistence/mappers)

### Functions

#### `func ToDBInventoryCheck(check *inventory.Check) (*models.InventoryCheck, error)`

#### `func ToDBInventoryCheckResult(result *inventory.CheckResult) (*models.InventoryCheckResult, error)`

#### `func ToDBOrder(entity order.Order) (*models.WarehouseOrder, []*models.WarehouseProduct, error)`

#### `func ToDBPosition(entity *position.Position) (*models.WarehousePosition, []*models.WarehousePositionImage)`

#### `func ToDBProduct(entity *product.Product) (*models.WarehouseProduct, error)`

#### `func ToDBUnit(unit *unit.Unit) *models.WarehouseUnit`

#### `func ToDomainInventoryCheck(dbInventoryCheck *models.InventoryCheck) (*inventory.Check, error)`

#### `func ToDomainInventoryCheckResult(result *models.InventoryCheckResult) (*inventory.CheckResult, error)`

#### `func ToDomainInventoryPosition(dbPosition *models.InventoryPosition) (*inventory.Position, error)`

#### `func ToDomainOrder(dbOrder *models.WarehouseOrder) (order.Order, error)`

#### `func ToDomainPosition(dbPosition *models.WarehousePosition, dbUnit *models.WarehouseUnit) (*position.Position, error)`

#### `func ToDomainProduct(dbProduct *models.WarehouseProduct, dbPosition *models.WarehousePosition, dbUnit *models.WarehouseUnit) (*product.Product, error)`

#### `func ToDomainUnit(dbUnit *models.WarehouseUnit) (*unit.Unit, error)`

---

## Package `models` (modules/warehouse/infrastructure/persistence/models)

### Types

#### InventoryCheck

```go
type InventoryCheck struct {
    ID uint
    TenantID string
    Status string
    Name string
    Results []*InventoryCheckResult `gorm:"foreignKey:InventoryCheckID"`
    CreatedAt time.Time
    FinishedAt *time.Time
    CreatedByID uint
    CreatedBy *coremodels.User `gorm:"foreignKey:CreatedByID"`
    FinishedByID *uint
    FinishedBy *coremodels.User `gorm:"foreignKey:FinishedByID"`
}
```

#### InventoryCheckResult

```go
type InventoryCheckResult struct {
    ID uint
    TenantID string
    InventoryCheckID uint
    PositionID uint
    Position *WarehousePosition
    ExpectedQuantity int
    ActualQuantity int
    Difference int
    CreatedAt time.Time
}
```

#### InventoryPosition

```go
type InventoryPosition struct {
    ID uint
    Title string
    Quantity int
    RfidTags pq.StringArray
}
```

#### WarehouseOrder

```go
type WarehouseOrder struct {
    ID uint
    TenantID string
    Type string
    Status string
    CreatedAt time.Time
}
```

#### WarehouseOrderItem

```go
type WarehouseOrderItem struct {
    WarehouseOrderID uint
    WarehouseProductID uint
}
```

#### WarehousePosition

```go
type WarehousePosition struct {
    ID uint
    TenantID string
    Title string
    Barcode string
    UnitID sql.NullInt32
    Images []coremodels.Upload `gorm:"many2many:warehouse_position_images;"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### WarehousePositionImage

```go
type WarehousePositionImage struct {
    UploadID uint
    WarehousePositionID uint
}
```

#### WarehouseProduct

```go
type WarehouseProduct struct {
    ID uint
    TenantID string
    PositionID uint
    Rfid sql.NullString
    Status string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### WarehouseUnit

```go
type WarehouseUnit struct {
    ID uint
    TenantID string
    Title string
    ShortTitle string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## Package `graph` (modules/warehouse/interfaces/graph)

### Types

#### ComplexityRoot

```go
type ComplexityRoot struct {
    InventoryPosition struct{...}
    Mutation struct{...}
    Order struct{...}
    OrderItem struct{...}
    PaginatedOrders struct{...}
    PaginatedProducts struct{...}
    PaginatedWarehousePositions struct{...}
    Product struct{...}
    Query struct{...}
    ValidateProductsResult struct{...}
    WarehousePosition struct{...}
}
```

#### Config

```go
type Config struct {
    Schema *ast.Schema
    Resolvers ResolverRoot
    Directives DirectiveRoot
    Complexity ComplexityRoot
}
```

#### DirectiveRoot

#### MutationResolver

##### Interface Methods

- `CompleteInventoryCheck(ctx context.Context, items []*model.InventoryItem) (bool, error)`

#### QueryResolver

##### Interface Methods

- `Hello(ctx context.Context, name *string) (*string, error)`
- `Inventory(ctx context.Context) ([]*model.InventoryPosition, error)`
- `Order(ctx context.Context, id int64) (*model.Order, error)`
- `Orders(ctx context.Context, query model.OrderQuery) (*model.PaginatedOrders, error)`
- `CompleteOrder(ctx context.Context, id int64) (*model.Order, error)`
- `WarehousePosition(ctx context.Context, id int64) (*model.WarehousePosition, error)`
- `WarehousePositions(ctx context.Context, offset int, limit int, sortBy []string) (*model.PaginatedWarehousePositions, error)`
- `Product(ctx context.Context, id int64) (*model.Product, error)`
- `Products(ctx context.Context, offset int, limit int, sortBy []string) (*model.PaginatedProducts, error)`
- `CreateProductsFromTags(ctx context.Context, input model.CreateProductsFromTags) ([]*model.Product, error)`
- `ValidateProducts(ctx context.Context, tags []string) (*model.ValidateProductsResult, error)`

#### Resolver

##### Methods

- `func (Resolver) Mutation() MutationResolver`
  Mutation returns MutationResolver implementation.
  

- `func (Resolver) Query() QueryResolver`
  Query returns QueryResolver implementation.
  

#### ResolverRoot

##### Interface Methods

- `Mutation() MutationResolver`
- `Query() QueryResolver`

### Functions

#### `func NewExecutableSchema(cfg Config) graphql.ExecutableSchema`

NewExecutableSchema creates an ExecutableSchema from the ResolverRoot interface.


### Variables and Constants

- Var: `[ProductsToGraphModel ProductsToTags InventoryPositionsToGraphModel]`

---

## Package `model` (modules/warehouse/interfaces/graph/gqlmodels)

### Types

#### CreateProductsFromTags

```go
type CreateProductsFromTags struct {
    PositionID int64 `json:"positionId"`
    Tags []string `json:"tags"`
}
```

#### InventoryItem

```go
type InventoryItem struct {
    PositionID int64 `json:"positionId"`
    Found int `json:"found"`
}
```

#### InventoryPosition

```go
type InventoryPosition struct {
    ID int64 `json:"id"`
    Title string `json:"title"`
    Tags []string `json:"tags"`
}
```

#### Mutation

#### Order

```go
type Order struct {
    ID int64 `json:"id"`
    Type string `json:"type"`
    Status string `json:"status"`
    Items []*OrderItem `json:"items"`
    CreatedAt time.Time `json:"createdAt"`
}
```

#### OrderItem

```go
type OrderItem struct {
    Position *WarehousePosition `json:"position"`
    Products []*Product `json:"products"`
    Quantity int `json:"quantity"`
}
```

#### OrderQuery

```go
type OrderQuery struct {
    Type *string `json:"type,omitempty"`
    Status *string `json:"status,omitempty"`
    Limit int `json:"limit"`
    Offset int `json:"offset"`
    SortBy []string `json:"sortBy,omitempty"`
}
```

#### PaginatedOrders

```go
type PaginatedOrders struct {
    Data []*Order `json:"data"`
    Total int64 `json:"total"`
}
```

#### PaginatedProducts

```go
type PaginatedProducts struct {
    Data []*Product `json:"data"`
    Total int64 `json:"total"`
}
```

#### PaginatedWarehousePositions

```go
type PaginatedWarehousePositions struct {
    Data []*WarehousePosition `json:"data"`
    Total int64 `json:"total"`
}
```

#### Product

```go
type Product struct {
    ID int64 `json:"id"`
    Position *WarehousePosition `json:"position"`
    PositionID int64 `json:"positionID"`
    Rfid string `json:"rfid"`
    Status string `json:"status"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

#### Query

#### ValidateProductsResult

```go
type ValidateProductsResult struct {
    Valid []string `json:"valid"`
    Invalid []string `json:"invalid"`
}
```

#### WarehousePosition

```go
type WarehousePosition struct {
    ID int64 `json:"id"`
    Title string `json:"title"`
    Barcode string `json:"barcode"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

---

## Package `mappers` (modules/warehouse/interfaces/graph/mappers)

### Functions

#### `func InventoryPositionToGraphModel(entity *inventory.Position) *model.InventoryPosition`

#### `func OrderItemsToGraphModel(item order.Item) *model.OrderItem`

#### `func OrderToGraphModel(o order.Order) *model.Order`

#### `func PositionToGraphModel(item *position.Position) *model.WarehousePosition`

#### `func ProductToGraphModel(entity *product.Product) *model.Product`

---

## Package `permissions` (modules/warehouse/permissions)

### Variables and Constants

- Var: `[ProductCreate ProductRead ProductUpdate ProductDelete PositionCreate PositionRead PositionUpdate PositionDelete OrderCreate OrderRead OrderUpdate OrderDelete UnitCreate UnitRead UnitUpdate UnitDelete InventoryCreate InventoryRead InventoryUpdate InventoryDelete]`

- Var: `[Permissions]`

- Const: `[ResourceProduct ResourcePosition ResourceOrder ResourceUnit ResourceInventory]`

---

## Package `assets` (modules/warehouse/presentation/assets)

### Variables and Constants

- Var: `[FS]`

---

## Package `controllers` (modules/warehouse/presentation/controllers)

### Types

#### InventoryCheckPaginatedResponse

```go
type InventoryCheckPaginatedResponse struct {
    Checks []*viewmodels.Check
    PaginationState *pagination.State
}
```

#### InventoryController

##### Methods

- `func (InventoryController) Create(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) GetEditDifference(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) Key() string`

- `func (InventoryController) List(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) Register(r *mux.Router)`

- `func (InventoryController) SearchPositions(w http.ResponseWriter, r *http.Request)`

- `func (InventoryController) Update(w http.ResponseWriter, r *http.Request)`

#### OrderItem

```go
type OrderItem struct {
    PositionID uint
    PositionTitle string
    Barcode string
    Unit string
    InStock uint
    Quantity uint
    Error string
}
```

#### OrderPaginatedResponse

```go
type OrderPaginatedResponse struct {
    Orders []*viewmodels.Order
    PaginationState *pagination.State
}
```

#### OrdersController

##### Methods

- `func (OrdersController) CreateInOrder(w http.ResponseWriter, r *http.Request)`

- `func (OrdersController) CreateOutOrder(w http.ResponseWriter, r *http.Request)`

- `func (OrdersController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (OrdersController) Key() string`

- `func (OrdersController) List(w http.ResponseWriter, r *http.Request)`

- `func (OrdersController) NewInOrder(w http.ResponseWriter, r *http.Request)`

- `func (OrdersController) NewOutOrder(w http.ResponseWriter, r *http.Request)`

- `func (OrdersController) OrderItems(w http.ResponseWriter, r *http.Request)`

- `func (OrdersController) Register(r *mux.Router)`

- `func (OrdersController) ViewOrder(w http.ResponseWriter, r *http.Request)`

#### PaginatedResponse

```go
type PaginatedResponse struct {
    Products []*viewmodels.Product
    PaginationState *pagination.State
}
```

#### PositionPaginatedResponse

```go
type PositionPaginatedResponse struct {
    Positions []*viewmodels2.Position
    PaginationState *pagination.State
}
```

#### PositionsController

##### Methods

- `func (PositionsController) Create(w http.ResponseWriter, r *http.Request)`

- `func (PositionsController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (PositionsController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (PositionsController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (PositionsController) GetUpload(w http.ResponseWriter, r *http.Request)`

- `func (PositionsController) HandleUpload(w http.ResponseWriter, r *http.Request)`

- `func (PositionsController) Key() string`

- `func (PositionsController) List(w http.ResponseWriter, r *http.Request)`

- `func (PositionsController) Register(r *mux.Router)`

- `func (PositionsController) Search(w http.ResponseWriter, r *http.Request)`

- `func (PositionsController) Update(w http.ResponseWriter, r *http.Request)`

#### ProductsController

##### Methods

- `func (ProductsController) Create(w http.ResponseWriter, r *http.Request)`

- `func (ProductsController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (ProductsController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (ProductsController) Key() string`

- `func (ProductsController) List(w http.ResponseWriter, r *http.Request)`

- `func (ProductsController) Register(r *mux.Router)`

- `func (ProductsController) Update(w http.ResponseWriter, r *http.Request)`

#### UnitPaginatedResponse

```go
type UnitPaginatedResponse struct {
    Units []*viewmodels.Unit
    PaginationState *pagination.State
}
```

#### UnitsController

##### Methods

- `func (UnitsController) Create(w http.ResponseWriter, r *http.Request)`

- `func (UnitsController) Delete(w http.ResponseWriter, r *http.Request)`

- `func (UnitsController) GetEdit(w http.ResponseWriter, r *http.Request)`

- `func (UnitsController) GetNew(w http.ResponseWriter, r *http.Request)`

- `func (UnitsController) Key() string`

- `func (UnitsController) List(w http.ResponseWriter, r *http.Request)`

- `func (UnitsController) Register(r *mux.Router)`

- `func (UnitsController) Update(w http.ResponseWriter, r *http.Request)`

### Functions

#### `func NewInventoryController(app application.Application) application.Controller`

#### `func NewOrdersController(app application.Application) application.Controller`

#### `func NewPositionsController(app application.Application) application.Controller`

#### `func NewProductsController(app application.Application) application.Controller`

#### `func NewUnitsController(app application.Application) application.Controller`

#### `func OrderInItemToViewModel(item OrderItem) orderin.OrderItem`

#### `func OrderOutItemToViewModel(item OrderItem) orderout.OrderItem`

### Variables and Constants

- Var: `[OrdersToViewModels]`

---

## Package `dtos` (modules/warehouse/presentation/controllers/dtos)

### Types

#### CreateOrderDTO

```go
type CreateOrderDTO struct {
    PositionIDs []uint `validate:"required"`
    Quantity map[uint]uint `validate:"required"`
}
```

##### Methods

- `func (CreateOrderDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### PositionsUploadDTO

```go
type PositionsUploadDTO struct {
    FileID uint `validate:"required"`
}
```

##### Methods

- `func (PositionsUploadDTO) Ok(l ut.Translator) (map[string]string, bool)`

#### UpdateOrderDTO

```go
type UpdateOrderDTO struct {
    PositionIDs []uint
    Quantities map[uint]uint
}
```

##### Methods

- `func (UpdateOrderDTO) Ok(ctx context.Context) (map[string]string, bool)`

---

## Package `mappers` (modules/warehouse/presentation/mappers)

### Functions

#### `func CheckResultToViewModel(entity *inventory.CheckResult) *viewmodels.CheckResult`

#### `func CheckToViewModel(entity *inventory.Check) *viewmodels.Check`

#### `func OrderItemToViewModel(entity order.Item, inStock int) viewmodels.OrderItem`

#### `func OrderToViewModel(entity order.Order, inStockByPosition map[uint]int) *viewmodels.Order`

#### `func PositionToViewModel(entity *position.Position) *viewmodels.Position`

#### `func ProductToViewModel(entity *product.Product) *viewmodels.Product`

#### `func UnitToViewModel(entity *unit.Unit) *viewmodels.Unit`

---

## Package `templates` (modules/warehouse/presentation/templates)

### Variables and Constants

- Var: `[FS]`

---

## Package `inventory` (modules/warehouse/presentation/templates/pages/inventory)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Check *viewmodels.Check
    Positions []*viewmodels.Position
    PaginationState *pagination.State
    Errors map[string]string
    SaveURL string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Check *viewmodels.Check
    Positions []*viewmodels.Position
    PaginationState *pagination.State
    Errors map[string]string
    DeleteURL string
    SaveURL string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Checks []*viewmodels.Check
    PaginationState *pagination.State
}
```

### Functions

#### `func AllPositionsTable(props *CreatePageProps) templ.Component`

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func InventoryContent(props *IndexPageProps) templ.Component`

#### `func InventoryTable(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

### Variables and Constants

---

## Package `orders` (modules/warehouse/presentation/templates/pages/orders)

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### IndexPageProps

```go
type IndexPageProps struct {
    Orders []*viewmodels.Order
    PaginationState *pagination.State
}
```

#### ViewPageProps

```go
type ViewPageProps struct {
    Order *viewmodels.Order
    DeleteURL string
}
```

### Functions

#### `func Index(props *IndexPageProps) templ.Component`

#### `func OrdersContent(props *IndexPageProps) templ.Component`

#### `func OrdersTable(props *IndexPageProps) templ.Component`

#### `func View(props *ViewPageProps) templ.Component`

### Variables and Constants

---

## Package `orderout` (modules/warehouse/presentation/templates/pages/orders/in)

templ: version: v0.3.857


### Types

#### FormProps

```go
type FormProps struct {
    Errors map[string]string
    Items []OrderItem
}
```

#### OrderItem

```go
type OrderItem struct {
    PositionID string
    PositionTitle string
    Barcode string
    Unit string
    InStock string
    Quantity string
    Error string
}
```

#### PageProps

```go
type PageProps struct {
    Errors map[string]string
    SaveURL string
    ItemsURL string
    Items []OrderItem
}
```

### Functions

#### `func Form(props *FormProps) templ.Component`

#### `func New(props *PageProps) templ.Component`

#### `func OrderItemsTable(items []OrderItem) templ.Component`

### Variables and Constants

---

## Package `orderout` (modules/warehouse/presentation/templates/pages/orders/out)

templ: version: v0.3.857


### Types

#### FormProps

```go
type FormProps struct {
    Errors map[string]string
    Items []OrderItem
}
```

#### OrderItem

```go
type OrderItem struct {
    PositionID string
    PositionTitle string
    Barcode string
    Unit string
    InStock string
    Quantity string
    Error string
}
```

#### PageProps

```go
type PageProps struct {
    Errors map[string]string
    SaveURL string
    ItemsURL string
    Items []OrderItem
}
```

### Functions

#### `func Form(props *FormProps) templ.Component`

#### `func New(props *PageProps) templ.Component`

#### `func OrderItemsTable(items []OrderItem) templ.Component`

### Variables and Constants

---

## Package `positions` (modules/warehouse/presentation/templates/pages/positions)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Positions []*viewmodels.Position
    Position *viewmodels.Position
    Units []*viewmodels.Unit
    Errors map[string]string
    SaveURL string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Position *viewmodels.Position
    Units []*viewmodels.Unit
    Errors map[string]string
    SaveURL string
    DeleteURL string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Positions []*viewmodels.Position
    Units []*viewmodels.Unit
    PaginationState *pagination.State
}
```

#### UnitSelectProps

```go
type UnitSelectProps struct {
    Value string
    Units []*viewmodels.Unit
    Attrs templ.Attributes
    Error string
}
```

#### UploadPageProps

```go
type UploadPageProps struct {
    Errors map[string]string
    SaveURL string
}
```

### Functions

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func PositionsContent(props *IndexPageProps) templ.Component`

#### `func PositionsTable(props *IndexPageProps) templ.Component`

#### `func UnitSelect(props *UnitSelectProps) templ.Component`

#### `func Upload(props *UploadPageProps) templ.Component`

#### `func UploadForm(props *UploadPageProps) templ.Component`

### Variables and Constants

---

## Package `products` (modules/warehouse/presentation/templates/pages/products)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Product *viewmodels.Product
    SaveURL string
    Errors map[string]string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Product *viewmodels.Product
    Errors map[string]string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Products []*viewmodels.Product
    PaginationState *pagination.State
}
```

#### PositionSelectProps

```go
type PositionSelectProps struct {
    Value string
    Attrs templ.Attributes
}
```

#### StatusSelectProps

```go
type StatusSelectProps struct {
    Value string
    Attrs templ.Attributes
}
```

#### StatusViewModel

```go
type StatusViewModel struct {
    MessageId string
    Value string
}
```

### Functions

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func PositionSelect(props *PositionSelectProps) templ.Component`

#### `func ProductsContent(props *IndexPageProps) templ.Component`

#### `func ProductsTable(props *IndexPageProps) templ.Component`

#### `func StatusSelect(props *StatusSelectProps) templ.Component`

### Variables and Constants

- Var: `[selectOnce InStock InDevelopment Approved Statuses]`

---

## Package `units` (modules/warehouse/presentation/templates/pages/units)

templ: version: v0.3.857

templ: version: v0.3.857

templ: version: v0.3.857


### Types

#### CreatePageProps

```go
type CreatePageProps struct {
    Unit *viewmodels.Unit
    Errors map[string]string
    SaveURL string
}
```

#### EditPageProps

```go
type EditPageProps struct {
    Unit *viewmodels.Unit
    Errors map[string]string
    DeleteURL string
}
```

#### IndexPageProps

```go
type IndexPageProps struct {
    Units []*viewmodels.Unit
    PaginationState *pagination.State
}
```

### Functions

#### `func CreateForm(props *CreatePageProps) templ.Component`

#### `func Edit(props *EditPageProps) templ.Component`

#### `func EditForm(props *EditPageProps) templ.Component`

#### `func Index(props *IndexPageProps) templ.Component`

#### `func New(props *CreatePageProps) templ.Component`

#### `func UnitsContent(props *IndexPageProps) templ.Component`

#### `func UnitsTable(props *IndexPageProps) templ.Component`

### Variables and Constants

---

## Package `viewmodels` (modules/warehouse/presentation/viewmodels)

### Types

#### Check

```go
type Check struct {
    ID string
    Type string
    Status string
    Name string
    Results []*CheckResult
    CreatedAt string
    FinishedAt string
    CreatedBy *viewmodels.User
    FinishedBy *viewmodels.User
}
```

##### Methods

- `func (Check) LocalizedStatus(l *i18n.Localizer) string`

- `func (Check) LocalizedType(l *i18n.Localizer) string`

#### CheckResult

```go
type CheckResult struct {
    ID string
    PositionID string
    Position *Position
    ExpectedQuantity string
    ActualQuantity string
    Difference string
    CreatedAt string
}
```

#### Order

```go
type Order struct {
    ID string
    Type string
    Status string
    Items []OrderItem
    CreatedAt string
    UpdatedAt string
}
```

##### Methods

- `func (Order) DistinctPositions() string`

- `func (Order) LocalizedStatus(l *i18n.Localizer) string`

- `func (Order) LocalizedTitle(l *i18n.Localizer) string`

- `func (Order) LocalizedType(l *i18n.Localizer) string`

- `func (Order) TotalProducts() string`

#### OrderItem

```go
type OrderItem struct {
    Position Position
    Products []Product
    InStock string
}
```

##### Methods

- `func (OrderItem) Quantity() string`

#### Position

```go
type Position struct {
    ID string
    Title string
    Barcode string
    UnitID string
    Unit Unit
    Images []*viewmodels.Upload
    CreatedAt string
    UpdatedAt string
}
```

#### Product

```go
type Product struct {
    ID string
    PositionID string
    Position *Position
    Rfid string
    Status string
    CreatedAt string
    UpdatedAt string
}
```

##### Methods

- `func (Product) LocalizedStatus(l *i18n.Localizer) string`

#### Unit

```go
type Unit struct {
    ID string
    Title string
    ShortTitle string
    CreatedAt string
    UpdatedAt string
}
```

---

## Package `services` (modules/warehouse/services)

### Types

#### InventoryService

##### Methods

- `func (InventoryService) Count(ctx context.Context) (uint, error)`

- `func (InventoryService) Create(ctx context.Context, data *inventory.CreateCheckDTO) (*inventory.Check, error)`

- `func (InventoryService) Delete(ctx context.Context, id uint) (*inventory.Check, error)`

- `func (InventoryService) GetAll(ctx context.Context) ([]*inventory.Check, error)`

- `func (InventoryService) GetByID(ctx context.Context, id uint) (*inventory.Check, error)`

- `func (InventoryService) GetByIDWithDifference(ctx context.Context, id uint) (*inventory.Check, error)`

- `func (InventoryService) GetPaginated(ctx context.Context, params *inventory.FindParams) ([]*inventory.Check, error)`

- `func (InventoryService) Positions(ctx context.Context) ([]*inventory.Position, error)`

- `func (InventoryService) Update(ctx context.Context, id uint, data *inventory.UpdateCheckDTO) error`

#### UnitService

##### Methods

- `func (UnitService) Count(ctx context.Context) (uint, error)`

- `func (UnitService) Create(ctx context.Context, data *unit.CreateDTO) (*unit.Unit, error)`

- `func (UnitService) Delete(ctx context.Context, id uint) (*unit.Unit, error)`

- `func (UnitService) GetAll(ctx context.Context) ([]*unit.Unit, error)`

- `func (UnitService) GetByID(ctx context.Context, id uint) (*unit.Unit, error)`

- `func (UnitService) GetByTitleOrShortTitle(ctx context.Context, name string) (*unit.Unit, error)`

- `func (UnitService) GetPaginated(ctx context.Context, params *unit.FindParams) ([]*unit.Unit, error)`

- `func (UnitService) Update(ctx context.Context, id uint, data *unit.UpdateDTO) error`

---

## Package `orderservice` (modules/warehouse/services/orderservice)

### Types

#### OrderService

##### Methods

- `func (OrderService) Complete(ctx context.Context, id uint) (order.Order, error)`

- `func (OrderService) Count(ctx context.Context) (int64, error)`

- `func (OrderService) Create(ctx context.Context, data order.CreateDTO) error`

- `func (OrderService) Delete(ctx context.Context, id uint) (order.Order, error)`

- `func (OrderService) FindByPositionID(ctx context.Context, queryOpts *product.FindByPositionParams) ([]*product.Product, error)`

- `func (OrderService) GetAll(ctx context.Context) ([]order.Order, error)`

- `func (OrderService) GetByID(ctx context.Context, id uint) (order.Order, error)`

- `func (OrderService) GetPaginated(ctx context.Context, params *order.FindParams) ([]order.Order, error)`

- `func (OrderService) Update(ctx context.Context, id uint, data order.UpdateDTO) error`

---

## Package `positionservice` (modules/warehouse/services/positionservice)

### Types

#### InvalidCellError

```go
type InvalidCellError struct {
    Col string
    Row uint
}
```

##### Methods

- `func (InvalidCellError) Localize(l *i18n.Localizer) string`

#### PositionService

##### Methods

- `func (PositionService) Count(ctx context.Context) (int64, error)`

- `func (PositionService) Create(ctx context.Context, data *position.CreateDTO) (*position.Position, error)`

- `func (PositionService) Delete(ctx context.Context, id uint) (*position.Position, error)`

- `func (PositionService) GetAll(ctx context.Context) ([]*position.Position, error)`

- `func (PositionService) GetByID(ctx context.Context, id uint) (*position.Position, error)`

- `func (PositionService) GetByIDs(ctx context.Context, ids []uint) ([]*position.Position, error)`

- `func (PositionService) GetPaginated(ctx context.Context, params *position.FindParams) ([]*position.Position, error)`

- `func (PositionService) LoadFromFilePath(ctx context.Context, path string) error`

- `func (PositionService) Update(ctx context.Context, id uint, data *position.UpdateDTO) error`

- `func (PositionService) UpdateWithFile(ctx context.Context, fileID uint) error`

#### XlsRow

```go
type XlsRow struct {
    Title string
    Barcode string
    Unit string
    Quantity int
}
```

### Functions

---

## Package `productservice` (modules/warehouse/services/productservice)

### Types

#### DuplicateRfidError

```go
type DuplicateRfidError struct {
    Rfid string
}
```

##### Methods

- `func (DuplicateRfidError) Localize(l *i18n.Localizer) string`

#### ProductService

##### Methods

- `func (ProductService) BulkCreate(ctx context.Context, data []*product.CreateDTO) ([]*product.Product, error)`

- `func (ProductService) Count(ctx context.Context, params *product.CountParams) (int64, error)`

- `func (ProductService) CountInStock(ctx context.Context, params *product.CountParams) (int64, error)`

- `func (ProductService) Create(ctx context.Context, data *product.CreateDTO) error`

- `func (ProductService) CreateProductsFromTags(ctx context.Context, data *product.CreateProductsFromTagsDTO) ([]*product.Product, error)`

- `func (ProductService) Delete(ctx context.Context, id uint) (*product.Product, error)`

- `func (ProductService) GetAll(ctx context.Context) ([]*product.Product, error)`

- `func (ProductService) GetByID(ctx context.Context, id uint) (*product.Product, error)`

- `func (ProductService) GetPaginated(ctx context.Context, params *product.FindParams) ([]*product.Product, error)`

- `func (ProductService) Update(ctx context.Context, id uint, data *product.UpdateDTO) error`

- `func (ProductService) ValidateProducts(ctx context.Context, tags []string) ([]*product.Product, []*product.Product, error)`

---

## Package `website` (modules/website)

### Types

#### Module

##### Methods

- `func (Module) Name() string`

- `func (Module) Register(app application.Application) error`

### Functions

#### `func NewModule() application.Module`

### Variables and Constants

- Var: `[AIChatLink]`

- Var: `[LocaleFiles]`

- Var: `[MigrationFiles]`

- Var: `[NavItems]`

- Var: `[WebsiteLink]`

---

## Package `aichatconfig` (modules/website/domain/entities/aichatconfig)

### Types

#### AIConfig

##### Interface Methods

- `ID() uuid.UUID`
- `TenantID() uuid.UUID`
- `ModelName() string`
- `ModelType() AIModelType`
- `SystemPrompt() string`
- `Temperature() float32`
- `MaxTokens() int`
- `BaseURL() string`
- `AccessToken() string`
- `IsDefault() bool`
- `CreatedAt() time.Time`
- `UpdatedAt() time.Time`
- `SetSystemPrompt(prompt string) AIConfig`
- `WithTemperature(temp float32) (AIConfig, error)`
- `WithMaxTokens(tokens int) (AIConfig, error)`
- `WithModelName(modelName string) (AIConfig, error)`
- `WithBaseURL(baseURL string) (AIConfig, error)`
- `SetAccessToken(accessToken string) AIConfig`
- `WithIsDefault(isDefault bool) (AIConfig, error)`

#### AIModelType

#### Option

#### Repository

##### Interface Methods

- `GetByID(ctx context.Context, id uuid.UUID) (AIConfig, error)`
- `GetDefault(ctx context.Context) (AIConfig, error)`
- `Save(ctx context.Context, config AIConfig) (AIConfig, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`
- `List(ctx context.Context) ([]AIConfig, error)`
- `SetDefault(ctx context.Context, id uuid.UUID) error`

### Variables and Constants

- Var: `[ErrInvalidID ErrInvalidTemperature ErrEmptyModelName ErrEmptyBaseURL ErrConfigNotFound]`

---

## Package `chatthread` (modules/website/domain/entities/chatthread)

### Types

#### ChatThread

##### Interface Methods

- `ID() uuid.UUID`
- `Timestamp() time.Time`
- `ChatID() uint`
- `Messages() []chat.Message`

#### Option

#### Repository

##### Interface Methods

- `GetByID(ctx context.Context, id uuid.UUID) (ChatThread, error)`
- `Save(ctx context.Context, thread ChatThread) (ChatThread, error)`
- `Delete(ctx context.Context, id uuid.UUID) error`
- `List(ctx context.Context) ([]ChatThread, error)`

### Variables and Constants

- Var: `[ErrChatThreadNotFound]`

---

## Package `persistence` (modules/website/infrastructure/persistence)

### Types

#### AIChatConfigRepository

##### Methods

- `func (AIChatConfigRepository) Delete(ctx context.Context, id uuid.UUID) error`

- `func (AIChatConfigRepository) GetByID(ctx context.Context, id uuid.UUID) (aichatconfig.AIConfig, error)`

- `func (AIChatConfigRepository) GetDefault(ctx context.Context) (aichatconfig.AIConfig, error)`

- `func (AIChatConfigRepository) List(ctx context.Context) ([]aichatconfig.AIConfig, error)`

- `func (AIChatConfigRepository) Save(ctx context.Context, config aichatconfig.AIConfig) (aichatconfig.AIConfig, error)`

- `func (AIChatConfigRepository) SetDefault(ctx context.Context, id uuid.UUID) error`

### Functions

#### `func NewAIChatConfigRepository() aichatconfig.Repository`

#### `func ToDBConfig(config aichatconfig.AIConfig) models.AIChatConfig`

ToDBConfig maps a domain entity to a database model


#### `func ToDomainConfig(model models.AIChatConfig) (aichatconfig.AIConfig, error)`

ToDomainConfig maps a database model to a domain entity


### Variables and Constants

---

## Package `models` (modules/website/infrastructure/persistence/models)

### Types

#### AIChatConfig

```go
type AIChatConfig struct {
    ID string
    TenantID string
    ModelName string
    ModelType string
    SystemPrompt string
    Temperature float32
    MaxTokens int
    BaseURL string
    AccessToken string
    IsDefault bool
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## Package `rag` (modules/website/infrastructure/rag)

### Types

#### DifyConfig

```go
type DifyConfig struct {
    BaseURL string
    APIKey string
    DatasetID string
    RetrievalModel *RetrievalModel
}
```

#### DifyProvider

##### Methods

- `func (DifyProvider) SearchRelevantContext(ctx context.Context, query string) ([]string, error)`

#### DifyRequest

```go
type DifyRequest struct {
    Query string `json:"query"`
    RetrievalModel RetrievalModel `json:"retrieval_model"`
}
```

#### DifyResponse

```go
type DifyResponse struct {
    Query Query `json:"query"`
    Records []Record `json:"records"`
}
```

#### Document

```go
type Document struct {
    ID string `json:"id"`
    DataSourceType string `json:"data_source_type"`
    Name string `json:"name"`
}
```

#### Provider

##### Interface Methods

- `SearchRelevantContext(ctx context.Context, query string) ([]string, error)`

#### Query

```go
type Query struct {
    Content string `json:"content"`
}
```

#### Record

```go
type Record struct {
    Segment Segment `json:"segment"`
    Score float64 `json:"score"`
}
```

#### RerankingMode

```go
type RerankingMode struct {
    RerankingProviderName string `json:"reranking_provider_name"`
    RerankingModelName string `json:"reranking_model_name"`
}
```

#### RetrievalModel

```go
type RetrievalModel struct {
    SearchMethod SearchMethod `json:"search_method"`
    RerankingEnable bool `json:"reranking_enable"`
    RerankingMode *RerankingMode `json:"reranking_mode"`
    Weights *float64 `json:"weights"`
    TopK int `json:"top_k"`
    ScoreThresholdEnabled bool `json:"score_threshold_enabled"`
    ScoreThreshold *float64 `json:"score_threshold"`
}
```

#### SearchMethod

#### Segment

```go
type Segment struct {
    ID string `json:"id"`
    Position int `json:"position"`
    DocumentID string `json:"document_id"`
    Content string `json:"content"`
    Answer *string `json:"answer"`
    WordCount int `json:"word_count"`
    Tokens int `json:"tokens"`
    Keywords []string `json:"keywords"`
    Document Document `json:"document"`
}
```

---

## Package `controllers` (modules/website/presentation/controllers)

### Types

#### AIChatAPIController

##### Methods

- `func (AIChatAPIController) Key() string`

- `func (AIChatAPIController) Register(r *mux.Router)`

#### AIChatAPIControllerConfig

```go
type AIChatAPIControllerConfig struct {
    BasePath string
    App application.Application
}
```

#### AIChatController

##### Methods

- `func (AIChatController) Key() string`

- `func (AIChatController) Register(r *mux.Router)`

#### AIChatControllerConfig

```go
type AIChatControllerConfig struct {
    BasePath string
    App application.Application
}
```

### Functions

#### `func NewAIChatAPIController(cfg AIChatAPIControllerConfig) application.Controller`

#### `func NewAIChatController(cfg AIChatControllerConfig) application.Controller`

---

## Package `dtos` (modules/website/presentation/controllers/dtos)

### Types

#### AIConfigDTO

```go
type AIConfigDTO struct {
    ModelName string `validate:"required"`
    SystemPrompt string `validate:"omitempty"`
    Temperature float32 `validate:"omitempty,gte=0,lte=2"`
    MaxTokens int `validate:"omitempty,gt=0"`
    BaseURL string `validate:"required,url"`
    AccessToken string `validate:"omitempty"`
}
```

##### Methods

- `func (AIConfigDTO) Apply(cfg aichatconfig.AIConfig, tenantID uuid.UUID) (aichatconfig.AIConfig, error)`

- `func (AIConfigDTO) Ok(ctx context.Context) (map[string]string, bool)`

#### AIConfigRequest

AIConfigRequest represents a request to create or update an AI chat configuration


```go
type AIConfigRequest struct {
    ModelName string `json:"ModelName"`
    ModelType string `json:"ModelType"`
    SystemPrompt string `json:"SystemPrompt"`
    Temperature float32 `json:"Temperature"`
    MaxTokens int `json:"MaxTokens"`
}
```

#### AIConfigResponse

AIConfigResponse represents the response for an AI chat configuration


```go
type AIConfigResponse struct {
    ID string `json:"id"`
    ModelName string `json:"model_name"`
    ModelType string `json:"model_type"`
    SystemPrompt string `json:"system_prompt"`
    Temperature float32 `json:"temperature"`
    MaxTokens int `json:"max_tokens"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}
```

#### APIErrorResponse

APIErrorResponse represents standardized API error responses


```go
type APIErrorResponse struct {
    Message string `json:"message"`
    Code string `json:"code"`
}
```

#### ChatMessage

```go
type ChatMessage struct {
    Message string `json:"message"`
    Phone string `json:"phone,omitempty"`
}
```

#### ChatResponse

```go
type ChatResponse struct {
    ThreadID string `json:"thread_id"`
}
```

#### ThreadMessage

```go
type ThreadMessage struct {
    Role string `json:"role"`
    Message string `json:"message"`
    Timestamp string `json:"timestamp"`
}
```

#### ThreadMessagesResponse

```go
type ThreadMessagesResponse struct {
    Messages []ThreadMessage `json:"messages"`
}
```

### Variables and Constants

- Const: `[ErrorCodeInvalidPhoneFormat ErrorCodeUnknownCountryCode ErrorCodeInvalidRequest ErrorCodeInternalServer ErrorCodeNotFound ErrorCodeThreadNotFound]`
  Common error codes
  

---

## Package `mappers` (modules/website/presentation/mappers)

### Functions

#### `func AIConfigToViewModel(config aichatconfig.AIConfig) *viewmodels.AIConfig`

AIConfigToViewModel maps an AI chat configuration domain entity to a view model


---

## Package `aichat` (modules/website/presentation/templates/pages/aichat)

templ: version: v0.3.857


### Types

#### ConfigureProps

```go
type ConfigureProps struct {
    Config *viewmodels.AIConfig
    FormAction string
    BasePath string
    Errors map[string]string
    ModelOptions []string
}
```

#### ModelSelectProps

```go
type ModelSelectProps struct {
    ModelOptions []string
    SelectedModel string
    Error string
}
```

### Functions

#### `func Configure(props ConfigureProps) templ.Component`

#### `func ConfigureForm(props ConfigureProps) templ.Component`

#### `func ModelSelectOptions(props ModelSelectProps) templ.Component`

### Variables and Constants

---

## Package `viewmodels` (modules/website/presentation/viewmodels)

### Types

#### AIConfig

AIConfig represents the view model for the AI chat configuration


```go
type AIConfig struct {
    ID string
    ModelName string
    SystemPrompt string
    Temperature float32
    MaxTokens int
    BaseURL string
    CreatedAt string
    UpdatedAt string
    IsDefault bool
}
```

---

## Package `seed` (modules/website/seed)

### Functions

#### `func AIChatConfigSeedFunc(configs ...aichatconfig.AIConfig) application.SeedFunc`

---

## Package `services` (modules/website/services)

### Types

#### AIChatConfigService

##### Methods

- `func (AIChatConfigService) Delete(ctx context.Context, id uuid.UUID) error`

- `func (AIChatConfigService) GetByID(ctx context.Context, id uuid.UUID) (aichatconfig.AIConfig, error)`

- `func (AIChatConfigService) GetDefault(ctx context.Context) (aichatconfig.AIConfig, error)`

- `func (AIChatConfigService) List(ctx context.Context) ([]aichatconfig.AIConfig, error)`

- `func (AIChatConfigService) Save(ctx context.Context, config aichatconfig.AIConfig) (aichatconfig.AIConfig, error)`

- `func (AIChatConfigService) SetDefault(ctx context.Context, id uuid.UUID) error`

#### CreateThreadDTO

```go
type CreateThreadDTO struct {
    Phone string
    Country country.Country
}
```

#### ReplyToThreadDTO

```go
type ReplyToThreadDTO struct {
    ThreadID uuid.UUID
    UserID uint
    Message string
}
```

#### SendMessageToThreadDTO

```go
type SendMessageToThreadDTO struct {
    ThreadID uuid.UUID
    Message string
}
```

#### ThreadsMap

#### WebsiteChatService

##### Methods

- `func (WebsiteChatService) CreateThread(ctx context.Context, dto CreateThreadDTO) (chatthread.ChatThread, error)`

- `func (WebsiteChatService) GetAvailableModels(ctx context.Context) ([]string, error)`

- `func (WebsiteChatService) GetAvailableModelsWithConfig(ctx context.Context, baseURL, accessToken string) ([]string, error)`

- `func (WebsiteChatService) GetThreadByID(ctx context.Context, threadID uuid.UUID) (chatthread.ChatThread, error)`

- `func (WebsiteChatService) ReplyToThread(ctx context.Context, dto ReplyToThreadDTO) (chatthread.ChatThread, error)`

- `func (WebsiteChatService) ReplyWithAI(ctx context.Context, threadID uuid.UUID) (chatthread.ChatThread, error)`

- `func (WebsiteChatService) SendMessageToThread(ctx context.Context, dto SendMessageToThreadDTO) (chatthread.ChatThread, error)`

#### WebsiteChatServiceConfig

```go
type WebsiteChatServiceConfig struct {
    AIConfigRepo aichatconfig.Repository
    UserRepo user.Repository
    ClientRepo client.Repository
    ChatRepo chat.Repository
    AIUserEmail internet.Email
    RAGProvider rag.Provider
}
```

### Functions

### Variables and Constants

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
- `Websocket() Huber`
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

#### ApplicationOptions

```go
type ApplicationOptions struct {
    Pool *pgxpool.Pool
    EventBus eventbus.EventBus
    Logger *logrus.Logger
    Bundle *i18n.Bundle
    Huber Huber
}
```

#### Connection

##### Interface Methods

- `ws.Connectioner`
- `User() user.User`

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

#### Huber

##### Interface Methods

- `http.Handler`
- `ForEach(channel string, f WsCallback) error`

#### HuberOptions

```go
type HuberOptions struct {
    Pool *pgxpool.Pool
    Bundle *i18n.Bundle
    Logger *logrus.Logger
    CheckOrigin func(r *http.Request) bool
    UserRepository user.Repository
}
```

#### MetaInfo

```go
type MetaInfo struct {
    UserID uint
    TenantID uuid.UUID
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

#### WsCallback

### Functions

#### `func LoadBundle() *i18n.Bundle`

#### `func MustParseURL(rawURL string) *url.URL`

### Variables and Constants

- Var: `[ErrAppNotFound]`

- Const: `[ChannelAuthenticated]`

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

#### `func UseTenantID(ctx context.Context) (uuid.UUID, error)`

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


#### `func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context`

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

- Var: `[ErrNoTenantIDFound]`

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
    Domain string `env:"DOMAIN" envDefault:"localhost:3200"`
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

#### BoolField

##### Interface Methods

- `Field`
- `DefaultValue() bool`
- `TrueLabel() string`
- `FalseLabel() string`

#### Builder

##### Interface Methods

- `Schema() <?>`
- `Repository() <?>`
- `Service() <?>`

#### BuilderOption

#### CreatedEvent

```go
type CreatedEvent struct {
    Data TEntity
    Result TEntity
}
```

##### Methods

- `func (CreatedEvent) SetResult(result TEntity)`

#### DateField

##### Interface Methods

- `Field`
- `MinDate() time.Time`
- `MaxDate() time.Time`
- `Format() string`
- `WeekdaysOnly() bool`

#### DateTimeField

##### Interface Methods

- `Field`
- `MinDateTime() time.Time`
- `MaxDateTime() time.Time`
- `Format() string`
- `Timezone() string`
- `WeekdaysOnly() bool`

#### DecimalField

##### Interface Methods

- `Field`
- `Precision() int`
- `Scale() int`
- `Min() string`
- `Max() string`

#### DeletedEvent

```go
type DeletedEvent struct {
    Data TEntity
}
```

#### Event

##### Interface Methods

- `SetResult(result TEntity)`

#### Field

##### Interface Methods

- `Key() bool`
- `Name() string`
- `Type() FieldType`
- `Readonly() bool`
- `Searchable() bool`
- `Hidden() bool`
- `Rules() []FieldRule`
- `Attrs() map[string]any`
- `InitialValue() any`
- `Value(value any) FieldValue`
- `AsStringField() (StringField, error)`
- `AsIntField() (IntField, error)`
- `AsBoolField() (BoolField, error)`
- `AsFloatField() (FloatField, error)`
- `AsDecimalField() (DecimalField, error)`
- `AsDateField() (DateField, error)`
- `AsTimeField() (TimeField, error)`
- `AsDateTimeField() (DateTimeField, error)`
- `AsTimestampField() (TimestampField, error)`
- `AsUUIDField() (UUIDField, error)`

#### FieldOption

#### FieldRule

#### FieldType

#### FieldValue

##### Interface Methods

- `Field() Field`
- `Value() any`
- `IsZero() bool`
- `AsString() (string, error)`
- `AsInt() (int, error)`
- `AsInt32() (int32, error)`
- `AsInt64() (int64, error)`
- `AsBool() (bool, error)`
- `AsFloat32() (float32, error)`
- `AsFloat64() (float64, error)`
- `AsDecimal() (string, error)`
- `AsTime() (time.Time, error)`
- `AsUUID() (uuid.UUID, error)`

#### Fields

##### Interface Methods

- `Names() []string`
- `Fields() []Field`
- `Searchable() []Field`
- `KeyField() Field`
- `Field(name string) (Field, error)`
- `FieldValues(values map[string]any) ([]FieldValue, error)`

#### Filter

#### FindParams

```go
type FindParams struct {
    Query string
    Filters []Filter
    Limit int
    Offset int
    SortBy SortBy
}
```

#### FlatMapper

##### Interface Methods

- `<?>`
- `ToEntity(ctx context.Context, values []FieldValue) (TEntity, error)`
- `ToFieldValues(ctx context.Context, entity TEntity) ([]FieldValue, error)`

#### FloatField

##### Interface Methods

- `Field`
- `Min() float64`
- `Max() float64`
- `Precision() int`
- `Step() float64`

#### Hook

#### Hooks

##### Interface Methods

- `OnCreate() <?>`
- `OnUpdate() <?>`
- `OnDelete() <?>`

#### IntField

##### Interface Methods

- `Field`
- `Min() int64`
- `Max() int64`
- `Step() int64`
- `MultipleOf() int64`

#### Mapper

##### Interface Methods

- `ToEntities(ctx context.Context, values ...[]FieldValue) ([]TEntity, error)`
- `ToFieldValuesList(ctx context.Context, entities ...TEntity) ([][]FieldValue, error)`

#### Repository

##### Interface Methods

- `GetAll(ctx context.Context) ([]TEntity, error)`
- `Get(ctx context.Context, value FieldValue) (TEntity, error)`
- `Exists(ctx context.Context, value FieldValue) (bool, error)`
- `Count(ctx context.Context, filters *FindParams) (int64, error)`
- `List(ctx context.Context, params *FindParams) ([]TEntity, error)`
- `Create(ctx context.Context, values []FieldValue) (TEntity, error)`
- `Update(ctx context.Context, values []FieldValue) (TEntity, error)`
- `Delete(ctx context.Context, value FieldValue) (TEntity, error)`

#### Schema

##### Interface Methods

- `Name() string`
- `Fields() Fields`
- `Mapper() <?>`
- `Validators() []<?>`
- `Hooks() <?>`

#### SchemaOption

#### SelectField

SelectField interface extends Field with select-specific functionality


##### Interface Methods

- `Field`
- `SelectType() SelectType`
- `SetSelectType(SelectType) SelectField`
- `Options() []SelectOption`
- `SetOptions([]SelectOption) SelectField`
- `OptionsLoader() (func(ctx context.Context) []SelectOption)`
- `SetOptionsLoader(func(ctx context.Context) []SelectOption) SelectField`
- `Endpoint() string`
- `SetEndpoint(string) SelectField`
- `Placeholder() string`
- `SetPlaceholder(string) SelectField`
- `Multiple() bool`
- `SetMultiple(bool) SelectField`
- `ValueType() FieldType`
- `SetValueType(FieldType) SelectField`
- `AsIntSelect() SelectField`
- `AsStringSelect() SelectField`
- `AsBoolSelect() SelectField`
- `AsSearchable(endpoint string) SelectField`
- `AsCombobox() SelectField`
- `WithStaticOptions(options ...SelectOption) SelectField`
- `WithSearchEndpoint(endpoint string) SelectField`
- `WithCombobox(endpoint string, multiple bool) SelectField`

#### SelectOption

SelectOption represents a single option in a select field


```go
type SelectOption struct {
    Value any
    Label string
}
```

#### SelectType

SelectType defines how the select field behaves in the UI


#### Service

##### Interface Methods

- `GetAll(ctx context.Context) ([]TEntity, error)`
- `Get(ctx context.Context, value FieldValue) (TEntity, error)`
- `Exists(ctx context.Context, value FieldValue) (bool, error)`
- `Count(ctx context.Context, params *FindParams) (int64, error)`
- `List(ctx context.Context, params *FindParams) ([]TEntity, error)`
- `Save(ctx context.Context, entity TEntity) (TEntity, error)`
- `Delete(ctx context.Context, value FieldValue) (TEntity, error)`

#### SortBy

#### StringField

##### Interface Methods

- `Field`
- `MinLen() int`
- `MaxLen() int`
- `Multiline() bool`
- `Pattern() string`
- `Trim() bool`
- `Uppercase() bool`
- `Lowercase() bool`

#### TimeField

##### Interface Methods

- `Field`
- `Format() string`

#### TimestampField

##### Interface Methods

- `Field`

#### UUIDField

##### Interface Methods

- `Field`
- `Version() int`

#### UpdatedEvent

```go
type UpdatedEvent struct {
    Data TEntity
    Result TEntity
}
```

##### Methods

- `func (UpdatedEvent) SetResult(result TEntity)`

#### Validator

### Functions

### Variables and Constants

- Var: `[ErrEmptyResult]`

- Var: `[ErrFieldTypeMismatch]`

- Const: `[MinLen MaxLen Multiline Min Max Precision Scale MinDate MaxDate Pattern Trim Uppercase Lowercase Step MultipleOf Format Timezone WeekdaysOnly UUIDVersion DefaultValue TrueLabel FalseLabel]`

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

## Package `excel` (pkg/excel)

### Types

#### AlignmentStyle

AlignmentStyle defines alignment


```go
type AlignmentStyle struct {
    Horizontal string
    Vertical string
    WrapText bool
}
```

#### BorderStyle

BorderStyle defines border styling


```go
type BorderStyle struct {
    Type string
    Color string
}
```

#### CellStyle

CellStyle defines styling for cells


```go
type CellStyle struct {
    Font *FontStyle
    Fill *FillStyle
    Border *BorderStyle
    Alignment *AlignmentStyle
}
```

#### ColumnOptions

ColumnOptions defines column-specific options


```go
type ColumnOptions struct {
    Width float64
    Format string
    DataType string
}
```

#### DataSource

DataSource provides data for Excel export


##### Interface Methods

- `GetHeaders() []string`
- `GetRows(ctx context.Context) (func() ([]interface{}, error), error)`
- `GetSheetName() string`

#### ExcelExporter

ExcelExporter implements Exporter using excelize


##### Methods

- `func (ExcelExporter) Export(ctx context.Context, datasource DataSource) ([]byte, error)`
  Export exports data from the datasource to Excel format
  

#### ExportOptions

ExportOptions configures the Excel export behavior


```go
type ExportOptions struct {
    IncludeHeaders bool
    AutoFilter bool
    FreezeHeader bool
    DateFormat string
    TimeFormat string
    DateTimeFormat string
    MaxRows int
}
```

#### Exporter

Exporter exports data to Excel format


##### Interface Methods

- `Export(ctx context.Context, datasource DataSource) ([]byte, error)`

#### FillStyle

FillStyle defines fill styling


```go
type FillStyle struct {
    Type string
    Pattern int
    Color string
}
```

#### FontStyle

FontStyle defines font styling


```go
type FontStyle struct {
    Bold bool
    Italic bool
    Size int
    Color string
}
```

#### FunctionDataSource

FunctionDataSource wraps a Go function as a DataSource


##### Methods

- `func (FunctionDataSource) GetHeaders() []string`
  GetHeaders returns the column headers
  

- `func (FunctionDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error)`
  GetRows returns an iterator function for fetching rows
  

- `func (FunctionDataSource) GetSheetName() string`
  GetSheetName returns the sheet name
  

- `func (FunctionDataSource) WithSheetName(name string) *FunctionDataSource`
  WithSheetName sets a custom sheet name
  

#### PgxDataSource

PgxDataSource implements DataSource for pgx/pgxpool queries


##### Methods

- `func (PgxDataSource) GetHeaders() []string`
  GetHeaders returns column headers from the query
  

- `func (PgxDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error)`
  GetRows returns an iterator function for fetching rows
  

- `func (PgxDataSource) GetSheetName() string`
  GetSheetName returns the sheet name
  

- `func (PgxDataSource) WithSheetName(name string) *PgxDataSource`
  WithSheetName sets a custom sheet name
  

#### PostgresDataSource

PostgresDataSource implements DataSource for PostgreSQL queries


##### Methods

- `func (PostgresDataSource) GetHeaders() []string`
  GetHeaders returns column headers from the query
  

- `func (PostgresDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error)`
  GetRows returns an iterator function for fetching rows
  

- `func (PostgresDataSource) GetSheetName() string`
  GetSheetName returns the sheet name
  

- `func (PostgresDataSource) WithSheetName(name string) *PostgresDataSource`
  WithSheetName sets a custom sheet name
  

#### SliceDataSource

SliceDataSource wraps Go slices as a DataSource


##### Methods

- `func (SliceDataSource) GetHeaders() []string`
  GetHeaders returns the column headers
  

- `func (SliceDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error)`
  GetRows returns an iterator function for fetching rows
  

- `func (SliceDataSource) GetSheetName() string`
  GetSheetName returns the sheet name
  

- `func (SliceDataSource) WithSheetName(name string) *SliceDataSource`
  WithSheetName sets a custom sheet name
  

#### StyleOptions

StyleOptions defines styling for the Excel file


```go
type StyleOptions struct {
    HeaderStyle *CellStyle
    DataStyle *CellStyle
    AlternateRow bool
}
```

### Functions

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

## Package `lens` (pkg/lens)

### Types

#### ActionConfig

ActionConfig represents the action to take when an event occurs


```go
type ActionConfig struct {
    Type ActionType `json:"type"`
    Navigation *NavigationAction `json:"navigation,omitempty"`
    DrillDown *DrillDownAction `json:"drillDown,omitempty"`
    Modal *ModalAction `json:"modal,omitempty"`
    Custom *CustomAction `json:"custom,omitempty"`
}
```

#### ActionType

ActionType represents the type of action to perform


#### ChartType

ChartType represents the type of visualization


#### ClickEvent

ClickEvent represents a general chart click event configuration


```go
type ClickEvent struct {
    Action ActionConfig `json:"action"`
}
```

#### Column

Column represents a data column


```go
type Column struct {
    Name string
    Type string
}
```

#### CustomAction

CustomAction represents a custom JavaScript action


```go
type CustomAction struct {
    Function string `json:"function"`
    Variables map[string]string `json:"variables,omitempty"`
}
```

#### DashboardConfig

DashboardConfig represents the dashboard configuration


```go
type DashboardConfig struct {
    ID string `json:"id"`
    Name string `json:"name"`
    Description string `json:"description"`
    Version string `json:"version"`
    Grid GridConfig `json:"grid"`
    Panels []PanelConfig `json:"panels"`
    Variables []Variable `json:"variables"`
}
```

##### Methods

- `func (DashboardConfig) MarshalJSON() ([]byte, error)`
  MarshalJSON serializes a DashboardConfig to JSON bytes
  

- `func (DashboardConfig) ToJSON() (string, error)`
  ToJSON converts a DashboardConfig to JSON string
  

- `func (DashboardConfig) UnmarshalJSON(data []byte) error`
  UnmarshalJSON deserializes JSON bytes into a DashboardConfig
  

#### DataPointContext

DataPointContext represents the context of a clicked data point


```go
type DataPointContext struct {
    X interface{} `json:"x"`
    Y interface{} `json:"y"`
    SeriesIndex int `json:"seriesIndex"`
    DataIndex int `json:"dataIndex"`
    Label string `json:"label"`
    Value interface{} `json:"value"`
    Color string `json:"color,omitempty"`
}
```

#### DataPointEvent

DataPointEvent represents a data point click event configuration


```go
type DataPointEvent struct {
    Action ActionConfig `json:"action"`
}
```

#### DataResult

DataResult represents query execution results


```go
type DataResult struct {
    Columns []Column
    Rows [][]any
    Error error
}
```

#### DataSourceConfig

DataSourceConfig represents data source configuration


```go
type DataSourceConfig struct {
    Type string `json:"type"`
    Ref string `json:"ref"`
}
```

#### DefaultEventHandler

DefaultEventHandler is the default implementation of EventHandler


##### Methods

- `func (DefaultEventHandler) HandleEvent(ctx context.Context, eventCtx *EventContext, action ActionConfig) (*EventResult, error)`
  HandleEvent processes an event based on the action configuration
  

#### DrillDownAction

DrillDownAction represents a drill-down action (filter current dashboard)


```go
type DrillDownAction struct {
    Dashboard string `json:"dashboard,omitempty"`
    Filters map[string]string `json:"filters"`
    Variables map[string]string `json:"variables,omitempty"`
}
```

#### EvaluatedDashboard

EvaluatedDashboard represents an evaluated dashboard ready for rendering


```go
type EvaluatedDashboard struct {
    Config DashboardConfig
    Layout Layout
    Panels []EvaluatedPanel
    Errors []EvaluationError
}
```

#### EvaluatedPanel

EvaluatedPanel represents an evaluated panel


```go
type EvaluatedPanel struct {
    Config PanelConfig
    ResolvedQuery string
    DataSourceRef string
    RenderConfig RenderConfig
}
```

#### EvaluationContext

EvaluationContext provides context for evaluation


```go
type EvaluationContext struct {
    TimeRange TimeRange
    Variables map[string]any
    User UserContext
}
```

#### EvaluationError

EvaluationError represents an evaluation error


```go
type EvaluationError struct {
    PanelID string
    Message string
    Cause error
}
```

##### Methods

- `func (EvaluationError) Error() string`

#### Evaluator

Evaluator evaluates dashboard configurations


##### Interface Methods

- `Evaluate(config *DashboardConfig, ctx EvaluationContext) (*EvaluatedDashboard, error)`

#### EventContext

EventContext represents the context information passed to event handlers


```go
type EventContext struct {
    PanelID string `json:"panelId"`
    ChartType ChartType `json:"chartType"`
    DataPoint *DataPointContext `json:"dataPoint,omitempty"`
    SeriesIndex *int `json:"seriesIndex,omitempty"`
    DataIndex *int `json:"dataIndex,omitempty"`
    Label string `json:"label,omitempty"`
    Value interface{} `json:"value,omitempty"`
    SeriesName string `json:"seriesName,omitempty"`
    CategoryName string `json:"categoryName,omitempty"`
    Variables map[string]interface{} `json:"variables,omitempty"`
    CustomData map[string]interface{} `json:"customData,omitempty"`
}
```

#### EventError

EventError represents an event handling error


```go
type EventError struct {
    Code string `json:"code"`
    Message string `json:"message"`
}
```

#### EventHandler

EventHandler interface for handling chart events


##### Interface Methods

- `HandleEvent(ctx context.Context, eventCtx *EventContext, action ActionConfig) (*EventResult, error)`

#### EventResult

EventResult represents the result of an event handling operation


```go
type EventResult struct {
    Type EventResultType `json:"type"`
    Data interface{} `json:"data,omitempty"`
    Redirect *RedirectResult `json:"redirect,omitempty"`
    Modal *ModalResult `json:"modal,omitempty"`
    Update *UpdateResult `json:"update,omitempty"`
    Error *EventError `json:"error,omitempty"`
}
```

#### EventResultType

EventResultType represents the type of event result


#### GridCSS

GridCSS represents grid-specific CSS (UI concern)


```go
type GridCSS struct {
    Classes []string
    Styles map[string]string
}
```

#### GridConfig

GridConfig represents the grid system configuration


```go
type GridConfig struct {
    Columns int `json:"columns"`
    RowHeight int `json:"rowHeight"`
    Breakpoints map[string]int `json:"breakpoints"`
}
```

#### GridDimensions

GridDimensions represents a panel's size in the grid


```go
type GridDimensions struct {
    Width int `json:"width"`
    Height int `json:"height"`
}
```

#### GridError

GridError represents a grid layout error


```go
type GridError struct {
    Message string
    Panels []string
}
```

##### Methods

- `func (GridError) Error() string`

#### GridPosition

GridPosition represents a panel's position in the grid


```go
type GridPosition struct {
    X int `json:"x"`
    Y int `json:"y"`
}
```

#### Layout

Layout represents the calculated dashboard layout (core business logic only)


```go
type Layout struct {
    Grid GridConfig
    Panels []PanelLayout
    Breakpoint string
}
```

#### LayoutEngine

LayoutEngine calculates panel layouts and handles grid positioning (business logic only)


##### Interface Methods

- `CalculateLayout(panels []PanelConfig, grid GridConfig) (Layout, error)`
- `DetectOverlaps(panels []PanelConfig) []GridError`
- `GetResponsiveLayout(layout Layout, breakpoint string) Layout`
- `ValidateLayout(panels []PanelConfig, grid GridConfig) []GridError`

#### LegendEvent

LegendEvent represents a legend click event configuration


```go
type LegendEvent struct {
    Action ActionConfig `json:"action"`
}
```

#### MarkerEvent

MarkerEvent represents a marker click event configuration


```go
type MarkerEvent struct {
    Action ActionConfig `json:"action"`
}
```

#### MetricValue

MetricValue represents a single metric value with metadata


```go
type MetricValue struct {
    Label string `json:"label"`
    Value float64 `json:"value"`
    FormattedValue string `json:"formattedValue,omitempty"`
    Unit string `json:"unit,omitempty"`
    Trend *Trend `json:"trend,omitempty"`
    Color string `json:"color,omitempty"`
    Icon string `json:"icon,omitempty"`
}
```

#### ModalAction

ModalAction represents a modal popup action


```go
type ModalAction struct {
    Title string `json:"title"`
    Content string `json:"content,omitempty"`
    URL string `json:"url,omitempty"`
    Variables map[string]string `json:"variables,omitempty"`
}
```

#### ModalResult

ModalResult represents a modal response


```go
type ModalResult struct {
    Title string `json:"title"`
    Content string `json:"content"`
    URL string `json:"url,omitempty"`
}
```

#### NavigationAction

NavigationAction represents a navigation action (redirect to URL)


```go
type NavigationAction struct {
    URL string `json:"url"`
    Target string `json:"target,omitempty"`
    Variables map[string]string `json:"variables,omitempty"`
}
```

#### PanelConfig

PanelConfig represents a single panel configuration


```go
type PanelConfig struct {
    ID string `json:"id"`
    Title string `json:"title"`
    Type ChartType `json:"type"`
    Position GridPosition `json:"position"`
    Dimensions GridDimensions `json:"dimensions"`
    DataSource DataSourceConfig `json:"dataSource"`
    Query string `json:"query"`
    Options map[string]any `json:"options"`
    Events *PanelEvents `json:"events,omitempty"`
}
```

#### PanelEvents

PanelEvents represents event configuration for a panel


```go
type PanelEvents struct {
    Click *ClickEvent `json:"click,omitempty"`
    DataPoint *DataPointEvent `json:"dataPoint,omitempty"`
    Legend *LegendEvent `json:"legend,omitempty"`
    Marker *MarkerEvent `json:"marker,omitempty"`
    XAxisLabel *XAxisLabelEvent `json:"xAxisLabel,omitempty"`
}
```

#### PanelLayout

PanelLayout represents a panel's layout information (core business logic only)


```go
type PanelLayout struct {
    PanelID string
    Position GridPosition
    Dimensions GridDimensions
}
```

#### RedirectResult

RedirectResult represents a redirect response


```go
type RedirectResult struct {
    URL string `json:"url"`
    Target string `json:"target,omitempty"`
}
```

#### RenderConfig

RenderConfig contains rendering configuration


```go
type RenderConfig struct {
    ChartType ChartType
    ChartOptions map[string]any
    GridCSS GridCSS
    RefreshRate int
}
```

#### TimeRange

TimeRange represents a time period for data queries


```go
type TimeRange struct {
    Start time.Time
    End time.Time
}
```

#### Trend

Trend represents trend information for a metric


```go
type Trend struct {
    Direction string `json:"direction"`
    Percentage float64 `json:"percentage"`
    IsPositive bool `json:"isPositive"`
}
```

#### UpdateResult

UpdateResult represents a dashboard update response


```go
type UpdateResult struct {
    PanelID string `json:"panelId,omitempty"`
    Variables map[string]interface{} `json:"variables,omitempty"`
    Filters map[string]interface{} `json:"filters,omitempty"`
}
```

#### UserContext

UserContext represents user information for permissions


```go
type UserContext struct {
    ID string
    Roles []string
}
```

#### ValidationError

ValidationError represents a validation error


```go
type ValidationError struct {
    Field string
    Message string
}
```

##### Methods

- `func (ValidationError) Error() string`

#### ValidationResult

ValidationResult contains validation results


```go
type ValidationResult struct {
    Valid bool
    Errors []ValidationError
}
```

##### Methods

- `func (ValidationResult) IsValid() bool`
  IsValid returns true if there are no validation errors
  

#### Validator

Validator validates dashboard configurations


##### Interface Methods

- `Validate(config *DashboardConfig) ValidationResult`
- `ValidatePanel(panel *PanelConfig, grid GridConfig) ValidationResult`
- `ValidateGrid(panels []PanelConfig, grid GridConfig) ValidationResult`

#### Variable

Variable represents a dashboard variable for templating


```go
type Variable struct {
    Name string `json:"name"`
    Type string `json:"type"`
    Default any `json:"default"`
    Value any `json:"value,omitempty"`
}
```

#### XAxisLabelEvent

XAxisLabelEvent represents an X-axis label click event configuration


```go
type XAxisLabelEvent struct {
    Action ActionConfig `json:"action"`
}
```

---

## Package `builder` (pkg/lens/builder)

### Types

#### DashboardBuilder

DashboardBuilder provides a fluent API for building dashboards


##### Interface Methods

- `ID(id string) DashboardBuilder`
- `Title(title string) DashboardBuilder`
- `Description(desc string) DashboardBuilder`
- `Grid(columns int, rowHeight int) DashboardBuilder`
- `RefreshRate(rate time.Duration) DashboardBuilder`
- `Variable(name string, value interface{}) DashboardBuilder`
- `Panel(panel lens.PanelConfig) DashboardBuilder`
- `Build() lens.DashboardConfig`

#### PanelBuilder

PanelBuilder provides a fluent API for building panels


##### Interface Methods

- `ID(id string) PanelBuilder`
- `Title(title string) PanelBuilder`
- `Type(chartType lens.ChartType) PanelBuilder`
- `Position(x, y int) PanelBuilder`
- `Size(width, height int) PanelBuilder`
- `DataSource(dsID string) PanelBuilder`
- `Query(query string) PanelBuilder`
- `RefreshRate(rate time.Duration) PanelBuilder`
- `Option(key string, value interface{}) PanelBuilder`
- `OnClick(action lens.ActionConfig) PanelBuilder`
- `OnDataPointClick(action lens.ActionConfig) PanelBuilder`
- `OnLegendClick(action lens.ActionConfig) PanelBuilder`
- `OnMarkerClick(action lens.ActionConfig) PanelBuilder`
- `OnXAxisLabelClick(action lens.ActionConfig) PanelBuilder`
- `OnNavigate(url string, target ...string) PanelBuilder`
- `OnDrillDown(filters map[string]string, dashboard ...string) PanelBuilder`
- `OnModal(title, content string, url ...string) PanelBuilder`
- `OnCustom(function string, variables ...map[string]string) PanelBuilder`
- `Build() lens.PanelConfig`

### Functions

#### `func ExampleDashboard() lens.DashboardConfig`

Example usage helper that demonstrates the builder pattern


#### `func FullWidthPanel(id, title string, chartType lens.ChartType, y, height int) lens.PanelConfig`

FullWidthPanel creates a panel that spans the full width


#### `func HalfWidthPanel(id, title string, chartType lens.ChartType, x, y, height int) lens.PanelConfig`

HalfWidthPanel creates a panel that spans half the width


#### `func QuarterWidthPanel(id, title string, chartType lens.ChartType, x, y, height int) lens.PanelConfig`

QuarterWidthPanel creates a panel that spans a quarter of the width


#### `func QuickPanel(id, title string, chartType lens.ChartType, x, y, width, height int) lens.PanelConfig`

QuickPanel creates a panel with basic configuration


#### `func ValidateDashboard(dashboard lens.DashboardConfig) error`

ValidateDashboard validates a dashboard configuration


#### `func ValidatePanel(panel lens.PanelConfig) error`

ValidatePanel validates a panel configuration


---

## Package `cache` (pkg/lens/cache)

### Types

#### Cache

Cache provides caching functionality for query results


##### Interface Methods

- `Get(ctx context.Context, key string) (*executor.ExecutionResult, bool)`
- `Set(ctx context.Context, key string, result *executor.ExecutionResult, ttl time.Duration) error`
- `Delete(ctx context.Context, key string) error`
- `Clear(ctx context.Context) error`
- `Stats() CacheStats`

#### CacheStats

CacheStats provides statistics about cache usage


```go
type CacheStats struct {
    Hits int64
    Misses int64
    Entries int
    HitRate float64
    LastCleanup time.Time
}
```

#### CachingExecutor

CachingExecutor wraps an executor with caching functionality


##### Methods

- `func (CachingExecutor) Close() error`
  Close closes the caching executor
  

- `func (CachingExecutor) Execute(ctx context.Context, query executor.ExecutionQuery) (*executor.ExecutionResult, error)`
  Execute executes a query with caching
  

- `func (CachingExecutor) ExecuteDashboard(ctx context.Context, dashboard lens.DashboardConfig) (*executor.DashboardResult, error)`
  ExecuteDashboard executes dashboard queries with caching
  

- `func (CachingExecutor) ExecutePanel(ctx context.Context, panel lens.PanelConfig, variables map[string]interface{}) (*executor.ExecutionResult, error)`
  ExecutePanel executes a panel query with caching
  

- `func (CachingExecutor) RegisterDataSource(id string, ds datasource.DataSource) error`
  RegisterDataSource registers a data source
  

#### MemoryCache

MemoryCache is an in-memory cache implementation


##### Methods

- `func (MemoryCache) Clear(ctx context.Context) error`
  Clear removes all cached results
  

- `func (MemoryCache) Close() error`
  Close stops the cache cleanup routine
  

- `func (MemoryCache) Delete(ctx context.Context, key string) error`
  Delete removes a cached result
  

- `func (MemoryCache) Get(ctx context.Context, key string) (*executor.ExecutionResult, bool)`
  Get retrieves a cached result by key
  

- `func (MemoryCache) Set(ctx context.Context, key string, result *executor.ExecutionResult, ttl time.Duration) error`
  Set stores a result in the cache with TTL
  

- `func (MemoryCache) Stats() CacheStats`
  Stats returns cache statistics
  

### Functions

#### `func Clear(ctx context.Context) error`

Clear removes all results from the default cache


#### `func Delete(ctx context.Context, key string) error`

Delete removes a result from the default cache


#### `func Get(ctx context.Context, key string) (*executor.ExecutionResult, bool)`

Get retrieves a result from the default cache


#### `func Set(ctx context.Context, key string, result *executor.ExecutionResult, ttl time.Duration) error`

Set stores a result in the default cache


---

## Package `datasource` (pkg/lens/datasource)

### Types

#### Capability

Capability represents what a data source can do


#### ColumnInfo

ColumnInfo describes a column in table format results


```go
type ColumnInfo struct {
    Name string
    Type DataType
    Unit string
}
```

#### DataPoint

DataPoint represents a single data point


```go
type DataPoint struct {
    Timestamp time.Time
    Value interface{}
    Labels map[string]string
    Fields map[string]interface{}
}
```

#### DataSource

DataSource defines the interface for data sources


##### Interface Methods

- `Query(ctx context.Context, query Query) (*QueryResult, error)`
- `TestConnection(ctx context.Context) error`
- `GetMetadata() DataSourceMetadata`
- `ValidateQuery(query Query) error`
- `Close() error`

#### DataSourceConfig

DataSourceConfig represents configuration for creating a data source


```go
type DataSourceConfig struct {
    Type DataSourceType
    Name string
    URL string
    Timeout time.Duration
    MaxRetries int
    Options map[string]interface{}
}
```

#### DataSourceMetadata

DataSourceMetadata contains information about a data source


```go
type DataSourceMetadata struct {
    ID string
    Name string
    Type DataSourceType
    Version string
    Description string
    Capabilities []Capability
    Config map[string]string
}
```

#### DataSourceType

DataSourceType represents different types of data sources


#### DataType

DataType represents the type of data in a column


#### ErrorCode

ErrorCode represents different types of query errors


#### Factory

Factory creates data sources from configuration


##### Interface Methods

- `Create(config DataSourceConfig) (DataSource, error)`
- `SupportedTypes() []DataSourceType`
- `ValidateConfig(config DataSourceConfig) error`

#### Query

Query represents a query to be executed


```go
type Query struct {
    ID string
    Raw string
    Variables map[string]interface{}
    TimeRange lens.TimeRange
    RefreshRate time.Duration
    MaxDataPoints int
    Format QueryFormat
}
```

#### QueryBuilder

QueryBuilder provides a fluent interface for building queries


##### Interface Methods

- `WithQuery(query string) QueryBuilder`
- `WithVariable(key string, value interface{}) QueryBuilder`
- `WithTimeRange(start, end time.Time) QueryBuilder`
- `WithRefreshRate(rate time.Duration) QueryBuilder`
- `WithMaxDataPoints(maxPoints int) QueryBuilder`
- `WithFormat(format QueryFormat) QueryBuilder`
- `Build() Query`

#### QueryError

QueryError represents an error that occurred during query execution


```go
type QueryError struct {
    Code ErrorCode
    Message string
    Details string
    Query string
}
```

##### Methods

- `func (QueryError) Error() string`

#### QueryFormat

QueryFormat represents the expected query result format


#### QueryResult

QueryResult represents the result of a query execution


```go
type QueryResult struct {
    Data []DataPoint
    Metadata ResultMetadata
    Error *QueryError
    Columns []ColumnInfo
    ExecTime time.Duration
}
```

#### Registry

Registry manages data source instances


##### Interface Methods

- `Register(id string, dataSource DataSource) error`
- `Unregister(id string) error`
- `Get(id string) (DataSource, error)`
- `List() []string`
- `ListByType(dsType DataSourceType) []string`
- `CreateFromConfig(config DataSourceConfig) (DataSource, error)`

#### ResultMetadata

ResultMetadata contains metadata about query execution


```go
type ResultMetadata struct {
    QueryID string
    ExecutedAt time.Time
    RowCount int
    DataSource string
    ProcessingTime time.Duration
}
```

### Functions

#### `func List() []string`

List returns all registered data source IDs from the default registry


#### `func Register(id string, dataSource DataSource) error`

Register adds a data source to the default registry


#### `func Unregister(id string) error`

Unregister removes a data source from the default registry


### Variables and Constants

- Var: `[DefaultRegistry]`
  DefaultRegistry is a global registry instance
  

---

## Package `postgres` (pkg/lens/datasource/postgres)

### Types

#### Config

Config contains PostgreSQL-specific configuration


```go
type Config struct {
    ConnectionString string
    MaxConnections int32
    MinConnections int32
    MaxConnLifetime time.Duration
    MaxConnIdleTime time.Duration
    QueryTimeout time.Duration
}
```

#### Factory

Factory creates PostgreSQL data sources


##### Methods

- `func (Factory) Create(config datasource.DataSourceConfig) (datasource.DataSource, error)`
  Create creates a PostgreSQL data source from configuration
  

- `func (Factory) SupportedTypes() []datasource.DataSourceType`
  SupportedTypes returns the data source types this factory supports
  

- `func (Factory) ValidateConfig(config datasource.DataSourceConfig) error`
  ValidateConfig validates a PostgreSQL data source configuration
  

#### PostgreSQLDataSource

PostgreSQLDataSource implements DataSource for PostgreSQL databases


##### Methods

- `func (PostgreSQLDataSource) Close() error`
  Close closes the datasource connection
  

- `func (PostgreSQLDataSource) GetMetadata() datasource.DataSourceMetadata`
  GetMetadata returns datasource metadata
  

- `func (PostgreSQLDataSource) Query(ctx context.Context, query datasource.Query) (*datasource.QueryResult, error)`
  Query executes a query and returns the result
  

- `func (PostgreSQLDataSource) TestConnection(ctx context.Context) error`
  TestConnection tests if the datasource is reachable
  

- `func (PostgreSQLDataSource) ValidateQuery(query datasource.Query) error`
  ValidateQuery validates a query before execution
  

---

## Package `evaluation` (pkg/lens/evaluation)

### Types

#### Breakpoint

Breakpoint represents responsive breakpoints


#### EvaluatedDashboard

EvaluatedDashboard represents a dashboard ready for rendering


```go
type EvaluatedDashboard struct {
    Config lens.DashboardConfig
    Layout Layout
    Panels []EvaluatedPanel
    Errors []EvaluationError
    EvaluatedAt time.Time
    Context *EvaluationContext
}
```

##### Methods

- `func (EvaluatedDashboard) GetAllErrors() []EvaluationError`
  GetAllErrors returns all errors from the dashboard and panels
  

- `func (EvaluatedDashboard) GetPanelByID(id string) (*EvaluatedPanel, bool)`
  GetPanelByID returns a panel by its ID
  

- `func (EvaluatedDashboard) HasErrors() bool`
  HasErrors returns true if the evaluated dashboard has any errors
  

- `func (EvaluatedDashboard) IsValid() bool`
  IsValid returns true if the evaluation completed without errors
  

#### EvaluatedPanel

EvaluatedPanel represents an evaluated panel ready for rendering


```go
type EvaluatedPanel struct {
    Config lens.PanelConfig
    ResolvedQuery string
    DataSourceRef string
    RenderConfig RenderConfig
    Variables map[string]any
    Errors []EvaluationError
}
```

#### EvaluationContext

EvaluationContext provides context for dashboard evaluation


```go
type EvaluationContext struct {
    TimeRange lens.TimeRange
    Variables map[string]any
    User UserContext
    Options EvaluationOptions
}
```

##### Methods

- `func (EvaluationContext) Clone() *EvaluationContext`
  Clone creates a copy of the evaluation context
  

- `func (EvaluationContext) GetVariable(name string) (any, bool)`
  GetVariable returns a variable value by name
  

- `func (EvaluationContext) HasPermission(permission string) bool`
  HasPermission checks if the user has a specific permission
  

- `func (EvaluationContext) HasRole(role string) bool`
  HasRole checks if the user has a specific role
  

- `func (EvaluationContext) SetVariable(name string, value any)`
  SetVariable sets a variable value
  

- `func (EvaluationContext) WithOptions(options EvaluationOptions) *EvaluationContext`
  WithOptions sets the evaluation options
  

- `func (EvaluationContext) WithUser(user UserContext) *EvaluationContext`
  WithUser sets the user context
  

#### EvaluationError

EvaluationError represents an error that occurred during evaluation


```go
type EvaluationError struct {
    PanelID string
    Phase EvaluationPhase
    Message string
    Cause error
    Timestamp time.Time
}
```

##### Methods

- `func (EvaluationError) Error() string`

#### EvaluationOptions

EvaluationOptions contains options for evaluation


```go
type EvaluationOptions struct {
    InterpolateVariables bool
    ValidateQueries bool
    CalculateLayout bool
    EnableCaching bool
    CacheTTL time.Duration
}
```

#### EvaluationPhase

EvaluationPhase represents the phase where an error occurred


#### Evaluator

Evaluator evaluates dashboard configurations into renderable structures


##### Interface Methods

- `Evaluate(config *lens.DashboardConfig, ctx *EvaluationContext) (*EvaluatedDashboard, error)`
- `EvaluatePanel(panel *lens.PanelConfig, ctx *EvaluationContext) (*EvaluatedPanel, error)`

#### GridCSS

GridCSS represents grid-specific CSS configuration


```go
type GridCSS struct {
    Classes []string
    Styles map[string]string
    GridArea string
    Position Position
}
```

#### GridTemplate

GridTemplate represents CSS Grid template configuration


```go
type GridTemplate struct {
    Columns string
    Rows string
    Areas []string
}
```

#### HTMXConfig

HTMXConfig contains HTMX-specific configuration for panels


```go
type HTMXConfig struct {
    Trigger string
    Target string
    Swap string
    PushURL bool
    Headers map[string]string
    Indicators []string
}
```

#### Layout

Layout represents the calculated dashboard layout


```go
type Layout struct {
    Grid lens.GridConfig
    Panels []PanelLayout
    Breakpoint Breakpoint
    CSS LayoutCSS
}
```

#### LayoutCSS

LayoutCSS represents layout-level CSS


```go
type LayoutCSS struct {
    ContainerClasses []string
    ContainerStyles map[string]string
    GridTemplate GridTemplate
}
```

#### LayoutEngine

LayoutEngine calculates panel layouts


##### Interface Methods

- `CalculateLayout(panels []lens.PanelConfig, grid lens.GridConfig) (*Layout, error)`

#### PanelCSS

PanelCSS represents panel-specific CSS configuration


```go
type PanelCSS struct {
    Classes []string
    Styles map[string]string
    GridArea string
    ResponsiveCSS map[Breakpoint]ResponsiveCSS
}
```

#### PanelLayout

PanelLayout represents a panel's calculated layout


```go
type PanelLayout struct {
    PanelID string
    Position lens.GridPosition
    Dimensions lens.GridDimensions
    CSS PanelCSS
    ZIndex int
}
```

#### Position

Position represents CSS position information


```go
type Position struct {
    Top string
    Left string
    Right string
    Bottom string
}
```

#### QueryProcessor

QueryProcessor handles query variable interpolation


##### Interface Methods

- `InterpolateQuery(query string, ctx *EvaluationContext) (string, error)`
- `ValidateQuery(query string, dataSourceType string) error`

#### RenderConfig

RenderConfig contains everything needed for rendering a panel


```go
type RenderConfig struct {
    ChartType lens.ChartType
    ChartOptions map[string]any
    GridCSS GridCSS
    RefreshRate time.Duration
    DataEndpoint string
    HTMXConfig HTMXConfig
}
```

#### RenderMapper

RenderMapper maps panels to render configurations


##### Interface Methods

- `MapToRenderConfig(panel *lens.PanelConfig, ctx *EvaluationContext) (*RenderConfig, error)`

#### ResponsiveCSS

ResponsiveCSS represents responsive CSS for different breakpoints


```go
type ResponsiveCSS struct {
    Classes []string
    Styles map[string]string
}
```

#### UserContext

UserContext represents user information for permission-based filtering


```go
type UserContext struct {
    ID string
    Username string
    Roles []string
    Permissions []string
    Tenant string
}
```

---

## Package `executor` (pkg/lens/executor)

### Types

#### DashboardResult

DashboardResult represents the result of executing all dashboard queries


```go
type DashboardResult struct {
    PanelResults map[string]*ExecutionResult
    Errors []error
    ExecutedAt time.Time
    Duration time.Duration
}
```

#### ExecutionMetadata

ExecutionMetadata contains metadata about query execution


```go
type ExecutionMetadata struct {
    QueryID string
    DataSourceID string
    ExecutedAt time.Time
    RowCount int
    ProcessingTime time.Duration
    QueryHash string
}
```

#### ExecutionQuery

ExecutionQuery represents a query to be executed


```go
type ExecutionQuery struct {
    DataSourceID string
    Query string
    Variables map[string]interface{}
    TimeRange lens.TimeRange
    MaxRows int
    Timeout time.Duration
    Format datasource.QueryFormat
}
```

#### ExecutionResult

ExecutionResult represents the result of query execution


```go
type ExecutionResult struct {
    Data []datasource.DataPoint
    Metadata ExecutionMetadata
    Error error
    Columns []datasource.ColumnInfo
    ExecTime time.Duration
    CacheHit bool
}
```

#### Executor

Executor handles query execution across multiple data sources


##### Interface Methods

- `Execute(ctx context.Context, query ExecutionQuery) (*ExecutionResult, error)`
- `ExecutePanel(ctx context.Context, panel lens.PanelConfig, variables map[string]interface{}) (*ExecutionResult, error)`
- `ExecuteDashboard(ctx context.Context, dashboard lens.DashboardConfig) (*DashboardResult, error)`
- `RegisterDataSource(id string, ds datasource.DataSource) error`
- `Close() error`

### Functions

---

## Package `layout` (pkg/lens/layout)

### Types

#### Breakpoint

Breakpoint represents responsive breakpoints


#### BreakpointConfig

BreakpointConfig represents breakpoint configuration


```go
type BreakpointConfig struct {
    Name Breakpoint
    MinWidth int
    MaxWidth int
    Columns int
    RowHeight int
}
```

#### ConflictReport

ConflictReport provides detailed conflict analysis


```go
type ConflictReport struct {
    TotalOverlaps int
    CriticalOverlaps int
    ModerateOverlaps int
    MinorOverlaps int
    AffectedPanels []string
    OverlapDetails []OverlapError
    GridUtilization float64
}
```

#### Engine

Engine calculates panel layouts and handles grid positioning


##### Interface Methods

- `CalculateLayout(panels []lens.PanelConfig, grid lens.GridConfig) (*Layout, error)`
- `DetectOverlaps(panels []lens.PanelConfig) []OverlapError`
- `GetResponsiveLayout(layout *Layout, breakpoint Breakpoint) *Layout`
- `ValidateLayout(panels []lens.PanelConfig, grid lens.GridConfig) []ValidationError`

#### GridTemplate

GridTemplate represents CSS Grid template configuration


```go
type GridTemplate struct {
    Columns string
    Rows string
    Areas []string
}
```

#### Layout

Layout represents the calculated dashboard layout


```go
type Layout struct {
    Grid lens.GridConfig
    Panels []PanelLayout
    Breakpoint Breakpoint
    CSS LayoutCSS
    Bounds LayoutBounds
}
```

#### LayoutBounds

LayoutBounds represents the overall layout boundaries


```go
type LayoutBounds struct {
    MaxX int
    MaxY int
    MinWidth int
    MinHeight int
}
```

#### LayoutCSS

LayoutCSS represents layout-level CSS


```go
type LayoutCSS struct {
    ContainerClasses []string
    ContainerStyles map[string]string
    GridTemplate GridTemplate
}
```

#### OverlapDetector

OverlapDetector detects and resolves panel overlaps


##### Interface Methods

- `DetectOverlaps(panels []lens.PanelConfig) []OverlapError`
- `DetectAllConflicts(panels []lens.PanelConfig) []ConflictReport`
- `SuggestResolution(panels []lens.PanelConfig, conflicts []OverlapError) []ResolutionSuggestion`
- `HasOverlaps(panels []lens.PanelConfig) bool`

#### OverlapError

OverlapError represents an overlap between two panels


```go
type OverlapError struct {
    Panel1 string
    Panel2 string
    Overlap OverlapRegion
    Severity OverlapSeverity
    Message string
}
```

##### Methods

- `func (OverlapError) Error() string`

#### OverlapRegion

OverlapRegion represents the overlapping area


```go
type OverlapRegion struct {
    X int
    Y int
    Width int
    Height int
}
```

#### OverlapSeverity

OverlapSeverity represents how severe the overlap is


#### PanelBounds

PanelBounds represents a panel's boundaries


```go
type PanelBounds struct {
    Left int
    Top int
    Right int
    Bottom int
}
```

#### PanelCSS

PanelCSS represents panel-specific CSS


```go
type PanelCSS struct {
    Classes []string
    Styles map[string]string
    GridArea string
    ResponsiveCSS map[Breakpoint]ResponsiveCSS
}
```

#### PanelLayout

PanelLayout represents a panel's calculated layout


```go
type PanelLayout struct {
    PanelID string
    Position lens.GridPosition
    Dimensions lens.GridDimensions
    CSS PanelCSS
    ZIndex int
    Bounds PanelBounds
}
```

#### ResolutionSuggestion

ResolutionSuggestion suggests how to resolve overlaps


```go
type ResolutionSuggestion struct {
    Type ResolutionType
    PanelID string
    NewPosition lens.GridPosition
    NewSize lens.GridDimensions
    Reason string
    Priority int
}
```

#### ResolutionType

ResolutionType represents the type of resolution


#### ResponsiveCSS

ResponsiveCSS represents responsive CSS for different breakpoints


```go
type ResponsiveCSS struct {
    Classes []string
    Styles map[string]string
}
```

#### ResponsiveEngine

ResponsiveEngine handles responsive layout adjustments


##### Interface Methods

- `AdjustLayout(layout *Layout, breakpoint Breakpoint) *Layout`
- `GetBreakpointConfig(breakpoint Breakpoint) BreakpointConfig`
- `CalculateResponsiveDimensions(panel lens.PanelConfig, breakpoint Breakpoint) lens.GridDimensions`
- `GetBreakpointFromWidth(width int) Breakpoint`

#### ValidationError

ValidationError represents a layout validation error


```go
type ValidationError struct {
    PanelID string
    Message string
}
```

##### Methods

- `func (ValidationError) Error() string`

### Functions

---

## Package `ui` (pkg/lens/ui)

templ: version: v0.3.857


### Types

#### Config

Config contains UI rendering configuration


```go
type Config struct {
    GridClasses GridClassConfig
    RefreshStrategy RefreshStrategy
}
```

#### GridClassConfig

GridClassConfig contains grid CSS class configuration


```go
type GridClassConfig struct {
    ContainerClass string
    PanelClass string
}
```

#### LayoutCSS

LayoutCSS represents layout-level CSS


```go
type LayoutCSS struct {
    ContainerClasses []string
    ContainerStyles map[string]string
}
```

#### LayoutWithCSS

LayoutWithCSS represents a layout with CSS information for UI rendering


```go
type LayoutWithCSS struct {
    CSS LayoutCSS
}
```

#### PanelCSS

PanelCSS represents panel-specific CSS (UI concern)


```go
type PanelCSS struct {
    Classes []string
    Styles map[string]string
}
```

#### PanelLayoutWithCSS

PanelLayoutWithCSS represents a panel layout with CSS information for UI rendering


```go
type PanelLayoutWithCSS struct {
    CSS PanelCSS
}
```

#### RefreshStrategy

RefreshStrategy defines how panels refresh


#### Renderer

Renderer renders lens dashboards to UI components


##### Interface Methods

- `RenderDashboard(dashboard *evaluation.EvaluatedDashboard) templ.Component`
- `RenderPanel(panel *evaluation.EvaluatedPanel) templ.Component`
- `RenderGrid(layout *evaluation.Layout) templ.Component`
- `RenderDashboardWithData(config lens.DashboardConfig, results *executor.DashboardResult) templ.Component`
- `RenderPanelWithData(config lens.PanelConfig, result *executor.ExecutionResult) templ.Component`
- `RenderError(err error) templ.Component`
- `RenderPanelError(config lens.PanelConfig, message string) templ.Component`

### Functions

#### `func ChartContent(config lens.PanelConfig, result *executor.ExecutionResult) templ.Component`

ChartContent renders chart from executor results


#### `func ChartPanel(panel *evaluation.EvaluatedPanel) templ.Component`

ChartPanel renders a chart panel from evaluated panel


#### `func Dashboard(dashboard *evaluation.EvaluatedDashboard) templ.Component`

Dashboard renders a complete dashboard with panels


#### `func DashboardWithData(config lens.DashboardConfig, results *executor.DashboardResult) templ.Component`

DashboardWithData renders a dashboard using executor results


#### `func ErrorContent(message string) templ.Component`

ErrorContent renders error message


#### `func GenerateCSS(gridConfig lens.GridConfig) string`

GenerateCSS generates the CSS for the dashboard using text templating, wrapped in style tags


#### `func Grid(layout *evaluation.Layout) templ.Component`

Grid renders just the grid layout


#### `func MetricCard(metric lens.MetricValue) templ.Component`

MetricCard renders a single metric value card


#### `func MetricContent(config lens.PanelConfig, result *executor.ExecutionResult) templ.Component`

MetricContent renders metric card from executor results


#### `func MetricPanel(panel *evaluation.EvaluatedPanel) templ.Component`

MetricPanel renders a metric panel from evaluated panel


#### `func Panel(panel *evaluation.EvaluatedPanel) templ.Component`

Panel renders a single evaluated panel


#### `func PanelError(config lens.PanelConfig, message string) templ.Component`

PanelError renders error state for a panel


#### `func PanelWithData(config lens.PanelConfig, result *executor.ExecutionResult) templ.Component`

PanelWithData renders a panel using executor results


#### `func TableContent(result *executor.ExecutionResult) templ.Component`

TableContent renders table data from executor results


#### `func TablePanel(panel *evaluation.EvaluatedPanel) templ.Component`

TablePanel renders a table panel from evaluated panel


### Variables and Constants

---

## Package `validation` (pkg/lens/validation)

### Types

#### DashboardValidationRule

DashboardValidationRule validates dashboard-level configuration


##### Methods

- `func (DashboardValidationRule) Name() string`

- `func (DashboardValidationRule) ValidateDashboard(config *lens.DashboardConfig) ValidationResult`

#### GridLayoutError

GridLayoutError represents a grid layout specific error


```go
type GridLayoutError struct {
    Message string
    Panels []string
    Code string
}
```

##### Methods

- `func (GridLayoutError) Error() string`

#### GridValidationRule

GridValidationRule validates grid layout configuration


##### Methods

- `func (GridValidationRule) Name() string`

- `func (GridValidationRule) ValidateGrid(panels []lens.PanelConfig, grid lens.GridConfig) ValidationResult`

#### PanelOverlap

PanelOverlap represents overlapping panels


```go
type PanelOverlap struct {
    Panel1 string
    Panel2 string
}
```

#### PanelValidationError

PanelValidationError represents a panel-specific validation error


```go
type PanelValidationError struct {
    PanelID string
    Field string
    Message string
    Code string
}
```

##### Methods

- `func (PanelValidationError) Error() string`

#### PanelValidationRule

PanelValidationRule validates panel configuration


##### Methods

- `func (PanelValidationRule) Name() string`

- `func (PanelValidationRule) ValidatePanel(panel *lens.PanelConfig, grid lens.GridConfig) ValidationResult`

#### ValidationError

ValidationError represents a validation error


```go
type ValidationError struct {
    Field string
    Message string
    Code string
}
```

##### Methods

- `func (ValidationError) Error() string`

#### ValidationErrorCode

ValidationErrorCode represents predefined error codes


#### ValidationErrors

ValidationErrors represents a collection of validation errors


##### Methods

- `func (ValidationErrors) Error() string`

- `func (ValidationErrors) First() *ValidationError`
  First returns the first validation error, or nil if none exist
  

- `func (ValidationErrors) GetErrorsByCode(code ValidationErrorCode) []ValidationError`
  GetErrorsByCode returns all errors with a specific code
  

- `func (ValidationErrors) GetErrorsByField(field string) []ValidationError`
  GetErrorsByField returns all errors for a specific field
  

- `func (ValidationErrors) HasErrors() bool`
  HasErrors returns true if there are any validation errors
  

#### ValidationResult

ValidationResult contains validation results


```go
type ValidationResult struct {
    Valid bool
    Errors []ValidationError
}
```

##### Methods

- `func (ValidationResult) IsValid() bool`
  IsValid returns true if there are no validation errors
  

#### ValidationRule

ValidationRule represents a validation rule


##### Interface Methods

- `Name() string`

#### Validator

Validator validates dashboard configurations


##### Interface Methods

- `Validate(config *lens.DashboardConfig) ValidationResult`
- `ValidatePanel(panel *lens.PanelConfig, grid lens.GridConfig) ValidationResult`
- `ValidateGrid(panels []lens.PanelConfig, grid lens.GridConfig) ValidationResult`

### Functions

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


#### `func PointerToSQLNullInt32(i *int) sql.NullInt32`

#### `func PointerToSQLNullString(s *string) sql.NullString`

#### `func PointerToSQLNullTime(t *time.Time) sql.NullTime`

#### `func SQLNullInt32ToPointer(v sql.NullInt32) *int`

#### `func SQLNullStringToUUID(ns sql.NullString) uuid.UUID`

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

#### `func ProvideDynamicLogo(app application.Application) mux.MiddlewareFunc`

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

WithTransaction is deprecated and will be removed in the future.


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

#### `func ParseUUID(r *http.Request) (uuid.UUID, error)`

#### `func Redirect(w http.ResponseWriter, r *http.Request, path string)`

#### `func SetFlash(w http.ResponseWriter, name string, value []byte)`

#### `func SetFlashMap(w http.ResponseWriter, name string, value map[K]V)`

### Variables and Constants

- Var: `[Decoder]`

- Var: `[Encoder]`

---

## Package `sidebar` (pkg/sidebar)

### Types

#### TabGroupBuilder

TabGroupBuilder is a function that takes navigation items and returns tab groups


### Functions

#### `func DefaultTabGroupBuilder(items []types.NavigationItem, localizer *i18n.Localizer) sidebar.TabGroupCollection`

DefaultTabGroupBuilder maintains current behavior (single "Core" tab)


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

#### DatabaseManager

DatabaseManager handles database lifecycle for tests


##### Methods

- `func (DatabaseManager) Close()`
  Close closes the pool
  

- `func (DatabaseManager) Pool() *pgxpool.Pool`
  Pool returns the database pool
  

#### TestEnv

TestEnv holds common test dependencies


```go
type TestEnv struct {
    Ctx context.Context
    Pool *pgxpool.Pool
    Tx pgx.Tx
    Tenant *composables.Tenant
    App application.Application
}
```

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

#### `func TestMiddleware(env *TestEnv, user user.User) mux.MiddlewareFunc`

TestMiddleware creates middleware that adds all required context values for controller tests


---

## Package `builder` (pkg/testutils/builder)

### Types

#### TestContext

TestContext provides a fluent API for building test contexts


##### Methods

- `func (TestContext) Build(tb testing.TB) *TestEnvironment`
  Build creates the test context with all dependencies
  

- `func (TestContext) WithDBName(tb testing.TB, name string) *TestContext`
  WithDBName sets a custom database name
  

- `func (TestContext) WithModules(modules ...application.Module) *TestContext`
  WithModules adds modules to the test context
  

- `func (TestContext) WithUser(u user.User) *TestContext`
  WithUser sets the user for the test context
  

#### TestEnvironment

TestEnvironment contains all test dependencies


```go
type TestEnvironment struct {
    Ctx context.Context
    Pool *pgxpool.Pool
    Tx pgx.Tx
    App application.Application
    Tenant *composables.Tenant
    User user.User
}
```

##### Methods

- `func (TestEnvironment) AssertNoError(tb testing.TB, err error)`
  AssertNoError fails the test if err is not nil
  

- `func (TestEnvironment) Service(service interface{}) interface{}`
  Service retrieves a service from the application
  

- `func (TestEnvironment) TenantID() uuid.UUID`
  TenantID returns the test tenant ID
  

---

## Package `controllertest` (pkg/testutils/controllertest)

### Types

#### Element

##### Methods

- `func (Element) Attr(name string) string`

- `func (Element) Exists() *Element`

- `func (Element) NotExists() *Element`

- `func (Element) Text() string`

#### HTML

##### Methods

- `func (HTML) Element(xpath string) *Element`

- `func (HTML) Elements(xpath string) []*html.Node`

- `func (HTML) HasErrorFor(fieldID string) bool`

#### MiddlewareFunc

MiddlewareFunc is a function that can modify the request context


#### Request

##### Methods

- `func (Request) Cookie(name, value string) *Request`

- `func (Request) Expect(t *testing.T) *Response`

- `func (Request) File(fieldName, fileName string, content []byte) *Request`

- `func (Request) Form(values url.Values) *Request`

- `func (Request) HTMX() *Request`

- `func (Request) Header(key, value string) *Request`

- `func (Request) JSON(v interface{}) *Request`

#### Response

##### Methods

- `func (Response) Body() string`

- `func (Response) Contains(text string) *Response`

- `func (Response) Cookies() []*http.Cookie`

- `func (Response) HTML() *HTML`

- `func (Response) Header(key string) string`

- `func (Response) NotContains(text string) *Response`

- `func (Response) Raw() *http.Response`

- `func (Response) RedirectTo(location string) *Response`

- `func (Response) Status(code int) *Response`

#### Suite

##### Methods

- `func (Suite) AsUser(u user.User) *Suite`

- `func (Suite) DELETE(path string) *Request`

- `func (Suite) Environment() *builder.TestEnvironment`

- `func (Suite) GET(path string) *Request`

- `func (Suite) POST(path string) *Request`

- `func (Suite) PUT(path string) *Request`

- `func (Suite) Register(controller interface{...}) *Suite`

- `func (Suite) WithMiddleware(middleware MiddlewareFunc) *Suite`
  WithMiddleware registers a custom middleware function that can modify the request context
  

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

- `func (PageContext) TSafe(k string, args ...map[string]interface{}) string`

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

- `func (Connection) Close() error`

- `func (Connection) SendMessage(message []byte) error`
  SendMessage sends a text message to the websocket connection
  

#### Connectioner

##### Interface Methods

- `io.Closer`
- `SendMessage(message []byte) error`

#### EventType

#### Hub

```go
type Hub struct {
    OnConnect func(r *http.Request, hub *Hub, conn *Connection) error
    OnDisconnect func(conn *Connection)
}
```

##### Methods

- `func (Hub) BroadcastToAll(message []byte)`

- `func (Hub) BroadcastToChannel(channel string, message []byte)`

- `func (Hub) ConnectionsAll() []*Connection`

- `func (Hub) ConnectionsInChannel(channel string) []*Connection`

- `func (Hub) JoinChannel(channel string, conn *Connection)`

- `func (Hub) LeaveChannel(channel string, conn *Connection)`

- `func (Hub) On(eventType EventType, handler func(conn *Connection, message []byte))`

- `func (Hub) ServeHTTP(w http.ResponseWriter, r *http.Request)`

#### HubOptions

```go
type HubOptions struct {
    Logger *logrus.Logger
    CheckOrigin func(r *http.Request) bool
    OnConnect func(r *http.Request, hub *Hub, conn *Connection) error
    OnDisconnect func(conn *Connection)
}
```

#### Huber

##### Interface Methods

- `http.Handler`
- `BroadcastToAll(message []byte)`
- `BroadcastToChannel(channel string, message []byte)`
- `On(eventType EventType, handler func(conn *Connection, message []byte))`
- `JoinChannel(channel string, conn *Connection)`
- `LeaveChannel(channel string, conn *Connection)`
- `ConnectionsInChannel(channel string) []*Connection`
- `ConnectionsAll() []*Connection`

#### Set

---

