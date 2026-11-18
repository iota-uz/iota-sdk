// Side-effect module: registers the SDK resolution hooks (see loader.ts) for every later import().
import { register } from "node:module";

register("./loader.mjs", import.meta.url, {
  data: {
    sdkDir: new URL("./sdk/", import.meta.url).href,
    // The build worker's vendored node_modules (absent in a plain host install).
    vendorDir: new URL("../build/vendor/", import.meta.url).href,
  },
});
