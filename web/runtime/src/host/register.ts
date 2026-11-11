// Side-effect module: registers the SDK resolution hooks (see loader.ts) for every later import().
import { register } from "node:module";

register("./loader.mjs", import.meta.url, { data: { sdkDir: new URL("./sdk/", import.meta.url).href } });
