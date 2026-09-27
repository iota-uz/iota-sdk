import { cp, mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const repositoryRoot = path.resolve(packageRoot, '../..')
const outputRoot = path.join(packageRoot, 'package-dist')
const sourceFonts = path.join(repositoryRoot, 'modules/core/presentation/assets/fonts')
const outputFonts = path.join(outputRoot, 'fonts')
const stylesheetPath = path.join(outputRoot, 'standalone.css')

const fontPaths = [
  'Inter.var.woff2',
  'Gilroy/Gilroy-Regular.woff2',
  'Gilroy/Gilroy-Medium.woff2',
  'Gilroy/Gilroy-Semibold.woff2',
]

await mkdir(outputFonts, { recursive: true })
for (const fontPath of fontPaths) {
  const destination = path.join(outputFonts, fontPath)
  await mkdir(path.dirname(destination), { recursive: true })
  await cp(path.join(sourceFonts, fontPath), destination)
}

const stylesheet = await readFile(stylesheetPath, 'utf8')
await writeFile(stylesheetPath, stylesheet.replaceAll('/assets/fonts/', './fonts/'))
