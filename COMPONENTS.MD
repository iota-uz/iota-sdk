# IOTA SDK Components

## Table of contents
* [Button](#button)
  * [Usage](#usage)
  * [API](#api)
* [Input](#input)
  * [Usage](#usage)
  * [API](#api)

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