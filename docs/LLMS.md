# IOTA SDK Documentation

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

## Package `modules` (modules)

### Functions

#### `func Load`

### Variables and Constants

- Var: `[BuiltInModules NavLinks]`

---

## Package `bichat` (modules/bichat)

### Types

#### type `Module`

##### Methods

- `func (Module) Name`

- `func (Module) Register`

### Functions

#### `func NewModule`

### Variables and Constants

- Var: `[BiChatLink]`

- Var: `[LocaleFiles]`

- Var: `[MigrationFiles]`

- Var: `[NavItems]`

---

## Package `dialogue` (modules/bichat/domain/entities/dialogue)

### Types

#### type `CreatedEvent`

#### type `DeletedEvent`

#### type `Dialogue`

#### type `FindParams`

#### type `Messages`

#### type `Reply`

#### type `Repository`

#### type `Start`

#### type `UpdatedEvent`

---

## Package `embedding` (modules/bichat/domain/entities/embedding)

### Types

#### type `SearchResult`

---

## Package `llm` (modules/bichat/domain/entities/llm)

### Types

#### type `ChatCompletionMessage`

#### type `ChatCompletionRequest`

ChatCompletionRequest represents a request structure for chat completion API.


#### type `ChatCompletionResponseFormat`

#### type `ChatCompletionResponseFormatJSONSchema`

#### type `ChatCompletionResponseFormatType`

#### type `ChatMessageImageURL`

#### type `ChatMessagePart`

#### type `ChatMessagePartType`

#### type `FunctionCall`

#### type `FunctionDefinition`

#### type `ImageURLDetail`

#### type `StreamOptions`

#### type `Tool`

#### type `ToolCall`

#### type `ToolChoice`

#### type `ToolFunction`

#### type `ToolType`

---

## Package `prompt` (modules/bichat/domain/entities/prompt)

### Types

#### type `Prompt`

#### type `Repository`

---

## Package `llmproviders` (modules/bichat/infrastructure/llmproviders)

### Types

#### type `OpenAIProvider`

##### Methods

- `func (OpenAIProvider) CreateChatCompletionStream`

### Functions

#### `func DomainChatCompletionMessageToOpenAI`

#### `func DomainFuncCallToOpenAI`

#### `func DomainFuncDefinitionToOpenAI`

#### `func DomainImageURLToOpenAI`

#### `func DomainMessagePartToOpenAI`

#### `func DomainToOpenAIChatCompletionRequest`

#### `func DomainToolCallToOpenAI`

#### `func DomainToolToOpenAI`

#### `func OpenAIChatCompletionMessageToDomain`

#### `func OpenAIToDomainFuncCall`

#### `func OpenAIToDomainImageURL`

#### `func OpenAIToDomainMessagePart`

#### `func OpenAIToDomainToolCall`

---

## Package `persistence` (modules/bichat/infrastructure/persistence)

### Types

#### type `GormDialogueRepository`

##### Methods

- `func (GormDialogueRepository) Count`

- `func (GormDialogueRepository) Create`

- `func (GormDialogueRepository) Delete`

- `func (GormDialogueRepository) GetAll`

- `func (GormDialogueRepository) GetByID`

- `func (GormDialogueRepository) GetByUserID`

- `func (GormDialogueRepository) GetPaginated`

- `func (GormDialogueRepository) Update`

### Functions

#### `func NewDialogueRepository`

### Variables and Constants

- Var: `[ErrDialogueNotFound]`

---

## Package `models` (modules/bichat/infrastructure/persistence/models)

### Types

#### type `Dialogue`

#### type `Prompt`

---

## Package `controllers` (modules/bichat/presentation/controllers)

### Types

#### type `BiChatController`

##### Methods

- `func (BiChatController) Create`

- `func (BiChatController) Delete`

- `func (BiChatController) Index`

- `func (BiChatController) Key`

- `func (BiChatController) Register`

### Functions

#### `func NewBiChatController`

---

## Package `dtos` (modules/bichat/presentation/controllers/dtos)

### Types

#### type `MessageDTO`

---

## Package `bichat` (modules/bichat/presentation/templates/pages/bichat)

templ: version: v0.3.819


### Types

#### type `ChatPageProps`

#### type `HistoryItem`

### Functions

#### `func BiChatPage`

#### `func ChatSideBar`

#### `func Index`

#### `func ModelSelect`

### Variables and Constants

---

## Package `services` (modules/bichat/services)

### Types

#### type `DialogueService`

##### Methods

- `func (DialogueService) ChatComplete`

- `func (DialogueService) Count`

- `func (DialogueService) Delete`

- `func (DialogueService) GetAll`

- `func (DialogueService) GetByID`

- `func (DialogueService) GetPaginated`

- `func (DialogueService) GetUserDialogues`

- `func (DialogueService) ReplyToDialogue`

- `func (DialogueService) StartDialogue`

- `func (DialogueService) Update`

#### type `EmbeddingService`

##### Methods

- `func (EmbeddingService) Search`

#### type `PromptService`

##### Methods

- `func (PromptService) Count`

- `func (PromptService) Create`

- `func (PromptService) Delete`

- `func (PromptService) GetAll`

- `func (PromptService) GetByID`

- `func (PromptService) GetPaginated`

- `func (PromptService) Update`

### Functions

#### `func NewSearchKnowledgeBase`

### Variables and Constants

- Var: `[ErrMessageTooLong ErrModelRequired]`

---

## Package `chatfuncs` (modules/bichat/services/chatfuncs)

### Types

### Functions

#### `func GetExchangeRate`

#### `func NewCurrencyConvert`

#### `func NewDoSQLQuery`

#### `func NewUnitConversion`

#### `func UnitConversion`

### Variables and Constants

- Var: `[SupportedCurrencies]`

- Var: `[SupportedUnits]`

---

## Package `core` (modules/core)

### Types

#### type `Module`

##### Methods

- `func (Module) Name`

- `func (Module) Register`

### Functions

#### `func NewModule`

### Variables and Constants

- Var: `[AdministrationLink]`

- Var: `[DashboardLink]`

- Var: `[LocaleFiles]`

- Var: `[MigrationFiles]`

- Var: `[NavItems]`

- Var: `[RolesLink]`

- Var: `[UsersLink]`

---

## Package `group` (modules/core/domain/aggregates/group)

### Types

#### type `CreatedEvent`

#### type `DeletedEvent`

#### type `Field`

#### type `FindParams`

#### type `Group`

#### type `Option`

#### type `Repository`

#### type `SortBy`

#### type `UpdatedEvent`

#### type `UserAddedEvent`

#### type `UserRemovedEvent`

---

## Package `project` (modules/core/domain/aggregates/project)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DeletedEvent`

#### type `Project`

#### type `Repository`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

---

## Package `role` (modules/core/domain/aggregates/role)

### Types

#### type `Field`

#### type `FindParams`

#### type `Repository`

#### type `Role`

#### type `SortBy`

---

## Package `user` (modules/core/domain/aggregates/user)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DeletedEvent`

#### type `Field`

#### type `FindParams`

#### type `Option`

#### type `Repository`

#### type `SortBy`

#### type `UILanguage`

##### Methods

- `func (UILanguage) IsValid`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

#### type `User`

---

## Package `authlog` (modules/core/domain/entities/authlog)

### Types

#### type `AuthenticationLog`

#### type `FindParams`

#### type `Repository`

---

## Package `costcomponent` (modules/core/domain/entities/costcomponent)

### Types

#### type `BillableHourEntity`

#### type `CostComponent`

#### type `ExpenseComponent`

#### type `UnifiedHourlyRateResult`

### Variables and Constants

- Var: `[HoursInMonth]`

---

## Package `currency` (modules/core/domain/entities/currency)

### Types

#### type `Code`

TODO: make this private


##### Methods

- `func (Code) IsValid`

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `Currency`

##### Methods

- `func (Currency) Ok`

#### type `DeletedEvent`

#### type `Field`

#### type `FindParams`

#### type `Repository`

#### type `SortBy`

#### type `Symbol`

TODO: make this private


##### Methods

- `func (Symbol) IsValid`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

### Variables and Constants

- Var: `[USD EUR TRY GBP RUB JPY CNY SOM AUD CAD CHF]`

- Var: `[ValidCodes ValidSymbols Currencies]`

---

## Package `passport` (modules/core/domain/entities/passport)

### Types

#### type `Option`

Option is a function type that configures a passport


#### type `Passport`

#### type `Repository`

---

## Package `permission` (modules/core/domain/entities/permission)

### Types

#### type `Action`

#### type `Field`

#### type `FindParams`

#### type `Modifier`

#### type `Permission`

##### Methods

- `func (Permission) Equals`

#### type `RBAC`

#### type `Repository`

#### type `Resource`

#### type `SortBy`

### Variables and Constants

- Var: `[ErrPermissionNotFound]`

---

## Package `session` (modules/core/domain/entities/session)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DeletedEvent`

#### type `Field`

#### type `FindParams`

#### type `Repository`

#### type `Session`

##### Methods

- `func (Session) IsExpired`

#### type `SortBy`

---

## Package `tab` (modules/core/domain/entities/tab)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `FindParams`

#### type `Repository`

#### type `Tab`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

---

## Package `telegramsession` (modules/core/domain/entities/telegramsession)

### Types

#### type `TelegramSession`

##### Methods

- `func (TelegramSession) ToGraph`

---

## Package `upload` (modules/core/domain/entities/upload)

Package upload README: Commented out everything until I find a way to solve import cycles.


### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DeletedEvent`

#### type `Field`

#### type `FindParams`

#### type `Repository`

#### type `Size`

#### type `SortBy`

#### type `Storage`

#### type `Upload`

#### type `UploadType`

##### Methods

- `func (UploadType) String`

---

## Package `country` (modules/core/domain/value_objects/country)

### Types

#### type `Country`

### Functions

#### `func IsValid`

IsValid checks if a given country code is valid.


### Variables and Constants

- Var: `[ErrInvalidCountry NilCountry]`

- Var: `[AllCountries]`

---

## Package `general` (modules/core/domain/value_objects/general)

### Types

#### type `Gender`

#### type `GenderEnum`

##### Methods

- `func (GenderEnum) String`

### Functions

#### `func IsValid`

IsValid checks if a given country code is valid.


### Variables and Constants

- Var: `[ErrInvalidGender NilGender]`

---

## Package `internet` (modules/core/domain/value_objects/internet)

### Types

#### type `Email`

#### type `IP`

#### type `IpVersion`

### Functions

#### `func IsValidEmail`

#### `func IsValidIP`

### Variables and Constants

- Var: `[ErrInvalidEmail]`

- Var: `[ErrInvalidIP]`

---

## Package `money` (modules/core/domain/value_objects/money)

### Types

#### type `Amount`

---

## Package `phone` (modules/core/domain/value_objects/phone)

### Types

#### type `AreaCode`

AreaCode represents the mapping between area codes and countries


#### type `Phone`

### Functions

#### `func IsValidGlobalPhoneNumber`

#### `func IsValidPhoneNumber`

#### `func IsValidUSPhoneNumber`

#### `func ParseCountry`

ParseCountry attempts to determine the country from a phone number


#### `func Strip`

### Variables and Constants

- Var: `[ErrInvalidPhoneNumber ErrUnknownCountry]`

- Var: `[PhoneCodeToCountry]`
  PhoneCodeToCountry maps phone number prefixes to their respective countries
  

---

## Package `tax` (modules/core/domain/value_objects/tax)

### Types

#### type `Pin`

Pin - Personal Identification Number (ПИНФЛ - Персональный идентификационный номер физического лица)


#### type `Tin`

Tin - Taxpayer Identification Number (ИНН - Идентификационный номер налогоплательщика)


### Functions

#### `func IsValidPin`

#### `func ValidateTin`

### Variables and Constants

- Var: `[ErrInvalidPin]`

- Var: `[ErrInvalidTin]`

---

## Package `handlers` (modules/core/handlers)

### Types

#### type `ActionLogEventHandler`

##### Methods

#### type `SessionEventsHandler`

##### Methods

---

## Package `persistence` (modules/core/infrastructure/persistence)

### Types

#### type `FSStorage`

##### Methods

- `func (FSStorage) Open`

- `func (FSStorage) Save`

#### type `GormAuthLogRepository`

##### Methods

- `func (GormAuthLogRepository) Count`

- `func (GormAuthLogRepository) Create`

- `func (GormAuthLogRepository) Delete`

- `func (GormAuthLogRepository) GetAll`

- `func (GormAuthLogRepository) GetByID`

- `func (GormAuthLogRepository) GetPaginated`

- `func (GormAuthLogRepository) Update`

#### type `GormCurrencyRepository`

##### Methods

- `func (GormCurrencyRepository) Count`

- `func (GormCurrencyRepository) Create`

- `func (GormCurrencyRepository) CreateOrUpdate`

- `func (GormCurrencyRepository) Delete`

- `func (GormCurrencyRepository) GetAll`

- `func (GormCurrencyRepository) GetByCode`

- `func (GormCurrencyRepository) GetPaginated`

- `func (GormCurrencyRepository) Update`

#### type `GormPermissionRepository`

##### Methods

- `func (GormPermissionRepository) Count`

- `func (GormPermissionRepository) Delete`

- `func (GormPermissionRepository) GetAll`

- `func (GormPermissionRepository) GetByID`

- `func (GormPermissionRepository) GetPaginated`

- `func (GormPermissionRepository) Save`

#### type `GormRoleRepository`

##### Methods

- `func (GormRoleRepository) Count`

- `func (GormRoleRepository) Create`

- `func (GormRoleRepository) Delete`

- `func (GormRoleRepository) GetAll`

- `func (GormRoleRepository) GetByID`

- `func (GormRoleRepository) GetPaginated`

- `func (GormRoleRepository) Update`

#### type `GormSessionRepository`

##### Methods

- `func (GormSessionRepository) Count`

- `func (GormSessionRepository) Create`

- `func (GormSessionRepository) Delete`

- `func (GormSessionRepository) GetAll`

- `func (GormSessionRepository) GetByToken`

- `func (GormSessionRepository) GetPaginated`

- `func (GormSessionRepository) Update`

#### type `GormUploadRepository`

##### Methods

- `func (GormUploadRepository) Count`

- `func (GormUploadRepository) Create`

- `func (GormUploadRepository) Delete`

- `func (GormUploadRepository) GetAll`

- `func (GormUploadRepository) GetByHash`

- `func (GormUploadRepository) GetByID`

- `func (GormUploadRepository) GetPaginated`

- `func (GormUploadRepository) Update`

#### type `PassportRepository`

##### Methods

- `func (PassportRepository) Create`

- `func (PassportRepository) Delete`

- `func (PassportRepository) Exists`

- `func (PassportRepository) GetByID`

- `func (PassportRepository) GetByPassportNumber`

- `func (PassportRepository) Save`

- `func (PassportRepository) Update`

#### type `PgGroupRepository`

##### Methods

- `func (PgGroupRepository) Count`

- `func (PgGroupRepository) Delete`

- `func (PgGroupRepository) Exists`

- `func (PgGroupRepository) GetByID`

- `func (PgGroupRepository) GetPaginated`

- `func (PgGroupRepository) Save`

#### type `PgUserRepository`

##### Methods

- `func (PgUserRepository) Count`

- `func (PgUserRepository) Create`

- `func (PgUserRepository) Delete`

- `func (PgUserRepository) GetAll`

- `func (PgUserRepository) GetByEmail`

- `func (PgUserRepository) GetByID`

- `func (PgUserRepository) GetByPhone`

- `func (PgUserRepository) GetPaginated`

- `func (PgUserRepository) Update`

- `func (PgUserRepository) UpdateLastAction`

- `func (PgUserRepository) UpdateLastLogin`

### Functions

#### `func BuildGroupFilters`

#### `func BuildUserFilters`

#### `func NewAuthLogRepository`

#### `func NewCurrencyRepository`

#### `func NewGroupRepository`

#### `func NewPassportRepository`

#### `func NewPermissionRepository`

#### `func NewRoleRepository`

#### `func NewSessionRepository`

#### `func NewTabRepository`

#### `func NewUploadRepository`

#### `func NewUserRepository`

#### `func ToDBCurrency`

#### `func ToDBGroup`

#### `func ToDBPassport`

#### `func ToDBTab`

#### `func ToDBUpload`

#### `func ToDomainCurrency`

#### `func ToDomainGroup`

#### `func ToDomainPassport`

Passport mappers


#### `func ToDomainPin`

#### `func ToDomainTab`

#### `func ToDomainTin`

#### `func ToDomainUpload`

#### `func ToDomainUser`

### Variables and Constants

- Var: `[ErrAuthlogNotFound]`

- Var: `[ErrCurrencyNotFound]`

- Var: `[ErrGroupNotFound]`

- Var: `[ErrPassportNotFound]`

- Var: `[ErrPermissionNotFound]`

- Var: `[ErrRoleNotFound]`

- Var: `[ErrSessionNotFound]`

- Var: `[ErrTabNotFound]`

- Var: `[ErrUploadNotFound]`

- Var: `[ErrUserNotFound]`

---

## Package `models` (modules/core/infrastructure/persistence/models)

### Types

#### type `AuthenticationLog`

#### type `Company`

#### type `Currency`

#### type `Group`

#### type `GroupRole`

#### type `GroupUser`

#### type `Passport`

#### type `Permission`

#### type `Role`

#### type `RolePermission`

#### type `Session`

#### type `Tab`

#### type `Upload`

#### type `UploadedImage`

#### type `User`

#### type `UserRole`

---

## Package `graph` (modules/core/interfaces/graph)

### Types

#### type `ComplexityRoot`

#### type `Config`

#### type `DirectiveRoot`

#### type `MutationResolver`

#### type `QueryResolver`

#### type `Resolver`

##### Methods

- `func (Resolver) Mutation`
  Mutation returns MutationResolver implementation.
  

- `func (Resolver) Query`
  Query returns QueryResolver implementation.
  

- `func (Resolver) Subscription`
  Subscription returns SubscriptionResolver implementation.
  

#### type `ResolverRoot`

#### type `SubscriptionResolver`

### Functions

#### `func NewExecutableSchema`

NewExecutableSchema creates an ExecutableSchema from the ResolverRoot interface.


### Variables and Constants

---

## Package `model` (modules/core/interfaces/graph/gqlmodels)

### Types

#### type `Mutation`

#### type `PaginatedUsers`

#### type `Query`

#### type `Session`

#### type `Subscription`

#### type `Upload`

#### type `UploadFilter`

#### type `User`

---

## Package `mappers` (modules/core/interfaces/graph/mappers)

### Functions

#### `func SessionToGraphModel`

#### `func UploadToGraphModel`

#### `func UserToGraphModel`

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

#### type `AccountController`

##### Methods

- `func (AccountController) Get`

- `func (AccountController) GetSettings`

- `func (AccountController) Key`

- `func (AccountController) PostSettings`

- `func (AccountController) Register`

- `func (AccountController) Update`

#### type `DIEmployeeController`

##### Methods

- `func (DIEmployeeController) Key`

- `func (DIEmployeeController) Register`

#### type `DashboardController`

##### Methods

- `func (DashboardController) Get`

- `func (DashboardController) Key`

- `func (DashboardController) Register`

#### type `GraphQLController`

##### Methods

- `func (GraphQLController) Key`

- `func (GraphQLController) Register`

#### type `LoginController`

##### Methods

- `func (LoginController) Get`

- `func (LoginController) GoogleCallback`

- `func (LoginController) Key`

- `func (LoginController) Post`

- `func (LoginController) Register`

#### type `LoginDTO`

##### Methods

- `func (LoginDTO) Ok`

#### type `LogoutController`

##### Methods

- `func (LogoutController) Key`

- `func (LogoutController) Logout`

- `func (LogoutController) Register`

#### type `RolesController`

##### Methods

- `func (RolesController) Create`

- `func (RolesController) Delete`

- `func (RolesController) GetEdit`

- `func (RolesController) GetNew`

- `func (RolesController) Key`

- `func (RolesController) List`

- `func (RolesController) Register`

- `func (RolesController) Update`

#### type `SpotlightController`

##### Methods

- `func (SpotlightController) Get`

- `func (SpotlightController) Key`

- `func (SpotlightController) Register`

#### type `StaticFilesController`

##### Methods

- `func (StaticFilesController) Key`

- `func (StaticFilesController) Register`

#### type `UploadController`

##### Methods

- `func (UploadController) Create`

- `func (UploadController) Key`

- `func (UploadController) Register`

#### type `UserRealtimeUpdates`

##### Methods

- `func (UserRealtimeUpdates) Register`

#### type `UsersController`

##### Methods

- `func (UsersController) Create`

- `func (UsersController) Delete`

- `func (UsersController) GetEdit`

- `func (UsersController) GetNew`

- `func (UsersController) Key`

- `func (UsersController) Register`

- `func (UsersController) Update`

- `func (UsersController) Users`

### Functions

#### `func Handler`

#### `func MethodNotAllowed`

#### `func NewAccountController`

#### `func NewDIExampleController`

#### `func NewDashboardController`

#### `func NewGraphQLController`

#### `func NewLoginController`

#### `func NewLogoutController`

#### `func NewRolesController`

#### `func NewSpotlightController`

#### `func NewStaticFilesController`

#### `func NewUploadController`

#### `func NewUsersController`

#### `func NotFound`

---

## Package `dtos` (modules/core/presentation/controllers/dtos)

### Types

#### type `CreateRoleDTO`

##### Methods

- `func (CreateRoleDTO) Ok`

- `func (CreateRoleDTO) ToEntity`

#### type `SaveAccountDTO`

##### Methods

- `func (SaveAccountDTO) Apply`

- `func (SaveAccountDTO) Ok`

#### type `UpdateRoleDTO`

##### Methods

- `func (UpdateRoleDTO) Ok`

- `func (UpdateRoleDTO) ToEntity`

---

## Package `mappers` (modules/core/presentation/mappers)

### Functions

#### `func CurrencyToViewModel`

#### `func RoleToViewModel`

#### `func TabToViewModel`

#### `func UploadToViewModel`

#### `func UserToViewModel`

---

## Package `components` (modules/core/presentation/templates/components)

templ: version: v0.3.819


### Types

#### type `CurrencySelectProps`

### Functions

#### `func CurrencySelect`

### Variables and Constants

---

## Package `layouts` (modules/core/presentation/templates/layouts)

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `AuthenticatedProps`

#### type `BaseProps`

### Functions

#### `func Authenticated`

#### `func Avatar`

#### `func Base`

#### `func DefaultHead`

#### `func DefaultLogo`

#### `func MapNavItemsToSidebar`

#### `func MobileSidebar`

#### `func Navbar`

#### `func SidebarFooter`

#### `func SidebarHeader`

#### `func SidebarTrigger`

#### `func ThemeSwitcher`

### Variables and Constants

---

## Package `account` (modules/core/presentation/templates/pages/account)

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `ProfilePageProps`

#### type `SettingsPageProps`

### Functions

#### `func Index`

#### `func NavItems`

#### `func ProfileForm`

#### `func Settings`

#### `func SettingsForm`

### Variables and Constants

---

## Package `dashboard` (modules/core/presentation/templates/pages/dashboard)

templ: version: v0.3.819


### Types

#### type `IndexPageProps`

### Functions

#### `func DashboardContent`

#### `func Index`

#### `func Revenue`

#### `func Sales`

### Variables and Constants

---

## Package `error_pages` (modules/core/presentation/templates/pages/error_pages)

templ: version: v0.3.819


### Functions

#### `func NotFoundContent`

### Variables and Constants

---

## Package `login` (modules/core/presentation/templates/pages/login)

templ: version: v0.3.819


### Types

#### type `LoginProps`

### Functions

#### `func GoogleIcon`

#### `func Header`

#### `func Index`

### Variables and Constants

---

## Package `roles` (modules/core/presentation/templates/pages/roles)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `Child`

#### type `CreateFormProps`

#### type `EditFormProps`

#### type `Group`

#### type `IndexPageProps`

#### type `SharedProps`

### Functions

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func New`

#### `func Permission`

#### `func RolesContent`

#### `func RolesTable`

### Variables and Constants

---

## Package `users` (modules/core/presentation/templates/pages/users)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreateFormProps`

#### type `EditFormProps`

#### type `IndexPageProps`

#### type `RoleSelectProps`

#### type `SharedProps`

### Functions

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func EmailInput`

#### `func Index`

#### `func New`

#### `func RoleSelect`

#### `func UserCreatedEvent`

#### `func UserRow`

#### `func UserRows`

#### `func UsersContent`

#### `func UsersTable`

### Variables and Constants

---

## Package `viewmodels` (modules/core/presentation/viewmodels)

### Types

#### type `Currency`

#### type `Permission`

#### type `Role`

#### type `Tab`

#### type `Upload`

#### type `User`

##### Methods

- `func (User) FullName`

- `func (User) Initials`

- `func (User) RolesVerbose`

---

## Package `seed` (modules/core/seed)

### Types

### Functions

#### `func CreateCurrencies`

#### `func CreatePermissions`

#### `func GroupsSeedFunc`

#### `func UserSeedFunc`

### Variables and Constants

---

## Package `services` (modules/core/services)

### Types

#### type `AuthLogService`

##### Methods

- `func (AuthLogService) Count`

- `func (AuthLogService) Create`

- `func (AuthLogService) Delete`

- `func (AuthLogService) GetAll`

- `func (AuthLogService) GetByID`

- `func (AuthLogService) GetPaginated`

- `func (AuthLogService) Update`

#### type `AuthService`

##### Methods

- `func (AuthService) Authenticate`

- `func (AuthService) AuthenticateGoogle`

- `func (AuthService) AuthenticateWithUserID`

- `func (AuthService) Authorize`

- `func (AuthService) CookieAuthenticate`

- `func (AuthService) CookieAuthenticateWithUserID`

- `func (AuthService) CookieGoogleAuthenticate`

- `func (AuthService) GoogleAuthenticate`

- `func (AuthService) Logout`

#### type `CurrencyService`

##### Methods

- `func (CurrencyService) Create`

- `func (CurrencyService) Delete`

- `func (CurrencyService) GetAll`

- `func (CurrencyService) GetByCode`

- `func (CurrencyService) GetPaginated`

- `func (CurrencyService) Update`

#### type `GroupService`

GroupService provides operations for managing groups


##### Methods

- `func (GroupService) AddUser`
  AddUser adds a user to a group
  

- `func (GroupService) AssignRole`
  AssignRole assigns a role to a group
  

- `func (GroupService) Count`
  Count returns the total number of groups
  

- `func (GroupService) Create`
  Create creates a new group
  

- `func (GroupService) Delete`
  Delete removes a group by its ID
  

- `func (GroupService) GetByID`
  GetByID returns a group by its ID
  

- `func (GroupService) GetPaginated`
  GetPaginated returns a paginated list of groups
  

- `func (GroupService) RemoveRole`
  RemoveRole removes a role from a group
  

- `func (GroupService) RemoveUser`
  RemoveUser removes a user from a group
  

- `func (GroupService) Update`
  Update updates an existing group
  

#### type `ProjectService`

##### Methods

- `func (ProjectService) Create`

- `func (ProjectService) Delete`

- `func (ProjectService) GetAll`

- `func (ProjectService) GetByID`

- `func (ProjectService) GetPaginated`

- `func (ProjectService) Update`

#### type `RoleService`

##### Methods

- `func (RoleService) Count`

- `func (RoleService) Create`

- `func (RoleService) Delete`

- `func (RoleService) GetAll`

- `func (RoleService) GetByID`

- `func (RoleService) GetPaginated`

- `func (RoleService) Update`

#### type `SessionService`

##### Methods

- `func (SessionService) Create`

- `func (SessionService) Delete`

- `func (SessionService) GetAll`

- `func (SessionService) GetByToken`

- `func (SessionService) GetCount`

- `func (SessionService) GetPaginated`

- `func (SessionService) Update`

#### type `TabService`

##### Methods

- `func (TabService) Create`

- `func (TabService) CreateManyUserTabs`

- `func (TabService) Delete`

- `func (TabService) GetAll`

- `func (TabService) GetByID`

- `func (TabService) GetUserTabs`

- `func (TabService) Update`

#### type `UhrProps`

#### type `UhrService`

##### Methods

- `func (UhrService) Calculate`

#### type `UploadService`

##### Methods

- `func (UploadService) Create`

- `func (UploadService) CreateMany`

- `func (UploadService) Delete`

- `func (UploadService) GetAll`

- `func (UploadService) GetByHash`

- `func (UploadService) GetByID`

- `func (UploadService) GetPaginated`

#### type `UserService`

##### Methods

- `func (UserService) Count`

- `func (UserService) Create`

- `func (UserService) Delete`

- `func (UserService) GetAll`

- `func (UserService) GetByEmail`

- `func (UserService) GetByID`

- `func (UserService) GetPaginated`

- `func (UserService) GetPaginatedWithTotal`

- `func (UserService) Update`

- `func (UserService) UpdateLastAction`

- `func (UserService) UpdateLastLogin`

### Functions

---

## Package `crm` (modules/crm)

### Types

#### type `Module`

##### Methods

- `func (Module) Name`

- `func (Module) Register`

### Functions

#### `func NewModule`

### Variables and Constants

- Var: `[CRMLink]`

- Var: `[ChatsLink]`

- Var: `[ClientsLink]`

- Var: `[NavItems]`

---

## Package `chat` (modules/crm/domain/aggregates/chat)

### Types

#### type `Chat`

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DeletedEvent`

#### type `Field`

#### type `FindParams`

#### type `Message`

#### type `MessageField`

#### type `MessageFindParams`

#### type `MessageRepository`

#### type `MessageSource`

#### type `MessagedAddedEvent`

#### type `Repository`

#### type `Sender`

#### type `SortBy`

### Variables and Constants

- Var: `[ErrEmptyMessage]`

---

## Package `client` (modules/crm/domain/aggregates/client)

### Types

#### type `Client`

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `DateRange`

#### type `Field`

#### type `FindParams`

#### type `Option`

#### type `Repository`

#### type `SortBy`

#### type `UpdatePassportDTO`

##### Methods

- `func (UpdatePassportDTO) Apply`

- `func (UpdatePassportDTO) Ok`

#### type `UpdatePersonalDTO`

##### Methods

- `func (UpdatePersonalDTO) Apply`

- `func (UpdatePersonalDTO) Ok`

#### type `UpdateTaxDTO`

##### Methods

- `func (UpdateTaxDTO) Apply`

- `func (UpdateTaxDTO) Ok`

---

## Package `messagetemplate` (modules/crm/domain/entities/message-template)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `FindParams`

#### type `MessageTemplate`

#### type `Repository`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Apply`

- `func (UpdateDTO) Ok`

---

## Package `handlers` (modules/crm/handlers)

### Types

#### type `NotificationHandler`

##### Methods

#### type `SMSHandler`

##### Methods

---

## Package `cpassproviders` (modules/crm/infrastructure/cpass-providers)

### Types

#### type `Config`

Config holds the Twilio service configuration


#### type `DownloadMediaDTO`

DownloadMediaDTO represents the data needed to download media


#### type `DownloadMediaResultDTO`

DownloadMediaResultDTO represents the result of a media download


#### type `InboundTwilioMessageDTO`

#### type `Provider`

#### type `ReceivedMessageEvent`

#### type `SendMessageDTO`

SendMessageDTO represents the data needed to send a message


#### type `TwilioProvider`

TwilioProvider handles Twilio-related operations


##### Methods

- `func (TwilioProvider) SendMessage`
  SendMessage sends a message using Twilio
  

- `func (TwilioProvider) WebhookHandler`

#### type `UploadResult`

UploadResult represents the result of a file upload


#### type `UploadsParams`

UploadsParams represents parameters for uploading a file


### Functions

---

## Package `persistence` (modules/crm/infrastructure/persistence)

### Types

#### type `ChatRepository`

##### Methods

- `func (ChatRepository) AddMessage`

- `func (ChatRepository) Count`

- `func (ChatRepository) Create`

- `func (ChatRepository) Delete`

- `func (ChatRepository) DeleteMessage`

- `func (ChatRepository) GetAll`

- `func (ChatRepository) GetByClientID`

- `func (ChatRepository) GetByID`

- `func (ChatRepository) GetMessageByID`

- `func (ChatRepository) GetPaginated`

- `func (ChatRepository) Update`

#### type `ClientRepository`

##### Methods

- `func (ClientRepository) Count`

- `func (ClientRepository) Create`

- `func (ClientRepository) Delete`

- `func (ClientRepository) GetAll`

- `func (ClientRepository) GetByID`

- `func (ClientRepository) GetByPhone`

- `func (ClientRepository) GetPaginated`

- `func (ClientRepository) Save`

- `func (ClientRepository) Update`

#### type `MessageTemplateRepository`

##### Methods

- `func (MessageTemplateRepository) Count`

- `func (MessageTemplateRepository) Create`

- `func (MessageTemplateRepository) Delete`

- `func (MessageTemplateRepository) GetAll`

- `func (MessageTemplateRepository) GetByID`

- `func (MessageTemplateRepository) GetPaginated`

- `func (MessageTemplateRepository) Update`

### Functions

#### `func NewChatRepository`

#### `func NewClientRepository`

#### `func NewMessageTemplateRepository`

#### `func ToDBChat`

#### `func ToDBClient`

#### `func ToDBMessage`

#### `func ToDBMessageTemplate`

#### `func ToDomainChat`

#### `func ToDomainClientComplete`

#### `func ToDomainMessage`

#### `func ToDomainMessageTemplate`

### Variables and Constants

- Var: `[ErrChatNotFound ErrMessageNotFound]`

- Var: `[ErrClientNotFound]`

- Var: `[ErrMessageTemplateNotFound]`

---

## Package `models` (modules/crm/infrastructure/persistence/models)

### Types

#### type `Chat`

#### type `Client`

#### type `Message`

#### type `MessageTemplate`

---

## Package `telegram` (modules/crm/infrastructure/telegram)

### Types

#### type `Bot`

##### Methods

- `func (Bot) SendMessage`

#### type `SendMessageOpts`

---

## Package `permissions` (modules/crm/permissions)

### Variables and Constants

- Var: `[ClientCreate ClientRead ClientUpdate ClientDelete]`

- Var: `[Permissions]`

- Const: `[ResourceClient]`

---

## Package `controllers` (modules/crm/presentation/controllers)

### Types

#### type `ChatController`

##### Methods

- `func (ChatController) Create`

- `func (ChatController) GetNew`

- `func (ChatController) Key`

- `func (ChatController) List`

- `func (ChatController) Register`

- `func (ChatController) Search`

- `func (ChatController) SendMessage`

#### type `ClientController`

##### Methods

- `func (ClientController) Create`

- `func (ClientController) Delete`

- `func (ClientController) GetPassportEdit`

- `func (ClientController) GetPersonalEdit`

- `func (ClientController) GetTaxEdit`

- `func (ClientController) Key`

- `func (ClientController) List`

- `func (ClientController) Register`

- `func (ClientController) UpdatePassport`

- `func (ClientController) UpdatePersonal`

- `func (ClientController) UpdateTax`

- `func (ClientController) View`

#### type `ClientsPaginatedResponse`

#### type `CreateChatDTO`

#### type `MessageTemplateController`

##### Methods

- `func (MessageTemplateController) Create`

- `func (MessageTemplateController) Delete`

- `func (MessageTemplateController) GetEdit`

- `func (MessageTemplateController) GetNew`

- `func (MessageTemplateController) Key`

- `func (MessageTemplateController) List`

- `func (MessageTemplateController) Register`

- `func (MessageTemplateController) Update`

#### type `SendMessageDTO`

#### type `TwillioController`

##### Methods

- `func (TwillioController) Key`

- `func (TwillioController) Register`

### Functions

#### `func NewChatController`

#### `func NewClientController`

#### `func NewMessageTemplateController`

---

## Package `mappers` (modules/crm/presentation/mappers)

### Functions

#### `func ChatToViewModel`

#### `func ClientToViewModel`

#### `func MessageTemplateToViewModel`

#### `func MessageToViewModel`

#### `func PassportToViewModel`

#### `func SenderToViewModel`

---

## Package `chatsui` (modules/crm/presentation/templates/pages/chats)

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `ChatInputProps`

#### type `IndexPageProps`

#### type `InstantMessagesDialogProps`

#### type `NewChatProps`

#### type `SelectedChatProps`

### Functions

#### `func ChatCard`

#### `func ChatInput`

#### `func ChatLayout`

#### `func ChatList`

#### `func ChatMessages`

#### `func ChatNotFound`

#### `func Index`

#### `func InstantMessagesDialog`

#### `func Message`

#### `func NewChat`

#### `func NewChatForm`

#### `func NoSelectedChat`

#### `func SelectedChat`

### Variables and Constants

---

## Package `clients` (modules/crm/presentation/templates/pages/clients)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CardHeaderProps`

#### type `ClientTab`

#### type `CreatePageProps`

#### type `IndexPageProps`

#### type `PassportInfoEditProps`

PassportInfoEditForm is a dedicated form for editing passport information


#### type `PersonalInfoEditProps`

PersonalInfoEditForm is a dedicated form for editing personal information


#### type `ProfileProps`

#### type `TaxInfoEditProps`

TaxInfoEditForm is a dedicated form for editing tax information


#### type `ViewDrawerProps`

### Functions

#### `func Avatar`

---- Utility Components ----


#### `func CardHeader`

#### `func Chats`

---- Chats -----


#### `func ClientsContent`

#### `func ClientsTable`

#### `func CreateForm`

#### `func Index`

#### `func NewClientDrawer`

#### `func NotFound`

---- Not Found ----


#### `func Notes`

#### `func PassportInfoCard`

PassportInfoCard shows passport information for the client


#### `func PassportInfoEditForm`

#### `func PersonalInfoCard`

PersonalInfoCardProps contains data needed for the personal info card


#### `func PersonalInfoEditForm`

#### `func Profile`

#### `func TaxInfoCard`

TaxInfoCard shows tax information for the client


#### `func TaxInfoEditForm`

#### `func ViewDrawer`

### Variables and Constants

---

## Package `messagetemplatesui` (modules/crm/presentation/templates/pages/message-templates)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

### Functions

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func New`

#### `func TemplatesContent`

#### `func TemplatesTable`

### Variables and Constants

---

## Package `viewmodels` (modules/crm/presentation/viewmodels)

### Types

#### type `Chat`

##### Methods

- `func (Chat) HasUnreadMessages`

- `func (Chat) LastMessage`

- `func (Chat) ReversedMessages`

- `func (Chat) UnreadMessagesFormatted`

#### type `Client`

##### Methods

- `func (Client) FullName`

- `func (Client) Initials`

#### type `Message`

##### Methods

- `func (Message) Date`

- `func (Message) Time`

#### type `MessageSender`

#### type `MessageTemplate`

#### type `Passport`

---

## Package `services` (modules/crm/services)

### Types

#### type `ChatService`

##### Methods

- `func (ChatService) Count`

- `func (ChatService) Create`

- `func (ChatService) Delete`

- `func (ChatService) GetAll`

- `func (ChatService) GetByClientID`

- `func (ChatService) GetByClientIDOrCreate`

- `func (ChatService) GetByID`

- `func (ChatService) GetPaginated`

- `func (ChatService) RegisterClientMessage`

- `func (ChatService) SendMessage`

- `func (ChatService) Update`

#### type `ClientService`

##### Methods

- `func (ClientService) Count`

- `func (ClientService) Create`

- `func (ClientService) Delete`

- `func (ClientService) GetAll`

- `func (ClientService) GetByID`

- `func (ClientService) GetPaginated`

- `func (ClientService) Save`

#### type `MessageMedia`

MessageMedia represents media attached to a message


#### type `MessageTemplateService`

##### Methods

- `func (MessageTemplateService) Count`

- `func (MessageTemplateService) Create`

- `func (MessageTemplateService) Delete`

- `func (MessageTemplateService) GetAll`

- `func (MessageTemplateService) GetByID`

- `func (MessageTemplateService) GetPaginated`

- `func (MessageTemplateService) Update`

#### type `SendMessageDTO`

SendMessageDTO represents the data needed to send a message


---

## Package `finance` (modules/finance)

### Types

#### type `Module`

##### Methods

- `func (Module) Name`

- `func (Module) Register`

### Functions

#### `func NewModule`

### Variables and Constants

- Var: `[ExpenseCategoriesItem PaymentsItem ExpensesItem AccountsItem]`

- Var: `[FinanceItem]`

- Var: `[NavItems]`

---

## Package `expense` (modules/finance/domain/aggregates/expense)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DateRange`

#### type `DeletedEvent`

#### type `Expense`

#### type `FindParams`

#### type `Repository`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

---

## Package `category` (modules/finance/domain/aggregates/expense_category)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DateRange`

#### type `DeletedEvent`

#### type `ExpenseCategory`

#### type `FindParams`

#### type `Repository`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

---

## Package `moneyaccount` (modules/finance/domain/aggregates/money_account)

### Types

#### type `Account`

##### Methods

- `func (Account) InitialTransaction`

- `func (Account) Ok`

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DateRange`

#### type `DeletedEvent`

#### type `FindParams`

#### type `Repository`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

---

## Package `payment` (modules/finance/domain/aggregates/payment)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `Created`

#### type `DateRange`

#### type `Deleted`

#### type `FindParams`

#### type `Payment`

#### type `Repository`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `Updated`

### Variables and Constants

---

## Package `counterparty` (modules/finance/domain/entities/counterparty)

### Types

#### type `Counterparty`

#### type `DateRange`

#### type `FindParams`

#### type `LegalType`

##### Methods

- `func (LegalType) IsValid`

#### type `Repository`

#### type `Type`

##### Methods

- `func (Type) IsValid`

---

## Package `transaction` (modules/finance/domain/entities/transaction)

### Types

#### type `DateRange`

#### type `FindParams`

#### type `Repository`

#### type `Transaction`

#### type `Type`

##### Methods

- `func (Type) IsValid`

---

## Package `persistence` (modules/finance/infrastructure/persistence)

### Types

#### type `GormCounterpartyRepository`

##### Methods

- `func (GormCounterpartyRepository) Count`

- `func (GormCounterpartyRepository) Create`

- `func (GormCounterpartyRepository) Delete`

- `func (GormCounterpartyRepository) GetAll`

- `func (GormCounterpartyRepository) GetByID`

- `func (GormCounterpartyRepository) GetPaginated`

- `func (GormCounterpartyRepository) Update`

#### type `GormExpenseCategoryRepository`

##### Methods

- `func (GormExpenseCategoryRepository) Count`

- `func (GormExpenseCategoryRepository) Create`

- `func (GormExpenseCategoryRepository) Delete`

- `func (GormExpenseCategoryRepository) GetAll`

- `func (GormExpenseCategoryRepository) GetByID`

- `func (GormExpenseCategoryRepository) GetPaginated`

- `func (GormExpenseCategoryRepository) Update`

#### type `GormExpenseRepository`

##### Methods

- `func (GormExpenseRepository) Count`

- `func (GormExpenseRepository) Create`

- `func (GormExpenseRepository) Delete`

- `func (GormExpenseRepository) GetAll`

- `func (GormExpenseRepository) GetByID`

- `func (GormExpenseRepository) GetPaginated`

- `func (GormExpenseRepository) Update`

#### type `GormMoneyAccountRepository`

##### Methods

- `func (GormMoneyAccountRepository) Count`

- `func (GormMoneyAccountRepository) Create`

- `func (GormMoneyAccountRepository) Delete`

- `func (GormMoneyAccountRepository) GetAll`

- `func (GormMoneyAccountRepository) GetByID`

- `func (GormMoneyAccountRepository) GetPaginated`

- `func (GormMoneyAccountRepository) RecalculateBalance`

- `func (GormMoneyAccountRepository) Update`

#### type `GormPaymentRepository`

##### Methods

- `func (GormPaymentRepository) Count`

- `func (GormPaymentRepository) Create`

- `func (GormPaymentRepository) Delete`

- `func (GormPaymentRepository) GetAll`

- `func (GormPaymentRepository) GetByID`

- `func (GormPaymentRepository) GetPaginated`

- `func (GormPaymentRepository) Update`

#### type `GormTransactionRepository`

##### Methods

- `func (GormTransactionRepository) Count`

- `func (GormTransactionRepository) Create`

- `func (GormTransactionRepository) Delete`

- `func (GormTransactionRepository) GetAll`

- `func (GormTransactionRepository) GetByID`

- `func (GormTransactionRepository) GetPaginated`

- `func (GormTransactionRepository) Update`

### Functions

#### `func NewCounterpartyRepository`

#### `func NewExpenseCategoryRepository`

#### `func NewExpenseRepository`

#### `func NewMoneyAccountRepository`

#### `func NewPaymentRepository`

#### `func NewTransactionRepository`

### Variables and Constants

- Var: `[ErrAccountNotFound]`

- Var: `[ErrCounterpartyNotFound]`

- Var: `[ErrExpenseCategoryNotFound]`

- Var: `[ErrExpenseNotFound]`

- Var: `[ErrPaymentNotFound]`

- Var: `[ErrTransactionNotFound]`

---

## Package `models` (modules/finance/infrastructure/persistence/models)

### Types

#### type `Counterparty`

#### type `Expense`

#### type `ExpenseCategory`

#### type `MoneyAccount`

#### type `Payment`

#### type `Transaction`

---

## Package `permissions` (modules/finance/permissions)

### Variables and Constants

- Var: `[PaymentCreate PaymentRead PaymentUpdate PaymentDelete ExpenseCreate ExpenseRead ExpenseUpdate ExpenseDelete ExpenseCategoryCreate ExpenseCategoryRead ExpenseCategoryUpdate ExpenseCategoryDelete]`

- Var: `[Permissions]`

- Const: `[ResourceExpense ResourcePayment ResourceExpenseCategory]`

---

## Package `controllers` (modules/finance/presentation/controllers)

### Types

#### type `AccountPaginatedResponse`

#### type `CounterpartiesController`

##### Methods

- `func (CounterpartiesController) Key`

- `func (CounterpartiesController) Register`

- `func (CounterpartiesController) Search`

#### type `ExpenseCategoriesController`

##### Methods

- `func (ExpenseCategoriesController) Create`

- `func (ExpenseCategoriesController) Delete`

- `func (ExpenseCategoriesController) GetEdit`

- `func (ExpenseCategoriesController) GetNew`

- `func (ExpenseCategoriesController) Key`

- `func (ExpenseCategoriesController) List`

- `func (ExpenseCategoriesController) Register`

- `func (ExpenseCategoriesController) Update`

#### type `ExpenseCategoryPaginatedResponse`

#### type `ExpenseController`

##### Methods

- `func (ExpenseController) Create`

- `func (ExpenseController) Delete`

- `func (ExpenseController) GetEdit`

- `func (ExpenseController) GetNew`

- `func (ExpenseController) Key`

- `func (ExpenseController) List`

- `func (ExpenseController) Register`

- `func (ExpenseController) Update`

#### type `ExpensePaginationResponse`

#### type `MoneyAccountController`

##### Methods

- `func (MoneyAccountController) Create`

- `func (MoneyAccountController) Delete`

- `func (MoneyAccountController) GetEdit`

- `func (MoneyAccountController) GetNew`

- `func (MoneyAccountController) Key`

- `func (MoneyAccountController) List`

- `func (MoneyAccountController) Register`

- `func (MoneyAccountController) Update`

#### type `PaymentPaginatedResponse`

#### type `PaymentsController`

##### Methods

- `func (PaymentsController) Create`

- `func (PaymentsController) Delete`

- `func (PaymentsController) GetEdit`

- `func (PaymentsController) GetNew`

- `func (PaymentsController) Key`

- `func (PaymentsController) Payments`

- `func (PaymentsController) Register`

- `func (PaymentsController) Update`

### Functions

#### `func NewCounterpartiesController`

#### `func NewExpenseCategoriesController`

#### `func NewExpensesController`

#### `func NewMoneyAccountController`

#### `func NewPaymentsController`

---

## Package `mappers` (modules/finance/presentation/mappers)

### Functions

#### `func CounterpartyToViewModel`

#### `func ExpenseCategoryToViewModel`

#### `func ExpenseToViewModel`

#### `func MoneyAccountToViewModel`

#### `func MoneyAccountToViewUpdateModel`

#### `func PaymentToViewModel`

---

## Package `templates` (modules/finance/presentation/templates)

### Variables and Constants

- Var: `[FS]`

---

## Package `components` (modules/finance/presentation/templates/components)

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `AccountSelectProps`

#### type `CounterpartySelectProps`

### Functions

#### `func AccountSelect`

#### `func CounterpartySelect`

### Variables and Constants

---

## Package `expense_categories` (modules/finance/presentation/templates/pages/expense_categories)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

### Functions

#### `func CategoriesContent`

#### `func CategoriesTable`

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func New`

#### `func SearchFields`

#### `func SearchFieldsTrigger`

### Variables and Constants

---

## Package `expenses` (modules/finance/presentation/templates/pages/expenses)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `AccountSelectProps`

#### type `CategorySelectProps`

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

### Functions

#### `func AccountSelect`

#### `func CategorySelect`

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func ExpensesContent`

#### `func ExpensesTable`

#### `func Index`

#### `func New`

### Variables and Constants

---

## Package `moneyaccounts` (modules/finance/presentation/templates/pages/moneyaccounts)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

### Functions

#### `func AccountsContent`

#### `func AccountsTable`

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func New`

### Variables and Constants

---

## Package `payments` (modules/finance/presentation/templates/pages/payments)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

### Functions

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func New`

#### `func PaymentsContent`

#### `func PaymentsTable`

#### `func SearchFields`

#### `func SearchFieldsTrigger`

### Variables and Constants

---

## Package `viewmodels` (modules/finance/presentation/viewmodels)

### Types

#### type `Counterparty`

#### type `Expense`

#### type `ExpenseCategory`

#### type `MoneyAccount`

#### type `MoneyAccountCreateDTO`

#### type `MoneyAccountUpdateDTO`

#### type `Payment`

---

## Package `services` (modules/finance/services)

### Types

#### type `CounterpartyService`

##### Methods

- `func (CounterpartyService) Count`

- `func (CounterpartyService) Delete`

- `func (CounterpartyService) GetAll`

- `func (CounterpartyService) GetByID`

- `func (CounterpartyService) GetPaginated`

#### type `ExpenseCategoryService`

##### Methods

- `func (ExpenseCategoryService) Count`

- `func (ExpenseCategoryService) Create`

- `func (ExpenseCategoryService) Delete`

- `func (ExpenseCategoryService) GetAll`

- `func (ExpenseCategoryService) GetByID`

- `func (ExpenseCategoryService) GetPaginated`

- `func (ExpenseCategoryService) Update`

#### type `ExpenseService`

##### Methods

- `func (ExpenseService) Count`

- `func (ExpenseService) Create`

- `func (ExpenseService) Delete`

- `func (ExpenseService) GetAll`

- `func (ExpenseService) GetByID`

- `func (ExpenseService) GetPaginated`

- `func (ExpenseService) Update`

#### type `MoneyAccountService`

##### Methods

- `func (MoneyAccountService) Count`

- `func (MoneyAccountService) Create`

- `func (MoneyAccountService) Delete`

- `func (MoneyAccountService) GetAll`

- `func (MoneyAccountService) GetByID`

- `func (MoneyAccountService) GetPaginated`

- `func (MoneyAccountService) RecalculateBalance`

- `func (MoneyAccountService) Update`

#### type `PaymentService`

##### Methods

- `func (PaymentService) Count`

- `func (PaymentService) Create`

- `func (PaymentService) Delete`

- `func (PaymentService) GetAll`

- `func (PaymentService) GetByID`

- `func (PaymentService) GetPaginated`

- `func (PaymentService) Update`

#### type `TransactionService`

##### Methods

- `func (TransactionService) Count`

- `func (TransactionService) Create`

- `func (TransactionService) Delete`

- `func (TransactionService) GetAll`

- `func (TransactionService) GetByID`

- `func (TransactionService) GetPaginated`

- `func (TransactionService) Update`

---

## Package `hrm` (modules/hrm)

### Types

#### type `Module`

##### Methods

- `func (Module) Name`

- `func (Module) Register`

### Functions

#### `func NewModule`

### Variables and Constants

- Var: `[EmployeesLink]`

- Var: `[HRMLink]`

- Var: `[LocaleFiles]`

- Var: `[MigrationFiles]`

- Var: `[NavItems]`

---

## Package `employee` (modules/hrm/domain/aggregates/employee)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DateRange`

#### type `DeletedEvent`

#### type `Employee`

#### type `FindParams`

#### type `Language`

#### type `Repository`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

### Functions

---

## Package `position` (modules/hrm/domain/entities/position)

### Types

#### type `FindParams`

#### type `Position`

#### type `Repository`

---

## Package `persistence` (modules/hrm/infrastructure/persistence)

### Types

#### type `GormEmployeeRepository`

##### Methods

- `func (GormEmployeeRepository) Count`

- `func (GormEmployeeRepository) Create`

- `func (GormEmployeeRepository) Delete`

- `func (GormEmployeeRepository) GetAll`

- `func (GormEmployeeRepository) GetByID`

- `func (GormEmployeeRepository) GetPaginated`

- `func (GormEmployeeRepository) Update`

#### type `GormPositionRepository`

##### Methods

- `func (GormPositionRepository) Count`

- `func (GormPositionRepository) Create`

- `func (GormPositionRepository) Delete`

- `func (GormPositionRepository) GetAll`

- `func (GormPositionRepository) GetByID`

- `func (GormPositionRepository) GetPaginated`

- `func (GormPositionRepository) Update`

### Functions

#### `func NewEmployeeRepository`

#### `func NewPositionRepository`

### Variables and Constants

- Var: `[ErrEmployeeNotFound]`

- Var: `[ErrPositionNotFound]`

---

## Package `models` (modules/hrm/infrastructure/persistence/models)

### Types

#### type `Employee`

#### type `EmployeeMeta`

#### type `EmployeePosition`

#### type `Position`

---

## Package `permissions` (modules/hrm/permissions)

### Variables and Constants

- Var: `[EmployeeCreate EmployeeRead EmployeeUpdate EmployeeDelete]`

- Var: `[Permissions]`

- Const: `[ResourceEmployee]`

---

## Package `controllers` (modules/hrm/presentation/controllers)

### Types

#### type `EmployeeController`

##### Methods

- `func (EmployeeController) Create`

- `func (EmployeeController) Delete`

- `func (EmployeeController) GetEdit`

- `func (EmployeeController) GetNew`

- `func (EmployeeController) Key`

- `func (EmployeeController) List`

- `func (EmployeeController) Register`

- `func (EmployeeController) Update`

### Functions

#### `func NewEmployeeController`

---

## Package `mappers` (modules/hrm/presentation/mappers)

### Functions

#### `func EmployeeToViewModel`

---

## Package `employees` (modules/hrm/presentation/templates/pages/employees)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

#### type `SharedProps`

### Functions

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func EmployeesContent`

#### `func EmployeesTable`

#### `func Index`

#### `func JoinDateInput`

#### `func New`

#### `func PassportInput`

#### `func PinInput`

#### `func ResignationDateInput`

#### `func SalaryInput`

#### `func TinInput`

### Variables and Constants

---

## Package `viewmodels` (modules/hrm/presentation/viewmodels)

### Types

#### type `Employee`

---

## Package `services` (modules/hrm/services)

### Types

#### type `EmployeeService`

##### Methods

- `func (EmployeeService) Count`

- `func (EmployeeService) Create`

- `func (EmployeeService) Delete`

- `func (EmployeeService) GetAll`

- `func (EmployeeService) GetByID`

- `func (EmployeeService) GetPaginated`

- `func (EmployeeService) Update`

#### type `PositionService`

##### Methods

- `func (PositionService) Count`

- `func (PositionService) Create`

- `func (PositionService) Delete`

- `func (PositionService) GetAll`

- `func (PositionService) GetByID`

- `func (PositionService) GetPaginated`

- `func (PositionService) Update`

---

## Package `logging` (modules/logging)

### Types

#### type `Module`

##### Methods

- `func (Module) Name`

- `func (Module) Register`

### Functions

#### `func NewModule`

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

#### type `Module`

##### Methods

- `func (Module) Name`

- `func (Module) Register`

### Functions

#### `func NewModule`

### Variables and Constants

- Var: `[ProductsItem PositionsItem OrdersItem InventoryItem UnitsItem Item]`

- Var: `[NavItems]`

---

## Package `order` (modules/warehouse/domain/aggregates/order)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) ToEntity`

#### type `DateRange`

#### type `ErrOrderIsComplete`

##### Methods

- `func (ErrOrderIsComplete) Localize`

#### type `ErrProductIsShipped`

##### Methods

- `func (ErrProductIsShipped) Localize`

#### type `FindParams`

#### type `Item`

#### type `Order`

#### type `Repository`

#### type `Status`

##### Methods

- `func (Status) IsValid`

#### type `Type`

##### Methods

- `func (Type) IsValid`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) ToEntity`

---

## Package `position` (modules/warehouse/domain/aggregates/position)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DateRange`

#### type `DeletedEvent`

#### type `FindParams`

#### type `Position`

#### type `Repository`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

---

## Package `product` (modules/warehouse/domain/aggregates/product)

### Types

#### type `CountParams`

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreateProductsFromTagsDTO`

#### type `CreatedEvent`

#### type `DateRange`

#### type `DeletedEvent`

#### type `FindByPositionParams`

#### type `FindParams`

#### type `Product`

#### type `Repository`

#### type `Status`

##### Methods

- `func (Status) IsValid`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

### Variables and Constants

- Var: `[ErrInvalidStatus]`

---

## Package `inventory` (modules/warehouse/domain/entities/inventory)

### Types

#### type `Check`

##### Methods

- `func (Check) AddResult`

#### type `CheckResult`

#### type `CreateCheckDTO`

##### Methods

- `func (CreateCheckDTO) Ok`

- `func (CreateCheckDTO) ToEntity`

#### type `CreatedEvent`

#### type `DateRange`

#### type `DeletedEvent`

#### type `FindParams`

#### type `Position`

#### type `PositionCheckDTO`

#### type `Repository`

#### type `Status`

##### Methods

- `func (Status) IsValid`

#### type `Type`

#### type `UpdateCheckDTO`

##### Methods

- `func (UpdateCheckDTO) Ok`

- `func (UpdateCheckDTO) ToEntity`

#### type `UpdatedEvent`

---

## Package `unit` (modules/warehouse/domain/entities/unit)

### Types

#### type `CreateDTO`

##### Methods

- `func (CreateDTO) Ok`

- `func (CreateDTO) ToEntity`

#### type `CreatedEvent`

#### type `DateRange`

#### type `DeletedEvent`

#### type `FindParams`

#### type `Repository`

#### type `Unit`

#### type `UpdateDTO`

##### Methods

- `func (UpdateDTO) Ok`

- `func (UpdateDTO) ToEntity`

#### type `UpdatedEvent`

---

## Package `persistence` (modules/warehouse/infrastructure/persistence)

### Types

#### type `GormInventoryRepository`

##### Methods

- `func (GormInventoryRepository) Count`

- `func (GormInventoryRepository) Create`

- `func (GormInventoryRepository) Delete`

- `func (GormInventoryRepository) GetAll`

- `func (GormInventoryRepository) GetByID`

- `func (GormInventoryRepository) GetByIDWithDifference`

- `func (GormInventoryRepository) GetPaginated`

- `func (GormInventoryRepository) Positions`

- `func (GormInventoryRepository) Update`

#### type `GormOrderRepository`

##### Methods

- `func (GormOrderRepository) Count`

- `func (GormOrderRepository) Create`

- `func (GormOrderRepository) Delete`

- `func (GormOrderRepository) GetAll`

- `func (GormOrderRepository) GetByID`

- `func (GormOrderRepository) GetPaginated`

- `func (GormOrderRepository) Update`

#### type `GormPositionRepository`

##### Methods

- `func (GormPositionRepository) Count`

- `func (GormPositionRepository) Create`

- `func (GormPositionRepository) CreateOrUpdate`

- `func (GormPositionRepository) Delete`

- `func (GormPositionRepository) GetAll`

- `func (GormPositionRepository) GetAllPositionIds`

- `func (GormPositionRepository) GetByBarcode`

- `func (GormPositionRepository) GetByID`

- `func (GormPositionRepository) GetByIDs`

- `func (GormPositionRepository) GetPaginated`

- `func (GormPositionRepository) Update`

#### type `GormProductRepository`

##### Methods

- `func (GormProductRepository) BulkCreate`

- `func (GormProductRepository) BulkDelete`

- `func (GormProductRepository) Count`

- `func (GormProductRepository) Create`

- `func (GormProductRepository) CreateOrUpdate`

- `func (GormProductRepository) Delete`

- `func (GormProductRepository) FindByPositionID`

- `func (GormProductRepository) GetAll`

- `func (GormProductRepository) GetByID`

- `func (GormProductRepository) GetByRfid`

- `func (GormProductRepository) GetByRfidMany`

- `func (GormProductRepository) GetPaginated`

- `func (GormProductRepository) Update`

- `func (GormProductRepository) UpdateStatus`

#### type `GormUnitRepository`

##### Methods

- `func (GormUnitRepository) Count`

- `func (GormUnitRepository) Create`

- `func (GormUnitRepository) CreateOrUpdate`

- `func (GormUnitRepository) Delete`

- `func (GormUnitRepository) GetAll`

- `func (GormUnitRepository) GetByID`

- `func (GormUnitRepository) GetByTitleOrShortTitle`

- `func (GormUnitRepository) GetPaginated`

- `func (GormUnitRepository) Update`

### Functions

#### `func NewInventoryRepository`

#### `func NewOrderRepository`

#### `func NewPositionRepository`

#### `func NewProductRepository`

#### `func NewUnitRepository`

### Variables and Constants

- Var: `[ErrInventoryCheckNotFound]`

- Var: `[ErrOrderNotFound]`

- Var: `[ErrPositionNotFound]`

- Var: `[ErrProductNotFound]`

- Var: `[ErrUnitNotFound]`

---

## Package `mappers` (modules/warehouse/infrastructure/persistence/mappers)

### Functions

#### `func ToDBInventoryCheck`

#### `func ToDBInventoryCheckResult`

#### `func ToDBOrder`

#### `func ToDBPosition`

#### `func ToDBProduct`

#### `func ToDBUnit`

#### `func ToDomainInventoryCheck`

#### `func ToDomainInventoryCheckResult`

#### `func ToDomainInventoryPosition`

#### `func ToDomainOrder`

#### `func ToDomainPosition`

#### `func ToDomainProduct`

#### `func ToDomainUnit`

---

## Package `models` (modules/warehouse/infrastructure/persistence/models)

### Types

#### type `InventoryCheck`

#### type `InventoryCheckResult`

#### type `InventoryPosition`

#### type `WarehouseOrder`

#### type `WarehouseOrderItem`

#### type `WarehousePosition`

#### type `WarehousePositionImage`

#### type `WarehouseProduct`

#### type `WarehouseUnit`

---

## Package `graph` (modules/warehouse/interfaces/graph)

### Types

#### type `ComplexityRoot`

#### type `Config`

#### type `DirectiveRoot`

#### type `MutationResolver`

#### type `QueryResolver`

#### type `Resolver`

##### Methods

- `func (Resolver) Mutation`
  Mutation returns MutationResolver implementation.
  

- `func (Resolver) Query`
  Query returns QueryResolver implementation.
  

#### type `ResolverRoot`

### Functions

#### `func NewExecutableSchema`

NewExecutableSchema creates an ExecutableSchema from the ResolverRoot interface.


### Variables and Constants

- Var: `[ProductsToGraphModel ProductsToTags InventoryPositionsToGraphModel]`

---

## Package `model` (modules/warehouse/interfaces/graph/gqlmodels)

### Types

#### type `CreateProductsFromTags`

#### type `InventoryItem`

#### type `InventoryPosition`

#### type `Mutation`

#### type `Order`

#### type `OrderItem`

#### type `OrderQuery`

#### type `PaginatedOrders`

#### type `PaginatedProducts`

#### type `PaginatedWarehousePositions`

#### type `Product`

#### type `Query`

#### type `ValidateProductsResult`

#### type `WarehousePosition`

---

## Package `mappers` (modules/warehouse/interfaces/graph/mappers)

### Functions

#### `func InventoryPositionToGraphModel`

#### `func OrderItemsToGraphModel`

#### `func OrderToGraphModel`

#### `func PositionToGraphModel`

#### `func ProductToGraphModel`

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

#### type `InventoryCheckPaginatedResponse`

#### type `InventoryController`

##### Methods

- `func (InventoryController) Create`

- `func (InventoryController) Delete`

- `func (InventoryController) GetEdit`

- `func (InventoryController) GetEditDifference`

- `func (InventoryController) GetNew`

- `func (InventoryController) Key`

- `func (InventoryController) List`

- `func (InventoryController) Register`

- `func (InventoryController) SearchPositions`

- `func (InventoryController) Update`

#### type `OrderItem`

#### type `OrderPaginatedResponse`

#### type `OrdersController`

##### Methods

- `func (OrdersController) CreateInOrder`

- `func (OrdersController) CreateOutOrder`

- `func (OrdersController) Delete`

- `func (OrdersController) Key`

- `func (OrdersController) List`

- `func (OrdersController) NewInOrder`

- `func (OrdersController) NewOutOrder`

- `func (OrdersController) OrderItems`

- `func (OrdersController) Register`

- `func (OrdersController) ViewOrder`

#### type `PaginatedResponse`

#### type `PositionPaginatedResponse`

#### type `PositionsController`

##### Methods

- `func (PositionsController) Create`

- `func (PositionsController) Delete`

- `func (PositionsController) GetEdit`

- `func (PositionsController) GetNew`

- `func (PositionsController) GetUpload`

- `func (PositionsController) HandleUpload`

- `func (PositionsController) Key`

- `func (PositionsController) List`

- `func (PositionsController) Register`

- `func (PositionsController) Search`

- `func (PositionsController) Update`

#### type `ProductsController`

##### Methods

- `func (ProductsController) Create`

- `func (ProductsController) GetEdit`

- `func (ProductsController) GetNew`

- `func (ProductsController) Key`

- `func (ProductsController) List`

- `func (ProductsController) Register`

- `func (ProductsController) Update`

#### type `UnitPaginatedResponse`

#### type `UnitsController`

##### Methods

- `func (UnitsController) Create`

- `func (UnitsController) Delete`

- `func (UnitsController) GetEdit`

- `func (UnitsController) GetNew`

- `func (UnitsController) Key`

- `func (UnitsController) List`

- `func (UnitsController) Register`

- `func (UnitsController) Update`

### Functions

#### `func NewInventoryController`

#### `func NewOrdersController`

#### `func NewPositionsController`

#### `func NewProductsController`

#### `func NewUnitsController`

#### `func OrderInItemToViewModel`

#### `func OrderOutItemToViewModel`

### Variables and Constants

- Var: `[OrdersToViewModels]`

---

## Package `dtos` (modules/warehouse/presentation/controllers/dtos)

### Types

#### type `CreateOrderDTO`

##### Methods

- `func (CreateOrderDTO) Ok`

#### type `PositionsUploadDTO`

##### Methods

- `func (PositionsUploadDTO) Ok`

#### type `UpdateOrderDTO`

##### Methods

- `func (UpdateOrderDTO) Ok`

---

## Package `mappers` (modules/warehouse/presentation/mappers)

### Functions

#### `func CheckResultToViewModel`

#### `func CheckToViewModel`

#### `func OrderItemToViewModel`

#### `func OrderToViewModel`

#### `func PositionToViewModel`

#### `func ProductToViewModel`

#### `func UnitToViewModel`

---

## Package `templates` (modules/warehouse/presentation/templates)

### Variables and Constants

- Var: `[FS]`

---

## Package `inventory` (modules/warehouse/presentation/templates/pages/inventory)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

### Functions

#### `func AllPositionsTable`

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func InventoryContent`

#### `func InventoryTable`

#### `func New`

### Variables and Constants

---

## Package `orders` (modules/warehouse/presentation/templates/pages/orders)

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `IndexPageProps`

#### type `ViewPageProps`

### Functions

#### `func Index`

#### `func OrdersContent`

#### `func OrdersTable`

#### `func View`

### Variables and Constants

---

## Package `orderout` (modules/warehouse/presentation/templates/pages/orders/in)

templ: version: v0.3.819


### Types

#### type `FormProps`

#### type `OrderItem`

#### type `PageProps`

### Functions

#### `func Form`

#### `func New`

#### `func OrderItemsTable`

### Variables and Constants

---

## Package `orderout` (modules/warehouse/presentation/templates/pages/orders/out)

templ: version: v0.3.819


### Types

#### type `FormProps`

#### type `OrderItem`

#### type `PageProps`

### Functions

#### `func Form`

#### `func New`

#### `func OrderItemsTable`

### Variables and Constants

---

## Package `positions` (modules/warehouse/presentation/templates/pages/positions)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

#### type `UnitSelectProps`

#### type `UploadPageProps`

### Functions

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func New`

#### `func PositionsContent`

#### `func PositionsTable`

#### `func UnitSelect`

#### `func Upload`

#### `func UploadForm`

### Variables and Constants

---

## Package `products` (modules/warehouse/presentation/templates/pages/products)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

#### type `PositionSelectProps`

#### type `StatusSelectProps`

#### type `StatusViewModel`

### Functions

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func New`

#### `func PositionSelect`

#### `func ProductsContent`

#### `func ProductsTable`

#### `func StatusSelect`

### Variables and Constants

- Var: `[selectOnce InStock InDevelopment Approved Statuses]`

---

## Package `units` (modules/warehouse/presentation/templates/pages/units)

templ: version: v0.3.819

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `CreatePageProps`

#### type `EditPageProps`

#### type `IndexPageProps`

### Functions

#### `func CreateForm`

#### `func Edit`

#### `func EditForm`

#### `func Index`

#### `func New`

#### `func UnitsContent`

#### `func UnitsTable`

### Variables and Constants

---

## Package `viewmodels` (modules/warehouse/presentation/viewmodels)

### Types

#### type `Check`

##### Methods

- `func (Check) LocalizedStatus`

- `func (Check) LocalizedType`

#### type `CheckResult`

#### type `Order`

##### Methods

- `func (Order) DistinctPositions`

- `func (Order) LocalizedStatus`

- `func (Order) LocalizedTitle`

- `func (Order) LocalizedType`

- `func (Order) TotalProducts`

#### type `OrderItem`

##### Methods

- `func (OrderItem) Quantity`

#### type `Position`

#### type `Product`

##### Methods

- `func (Product) LocalizedStatus`

#### type `Unit`

---

## Package `services` (modules/warehouse/services)

### Types

#### type `InventoryService`

##### Methods

- `func (InventoryService) Count`

- `func (InventoryService) Create`

- `func (InventoryService) Delete`

- `func (InventoryService) GetAll`

- `func (InventoryService) GetByID`

- `func (InventoryService) GetByIDWithDifference`

- `func (InventoryService) GetPaginated`

- `func (InventoryService) Positions`

- `func (InventoryService) Update`

#### type `UnitService`

##### Methods

- `func (UnitService) Count`

- `func (UnitService) Create`

- `func (UnitService) Delete`

- `func (UnitService) GetAll`

- `func (UnitService) GetByID`

- `func (UnitService) GetByTitleOrShortTitle`

- `func (UnitService) GetPaginated`

- `func (UnitService) Update`

---

## Package `orderservice` (modules/warehouse/services/orderservice)

### Types

#### type `OrderService`

##### Methods

- `func (OrderService) Complete`

- `func (OrderService) Count`

- `func (OrderService) Create`

- `func (OrderService) Delete`

- `func (OrderService) FindByPositionID`

- `func (OrderService) GetAll`

- `func (OrderService) GetByID`

- `func (OrderService) GetPaginated`

- `func (OrderService) Update`

---

## Package `positionservice` (modules/warehouse/services/positionservice)

### Types

#### type `ErrInvalidCell`

##### Methods

- `func (ErrInvalidCell) Localize`

#### type `PositionService`

##### Methods

- `func (PositionService) Count`

- `func (PositionService) Create`

- `func (PositionService) Delete`

- `func (PositionService) GetAll`

- `func (PositionService) GetByID`

- `func (PositionService) GetByIDs`

- `func (PositionService) GetPaginated`

- `func (PositionService) LoadFromFilePath`

- `func (PositionService) Update`

- `func (PositionService) UpdateWithFile`

#### type `XlsRow`

### Functions

---

## Package `productservice` (modules/warehouse/services/productservice)

### Types

#### type `ErrDuplicateRfid`

##### Methods

- `func (ErrDuplicateRfid) Localize`

#### type `ProductService`

##### Methods

- `func (ProductService) BulkCreate`

- `func (ProductService) Count`

- `func (ProductService) CountInStock`

- `func (ProductService) Create`

- `func (ProductService) CreateProductsFromTags`

- `func (ProductService) Delete`

- `func (ProductService) GetAll`

- `func (ProductService) GetByID`

- `func (ProductService) GetPaginated`

- `func (ProductService) Update`

- `func (ProductService) ValidateProducts`

---

## Package `website` (modules/website)

### Types

#### type `Module`

##### Methods

- `func (Module) Name`

- `func (Module) Register`

### Functions

#### `func NewModule`

### Variables and Constants

- Var: `[AIChatLink]`

- Var: `[NavItems]`

- Var: `[WebsiteLink]`

---

## Package `assets` (modules/website/presentation/assets)

### Variables and Constants

- Var: `[FS]`

- Var: `[HashFS]`

---

## Package `controllers` (modules/website/presentation/controllers)

### Types

#### type `AIChatController`

##### Methods

- `func (AIChatController) Key`

- `func (AIChatController) Register`

### Functions

#### `func NewAIChatController`

---

## Package `aichat` (modules/website/presentation/templates/pages/aichat)

templ: version: v0.3.819

templ: version: v0.3.819


### Types

#### type `Props`

### Functions

#### `func Chat`

#### `func ChatIcon`

#### `func Configure`

---- Configuration ----


#### `func WebComponent`

### Variables and Constants

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

