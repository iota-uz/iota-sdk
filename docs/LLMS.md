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

#### type `UploadInputProps`

UploadInputProps defines the properties for the UploadInput component.
It provides configuration options for the file upload interface.


##### Methods

### Functions

#### `func UploadInput`

UploadInput renders a file upload input with preview capability.
It displays existing uploads and allows selecting new files.


#### `func UploadPreview`

### Variables and Constants

---

## Package `base` (components/base)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `BaseLabelProps`

#### type `ComboboxOption`

#### type `ComboboxProps`

#### type `DetailsDropdownProps`

##### Methods

#### type `DropdownItemProps`

#### type `SelectProps`

##### Methods

#### type `TableColumn`

#### type `TableProps`

#### type `TableRowProps`

#### type `Trigger`

#### type `TriggerProps`

### Functions

#### `func BaseLabel`

#### `func Combobox`

#### `func ComboboxOptions`

#### `func DetailsDropdown`

#### `func DropdownIndicator`

#### `func DropdownItem`

#### `func Select`

#### `func SelectedValues`

#### `func Table`

#### `func TableCell`

#### `func TableRow`

### Variables and Constants

---

## Package `alert` (components/base/alert)

templ: version: v0.3.819


### Functions

#### `func Error`

### Variables and Constants

---

## Package `avatar` (components/base/avatar)

templ: version: v0.3.819


### Types

#### type `Props`

### Functions

#### `func Avatar`

### Variables and Constants

---

## Package `button` (components/base/button)

templ: version: v0.3.819


### Types

#### type `Props`

#### type `Size`

#### type `Variant`

### Functions

#### `func Danger`

#### `func Ghost`

#### `func Primary`

#### `func PrimaryOutline`

#### `func Secondary`

#### `func Sidebar`

### Variables and Constants

- Const: `[VariantPrimary VariantSecondary VariantPrimaryOutline VariantSidebar VariantDanger VariantGhost]`

- Const: `[SizeNormal SizeMD SizeSM SizeXS]`

---

## Package `card` (components/base/card)

templ: version: v0.3.819


### Types

#### type `Props`

### Functions

#### `func Card`

#### `func DefaultHeader`

### Variables and Constants

---

## Package `dialog` (components/base/dialog)

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `Direction`

#### type `DrawerProps`

#### type `Props`

#### type `StdDrawerProps`

### Functions

#### `func Confirmation`

#### `func Drawer`

#### `func StdViewDrawer`

### Variables and Constants

---

## Package `input` (components/base/input)

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `Addon`

#### type `CheckboxProps`

##### Methods

#### type `Props`

##### Methods

#### type `SwitchProps`

##### Methods

### Functions

#### `func Checkbox`

#### `func Date`

#### `func Email`

#### `func Number`

#### `func Password`

#### `func Switch`

#### `func Text`

### Variables and Constants

---

## Package `pagination` (components/base/pagination)

templ: version: v0.3.819


### Types

#### type `Page`

##### Methods

- `func (Page) Classes`

#### type `State`

##### Methods

- `func (State) NextLink`

- `func (State) NextLinkClasses`

- `func (State) Pages`

- `func (State) PrevLink`

- `func (State) PrevLinkClasses`

- `func (State) TotalStr`

### Functions

#### `func Pagination`

### Variables and Constants

---

## Package `selects` (components/base/selects)

templ: version: v0.3.819


### Types

#### type `SearchOptionsProps`

#### type `SearchSelectProps`

#### type `Value`

### Functions

#### `func SearchOptions`

#### `func SearchSelect`

### Variables and Constants

---

## Package `tab` (components/base/tab)

templ: version: v0.3.819


### Types

#### type `BoostLinkProps`

#### type `ListProps`

#### type `Props`

### Functions

#### `func BoostedContent`

#### `func BoostedLink`

#### `func Button`

#### `func Content`

#### `func Link`

--- Pure Tabs ---


#### `func List`

#### `func Root`

### Variables and Constants

---

## Package `textarea` (components/base/textarea)

templ: version: v0.3.819


### Types

#### type `Props`

##### Methods

### Functions

#### `func Basic`

### Variables and Constants

---

## Package `toggle` (components/base/toggle)

templ: version: v0.3.819


### Types

#### type `ToggleAlignment`

#### type `ToggleOption`

##### Methods

#### type `ToggleProps`

##### Methods

#### type `ToggleRounded`

#### type `ToggleSize`

### Functions

#### `func Toggle`

### Variables and Constants

---

## Package `charts` (components/charts)

### Types

#### type `BarConfig`

#### type `BarLabels`

#### type `ChartConfig`

#### type `ChartOptions`

#### type `DataLabelStyle`

#### type `DataLabels`

#### type `DropShadow`

#### type `GridConfig`

#### type `LabelFormatter`

#### type `LabelStyle`

#### type `PlotOptions`

#### type `Props`

Props defines the configuration options for a Chart component.


#### type `Series`

#### type `Toolbar`

#### type `XAxisConfig`

#### type `YAxisConfig`

### Functions

#### `func Chart`

Chart renders a chart with the specified options.
It generates a random ID for the chart container and initializes
the ApexCharts library to render the chart on the client side.


### Variables and Constants

---

## Package `filters` (components/filters)

templ: version: v0.3.819


### Types

#### type `DrawerProps`

#### type `Props`

Props defines configuration options for the Default filter component.


#### type `SearchField`

SearchField represents a field that can be searched on.


### Functions

#### `func CreatedAt`

CreatedAt renders a date range filter for filtering by creation date.
It provides common options like today, yesterday, this week, etc.


#### `func Default`

Default renders a complete filter bar with search, page size, and date filters.
It combines multiple filter components into a single interface.


#### `func Drawer`

#### `func PageSize`

PageSize renders a select dropdown for choosing the number of items per page.


#### `func Search`

Search renders a search input with field selection.
It includes a search icon and allows selecting which field to search on.


#### `func SearchFields`

SearchFields renders a dropdown list of available search fields.
For a single field, it creates a hidden select. For multiple fields,
it creates a combobox for selecting which field to search on.


#### `func SearchFieldsTrigger`

### Variables and Constants

---

## Package `loaders` (components/loaders)

### Functions

#### `func Hand`

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

#### type `TableColumn`

TableColumn defines a column in a dynamic table.


#### type `TableConfig`

TableConfig holds the configuration for a dynamic table.


##### Methods

- `func (TableConfig) AddActionsColumn`
  AddActionsColumn adds an actions column with edit button
  

- `func (TableConfig) AddColumn`
  AddColumn adds a column to the table configuration
  

- `func (TableConfig) AddDateColumn`
  AddDateColumn adds a date column with automatic formatting
  

#### type `TableData`

TableData contains the data to be displayed in the table.


##### Methods

- `func (TableData) AddItem`
  AddItem adds an item to the table data
  

### Functions

#### `func Content`

Content renders the complete scaffold page content with filters and table


#### `func Page`

Page renders a complete authenticated page with the scaffolded content


#### `func Table`

Table renders a dynamic table based on configuration and data


### Variables and Constants

---

## Package `selects` (components/selects)

### Types

#### type `CountriesSelectProps`

CountriesSelectProps defines the properties for the CountriesSelect component.


### Functions

#### `func CountriesSelect`

CountriesSelect renders a select dropdown with a list of countries.
Countries are translated according to the current locale.


### Variables and Constants

---

## Package `sidebar` (components/sidebar)

templ: version: v0.3.819

Package sidebar provides navigation components for application layout.

It implements a sidebar with support for nested navigation groups,
active state highlighting, and collapsible sections.


### Types

#### type `Group`

Group represents a collection of navigation items that can be expanded/collapsed.


#### type `Item`

Item is the base interface for navigation elements in the sidebar.


#### type `Link`

Link represents a navigation link in the sidebar.


#### type `Props`

### Functions

#### `func AccordionGroup`

#### `func AccordionLink`

#### `func Sidebar`

### Variables and Constants

---

## Package `spotlight` (components/spotlight)

### Types

#### type `Item`

Item represents a search result in the Spotlight component.


### Functions

#### `func Spotlight`

Spotlight renders a search dialog component that can be triggered
with a button click or keyboard shortcut.


#### `func SpotlightItems`

SpotlightItems renders a list of search results in the Spotlight component.
If no items are found, it displays a "nothing found" message.


### Variables and Constants

---

## Package `usercomponents` (components/user)

### Types

#### type `LanguageSelectProps`

LanguageSelectProps defines the properties for the LanguageSelect component.


### Functions

#### `func LanguageSelect`

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

#### type `DefaultOptions`

### Functions

#### `func Default`

---

## Package `application` (pkg/application)

### Types

#### type `Application`

Application with a dynamically extendable service registry


#### type `Controller`

#### type `GraphSchema`

#### type `MigrationManager`

MigrationManager is an interface for handling database migrations


#### type `Module`

#### type `SeedFunc`

#### type `Seeder`

### Functions

### Variables and Constants

- Var: `[ErrAppNotFound]`

---

## Package `commands` (pkg/commands)

### Functions

#### `func Migrate`

### Variables and Constants

- Var: `[ErrNoCommand]`

---

## Package `composables` (pkg/composables)

### Types

#### type `PaginationParams`

#### type `Params`

### Functions

#### `func BeginTx`

#### `func CanUser`

#### `func MustT`

MustT returns the translation for the given message ID.
If the translation is not found, it will panic.


#### `func MustUseHead`

MustUseHead returns the head component from the context or panics


#### `func MustUseLocalizer`

MustUseLocalizer returns the localizer from the context.
If the localizer is not found, it will panic.


#### `func MustUseLogo`

MustUseLogo returns the logo component from the context or panics


#### `func MustUseUser`

MustUseUser returns the user from the context. If no user is found, it panics.


#### `func UseAllNavItems`

#### `func UseApp`

UseApp returns the application from the context.


#### `func UseAuthenticated`

UseAuthenticated returns whether the user is authenticated and the second return value is true.
If the user is not authenticated, the second return value is false.


#### `func UseFlash`

#### `func UseFlashMap`

#### `func UseForm`

#### `func UseHead`

UseHead returns the head component from the context


#### `func UseIP`

UseIP returns the IP address from the context.
If the IP address is not found, the second return value will be false.


#### `func UseLocale`

UseLocale returns the locale from the context.
If the locale is not found, the second return value will be false.


#### `func UseLocalizedOrFallback`

#### `func UseLocalizer`

UseLocalizer returns the localizer from the context.
If the localizer is not found, the second return value will be false.


#### `func UseLogger`

UseLogger returns the logger from the context.
If the logger is not found, the second return value will be false.


#### `func UseLogo`

UseLogo returns the logo component from the context


#### `func UseNavItems`

#### `func UsePageCtx`

UsePageCtx returns the page context from the context.
If the page context is not found, function will panic.


#### `func UsePool`

#### `func UseQuery`

#### `func UseRequest`

UseRequest returns the request from the context.
If the request is not found, the second return value will be false.


#### `func UseSession`

UseSession returns the session from the context.


#### `func UseTabs`

#### `func UseTx`

#### `func UseUniLocalizer`

#### `func UseUser`

UseUser returns the user from the context.


#### `func UseUserAgent`

UseUserAgent returns the user agent from the context.
If the user agent is not found, the second return value will be false.


#### `func UseWriter`

UseWriter returns the response writer from the context.
If the response writer is not found, the second return value will be false.


#### `func WithLocalizer`

#### `func WithPageCtx`

WithPageCtx returns a new context with the page context.


#### `func WithParams`

WithParams returns a new context with the request parameters.


#### `func WithPool`

#### `func WithSession`

WithSession returns a new context with the session.


#### `func WithTx`

#### `func WithUser`

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

#### type `Configuration`

##### Methods

- `func (Configuration) Address`

- `func (Configuration) Logger`

- `func (Configuration) LogrusLogLevel`

- `func (Configuration) Scheme`

- `func (Configuration) Unload`
  unload handles a graceful shutdown.
  

#### type `DatabaseOptions`

##### Methods

- `func (DatabaseOptions) ConnectionString`

#### type `GoogleOptions`

#### type `TwilioOptions`

### Functions

#### `func LoadEnv`

### Variables and Constants

- Const: `[Production]`

---

## Package `constants` (pkg/constants)

### Types

#### type `ContextKey`

### Variables and Constants

- Var: `[Validate]`

---

## Package `di` (pkg/di)

### Types

#### type `DIHandler`

DIHandler is a handler that uses dependency injection to resolve its arguments


##### Methods

- `func (DIHandler) Handler`

#### type `Provider`

Provider is an interface that can provide a value for a given type


---

## Package `eventbus` (pkg/eventbus)

### Types

#### type `EventBus`

#### type `Subscriber`

### Functions

#### `func MatchSignature`

---

## Package `fp` (pkg/fp)

### Types

#### type `Lazy`

Callback function that returns a specific value type


#### type `LazyVal`

Callback function that takes an argument and return a value of the same type


### Functions

#### `func Compose10`

Performs right-to-left function composition of 10 functions


#### `func Compose11`

Performs right-to-left function composition of 11 functions


#### `func Compose12`

Performs right-to-left function composition of 12 functions


#### `func Compose13`

Performs right-to-left function composition of 13 functions


#### `func Compose14`

Performs right-to-left function composition of 14 functions


#### `func Compose15`

Performs right-to-left function composition of 15 functions


#### `func Compose16`

Performs right-to-left function composition of 16 functions


#### `func Compose2`

Performs right-to-left function composition of two functions


#### `func Compose3`

Performs right-to-left function composition of three functions


#### `func Compose4`

Performs right-to-left function composition of four functions


#### `func Compose5`

Performs right-to-left function composition of 5 functions


#### `func Compose6`

Performs right-to-left function composition of 6 functions


#### `func Compose7`

Performs right-to-left function composition of 7 functions


#### `func Compose8`

Performs right-to-left function composition of 8 functions


#### `func Compose9`

Performs right-to-left function composition of 9 functions


#### `func Curry10`

Allow to transform a function that receives 10 params in a sequence of unary functions


#### `func Curry11`

Allow to transform a function that receives 11 params in a sequence of unary functions


#### `func Curry12`

Allow to transform a function that receives 12 params in a sequence of unary functions


#### `func Curry13`

Allow to transform a function that receives 13 params in a sequence of unary functions


#### `func Curry14`

Allow to transform a function that receives 14 params in a sequence of unary functions


#### `func Curry15`

Allow to transform a function that receives 15 params in a sequence of unary functions


#### `func Curry16`

Allow to transform a function that receives 16 params in a sequence of unary functions


#### `func Curry2`

Allow to transform a function that receives 2 params in a sequence of unary functions


#### `func Curry3`

Allow to transform a function that receives 3 params in a sequence of unary functions


#### `func Curry4`

Allow to transform a function that receives 4 params in a sequence of unary functions


#### `func Curry5`

Allow to transform a function that receives 5 params in a sequence of unary functions


#### `func Curry6`

Allow to transform a function that receives 6 params in a sequence of unary functions


#### `func Curry7`

Allow to transform a function that receives 7 params in a sequence of unary functions


#### `func Curry8`

Allow to transform a function that receives 8 params in a sequence of unary functions


#### `func Curry9`

Allow to transform a function that receives 9 params in a sequence of unary functions


#### `func Every`

Determines whether all the members of an array satisfy the specified test.


#### `func EveryWithIndex`

See Every but callback receives index of element.


#### `func EveryWithSlice`

Like Every but callback receives index of element and the whole array.


#### `func Filter`

Filter Returns the elements of an array that meet the condition specified in a callback function.


#### `func FilterWithIndex`

FilterWithIndex See Filter but callback receives index of element.


#### `func FilterWithSlice`

FilterWithSlice Like Filter but callback receives index of element and the whole array.


#### `func Flat`

Returns a new array with all sub-array elements concatenated into it recursively up to the specified depth.


#### `func FlatMap`

Calls a defined callback function on each element of an array. Then, flattens the result into a new array. This is identical to a map followed by flat with depth 1.


#### `func FlatMapWithIndex`

See FlatMap but callback receives index of element.


#### `func FlatMapWithSlice`

Like FlatMap but callback receives index of element and the whole array.


#### `func Map`

Calls a defined callback function on each element of an array, and returns an array that contains the results.


#### `func MapWithIndex`

See Map but callback receives index of element.


#### `func MapWithSlice`

Like Map but callback receives index of element and the whole array.


#### `func Pipe10`

Performs left-to-right function composition of 10 functions


#### `func Pipe11`

Performs left-to-right function composition of 11 functions


#### `func Pipe12`

Performs left-to-right function composition of 12 functions


#### `func Pipe13`

Performs left-to-right function composition of 13 functions


#### `func Pipe14`

Performs left-to-right function composition of 14 functions


#### `func Pipe15`

Performs left-to-right function composition of 15 functions


#### `func Pipe16`

Performs left-to-right function composition of 16 functions


#### `func Pipe2`

Performs left-to-right function composition of two functions


#### `func Pipe3`

Performs left-to-right function composition of three functions


#### `func Pipe4`

Performs left-to-right function composition of four functions


#### `func Pipe5`

Performs left-to-right function composition of five functions


#### `func Pipe6`

Performs left-to-right function composition of 6 functions


#### `func Pipe7`

Performs left-to-right function composition of 7 functions


#### `func Pipe8`

Performs left-to-right function composition of 8 functions


#### `func Pipe9`

Performs left-to-right function composition of 9 functions


#### `func Reduce`

Reduce Calls the specified callback function for all the elements in an array. The return value of the callback function is the accumulated result, and is provided as an argument in the next call to the callback function.


#### `func ReduceWithIndex`

ReduceWithIndex See Reduce but callback receives index of element.


#### `func ReduceWithSlice`

ReduceWithSlice Like Reduce but callback receives index of element and the whole array.


#### `func Some`

Determines whether the specified callback function returns true for any element of an array.


#### `func SomeWithIndex`

See Some but callback receives index of element.


#### `func SomeWithSlice`

Like Some but callback receives index of element and the whole array.


---

## Package `either` (pkg/fp/either)

### Types

#### type `Either`

BaseError struct


### Functions

#### `func Exists`

Returns `false` if `Left` or returns the boolean result of the application of the given predicate to the `Right` value


#### `func FromOption`

Constructor of Either from an Option.
Returns a Left in case of None storing the callback return value as the error argument
Returns a Right in case of Some with the option value.


#### `func FromPredicate`

Constructor of Either from a predicate.
Returns a Left if the predicate function over the value return false.
Returns a Right if the predicate function over the value return true.


#### `func GetOrElse`

Extracts the value out of the Either, if it exists. Otherwise returns the result of the callback function that takes the error as argument.


#### `func IsLeft`

Helper to check if the Either has an error


#### `func IsRight`

Helper to check if the Either has a value


#### `func Map`

Map over the Either value if it exists. Otherwise return the Either itself


#### `func MapLeft`

Map over the Either error if it exists. Otherwise return the Either with the new error type


#### `func Match`

Extracts the value out of the Either.
Returns a new type running the succes or error callbacks which are taking respectively the error or value as an argument.


---

## Package `opt` (pkg/fp/option)

### Types

#### type `Option`

BaseError struct


### Functions

#### `func Chain`

Execute a function that returns an Option on the Option value if it exists. Otherwise return the empty Option itself


#### `func Exists`

Returns `false` if `None` or returns the boolean result of the application of the given predicate to the `Some` value


#### `func FromPredicate`

Constructor of Option from a predicate.
Returns a None if the predicate function over the value return false.
Returns a Some if the predicate function over the value return true.


#### `func GetOrElse`

Extracts the value out of the Option, if it exists. Otherwise returns the function with a default value


#### `func IsNone`

Helper to check if the Option is missing the value


#### `func IsSome`

Helper to check if the Option has a value


#### `func Map`

Execute the function on the Option value if it exists. Otherwise return the empty Option itself


#### `func Match`

Extracts the value out of the Option, if it exists, with a function. Otherwise returns the function with a default value


---

## Package `graphql` (pkg/graphql)

### Types

#### type `FieldFunc`

##### Methods

- `func (FieldFunc) ExtensionName`

- `func (FieldFunc) InterceptField`

- `func (FieldFunc) Validate`

#### type `Handler`

##### Methods

- `func (Handler) AddExecutor`

- `func (Handler) AddTransport`

- `func (Handler) AroundFields`
  AroundFields is a convenience method for creating an extension that only implements field middleware
  

- `func (Handler) AroundOperations`
  AroundOperations is a convenience method for creating an extension that only implements operation middleware
  

- `func (Handler) AroundResponses`
  AroundResponses is a convenience method for creating an extension that only implements response middleware
  

- `func (Handler) AroundRootFields`
  AroundRootFields is a convenience method for creating an extension that only implements field middleware
  

- `func (Handler) ServeHTTP`

- `func (Handler) SetDisableSuggestion`

- `func (Handler) SetErrorPresenter`

- `func (Handler) SetParserTokenLimit`

- `func (Handler) SetQueryCache`

- `func (Handler) SetRecoverFunc`

- `func (Handler) Use`

#### type `MyPOST`

##### Methods

- `func (MyPOST) Do`

- `func (MyPOST) Supports`

#### type `OperationFunc`

##### Methods

- `func (OperationFunc) ExtensionName`

- `func (OperationFunc) InterceptOperation`

- `func (OperationFunc) Validate`

#### type `Resolver`

#### type `ResponseFunc`

##### Methods

- `func (ResponseFunc) ExtensionName`

- `func (ResponseFunc) InterceptResponse`

- `func (ResponseFunc) Validate`

### Functions

### Variables and Constants

---

## Package `htmx` (pkg/htmx)

### Functions

#### `func CurrentUrl`

CurrentUrl retrieves the current URL of the browser from the HX-Current-URL request header.


#### `func IsBoosted`

IsBoosted checks if the request was triggered by an element with hx-boost.


#### `func IsHistoryRestoreRequest`

IsHistoryRestoreRequest checks if the request is for history restoration after a miss in the local history cache.


#### `func IsHxRequest`

IsHxRequest checks if the request is an HTMX request.


#### `func Location`

Location sets the HX-Location header to trigger a client-side navigation.


#### `func PromptResponse`

PromptResponse retrieves the user's response to an hx-prompt from the HX-Prompt request header.


#### `func PushUrl`

PushUrl sets the HX-Push-Url header to push a new URL into the browser history stack.


#### `func Redirect`

Redirect sets the HX-Redirect header to redirect the client to a new URL.


#### `func Refresh`

Refresh sets the HX-Refresh header to true, instructing the client to perform a full page refresh.


#### `func ReplaceUrl`

ReplaceUrl sets the HX-Replace-Url header to replace the current URL in the browser location bar.


#### `func Reselect`

Reselect sets the HX-Reselect header to specify which part of the response should be swapped in.


#### `func Reswap`

Reswap sets the HX-Reswap header to specify how the response will be swapped.


#### `func Retarget`

Retarget sets the HX-Retarget header to specify a new target element.


#### `func SetTrigger`

Trigger sets the HX-Trigger header to trigger client-side events.


#### `func Target`

Target returns the ID of the element that triggered the request.


#### `func Trigger`

Trigger retrieves the ID of the triggered element from the HX-Trigger request header.


#### `func TriggerAfterSettle`

TriggerAfterSettle sets the HX-Trigger-After-Settle header to trigger client-side events after the settle step.


#### `func TriggerAfterSwap`

TriggerAfterSwap sets the HX-Trigger-After-Swap header to trigger client-side events after the swap step.


#### `func TriggerName`

TriggerName retrieves the name of the triggered element from the HX-Trigger-Name request header.


---

## Package `client1c` (pkg/integrations/1c)

### Types

#### type `Client`

##### Methods

- `func (Client) GetOdataServices`

#### type `OdataService`

#### type `OdataServices`

---

## Package `intl` (pkg/intl)

### Types

#### type `SupportedLanguage`

### Variables and Constants

- Var: `[SupportedLanguages]`

---

## Package `llm` (pkg/llm)

---

## Package `functions` (pkg/llm/gpt-functions)

### Types

#### type `ChatFunctionDefinition`

#### type `ChatTools`

##### Methods

- `func (ChatTools) Add`

- `func (ChatTools) Call`

- `func (ChatTools) Funcs`

- `func (ChatTools) OpenAiTools`

#### type `Column`

#### type `CompletionFunc`

#### type `DBColumn`

#### type `Enum`

#### type `Ref`

#### type `Table`

### Functions

#### `func GetFkRelations`

#### `func GetTables`

---

## Package `logging` (pkg/logging)

### Functions

#### `func ConsoleLogger`

#### `func FileLogger`

---

## Package `mapping` (pkg/mapping)

### Functions

#### `func MapDBModels`

MapDBModels maps entities to db models


#### `func MapViewModels`

MapViewModels maps entities to view models


#### `func Pointer`

Pointer is a utility function that returns a pointer to the given value.


#### `func PointerSlice`

PointerSlice is a utility function that returns a slice of pointers from a slice of values.


#### `func PointerToSQLNullString`

#### `func PointerToSQLNullTime`

#### `func SQLNullTimeToPointer`

#### `func Value`

Value is a utility function that returns the value of the given pointer.


#### `func ValueSlice`

ValueSlice is a utility function that returns a slice of values from a slice of pointers.


#### `func ValueToSQLNullFloat64`

#### `func ValueToSQLNullInt32`

#### `func ValueToSQLNullInt64`

#### `func ValueToSQLNullString`

#### `func ValueToSQLNullTime`

---

## Package `middleware` (pkg/middleware)

### Types

#### type `GenericConstructor`

### Functions

#### `func Authorize`

#### `func ContextKeyValue`

#### `func Cors`

#### `func NavItems`

#### `func Provide`

#### `func ProvideUser`

#### `func RedirectNotAuthenticated`

#### `func RequestParams`

#### `func RequireAuthorization`

#### `func Tabs`

#### `func WithLocalizer`

#### `func WithLogger`

#### `func WithPageContext`

#### `func WithTransaction`

### Variables and Constants

- Var: `[AllowMethods]`

---

## Package `multifs` (pkg/multifs)

Package multifs MultiHashFS combines multiple hashfs instances to serve files from each.


### Types

#### type `MultiHashFS`

##### Methods

- `func (MultiHashFS) Open`
  Open attempts to open a file from any of the hashfs instances.
  

---

## Package `repo` (pkg/repo)

Package repo provides database utility functions and interfaces for working with PostgreSQL.


### Types

#### type `Expr`

Expr represents a comparison expression type for filtering queries.


#### type `ExtendedFieldSet`

ExtendedFieldSet is an interface that must be implemented to persist custom fields with a repository.
It allows repositories to work with custom field sets by providing field names and values.


#### type `Filter`

Filter defines a filter condition for queries.
Combines an expression type with a value to be used in WHERE clauses.


#### type `SortBy`

SortBy defines sorting criteria for queries with generic field type support.
Use with OrderBy function to generate ORDER BY clauses.


#### type `Tx`

Tx is an interface that abstracts database transaction operations.
It provides a subset of pgx.Tx functionality needed for common database operations.


### Functions

#### `func BatchInsertQueryN`

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


#### `func FormatLimitOffset`

FormatLimitOffset generates SQL LIMIT and OFFSET clauses based on the provided values.

If both limit and offset are positive, it returns "LIMIT x OFFSET y".
If only limit is positive, it returns "LIMIT x".
If only offset is positive, it returns "OFFSET y".
If neither is positive, it returns an empty string.

Example usage:

	query := "SELECT * FROM users " + repo.FormatLimitOffset(10, 20)
	// Returns: "SELECT * FROM users LIMIT 10 OFFSET 20"


#### `func Insert`

Insert creates a parameterized SQL query for inserting a single row.
Optionally returns specified columns with the RETURNING clause.

Example usage:

	query := repo.Insert("users", []string{"name", "email", "password"}, "id", "created_at")
	// Returns: "INSERT INTO users (name, email, password) VALUES ($1, $2, $3) RETURNING id, created_at"


#### `func Join`

Join combines multiple SQL expressions with spaces between them.

Example usage:

	query := repo.Join("SELECT *", "FROM users", "WHERE active = true")
	// Returns: "SELECT * FROM users WHERE active = true"


#### `func JoinWhere`

JoinWhere creates an SQL WHERE clause by joining multiple conditions with AND.

Example usage:

	conditions := []string{"status = $1", "created_at > $2"}
	query := "SELECT * FROM orders " + repo.JoinWhere(conditions...)
	// Returns: "SELECT * FROM orders WHERE status = $1 AND created_at > $2"


#### `func OrderBy`

OrderBy generates an SQL ORDER BY clause for the given fields and sort direction.
Returns an empty string if no fields are provided.

Example usage:

	query := "SELECT * FROM users " + repo.OrderBy([]string{"created_at", "name"}, false)
	// Returns: "SELECT * FROM users ORDER BY created_at, name DESC"


#### `func Update`

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

#### type `ContentAdapter`

ContentAdapter adapts scaffold.Content to support search and pagination


##### Methods

- `func (ContentAdapter) Render`
  Render implements templ.Component interface
  

#### type `LayoutAdapter`

LayoutAdapter adapts a content component with a layout


##### Methods

- `func (LayoutAdapter) Render`
  Render implements templ.Component interface
  

#### type `TableAdapter`

TableAdapter adapts scaffold.Table to support pagination


##### Methods

- `func (TableAdapter) Render`
  Render implements templ.Component interface
  

#### type `TableControllerBuilder`

TableControllerBuilder helps to quickly build controllers for displaying tables


##### Methods

- `func (TableControllerBuilder) Key`

- `func (TableControllerBuilder) List`
  List handles listing entities in a table
  

- `func (TableControllerBuilder) Register`
  Register registers the table route
  

- `func (TableControllerBuilder) WithFindParamsFunc`
  WithFindParamsFunc sets a custom function for creating find parameters
  

#### type `TableService`

TableService defines the minimal interface for table data services


#### type `TableViewModel`

TableViewModel defines the interface for mapping entity to view model


### Functions

#### `func ExtendedContent`

ExtendedContent creates a content component with search and pagination


#### `func ExtendedTable`

ExtendedTable creates a table with pagination support


#### `func PageWithLayout`

PageWithLayout wraps content with a layout


---

## Package `collector` (pkg/schema/collector)

### Types

#### type `Collector`

##### Methods

- `func (Collector) CollectMigrations`

- `func (Collector) StoreMigrations`

#### type `Config`

#### type `FileLoader`

##### Methods

- `func (FileLoader) LoadExistingSchema`

- `func (FileLoader) LoadModuleSchema`

#### type `LoaderConfig`

#### type `SchemaLoader`

### Functions

#### `func CollectSchemaChanges`

CollectSchemaChanges compares two schemas and generates both up and down change sets


#### `func CompareTables`

---

## Package `common` (pkg/schema/common)

### Types

#### type `ChangeSet`

ChangeSet represents a collection of related schema changes


#### type `Schema`

Schema represents a database schema containing all objects


#### type `SchemaObject`

SchemaObject represents a generic schema object that can be different types
from the postgresql-parser tree package


### Functions

#### `func AllReferencesSatisfied`

#### `func HasReferences`

#### `func SortTableDefs`

---

## Package `serrors` (pkg/serrors)

### Types

#### type `Base`

#### type `BaseError`

##### Methods

- `func (BaseError) Error`

- `func (BaseError) Localize`

- `func (BaseError) WithTemplateData`
  WithTemplateData adds template data to the error for localization
  

#### type `ValidationError`

ValidationError represents a field validation error


##### Methods

- `func (ValidationError) WithDetails`
  WithDetails adds error details to the template data
  

- `func (ValidationError) WithFieldName`
  WithFieldName adds the field name to the template data
  

#### type `ValidationErrors`

ValidationErrors is a map of field names to validation errors


### Functions

#### `func LocalizeValidationErrors`

LocalizeValidationErrors localizes all validation errors in the map


#### `func UnauthorizedGQLError`

#### `func Unmarshal`

---

## Package `server` (pkg/server)

### Types

#### type `HTTPServer`

##### Methods

- `func (HTTPServer) Start`

### Functions

#### `func WsHub`

### Variables and Constants

- Const: `[ChannelChat]`

---

## Package `shared` (pkg/shared)

### Types

#### type `DateOnly`

#### type `FormAction`

##### Methods

- `func (FormAction) IsValid`

### Functions

#### `func ParseID`

#### `func Redirect`

#### `func SetFlash`

#### `func SetFlashMap`

### Variables and Constants

- Var: `[Decoder]`

- Var: `[Encoder]`

---

## Package `spotlight` (pkg/spotlight)

Package spotlight is a package that provides a way to show a list of items in a spotlight.


### Types

#### type `Item`

#### type `Spotlight`

---

## Package `testutils` (pkg/testutils)

### Types

#### type `TestFixtures`

### Functions

#### `func CreateDB`

#### `func DbOpts`

#### `func DefaultParams`

#### `func MockSession`

#### `func MockUser`

#### `func NewPool`

#### `func SetupApplication`

---

## Package `tgserver` (pkg/tgServer)

### Types

#### type `DBSession`

##### Methods

- `func (DBSession) LoadSession`
  LoadSession loads session from memory.
  

- `func (DBSession) StoreSession`
  StoreSession stores session to memory.
  

#### type `Server`

##### Methods

- `func (Server) Start`

---

## Package `types` (pkg/types)

### Types

#### type `NavigationItem`

##### Methods

- `func (NavigationItem) HasPermission`

#### type `PageContext`

##### Methods

- `func (PageContext) T`

#### type `PageData`

---

## Package `ws` (pkg/ws)

### Types

#### type `Connection`

##### Methods

- `func (Connection) Channels`

- `func (Connection) Close`

- `func (Connection) GetContext`

- `func (Connection) SendMessage`

- `func (Connection) Session`

- `func (Connection) SetContext`

- `func (Connection) Subscribe`

- `func (Connection) Unsubscribe`

- `func (Connection) UserID`

#### type `Connectioner`

#### type `Hub`

##### Methods

- `func (Hub) BroadcastToAll`

- `func (Hub) BroadcastToChannel`

- `func (Hub) BroadcastToUser`

- `func (Hub) ConnectionsAll`

- `func (Hub) ConnectionsInChannel`

- `func (Hub) ServeHTTP`

#### type `Huber`

#### type `Set`

#### type `SubscriptionMessage`

### Variables and Constants

---

## Package `main` (tools)

### Types

#### type `Config`

#### type `JSONKeys`

#### type `KeyStore`

Add a mutex to protect our key operations


#### type `LintError`

##### Methods

- `func (LintError) Error`

#### type `LinterConfig`

### Functions

### Variables and Constants

- Var: `[JSONLinter]`

---

