// Package theme holds the Lens behavior defaults that the Go side owns and
// ships to the runtime inside the document contract.
//
// Lens design tokens (colors, hairlines, radii, typography, shadows) are not
// declared here: they live in web/lens/src/styles.css as --lens-* custom
// properties, which is what actually renders. There is no Go mirror of them to
// keep in sync.
package theme

// DebounceMs is the default debounce, in milliseconds, for Lens filter and
// search inputs. It is emitted as the document's theme.debounceMs so the React
// runtime and any other renderer share one value.
const DebounceMs = 500
