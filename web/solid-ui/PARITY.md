# templ to Solid UI parity

This audit covers every public `templ` function in `components/**/*.templ` at this checkout. Generated `*_templ.go` files are excluded. The source contains **185** public functions, including the three generic kanban functions; the earlier estimate of 183 did not include the complete current export set.

Statuses are deliberately binary and concrete:

- **DONE** — a public Solid component is feature-equivalent for the client-rendered contract.
- **COMPOSED** — the templ helper's exact DOM responsibility is rendered by the named public Solid component; a second public helper would duplicate the parent API.
- **ADAPTER** — server context, HTMX, script, or transport behavior is represented by the named typed Solid callback/provider contract.
- **MISSING** — there is no feature-equivalent public Solid or Lens implementation.

`COMPOSED` and `ADAPTER` count as mapped. They do not hide reduced behavior: any behavior without a concrete replacement is `MISSING`.

| templ source | Public templ functions | Solid UI / SDK mapping | Status |
|---|---|---|---|
| `auth/permission_guard.templ` | `Guard` | `PermissionGuard<P>` + typed `PermissionPredicate<P>` replaces request-context RBAC lookup. | ADAPTER |
| `base/alert/alert.templ` | `Error`, `Success` | `ErrorAlert`, `SuccessAlert` (`Alert` presets). | DONE |
| `base/avatar/avatar.templ` | `Avatar` | `Avatar`. | DONE |
| `base/badge/badge.templ` | `New` | `Badge` with the canonical tone, size, and native attributes. | DONE |
| `base/breadcrumb/breadcrumb.templ` | `List`, `Item`, `Link`, `Separator`, `SlashSeparator` | `Breadcrumbs`, `BreadcrumbItem`, `BreadcrumbLink`, `BreadcrumbSeparator`; slash is the separator's child/content variant. | DONE |
| `base/button/button.templ` | `Primary`, `Secondary`, `PrimaryOutline`, `Danger`, `Sidebar`, `Ghost` | `PrimaryButton`, `SecondaryButton`, `PrimaryOutlineButton`, `DangerButton`, `SidebarButton`, `GhostButton`. | DONE |
| `base/card/accent_color.templ` | `AccentColor` | `AccentColor`, including controlled/uncontrolled selection, native form attributes, disabled state, and CSS color variable. | DONE |
| `base/card/card.templ` | `DefaultHeader`, `Card` | `CardHeader`, `Card`. | DONE |
| `base/combobox.templ` | `ComboboxOptions`, `DropdownIndicator`, `SelectedValues`, `Combobox` | `Combobox`; option list, indicator, and selected-value DOM are internal states of the public component. | COMPOSED |
| `base/date_range_clear.templ` | `DateRangeClearButton` | `DateRangeClearButton`. | DONE |
| `base/description-list/description_list.templ` | `RegularItem`, `DetailsItem`, `Label`, `Text`, `Link`, `ListTitle`, `ListSubtitle`, `Details`, `List` | `DescriptionListItem`, `DescriptionListDetails`, `DescriptionListLabel`, `DescriptionListValue`, `DescriptionListLink`, `DescriptionList`; title/subtitle are typed `title`/`subtitle` slots. | DONE |
| `base/dialog/dialog.templ` | `Confirmation` | `ConfirmationDialog`. | DONE |
| `base/dialog/drawer.templ` | `Drawer`, `StdViewDrawer` | `Drawer`, `ViewDrawer`. | DONE |
| `base/dropdown.templ` | `DropdownItem`, `DropdownFormItem`, `DetailsDropdown` | `DropdownItem`, `DropdownFormItem`, `Dropdown`. | DONE |
| `base/input/datepicker.templ` | `DatePicker` | `DatePicker` and `DateRangePicker`; controlled/uncontrolled modes and calendar keyboard/focus behavior are public. | DONE |
| `base/input/input.templ` | `Text`, `Number`, `Email`, `Tel`, `Date`, `DateTime`, `Color`, `Checkbox`, `Password`, `Money` | `TextInput`, `NumberInput`, `EmailInput`, `TelInput`, `DateInput`, `DateTimeInput`, `ColorInput`, `Checkbox`, `PasswordInput`, `MoneyInput`. | DONE |
| `base/input/switch.templ` | `Switch` | `Switch`. | DONE |
| `base/input/textarea.templ` | `TextArea` | `Textarea`. | DONE |
| `base/label.templ` | `BaseLabel` | `Label`. | DONE |
| `base/navtabs/navtabs.templ` | `Root`, `List`, `Button`, `Content` | `NavTabs`, `NavTabsList`, `NavTabsTrigger`, `NavTabsContent`. | DONE |
| `base/pagination/pagination.templ` | `Pagination` | `Pagination`; navigation is a typed `onPageChange` callback or native links. | DONE |
| `base/phone/phone.templ` | `Link`, `Text` | `PhoneLink`, `PhoneText`, and `formatPhoneDisplay`, including the six canonical country formats, fallback grouping, stripped `tel:` URL, icon, and native attributes. | DONE |
| `base/progress.templ` | `Progress` | `Progress`. | DONE |
| `base/radio/radio.templ` | `RadioGroup`, `CardItem` | `RadioGroup`, `Radio` with card presentation. | DONE |
| `base/select.templ` | `Select` | `Select`. | DONE |
| `base/selects/search_select.templ` | `SearchOptions`, `SearchSelect` | `SearchSelect`; async options, loading/error/empty states, and the option list are part of its typed API. | COMPOSED |
| `base/slider/slider.templ` | `Slider` | `Slider`. | DONE |
| `base/slot/slot.templ` | `Slot`, `Streamer` | `Slot`, `Streamer`, `SlotManagerProvider`, and `createSlotManager`; streamed fragments use typed tagged chunks. | ADAPTER |
| `base/tab/tab.templ` | `Root`, `List`, `Button`, `Content`, `Link` | `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent`, `TabLink`. | DONE |
| `base/tab/tab.templ` | `BoostedLink`, `BoostedContent` | `BoostedLink` + `BoostedContent` with `BoostedTabLoader`; requests are abortable, stale responses are ignored, selection is controlled by `Tabs`, and URL push is opt-in. | ADAPTER |
| `base/table.templ` | `TableRow`, `TableCell`, `Table` | `TableRow`, `TableCell`, `Table` (plus public head/body/caption primitives). | DONE |
| `base/table_empty_state.templ` | `TableEmptyState` | `TableEmptyState`. | DONE |
| `base/toast/toast.templ` | `Container` | `ToastProvider` / `ToastContainer` owns the bounded event-driven queue, timer lifecycle, pause/resume, imperative context, and canonical live region. | DONE |
| `base/toggle/toggle.templ` | `Toggle` | `Toggle`. | DONE |
| `charts/chars.templ` | `Chart` | `Chart` accepts arbitrary Apex options and an injected `ApexChartFactory`/constructor, queues render/update/recreate/destroy, restores scope-shared hidden series, masks circular series, handles `sdk:rerenderCharts`, cleanup, errors, and retry. | ADAPTER |
| `copy_button/copy_button.templ` | `CopyButton`, `CopyableText` | `CopyButton`, `CopyableText`; clipboard behavior is injectable and reports success/failure. | DONE |
| `export/export_dropdown.templ` | `ExportDropdown` | `ExportDropdown` + typed `ExportRequest`/`ExportFile` adapter, with default URL fetch and download path. | ADAPTER |
| `filters/default.templ` | `SearchFieldsTrigger`, `SearchFields`, `Search`, `PageSize`, `CreatedAt`, `Default` | `DefaultFilters` plus the six individually exported presets map the canonical native query names to typed controlled/uncontrolled `FilterQueryState`. | DONE |
| `filters/drawer.templ` | `Drawer` | `FiltersDrawer`. | DONE |
| `help/context.templ` | `Context`, `Hint` | `HelpContext`, `HelpHint`. | DONE |
| `help/link.templ` | `Link` | `HelpLink` + `helpDocURL`. | DONE |
| `illustrations/empty_table.templ` | `EmptyTable` | `EmptyTable` (backed by `EmptyTableIllustration`). | DONE |
| `import/example_table.templ` | `ExampleTable` | `ExampleTable`. | DONE |
| `import/import_errors.templ` | `ImportErrors` | `ImportErrors`. | DONE |
| `import/import_form.templ` | `ImportFormFields`, `DownloadTemplateButton` | `ImportForm`; fields and template download are typed props/states inside the public form. | COMPOSED |
| `import/import_page.templ` | `ImportPage`, `ImportPageContent`, `ImportContent`, `ColumnList`, `ExampleSection` | `ImportPage`; page/content, column list, example, validation, upload, cancel, retry, progress, and result states are composed from its typed config/adapters. | COMPOSED |
| `import/run_progress.templ` | `RunProgress`, `RunResult` | `RunProgress`, `RunResult`; polling/submission is supplied through `ImportSubmitAdapter`/`ImportUploadAdapter` and abort signals. | ADAPTER |
| `loaders/hand.templ` | `Hand` | `Hand` / `HandLoader`. | DONE |
| `loaders/lazy.templ` | `DefaultLazyLoader` | `Loader` or `Spinner` is the feature-equivalent default visual. | DONE |
| `loaders/lazy.templ` | `LazyLoad` | `LazyLoad` + typed `LazyLoadAdapter`; load/visible triggers, encoded params, abort, retry/error states, raw remote fragments, injectable HTML rendering, and inner/outer replacement are public. | ADAPTER |
| `loaders/skeleton.templ` | `Skeleton`, `SkeletonText`, `SkeletonCard`, `SkeletonTable` | `Skeleton`, `SkeletonText`, `SkeletonCard`, `SkeletonTable`. | DONE |
| `loaders/spinner.templ` | `Spinner` | `Spinner`. | DONE |
| `multilang/details_view.templ` | `DetailsView`, `DetailsViewCompact` | `DetailsView`, `DetailsViewCompact`; locale labels are typed client input. | DONE |
| `multilang/form_input.templ` | `FormInputWithLabel`, `LocaleInput`, `FormInput`, `FormInputWithJS`, `FormInputWithJSAndLabel` | `MultiLangFormInput` / `FormInput`; four locale tabs, labels, validation, add/remove, controlled state, and keyboard behavior replace the script variants. | ADAPTER |
| `multilang/table_cell.templ` | `TableCell` | multilang `TableCell` / `MultiLangTableCell`. | DONE |
| `scaffold/actions/actions.templ` | `Action`, `Actions`, `RowActions` | `Action`, `Actions`, `RowActions`. | DONE |
| `scaffold/filterbuilder/builder.templ` | `Builder`, `BuilderOOB` | `FilterBuilder`; controlled conditions and callbacks replace the OOB swap endpoint. | ADAPTER |
| `scaffold/filterbuilder/chip.templ` | `Chip` | `FilterChip`. | DONE |
| `scaffold/filterbuilder/options.templ` | `GroupedOptions` | `GroupedOptions`. | DONE |
| `scaffold/filters/filters.templ` | `Dropdown`, `DropdownItem`, `Component`, `AsSideFilter` | `FilterDropdown`, `FilterDropdownItem`, `FiltersBar`, `SideFilter`. | DONE |
| `scaffold/form/form.templ` | `FormFields`, `FormFooter`, `Form`, `FormContent`, `FormWithErrors`, `Page` | `FormLayout` / `ScaffoldForm`, `FormContent`, `FormActions`; field errors, header/body/footer/page layout, native submit, and callbacks are typed props. | COMPOSED |
| `scaffold/kanban/kanban.templ` | `Page`, `Content`, `ColumnCards` | `Kanban`, `KanbanBoard`, `KanbanColumn`, `KanbanCard`; typed move/activate callbacks replace endpoint submission. | ADAPTER |
| `scaffold/table/drawers.templ` | `DefaultDrawer`, `DetailsDrawer` | `DefaultTableDrawer`, `DetailsDrawer`. | DONE |
| `scaffold/table/htmx_handler.templ` | `ContentHTMX` | `ContentHTMX` + `ScaffoldTableAdapter` replaces target-dependent HTMX responses with typed query loads, abort, stale-response suppression, retry, and infinite append. | ADAPTER |
| `scaffold/table/table.templ` | `DateTime`, `SearchClearButton`, `Rows`, `InfiniteScrollSpinner`, `FillerRowsWrapper`, `Table`, `TableContent`, `TableSection`, `DeferredPanels`, `DefaultPanelSkeleton`, `TableSettingsTrigger`, `EmbeddedContent`, `Content`, `Page` | `TableWorkflow` exports all 14 presentation/workflow components with canonical classes, responsive priority, sorting, search clearing, infinite loading, filler wrappers, settings trigger, deferred panels, and embedded/content/page shells. | DONE |
| `selects/countries.templ` | `CountriesSelect` | `CountriesSelect` / `CountrySelect`, with injectable localized country labels. | DONE |
| `sidebar/sidebar.templ` | `Sidebar`, `SidebarPinnedNav`, `SidebarCollapsedWorkspaceTabs`, `SidebarNav`, `SidebarNode`, `SidebarLinkNode`, `SidebarGroupNode`, `CollapsedGroupFlyout`, `SidebarFallbackIcon`, `SidebarBetaBadge` | `Sidebar`, `SidebarNavigation`, `SidebarGroup`, typed `SidebarNode`, `SidebarFallbackIcon`, `SidebarBetaBadge`; pinned/workspace/collapsed/flyout DOM is composed by `Sidebar`. | COMPOSED |
| `spotlight/spotlight.templ` | `Spotlight`, `ResultBadgeItem`, `LinkItem`, `SpotlightItem`, `NotFound`, `SpotlightResults`, `SpotlightItems`, `SpotlightGroup`, `SpotlightItemsCollapsible` | `Spotlight`, `SpotlightResultBadge`, `SpotlightLinkItem`, `SpotlightItem`, `SpotlightNotFound`, `SpotlightResults`, result iteration, `SpotlightGroup`, `SpotlightItemsCollapsible`; search is a typed async provider. | ADAPTER |
| `upload_input.templ` | `UploadTarget`, `UploadHelpersScript`, `UploadInput` | `UploadDropzone` and `UploadList`; drag/drop, validation, progress, errors, retry/remove, native selection, and callbacks replace the helper script/backend coupling. | ADAPTER |
| `user/language-select.templ` | `LanguageSelect` | `LanguageSelect`. | DONE |

## Remaining gaps

All **185 of 185** public templ functions now have a concrete Solid UI or typed adapter mapping. No public subscription UI templ component exists under `components`; the non-visual server package under `pkg/subscription` remains outside this UI export audit.
