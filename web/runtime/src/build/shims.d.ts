declare module "babel-preset-solid" {
  import type { PluginItem } from "@babel/core";
  const preset: (context: object, options?: { generate?: "dom" | "ssr" | "universal"; hydratable?: boolean }) => { plugins: PluginItem[] };
  export default preset;
}
