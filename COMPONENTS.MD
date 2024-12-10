# IOTA SDK Components

## Requirements
- [Alpine.js](https://alpinejs.dev)
- [HTMX](https://htmx.org)
- [Templ](https://templ.guide)

## Table of contents
* [Button](#button)
  * [Usage](#usage)
  * [API](#api)
* [Input](#input)
  * [Usage](#usage-1)
  * [API](#api-1)
* [Select](#select)
  * [Usage](#usage-2)
  * [API](#api-2)
* [Combobox](#combobox)
  * [Usage](#usage-3)
  * [API](#api-3)
* [Dialog](#dialog)
  * [Usage](#usage-4)
  * [API](#api-4)
* [Dropdown](#dropdown)
  * [Usage](#usage-5)
  * [API](#api-5)
* [UploadInput](#upload-input)
  * [Usage](#usage-6)
  * [API](#api-6)

## Button
Button component to render a button or a link

### Usage
#### Primary button
```go
import "github.com/iota-agency/iota-sdk/components/base/button"

templ PrimaryButtonDemo() {
  @button.Primary(button.Props{Size: button.SizeNormal}) {
    Primary
  }
}
```
#### Button link
```go
import "github.com/iota-agency/iota-sdk/components/base/button"

templ PrimaryLinkDemo() {
  @button.Primary(button.Props{Size: button.SizeNormal, Href: "https://iota.uz"}) {
    IOTA.uz
  }
}
```
#### Danger button with icon
```go
import "github.com/iota-agency/iota-sdk/components/base/button"

templ PrimaryLinkDemo() {
  @button.Danger(button.Props{Size: button.SizeNormal, Icon: icons.PlusCirlce(icons.Props{Size:"16"})}) {
    New
  }
}
```

### API

| Name    | Type                                                | Description
|---      |---                                                  |---
| Size    | [`Size`](#size)                                     | Size of the button. `SizeNormal` is default
| Fixed   | `boolean`                                           | Determines if y-axis padding is same as x-axis padding. `false` by default, set it to `true` if you want to have a square button or a link
| Href    | `string`                                            | Link to some resource. This turns the component into an anchor tag.
| Rounded | `boolean`                                           | Make button fully round. `false` by default.
| Loading | `boolean`                                           | Loading state of the button. `false` by default.
| Class   | `string`                                            | Additional classes to button element.
| Icon    | `templ.Component`                                   | Icon component that appears on the left side of the button.
| Attrs   | `templ.Attributes`                                  | Additional HTML atttributes to the button element.

### Size
* `SizeNormal` - default size of the button
* `SizeMD` - medium size of the button
* `SizeSM` - small size of the button
* `SizeXS` - extra small size of the button

## Input
Input component to render an input element

### Usage

#### Text input
```go
import "github.com/iota-agency/iota-sdk/components/base/input"

templ TextInputDemo() {
  @input.Text(&input.Props{
    Label: "Label",
    Placeholder: "Input placeholder"
  })
}
```
#### Search input
```go
import "github.com/iota-agency/iota-sdk/components/base/input"

templ SearchInputDemo() {
  @input.Text(&input.Props{
    Addon: &input.Addon{
      Position:  input.AddonLeft,
      Component: icons.MagnifyingGlass(icons.Props{Size: "20"}),
    },
    Placeholder: "Search",
  })
}
```
#### Email input
```go
import "github.com/iota-agency/iota-sdk/components/base/input"

templ EmailInputDemo() {
  @input.Email(&input.Props{
    Label: "Email",
    Placeholder: "t@iota.uz"
  })
}
```
#### Date input
```go
import "github.com/iota-agency/iota-sdk/components/base/input"

templ DateInputDemo() {
  @input.Date(&input.Props{
    Label: "Supplied at"
  })
}
```
#### Password input
```go
import "github.com/iota-agency/iota-sdk/components/base/input"

templ PasswordIputDemo() {
  @input.Password(&input.Props{
    Label: "Password"
  })
}
```

### API

| Name          | Type                                                | Description
|---            |---                                                  |---
| Placeholder   | `string`                                            | Placeholder text of the input
| Label         | `string`                                            | Label text of the input
| Class         | `string`                                            | Additional classes to the input element
| Attrs         | `templ.Attributes`                                  | Additional HTML attributes to the input element
| Addon         | [`Addon`](#addon)                                   | Input addon.
| Error         | `string`                                            | Error message underneath the input element.

### Addon
| Name        | Type                                                | Description
|---          |---                                                  |---
| Render      | `func (props *Props) templ.Component`               | Custom render function if don't want to provide `Component`
| Component   | `templ.Component`                                   | Addon component to render
| Position    | [`AddonPosition`](#addonposition)

### AddonPosition
* `AddonRight` - right addon position
* `AddonLeft` - left addon position

## Select
Basic select component

### Usage

#### Basic select
```go
import "github.com/iota-agency/iota-sdk/components/base"

templ BasicSelectDemo() {
  @base.Select(&base.SelectProps{
    Label: "Language",
    Placeholder: "Select language"
  }) {
    <option value="en">en</option>
    <option value="uz">uz</option>
  }
}
```
#### Select with prefix
```go
import "github.com/iota-agency/iota-sdk/components/base"

templ BasicSelectWithPrefixDemo() {
  @base.Select(&base.SelectProps{
    Prefix: "Per page",
  }) {
    <option>15</option>
    <option>25</option>
    <option>50</option>
    <option>100</option>
  }
}
```

### API
| Name          | Type                                                | Description
|---            |---                                                  |---
| Placeholder   | `string`                                            | Placeholder text of the select
| Label         | `string`                                            | Label text of the select
| Class         | `string`                                            | Additional classes to the select element
| Attrs         | `templ.Attributes`                                  | Additional HTML attributes to the select element
| Prefix        | `string`                                            | Prefix test of the select
| Error         | `string`                                            | Error message underneath the select element.


## Combobox
A more advanced select

### Usage

#### Basic
```go
import "github.com/iota-agency/iota-sdk/components/base"

templ BasicComboboxDemo() {
  @base.Combobox(base.ComboboxProps{
    Label: "Tags",
    Placeholder: "Select tags",
    Multiple: true,
  }) {
    <option value="tag-1">Tag 1</option>
    <option value="tag-2">Tag 2</option>
  }
}
```

#### Dynamic
```go
import "github.com/iota-agency/iota-sdk/components/base"

templ BasicComboboxDemo() {
  @base.Combobox(base.ComboboxProps{
    Label: "Tag",
    Placeholder: "Select tag",
    Endpoint: "/endpoint-that-returns-options",
    Searchable: true,
    NotFoundText: "No tags found",
    Name: "tagID"
  }) {}
}
```

### API
| Name          | Type                                                | Description
|---            |---                                                  |---
| Placeholder   | `string`                                            | Placeholder text of the combobox
| Label         | `string`                                            | Label text of the combobox
| Class         | `string`                                            | Additional classes to the combobox element
| Multiple      | `boolean`                                           | Whether multiple options can be selected. `false` by default
| Name          | `string`                                            | Name of submitted form values
| Form          | `string`                                            | Name of the form this combobox belongs to
| Endpoint      | `string`                                            | Endpoint for fettching dynamic list of options.
| Searchable    | `boolean`                                           | Renders an `input` element as a trigger for search. `false` by default 
| NotFoundText  | `string`                                            | Text to render if there are no options. 

## Dialog
A simple dialog component. Only confirmation dialog supported for now.

### Usage

#### Confirmation dialog
```go
import "github.com/iota-agency/iota-sdk/components/base/dialog"

templ ConfirmationDemo() {
  @dialog.Confirmation(&dialog.Props{
    Heading:   "Delete this item",
    Text:      "Are you sure you want to delete this item?",
    Icon:      icons.Trash(icons.Props{Size: "20"}),
    Action:    "open-delete-item-confirmation",
    Attrs: templ.Attributes{
      "@closing": `({target}) => {
        if (target.returnValue === "confirm") {
          let deleteForm = document.getElementById("delete-form");
          htmx.trigger(deleteForm, "submit");
        }
      }`,
    },
  })
  <button @click="$dispatch('open-delete-item-confirmation')">Delete item</button>
}
```

### API
| Name          | Type                                                | Description
|---            |---                                                  |---
| Heading       | `string`                                            | Heading text of dialog
| Text          | `string`                                            | Text description of dialog
| CancelText    | `string`                                            | Text content of the cancel button
| ConfirmText   | `string`                                            | Text content of the confirm button
| Action        | `string`                                            | Name of the action to listen to on the window to toggle the dialog. 
| Attrs         | `templ.Attributes`                                  | Additional HTML attributes to the dialog element
| Icon          | `templ.Component`                                   | An icon that's rendered in the middle of dialog

## Dropdown

### Usage
```go
import "github.com/iota-agency/iota-sdk/components/base"

templ summary() {
  <summary>Menu</summary>
}

templ DropdownDemo(summary()) {
  @base.DetailsDropdown(summary()) {
    @base.DropdownItem(base.DropdownItemProps{Href: "/account"}) {
      Account
    }
    @base.DropdownItem(base.DropdownItemProps{Href: "/account/settings"}) {
      Settings
    }
    @base.DropdownItem(base.DropdownItemProps{Href: "/logout"}) {
      Log out
    }
  }
}

```
### API

summary - `templ.Component`

### Dropdown Item
| Name          | Type                                                | Description
|---            |---                                                  |---
| Href          | `string`                                            | Link to resource. 


## Upload input

### Usage

```go

templ UploadInputDemo() {
  @components.UploadInput(&components.UploadInputProps{
    Label:       "Select file",
    Placeholder: "CSV, XLSX",
    Accept:      "text/csv, application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, application/vnd.ms-excel",
    Name:        "FileID",
  })
}
```

### API
| Name          | Type                                                | Description
|---            |---                                                  |---
| ID            | `string`                                            | ID of the file input 
| Label         | `string`                                            | Label text of the file input
| Placeholder   | `string`                                            | Placeholder text of the file input
| Error         | `string`                                            | Error message underneath the upload box.
| Accept        | `string`                                            | Accept attribute of file input
| Name          | `string`                                            | Name of submitted form values
| Form          | `string`                                            | Name of the form this file input belongs to
| Class         | `string`                                            | Additional classes to the upload box
| Multiple      | `boolean`                                           | Whether multiple files can be selected. `false` by default
| Uploads       | `[]*viewmodels.Upload`                               | Initial list of uploads  