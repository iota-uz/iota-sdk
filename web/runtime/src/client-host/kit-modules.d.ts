// The UI kit is shared by value (kit.ts only re-exports its namespace) and its stylesheet is a generated string module; neither
// is typed here, so the runtime package does not compile the kit's Solid sources. Bundlers resolve both through the SDK package.
declare module '@iota-uz/sdk/solid-ui' {}
declare module '@iota-uz/sdk/solid-ui/embedded-css' {
  const css: string
  export default css
}
