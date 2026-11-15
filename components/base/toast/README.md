# Toast Notification Component

A flexible toast notification system for IOTA SDK using Alpine.js and HTMX.

## Features

- 4 notification variants: Success, Error, Warning, Info
- Auto-dismiss after 8 seconds (configurable)
- Pause on hover
- Smooth animations
- Up to 20 notifications in stack
- Dark mode support
- HTMX integration for server-side triggers

## Installation

### 1. Add Container to Layout

Add the toast container to your main layout file (only once per page):

```go
import "github.com/iota-uz/iota-sdk/components/base/toast"

templ Layout() {
    // ... your layout content
    @toast.Container()
}
```

### 2. Client-Side Usage (Alpine.js)

Trigger toasts from the client using Alpine.js `$dispatch`:

```html
<button @click="$dispatch('notify', {
    variant: 'success',
    title: 'Success!',
    message: 'Your changes have been saved.'
})">
    Save Changes
</button>
```

### 3. Server-Side Usage (HTMX)

Trigger toasts from Go controllers using the htmx package:

```go
import "github.com/iota-uz/iota-sdk/pkg/htmx"

func (c *Controller) SaveData(w http.ResponseWriter, r *http.Request) {
    // ... save data logic

    // Trigger success toast
    htmx.ToastSuccess(w, "Saved!", "Your data has been saved successfully.")

    // Or use specific variants
    htmx.ToastError(w, "Error!", "Failed to save data.")
    htmx.ToastWarning(w, "Warning!", "This action cannot be undone.")
    htmx.ToastInfo(w, "Info", "Here is some useful information.")

    w.WriteHeader(http.StatusOK)
}
```

### 4. Failed requests

A global listener turns `htmx:responseError`, `htmx:sendError` and `htmx:timeout`
into an error toast, so a 4xx/5xx after a save is never silent. The toast shows
the plain-text or JSON (`message`/`error`/`detail`) body when there is one, and a
generic "Could not save changes" / "Could not load data" title otherwise. Opt a
request out with `data-htmx-error-toast="off"` on the element or an ancestor, or
call `event.preventDefault()` from an earlier listener.

### 5. Refreshing only the changed region

```go
// One HX-Trigger header: success toast + reload of the matching regions.
htmx.ToastSuccessAndRefresh(w, "Saved", "Order updated.", "#orders-table")
// or, without a toast
htmx.RefreshTargets(w, "#orders-table")
```

```html
<div id="orders-table" hx-get="/orders/table" hx-trigger="iota:refresh" hx-swap="outerHTML">…</div>
```

The region re-runs its own request, so scroll position and open drawers stay put.

## Available Variants

- `success` - Green checkmark icon
- `error` / `danger` - Red X icon
- `warning` - Yellow warning icon
- `info` - Blue info icon

## API Reference

### Client-Side

**Event Name:** `notify`

**Event Detail:**
```javascript
{
    variant: 'success' | 'error' | 'danger' | 'warning' | 'info',
    title: string,
    message: string
}
```

### Server-Side (Go)

```go
// Convenience functions
htmx.ToastSuccess(w http.ResponseWriter, title, message string)
htmx.ToastError(w http.ResponseWriter, title, message string)
htmx.ToastWarning(w http.ResponseWriter, title, message string)
htmx.ToastInfo(w http.ResponseWriter, title, message string)

// Advanced functions with timing control
htmx.TriggerToast(w http.ResponseWriter, variant htmx.ToastVariant, title, message string)
htmx.TriggerToastAfterSwap(w http.ResponseWriter, variant htmx.ToastVariant, title, message string)
htmx.TriggerToastAfterSettle(w http.ResponseWriter, variant htmx.ToastVariant, title, message string)

// Available variants
htmx.ToastVariantSuccess
htmx.ToastVariantError
htmx.ToastVariantDanger
htmx.ToastVariantWarning
htmx.ToastVariantInfo
```

## Example

See the showcase page at `/_dev/components/other` for a live example.

## Customization

The toast container is positioned at the top-right corner by default. To customize:

1. Modify the container classes in `toast.templ`
2. Adjust the `displayDuration` in the Alpine.js data object (default: 8000ms)
3. Change maximum notifications by modifying the slice limit (default: 20)

## Accessibility

- Uses appropriate ARIA roles (`role="status"` for info/success, `role="alert"` for errors/warnings)
- Includes `aria-live="assertive"` on the container
- Close buttons have `aria-label="Close notification"`
