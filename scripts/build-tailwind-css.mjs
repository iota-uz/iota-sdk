import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..");

const inputPath = path.join(repoRoot, "styles/tailwind/input.css");
const generatedPath = path.join(repoRoot, "styles/tailwind/main.generated.css");

const sourceGlobs = [
  "../../cmd/**/*.{go,templ,html,js,ts,tsx}",
  "../../components/**/*.{go,templ,html,js,ts,tsx}",
  "../../modules/**/*.{go,templ,html,js,ts,tsx}",
  "../../pkg/**/*.{go,templ,html,js,ts,tsx}",
];

function buildSourceBlock() {
  const lines = sourceGlobs
    .map((glob) => `@source "${glob}";`)
    .join("\n");

  return [
    "/* AUTO-GENERATED SOURCES START */",
    lines,
    "/* AUTO-GENERATED SOURCES END */",
  ].join("\n");
}

async function run() {
  const input = await readFile(inputPath, "utf8");
  const sourceBlock = buildSourceBlock();

  const withoutGeneratedBlock = input
    .replace(/\/\* AUTO-GENERATED SOURCES START \*\/[\s\S]*?\/\* AUTO-GENERATED SOURCES END \*\/\n?/g, "")
    .trimEnd();

  const output = `${withoutGeneratedBlock}\n\n${sourceBlock}\n`;
  await writeFile(generatedPath, output, "utf8");
}

run().catch((error) => {
  console.error("Failed to generate Tailwind input:", error);
  process.exitCode = 1;
});
