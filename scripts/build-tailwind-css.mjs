import { spawn } from "node:child_process";
import { readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..");

const argv = process.argv.slice(2);
const options = {
  clean: false,
  configPath: "styles/tailwind/pipeline.config.json",
  generateOnly: false,
  minify: false,
  tailwindArgs: [],
};

for (let i = 0; i < argv.length; i += 1) {
  const arg = argv[i];

  if (arg === "--clean") {
    options.clean = true;
    continue;
  }

  if (arg === "--generate-only") {
    options.generateOnly = true;
    continue;
  }

  if (arg === "--minify") {
    options.minify = true;
    continue;
  }

  if (arg === "--config") {
    const configPath = argv[i + 1];
    if (!configPath) {
      throw new Error("Missing value for --config");
    }
    options.configPath = configPath;
    i += 1;
    continue;
  }

  options.tailwindArgs.push(arg);
}

function createSourceBlock(sources) {
  const stableSources = [...new Set(sources)].sort();
  const lines = stableSources
    .map((source) => `@source "${source}";`)
    .join("\n");

  return [
    "/* AUTO-GENERATED SOURCES START */",
    lines,
    "/* AUTO-GENERATED SOURCES END */",
  ].join("\n");
}

function removeGeneratedSourceBlock(css) {
  return css.replace(
    /\/\* AUTO-GENERATED SOURCES START \*\/[\s\S]*?\/\* AUTO-GENERATED SOURCES END \*\/\n?/g,
    "",
  );
}

async function loadPipelineConfig(configPath) {
  const resolvedPath = path.resolve(repoRoot, configPath);
  const content = await readFile(resolvedPath, "utf8");
  const config = JSON.parse(content);

  return {
    generated: path.resolve(repoRoot, config.generated),
    input: path.resolve(repoRoot, config.input),
    output: path.resolve(repoRoot, config.output),
    sources: Array.isArray(config.sources) ? config.sources : [],
  };
}

async function generateInputFile(config) {
  const baseInput = await readFile(config.input, "utf8");
  const sourceBlock = createSourceBlock(config.sources);
  const normalizedInput = removeGeneratedSourceBlock(baseInput).trimEnd();
  const output = `${normalizedInput}\n\n${sourceBlock}\n`;

  await writeFile(config.generated, output, "utf8");
}

function runTailwind(config) {
  return new Promise((resolve, reject) => {
    const args = [
      "exec",
      "tailwindcss",
      "--input",
      path.relative(repoRoot, config.generated),
      "--output",
      path.relative(repoRoot, config.output),
      ...options.tailwindArgs,
    ];

    if (options.minify) {
      args.push("--minify");
    }

    const child = spawn("pnpm", args, {
      cwd: repoRoot,
      stdio: "inherit",
    });

    child.on("close", (code) => {
      if (code === 0) {
        resolve();
        return;
      }

      reject(new Error(`tailwindcss exited with code ${code}`));
    });
  });
}

async function cleanFiles(config) {
  await Promise.all([
    rm(config.generated, { force: true }),
    rm(config.output, { force: true }),
  ]);
}

async function run() {
  const config = await loadPipelineConfig(options.configPath);

  if (options.clean) {
    await cleanFiles(config);
    return;
  }

  await generateInputFile(config);

  if (options.generateOnly) {
    return;
  }

  await runTailwind(config);
}

run().catch((error) => {
  console.error("Failed to build Tailwind CSS:", error);
  process.exitCode = 1;
});
