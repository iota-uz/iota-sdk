import { createEffect, For, type Component } from 'solid-js'
import { Dynamic } from 'solid-js/web'
import { Alert } from './display/Alert'
import { Badge } from './display/Badge'
import { Card, CardHeader } from './display/Card'
import { Progress } from './display/Progress'
import { Skeleton } from './display/Skeleton'
import { Spinner } from './display/Spinner'
import { Button } from './forms/Button'
import { Checkbox } from './forms/Checkbox'
import { Input, MoneyInput, PasswordInput } from './forms/Input'
import { Select } from './forms/Select'
import { Textarea } from './forms/Textarea'
import { DateRangeClearButton } from './forms-advanced/DateRangeClearButton'
import { PhoneInput } from './forms-advanced/PhoneInput'
import { Radio, RadioGroup } from './forms-advanced/Radio'
import { Slider } from './forms-advanced/Slider'
import { Switch } from './forms-advanced/Switch'
import { Toggle } from './forms-advanced/Toggle'
import { UploadDropzone } from './forms-advanced/Upload'
import { BreadcrumbItem, BreadcrumbLink, BreadcrumbSeparator, Breadcrumbs } from './navigation/Breadcrumbs'
import { Dropdown, DropdownItem } from './navigation/Dropdown'
import { NavTabs, NavTabsContent, NavTabsList, NavTabsTrigger } from './navigation/NavTabs'
import { Pagination } from './navigation/Pagination'
import { Tabs, TabsContent, TabsList, TabsTrigger } from './navigation/Tabs'
import { ConfirmationDialog } from './overlays/Dialog'
import { ViewDrawer } from './overlays/Drawer'
import { Toast, ToastRegion } from './overlays/Toast'
import { Combobox } from './selection/Combobox'
import { CountriesSelect } from './selection/Countries'
import { DatePicker, DateRangePicker } from './selection/DatePicker'
import { SearchSelect } from './selection/SearchSelect'
import { Avatar, AvatarGroup } from './data/Avatar'
import { DescriptionList, DescriptionListItem, DescriptionListLabel, DescriptionListValue } from './data/DescriptionList'
import { ContentLoader } from './data/Loader'
import { Body, Caption, Cell, Head, Header, Row, Table } from './data/Table'
import { CopyButton, CopyableText } from './utilities/CopyButton'
import { ExportDropdown } from './utilities/ExportDropdown'
import { HelpContext } from './utilities/Help'
import { LanguageSelect } from './utilities/LanguageSelect'
import { ActionMenu, Actions } from './scaffold/Actions'
import { FilterDropdown, FiltersBar, SideFilter } from './scaffold/Filters'
import { SPECIMENS, stateFor, type SpecimenID, type SpecimenState, type Theme } from './catalog'

type AppProps = {
  specimenID: SpecimenID
  theme: Theme
}

type SpecimenProps = { state: SpecimenState }

function GalleryShell() {
  return (
    <section class="specimen" aria-labelledby="gallery-shell-title" data-testid="specimen-gallery-shell">
      <div class="specimen__heading">
        <p class="specimen__eyebrow">Solid UI parity</p>
        <h2 id="gallery-shell-title">Component specimen host</h2>
        <p>Stable tokens, states, and geometry for browser comparison with templ components.</p>
      </div>
      <div class="token-grid" aria-label="Reference surfaces">
        <For each={['Canvas', 'Surface', 'Border', 'Text']}>
          {(label, index) => (
            <div class={`token token--${index() + 1}`}>
              <span>{label}</span>
              <strong>{index() + 1}</strong>
            </div>
          )}
        </For>
      </div>
    </section>
  )
}

function ButtonsSpecimen(props: SpecimenProps) {
  const disabled = () => props.state === 'disabled'
  const loading = () => props.state === 'loading'
  return (
    <section class="specimen" aria-label="Button specimens" data-testid="specimen-buttons">
      <div class="specimen-row specimen-row--wrap">
        <Button data-vr-target="button-primary" disabled={disabled()} loading={loading()}>Primary action</Button>
        <Button variant="secondary">Secondary</Button>
        <Button variant="primary-outline">Outline</Button>
        <Button variant="danger">Delete</Button>
        <Button variant="ghost">Ghost</Button>
      </div>
      <div class="specimen-row specimen-row--wrap">
        <Button size="md">Medium</Button>
        <Button size="sm">Small</Button>
        <Button size="xs">Extra small</Button>
        <Button fixed aria-label="Add" icon={<span aria-hidden="true">+</span>} />
        <Button href="#button-link">Link button</Button>
      </div>
    </section>
  )
}

function FormControlsSpecimen(props: SpecimenProps) {
  const disabled = () => props.state === 'disabled'
  const error = () => props.state === 'error' ? 'This field is required' : undefined
  const checked = () => props.state === 'checked'
  return (
    <section class="specimen specimen-grid" aria-label="Form control specimens" data-testid="specimen-form-controls">
      <Input
        id="gallery-name"
        data-vr-target="form-input"
        label="Product name"
        placeholder="Enter a name"
        defaultValue="Insurance product"
        error={error()}
        disabled={disabled()}
        required
      />
      <Select
        id="gallery-kind"
        data-vr-target="form-select"
        label="Insurance type"
        defaultValue="voluntary"
        options={[
          { value: 'mandatory', label: 'Mandatory' },
          { value: 'voluntary', label: 'Voluntary' },
        ]}
        disabled={disabled()}
        error={error()}
      />
      <PasswordInput id="gallery-password" label="Password" defaultValue="secret-value" disabled={disabled()} />
      <MoneyInput id="gallery-amount" name="amount" label="Insurance amount" currency="UZS" defaultValue={125000000} addonRight="UZS" disabled={disabled()} />
      <Textarea id="gallery-notes" label="Notes" defaultValue="Terms and exclusions" rows={3} disabled={disabled()} error={error()} />
      <div class="specimen-checks">
        <Checkbox id="gallery-enabled" label="Product enabled" defaultChecked={checked()} disabled={disabled()} error={error()} />
        <Checkbox id="gallery-indeterminate" label="Partially selected" indeterminate disabled={disabled()} />
      </div>
    </section>
  )
}

function DisplaySpecimen() {
  return (
    <section class="specimen specimen-stack" aria-label="Display specimens" data-testid="specimen-display">
      <Card data-vr-target="display-card" header={<CardHeader>Product summary</CardHeader>}>
        A shared surface with the same header, border, radius, and spacing as templ.
      </Card>
      <div class="specimen-row">
        <Alert variant="success">Product saved successfully</Alert>
        <Alert variant="error">Unable to save product</Alert>
      </div>
      <div class="specimen-row specimen-row--wrap">
        <For each={['pink', 'yellow', 'green', 'blue', 'purple', 'gray'] as const}>
          {(variant) => <Badge variant={variant} class="specimen-badge" data-vr-target={variant === 'pink' ? 'display-badge' : undefined}>{variant}</Badge>}
        </For>
      </div>
      <Progress value={68} target={100} valueLabel="68%" targetLabel="100%" />
      <div class="specimen-row">
        <Spinner label="Loading products" />
        <Skeleton variant="card" class="specimen-skeleton" />
      </div>
    </section>
  )
}

function AdvancedFormsSpecimen(props: SpecimenProps) {
  const disabled = () => props.state === 'disabled'
  const error = () => props.state === 'error' ? 'Choose a valid value' : undefined
  const checked = () => props.state === 'checked'
  return (
    <section class="specimen specimen-grid" aria-label="Advanced form specimens" data-testid="specimen-advanced-forms">
      <RadioGroup label="Payment frequency" name="frequency" defaultValue={checked() ? 'monthly' : 'annual'} disabled={disabled()} error={error()} orientation="horizontal">
        <Radio value="annual" label="Annual" />
        <Radio data-vr-target="advanced-radio" value="monthly" label="Monthly" />
      </RadioGroup>
      <div class="specimen-stack specimen-stack--compact">
        <Switch data-vr-target="advanced-switch" label="Automatic renewal" defaultChecked={checked()} disabled={disabled()} error={error()} />
        <Toggle
          aria-label="Policy mode"
          options={[{ value: 'standard', label: 'Standard' }, { value: 'custom', label: 'Custom', disabled: disabled() }]}
          defaultValue={checked() ? 'custom' : 'standard'}
          rounded="smooth"
        />
      </div>
      <Slider label="Coverage" defaultValue={checked() ? 80 : 45} min={0} max={100} disabled={disabled()} error={error()} data-vr-target="advanced-slider" />
      <PhoneInput id="gallery-phone" label="Phone number" defaultValue="+998 90 123 45 67" disabled={disabled()} error={error()} />
      <div class="specimen-date-control">
        <Input id="gallery-date-range" label="Coverage dates" defaultValue="14 Sep 2026 – 14 Sep 2027" disabled={disabled()} error={error()} />
        <DateRangeClearButton value="2026-09-14/2027-09-14" disabled={disabled()} />
      </div>
      <UploadDropzone
        label="Policy attachments"
        placeholder="Drop PDF or image here"
        accept="application/pdf,image/*"
        error={error()}
        inputProps={{ disabled: disabled() }}
        defaultItems={[{ id: 'policy-1', name: 'policy-terms.pdf', mimeType: 'application/pdf', size: '128 KB' }]}
      />
    </section>
  )
}

function NavigationSpecimen(props: SpecimenProps) {
  const selected = () => props.state === 'selected' ? 'history' : 'details'
  const open = () => props.state === 'open'
  return (
    <section class="specimen specimen-stack" aria-label="Navigation specimens" data-testid="specimen-navigation">
      <Breadcrumbs>
        <BreadcrumbItem><BreadcrumbLink href="#products">Products</BreadcrumbLink></BreadcrumbItem>
        <BreadcrumbSeparator />
        <BreadcrumbItem>New product</BreadcrumbItem>
      </Breadcrumbs>
      <Tabs defaultValue={selected()}>
        <TabsList aria-label="Product sections">
          <TabsTrigger data-vr-target="navigation-tab" value="details">Details</TabsTrigger>
          <TabsTrigger value="history">History</TabsTrigger>
          <TabsTrigger value="documents" disabled>Documents</TabsTrigger>
        </TabsList>
        <TabsContent value="details" class="specimen-tab-content">Product details</TabsContent>
        <TabsContent value="history" class="specimen-tab-content">Change history</TabsContent>
        <TabsContent value="documents" class="specimen-tab-content">Documents</TabsContent>
      </Tabs>
      <NavTabs defaultValue={selected()}>
        <NavTabsList aria-label="Product views">
          <NavTabsTrigger value="details">Details</NavTabsTrigger>
          <NavTabsTrigger value="history">History</NavTabsTrigger>
        </NavTabsList>
        <NavTabsContent value="details" class="specimen-tab-content">Product details view</NavTabsContent>
        <NavTabsContent value="history" class="specimen-tab-content">Product history view</NavTabsContent>
      </NavTabs>
      <div class="specimen-row specimen-row--spread">
        <Pagination data-vr-target="navigation-pagination" current={4} totalPages={7} href={(page) => `#page-${page}`} />
        <Dropdown
          label="Product actions"
          defaultOpen={open()}
          trigger={<Button data-vr-target="navigation-dropdown" variant="secondary">Actions</Button>}
        >
          <DropdownItem>Duplicate</DropdownItem>
          <DropdownItem href="#archive">Archive</DropdownItem>
          <DropdownItem disabled>Delete</DropdownItem>
        </Dropdown>
      </div>
    </section>
  )
}

function DialogSpecimen(props: SpecimenProps) {
  return (
    <section class="specimen specimen-stack" aria-label="Dialog specimen" data-testid="specimen-dialog">
      <p>Confirmation dialogs retain focus and modal geometry across runtimes.</p>
      <ConfirmationDialog
        defaultOpen={props.state === 'open'}
        heading="Delete insurance product?"
        text="This action cannot be undone."
        cancelText="Cancel"
        confirmText="Delete"
      />
    </section>
  )
}

function DrawerSpecimen(props: SpecimenProps) {
  return (
    <section class="specimen specimen-stack" aria-label="Drawer specimen" data-testid="specimen-drawer">
      <p>View drawers preserve the canonical edge, surface, and responsive width.</p>
      <ViewDrawer defaultOpen={props.state === 'open'} title="Product details">
        <div class="specimen-drawer-content">
          <strong>Travel insurance</strong>
          <p>Contract series TRV-2026 with worldwide coverage.</p>
        </div>
      </ViewDrawer>
    </section>
  )
}

function ToastsSpecimen(props: SpecimenProps) {
  return (
    <section class="specimen specimen-stack" aria-label="Toast specimens" data-testid="specimen-toasts">
      <p>Notifications use a fixed region when the specimen is open.</p>
      {props.state === 'open' && (
        <ToastRegion>
          <Toast variant="success" title="Product saved" message="The draft is ready for review." duration={0} />
          <Toast variant="warning" title="Missing tariff" message="Add a tariff before publishing." duration={0} />
          <Toast variant="error" title="Publish failed" message="Try again after checking the fields." duration={0} />
        </ToastRegion>
      )}
    </section>
  )
}

const productOptions = [
  { value: 'travel', label: 'Travel insurance' },
  { value: 'property', label: 'Property insurance' },
  { value: 'health', label: 'Health insurance', disabled: true },
] as const

function SelectionSpecimen(props: SpecimenProps) {
  const disabled = () => props.state === 'disabled'
  const selected = () => props.state === 'selected'
  return (
    <section class="specimen specimen-grid" aria-label="Selection specimens" data-testid="specimen-selection">
      <Combobox
        data-vr-target="selection-combobox"
        label="Product"
        placeholder="Choose a product"
        options={productOptions}
        defaultValue={selected() ? 'travel' : undefined}
        searchable
        disabled={disabled()}
      />
      <SearchSelect
        id="search-select-solid"
        label="Customer"
        placeholder="Search customers"
        defaultSelectedOption={selected() ? { value: 'customer-1', label: 'Acme Insurance LLC' } : undefined}
        loadOptions={async () => [{ value: 'customer-1', label: 'Acme Insurance LLC' }, { value: 'customer-2', label: 'Atlas Group' }]}
        minQueryLength={0}
        debounceMs={0}
        disabled={disabled()}
      />
      <CountriesSelect label="Country" countries={['UZ', 'KZ', 'KG']} defaultValue={selected() ? 'UZ' : ''} placeholder="Choose a country" disabled={disabled()} />
      <DatePicker id="date-picker-solid" label="Start date" placeholder="Choose a date" defaultValue={selected() ? ['2026-09-14'] : []} locale="en-US" disabled={disabled()} />
      <DateRangePicker label="Coverage period" placeholder="Choose a period" defaultValue={selected() ? ['2026-09-14', '2027-09-14'] : []} locale="en-US" disabled={disabled()} />
    </section>
  )
}

function DataSpecimen(props: SpecimenProps) {
  const selected = () => props.state === 'selected'
  return (
    <section class="specimen specimen-stack" aria-label="Data display specimens" data-testid="specimen-data">
      <div class="specimen-row specimen-row--spread">
        <div class="specimen-row">
          <Avatar data-vr-target="data-avatar" initials="DK" />
          <Avatar initials="AI" variant="square" />
          <AvatarGroup avatars={[{ initials: 'AA' }, { initials: 'BB' }, { initials: 'CC' }, { initials: 'DD' }]} limit={3} />
        </div>
        <Badge variant="green">Active</Badge>
      </div>
      <DescriptionList title="Product details" subtitle="TRV-2026" collapsible defaultOpen={props.state === 'open'}>
        <DescriptionListItem><DescriptionListLabel>Class</DescriptionListLabel><DescriptionListValue>Travel</DescriptionListValue></DescriptionListItem>
        <DescriptionListItem><DescriptionListLabel>Currency</DescriptionListLabel><DescriptionListValue>UZS</DescriptionListValue></DescriptionListItem>
      </DescriptionList>
      <ContentLoader loading={props.state === 'loading'}>
        <Table data-vr-target="data-table">
          <Caption>Insurance products</Caption>
          <Header><Row><Head sortable sortDirection="asc">Product</Head><Head>Code</Head><Head priority={2}>Status</Head></Row></Header>
          <Body>
            <Row selected={selected()} onSelectedChange={() => undefined}><Cell>Travel insurance</Cell><Cell>TRV-2026</Cell><Cell priority={2}><Badge variant="green">Active</Badge></Cell></Row>
            <Row><Cell>Property insurance</Cell><Cell>PRP-2026</Cell><Cell priority={2}><Badge variant="yellow">Draft</Badge></Cell></Row>
          </Body>
        </Table>
      </ContentLoader>
    </section>
  )
}

function UtilitiesSpecimen(props: SpecimenProps) {
  const open = () => props.state === 'open'
  return (
    <section class="specimen specimen-stack" aria-label="Utility specimens" data-testid="specimen-utilities">
      <div class="specimen-row specimen-row--wrap">
        <CopyButton data-vr-target="utilities-copy" text="TRV-2026" />
        <CopyableText text="TRV-2026" />
        <LanguageSelect data-vr-target="utilities-language" label="Language" defaultValue="en" />
      </div>
      <div class="specimen-row specimen-row--wrap">
        <ExportDropdown formats={['excel', 'csv', 'json']} defaultOpen={open()} exporting={props.state === 'loading'} />
        <HelpContext
          title="Product configurator"
          summary="Configure classes, risks, and coverage terms."
          sections={[{ title: 'Before publishing', items: ['Add a tariff', 'Review required fields'] }]}
          defaultOpen={false}
          articlePath="product-configurator"
          articleLabel="Read guide"
        />
      </div>
    </section>
  )
}

function ScaffoldSpecimen(props: SpecimenProps) {
  const disabled = () => props.state === 'disabled'
  return (
    <section class="specimen specimen-stack" aria-label="Page scaffold specimens" data-testid="specimen-scaffold">
      <div class="specimen-row specimen-row--spread">
        <div class="specimen-row specimen-row--wrap"><Actions actions={[{ type: 'create', label: 'Create product', disabled: disabled() }, { type: 'import', label: 'Import', disabled: disabled() }]} /></div>
        <ActionMenu
          label="More actions"
          trigger={<Button data-vr-target="scaffold-actions" variant="secondary">More</Button>}
          defaultOpen={props.state === 'open'}
          actions={[{ label: 'Duplicate' }, { label: 'Archive' }, { label: 'Delete', disabled: true }]}
        />
      </div>
      <FiltersBar search={<Input label="Search" placeholder="Product name" disabled={disabled()} error={props.state === 'error' ? 'Check the query' : undefined} />} actions={<Button variant="secondary">Reset</Button>}>
        <FilterDropdown label="Status" name="status" options={[{ value: 'active', label: 'Active' }, { value: 'draft', label: 'Draft' }]} defaultValue={['active']} />
      </FiltersBar>
      <SideFilter name="class" selectAllLabel="All classes" options={[{ value: 'travel', label: 'Travel' }, { value: 'property', label: 'Property' }, { value: 'health', label: 'Health', disabled: true }]} defaultValue={['travel']} />
    </section>
  )
}

const SPECIMEN_RENDERERS: Record<SpecimenID, Component<SpecimenProps>> = {
  'gallery-shell': GalleryShell,
  buttons: ButtonsSpecimen,
  'form-controls': FormControlsSpecimen,
  'advanced-forms': AdvancedFormsSpecimen,
  display: DisplaySpecimen,
  navigation: NavigationSpecimen,
  dialog: DialogSpecimen,
  drawer: DrawerSpecimen,
  toasts: ToastsSpecimen,
  selection: SelectionSpecimen,
  data: DataSpecimen,
  utilities: UtilitiesSpecimen,
  scaffold: ScaffoldSpecimen,
}

export function App(props: AppProps) {
  const specimen = () => SPECIMENS.find((item) => item.id === props.specimenID)
  const renderer = () => SPECIMEN_RENDERERS[props.specimenID]
  const state = () => stateFor(props.specimenID, new URLSearchParams(window.location.search).get('state'))
  createEffect(() => document.documentElement.classList.toggle('dark', props.theme === 'dark'))

  return (
    <main class="gallery" data-specimen-id={props.specimenID} data-theme={props.theme}>
      <header class="gallery__header">
        <div>
          <p>IOTA SDK</p>
          <h1>Solid UI component gallery</h1>
        </div>
        <span class="theme-badge">{props.theme}</span>
      </header>
      <section class="gallery__description" aria-label="Selected specimen">
        <strong>{specimen()?.title}</strong>
        <span>{specimen()?.description}</span>
      </section>
      <Dynamic component={renderer()} state={state()} />
    </main>
  )
}
