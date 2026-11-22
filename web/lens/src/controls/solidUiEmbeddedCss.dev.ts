// Development, test and story builds resolve `@iota-uz/sdk/solid-ui/embedded-css`
// here: the kit's utility sheet compiled from its sources (see kitCss.ts).
// The published package keeps the import external and the host's
// @iota-uz/sdk install serves the prebuilt string module.
import css from '../../../solid-ui/src/embedded.css?kit-css'

export default css
