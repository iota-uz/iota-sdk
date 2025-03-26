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

templ: version: v0.3.819


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

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


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

#### TableColumn

```go
type TableColumn struct {
    Label string
    Key string
    Width int
    Class string
    DateFormat string
    Duration bool
    Sortable bool
}
```

#### TableProps

```go
type TableProps struct {
    Columns []*TableColumn
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

#### `func Select(p *SelectProps) templ.Component`

#### `func SelectedValues() templ.Component`

#### `func Table(props *TableProps) templ.Component`

#### `func TableCell() templ.Component`

#### `func TableRow(props *TableRowProps) templ.Component`

### Variables and Constants

---

## Package `alert` (components/base/alert)

templ: version: v0.3.819


### Functions

#### `func Error() templ.Component`

### Variables and Constants

---

## Package `avatar` (components/base/avatar)

templ: version: v0.3.819


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

## Package `button` (components/base/button)

templ: version: v0.3.819


### Types

#### Props

```go
type Props struct {
    Size Size
    Fixed bool
    Href string
    Rounded bool
    Loading bool
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

templ: version: v0.3.819


### Types

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

#### `func Card(props Props) templ.Component`

#### `func DefaultHeader(text string) templ.Component`

### Variables and Constants

---

## Package `dialog` (components/base/dialog)

templ: version: v0.3.819

templ: version: v0.3.819


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

templ: version: v0.3.819

templ: version: v0.3.819


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
    Class string
    ID string
}
```

##### Methods

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
    Label string
    LabelComp templ.Component
    Error string
    Checked bool
    Attrs templ.Attributes
    Class string
    ID string
}
```

##### Methods

### Functions

#### `func Checkbox(p *CheckboxProps) templ.Component`

#### `func Date(props *Props) templ.Component`

#### `func Email(props *Props) templ.Component`

#### `func Number(props *Props) templ.Component`

#### `func Password(props *Props) templ.Component`

#### `func Switch(p *SwitchProps) templ.Component`

#### `func Text(props *Props) templ.Component`

### Variables and Constants

---

## Package `pagination` (components/base/pagination)

templ: version: v0.3.819


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

#### RadioGroupProps

RadioGroupProps defines properties for the RadioGroup component.


```go
type RadioGroupProps struct {
    Name string
    Label string
    Error string
    Class string
    Attrs templ.Attributes
    WrapperProps templ.Attributes
    Orientation string
}
```

##### Methods

#### RadioItemProps

RadioItemProps defines properties for individual RadioItem components.


```go
type RadioItemProps struct {
    Value string
    Label string
    LabelComp templ.Component
    Checked bool
    Disabled bool
    Class string
    Attrs templ.Attributes
    GroupName string
    ID string
}
```

##### Methods

### Functions

#### `func RadioGroup(props RadioGroupProps) templ.Component`

RadioGroup wraps multiple RadioItem components as a form control.


#### `func RadioItem(props RadioItemProps) templ.Component`

RadioItem renders a single radio button with its label.


### Variables and Constants

---

## Package `selects` (components/base/selects)

templ: version: v0.3.819


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

## Package `tab` (components/base/tab)

templ: version: v0.3.819


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

## Package `textarea` (components/base/textarea)

templ: version: v0.3.819


### Types

#### Props

```go
type Props struct {
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

#### `func Basic(props *Props) templ.Component`

### Variables and Constants

---

## Package `toggle` (components/base/toggle)

templ: version: v0.3.819


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

#### BarConfig

```go
type BarConfig struct {
    BorderRadius int `json:"borderRadius"`
    ColumnWidth string `json:"columnWidth"`
    DataLabels BarLabels `json:"dataLabels"`
}
```

#### BarLabels

```go
type BarLabels struct {
    Position string `json:"position"`
}
```

#### ChartConfig

```go
type ChartConfig struct {
    Type string `json:"type"`
    Height string `json:"height"`
    Toolbar Toolbar `json:"toolbar"`
}
```

#### ChartOptions

```go
type ChartOptions struct {
    Chart ChartConfig `json:"chart"`
    Series []Series `json:"series"`
    XAxis XAxisConfig `json:"xaxis"`
    YAxis YAxisConfig `json:"yaxis"`
    Colors []string `json:"colors"`
    DataLabels DataLabels `json:"dataLabels"`
    Grid GridConfig `json:"grid"`
    PlotOptions PlotOptions `json:"plotOptions"`
}
```

#### DataLabelStyle

```go
type DataLabelStyle struct {
    Colors []string `json:"colors"`
    FontSize string `json:"fontSize"`
    FontWeight string `json:"fontWeight"`
}
```

#### DataLabels

```go
type DataLabels struct {
    Enabled bool `json:"enabled"`
    Formatter templ.JSExpression `json:"formatter,omitempty"`
    Style DataLabelStyle `json:"style"`
    OffsetY int `json:"offsetY"`
    DropShadow DropShadow `json:"dropShadow"`
}
```

#### DropShadow

```go
type DropShadow struct {
    Enabled bool `json:"enabled"`
    Top int `json:"top"`
    Left int `json:"left"`
    Blur int `json:"blur"`
    Color string `json:"color"`
    Opacity float64 `json:"opacity"`
}
```

#### GridConfig

```go
type GridConfig struct {
    BorderColor string `json:"borderColor"`
}
```

#### LabelFormatter

```go
type LabelFormatter struct {
    Style LabelStyle `json:"style"`
}
```

#### LabelStyle

```go
type LabelStyle struct {
    Colors string `json:"colors"`
    FontSize string `json:"fontSize"`
}
```

#### PlotOptions

```go
type PlotOptions struct {
    Bar BarConfig `json:"bar"`
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

#### Series

```go
type Series struct {
    Name string `json:"name"`
    Data []float64 `json:"data"`
}
```

#### Toolbar

```go
type Toolbar struct {
    Show bool `json:"show"`
}
```

#### XAxisConfig

```go
type XAxisConfig struct {
    Categories []string `json:"categories"`
    Labels LabelFormatter `json:"labels"`
}
```

#### YAxisConfig

```go
type YAxisConfig struct {
    Labels LabelFormatter `json:"labels"`
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

templ: version: v0.3.819


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

## Package `loaders` (components/loaders)

### Functions

#### `func Hand() templ.Component`

Hand renders a hand-shaped loading animation.
It's a stylized animation for use during loading states, providing
visual feedback to users while content or data is being processed.


### Variables and Constants

---

## Package `scaffold` (components/scaffold)

Package scaffold provides utilities for generating dynamic UI components.

It simplifies the creation of consistent data tables and other UI elements
based on configuration and data, reducing boilerplate code.


### Types

#### TableColumn

TableColumn defines a column in a dynamic table.


```go
type TableColumn struct {
    Key string
    Label string
    Class string
    Width string
    Format func(any) string
}
```

#### TableConfig

TableConfig holds the configuration for a dynamic table.


```go
type TableConfig struct {
    Columns []TableColumn
    Title string
}
```

##### Methods

- `func (TableConfig) AddActionsColumn() *TableConfig`
  AddActionsColumn adds an actions column with edit button
  

- `func (TableConfig) AddColumn(key, label, class string) *TableConfig`
  AddColumn adds a column to the table configuration
  

- `func (TableConfig) AddDateColumn(key, label string) *TableConfig`
  AddDateColumn adds a date column with automatic formatting
  

#### TableData

TableData contains the data to be displayed in the table.


```go
type TableData struct {
    Items []map[string]any
}
```

##### Methods

- `func (TableData) AddItem(item map[string]any) *TableData`
  AddItem adds an item to the table data
  

### Functions

#### `func Content(config TableConfig, data TableData) templ.Component`

Content renders the complete scaffold page content with filters and table


#### `func Page(config TableConfig, data TableData) templ.Component`

Page renders a complete authenticated page with the scaffolded content


#### `func Table(config TableConfig, data TableData) templ.Component`

Table renders a dynamic table based on configuration and data


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

templ: version: v0.3.819

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

### Types

#### Item

Item represents a search result in the Spotlight component.


```go
type Item struct {
    Title string
    Icon templ.Component
    Link string
}
```

### Functions

#### `func Spotlight() templ.Component`

Spotlight renders a search dialog component that can be triggered
with a button click or keyboard shortcut.


#### `func SpotlightItems(items []*Item) templ.Component`

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

### Functions

#### `func Migrate(mods ...application.Module) error`

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

### Functions

#### `func BeginTx(ctx context.Context) (pgx.Tx, error)`

#### `func CanUser(ctx context.Context, permission *permission.Permission) error`

#### `func CanUserAll(ctx context.Context, perms ...rbac.Permission) error`

#### `func MustT(ctx context.Context, msgID string) string`

MustT returns the translation for the given message ID.
If the translation is not found, it will panic.


#### `func MustUseHead(ctx context.Context) templ.Component`

MustUseHead returns the head component from the context or panics


#### `func MustUseLocalizer(ctx context.Context) *i18n.Localizer`

MustUseLocalizer returns the localizer from the context.
If the localizer is not found, it will panic.


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


#### `func UseFlash(w http.ResponseWriter, r *http.Request, name string) (val []byte, err error)`

#### `func UseFlashMap(w http.ResponseWriter, r *http.Request, name string) (map[K]V, error)`

#### `func UseForm(v T, r *http.Request) (T, error)`

#### `func UseHead(ctx context.Context) (templ.Component, error)`

UseHead returns the head component from the context


#### `func UseIP(ctx context.Context) (string, bool)`

UseIP returns the IP address from the context.
If the IP address is not found, the second return value will be false.


#### `func UseLocale(ctx context.Context, defaultLocale language.Tag) language.Tag`

UseLocale returns the locale from the context.
If the locale is not found, the second return value will be false.


#### `func UseLocalizedOrFallback(ctx context.Context, key string, fallback string) string`

#### `func UseLocalizer(ctx context.Context) (*i18n.Localizer, bool)`

UseLocalizer returns the localizer from the context.
If the localizer is not found, the second return value will be false.


#### `func UseLogger(ctx context.Context) (*logrus.Entry, error)`

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

#### `func UseRequest(ctx context.Context) (*http.Request, bool)`

UseRequest returns the request from the context.
If the request is not found, the second return value will be false.


#### `func UseSession(ctx context.Context) (*session.Session, error)`

UseSession returns the session from the context.


#### `func UseTabs(ctx context.Context) ([]*tab.Tab, error)`

#### `func UseTx(ctx context.Context) (repo.Tx, error)`

#### `func UseUniLocalizer(ctx context.Context) (ut.Translator, error)`

#### `func UseUser(ctx context.Context) (user.User, error)`

UseUser returns the user from the context.


#### `func UseUserAgent(ctx context.Context) (string, bool)`

UseUserAgent returns the user agent from the context.
If the user agent is not found, the second return value will be false.


#### `func UseWriter(ctx context.Context) (http.ResponseWriter, bool)`

UseWriter returns the response writer from the context.
If the response writer is not found, the second return value will be false.


#### `func WithLocalizer(ctx context.Context, l *i18n.Localizer) context.Context`

#### `func WithPageCtx(ctx context.Context, pageCtx *types.PageContext) context.Context`

WithPageCtx returns a new context with the page context.


#### `func WithParams(ctx context.Context, params *Params) context.Context`

WithParams returns a new context with the request parameters.


#### `func WithPool(ctx context.Context, pool *pgxpool.Pool) context.Context`

#### `func WithSession(ctx context.Context, sess *session.Session) context.Context`

WithSession returns a new context with the session.


#### `func WithTx(ctx context.Context, tx pgx.Tx) context.Context`

#### `func WithUser(ctx context.Context, u user.User) context.Context`

WithUser returns a new context with the user.


### Variables and Constants

- Var: `[ErrNoSessionFound ErrNoUserFound]`

- Var: `[ErrNoTx ErrNoPool]`

- Var: `[ErrInvalidPassword ErrNotFound ErrUnauthorized ErrForbidden ErrInternal]`

- Var: `[ErrNoLogoFound ErrNoHeadFound]`

- Var: `[ErrNoLocalizer ErrNoLogger]`

- Var: `[ErrNavItemsNotFound]`

- Var: `[ErrTabsNotFound]`

---

## Package `configuration` (pkg/configuration)

### Types

#### Configuration

```go
type Configuration struct {
    Database DatabaseOptions
    Google GoogleOptions
    Twilio TwilioOptions
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

## Package `di` (pkg/di)

### Types

#### DIHandler

DIHandler is a handler that uses dependency injection to resolve its arguments


##### Methods

- `func (DIHandler) Handler() http.HandlerFunc`

#### Provider

Provider is an interface that can provide a value for a given type


##### Interface Methods

- `Ok(t reflect.Type) bool`
- `Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error)`

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

### Variables and Constants

- Var: `[SupportedLanguages]`

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

### Functions

#### `func ConsoleLogger(level logrus.Level) *logrus.Logger`

#### `func FileLogger(level logrus.Level) (*os.File, *logrus.Logger, error)`

---

## Package `mapping` (pkg/mapping)

### Functions

#### `func MapDBModels(entities []T, mapFunc func(T) (V, error)) ([]V, error)`

MapDBModels maps entities to db models


#### `func MapViewModels(entities []T, mapFunc func(T) V) []V`

MapViewModels maps entities to view models


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

### Functions

#### `func Authorize() mux.MiddlewareFunc`

#### `func ContextKeyValue(key interface{}, constructor GenericConstructor) mux.MiddlewareFunc`

#### `func Cors(allowOrigins ...string) mux.MiddlewareFunc`

#### `func NavItems() mux.MiddlewareFunc`

#### `func Provide(k constants.ContextKey, v any) mux.MiddlewareFunc`

#### `func ProvideUser() mux.MiddlewareFunc`

#### `func RedirectNotAuthenticated() mux.MiddlewareFunc`

#### `func RequestParams() mux.MiddlewareFunc`

#### `func RequireAuthorization() mux.MiddlewareFunc`

#### `func Tabs() mux.MiddlewareFunc`

#### `func WithLocalizer(bundle *i18n.Bundle) mux.MiddlewareFunc`

#### `func WithLogger(logger *logrus.Logger) mux.MiddlewareFunc`

#### `func WithPageContext() mux.MiddlewareFunc`

#### `func WithTransaction() mux.MiddlewareFunc`

### Variables and Constants

- Var: `[AllowMethods]`

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

### Variables and Constants

- Var: `[ErrPermissionNotFound]`

---

## Package `repo` (pkg/repo)

Package repo provides database utility functions and interfaces for working with PostgreSQL.


### Types

#### Expr

Expr represents a comparison expression type for filtering queries.


#### ExtendedFieldSet

ExtendedFieldSet is an interface that must be implemented to persist custom fields with a repository.
It allows repositories to work with custom field sets by providing field names and values.


##### Interface Methods

- `Fields() []string`
- `Value(k string) interface{}`

#### Filter

Filter defines a filter condition for queries.
Combines an expression type with a value to be used in WHERE clauses.


```go
type Filter struct {
    Expr Expr
    Value any
}
```

#### SortBy

SortBy defines sorting criteria for queries with generic field type support.
Use with OrderBy function to generate ORDER BY clauses.


```go
type SortBy struct {
    Fields []T
    Ascending bool
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


#### `func OrderBy(fields []string, ascending bool) string`

OrderBy generates an SQL ORDER BY clause for the given fields and sort direction.
Returns an empty string if no fields are provided.

Example usage:

	query := "SELECT * FROM users " + repo.OrderBy([]string{"created_at", "name"}, false)
	// Returns: "SELECT * FROM users ORDER BY created_at, name DESC"


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


---

## Package `scaffold` (pkg/scaffold)

### Types

#### ContentAdapter

ContentAdapter adapts scaffold.Content to support search and pagination


```go
type ContentAdapter struct {
    Config *scaffold.TableConfig
    Data scaffold.TableData
    Search string
    Page int
    TotalPages int
    PageCtx *types.PageContext
}
```

##### Methods

- `func (ContentAdapter) Render(ctx context.Context, w io.Writer) error`
  Render implements templ.Component interface
  

#### LayoutAdapter

LayoutAdapter adapts a content component with a layout


```go
type LayoutAdapter struct {
    Content templ.Component
    PageCtx *types.PageContext
}
```

##### Methods

- `func (LayoutAdapter) Render(ctx context.Context, w io.Writer) error`
  Render implements templ.Component interface
  

#### TableAdapter

TableAdapter adapts scaffold.Table to support pagination


```go
type TableAdapter struct {
    Config *scaffold.TableConfig
    Data scaffold.TableData
    Page int
    TotalPages int
    PageCtx *types.PageContext
}
```

##### Methods

- `func (TableAdapter) Render(ctx context.Context, w io.Writer) error`
  Render implements templ.Component interface
  

#### TableControllerBuilder

TableControllerBuilder helps to quickly build controllers for displaying tables


##### Methods

- `func (TableControllerBuilder) Key() string`

- `func (TableControllerBuilder) List(w http.ResponseWriter, r *http.Request)`
  List handles listing entities in a table
  

- `func (TableControllerBuilder) Register(r *mux.Router)`
  Register registers the table route
  

- `func (TableControllerBuilder) WithFindParamsFunc(fn func(r *http.Request) interface{}) *<?>`
  WithFindParamsFunc sets a custom function for creating find parameters
  

#### TableService

TableService defines the minimal interface for table data services


##### Interface Methods

- `GetPaginated(ctx context.Context, params interface{}) ([]T, error)`
- `Count(ctx context.Context, params interface{}) (int64, error)`

#### TableViewModel

TableViewModel defines the interface for mapping entity to view model


##### Interface Methods

- `MapToViewModel(entity T) map[string]interface{}`

### Functions

#### `func ExtendedContent(config *scaffold.TableConfig, data scaffold.TableData, search string, page int, totalPages int, pageCtx *types.PageContext) templ.Component`

ExtendedContent creates a content component with search and pagination


#### `func ExtendedTable(config *scaffold.TableConfig, data scaffold.TableData, page int, totalPages int, pageCtx *types.PageContext) templ.Component`

ExtendedTable creates a table with pagination support


#### `func PageWithLayout(content templ.Component, pageCtx *types.PageContext) templ.Component`

PageWithLayout wraps content with a layout


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

#### `func SortTableDefs(tables []*tree.CreateTable) []*tree.CreateTable`

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

#### Item

##### Interface Methods

- `Icon() templ.Component`
- `Localized(localizer *i18n.Localizer) string`
- `Link() string`

#### Spotlight

##### Interface Methods

- `Find(localizer *i18n.Localizer, q string) []Item`
- `Register(...Item)`

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

## Package `main` (tools)

### Types

#### Config

```go
type Config struct {
    ExcludeDirs []string `yaml:"exclude-dirs"`
    CheckZeroByteFiles bool `yaml:"check-zero-byte-files"`
}
```

#### JSONKeys

```go
type JSONKeys struct {
    Keys map[string]bool
    Path string
}
```

#### KeyStore

Add a mutex to protect our key operations


#### LintError

```go
type LintError struct {
    File string
    Line int
    Message string
}
```

##### Methods

- `func (LintError) Error() string`

#### LinterConfig

```go
type LinterConfig struct {
    LintersSettings struct{...} `yaml:"linters-settings"`
}
```

### Functions

### Variables and Constants

- Var: `[JSONLinter]`

---

