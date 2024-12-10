# IOTA SDK Components

## Table of contents
- [Button](#button)
- [Input](#input)

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

### Props

| Name    | Type                                                | Description
|---      |---                                                  |---
| Size    | `Size` - `SizeNormal`, `SizeMD`, `SizeSM`, `SizeXS` | Size of the button
| Fixed   | `boolean`                                           | Determines if y-axis padding is same as x-axis padding. `false` by default, set it to `true` if you want to have a square button or a link
| Href    | `string`                                            | Link to some resource. This turns the component into an anchor tag.
| Rounded | `boolean`                                           | Make button fully round. `false` by default.
| Loading | `boolean`                                           | Loading state of the button. `false` by default.
| Class   | `string`                                            | Additional classes to button element.
| Icon    | `templ.Component`                                   | Icon component that appears on the left side of the button.
| Attrs   | `templ.Attributes`                                  | Additional HTML atttributes to the button element.