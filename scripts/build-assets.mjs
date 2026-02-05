import { cp, mkdir, readdir, stat } from 'node:fs/promises'
import path from 'node:path'

const repoRoot = path.resolve(process.cwd())
const srcFontsDir = path.join(repoRoot, 'modules/core/presentation/assets/fonts')
const outFontsDir = path.join(repoRoot, 'assets/fonts')
const srcBichatCss = path.join(repoRoot, 'ui/src/bichat/styles.css')
const outBichatCss = path.join(repoRoot, 'dist/bichat/styles.css')

async function copyDir(src, dst) {
  await mkdir(dst, { recursive: true })
  const entries = await readdir(src)
  await Promise.all(
    entries.map(async (name) => {
      const srcPath = path.join(src, name)
      const dstPath = path.join(dst, name)
      const s = await stat(srcPath)
      if (s.isDirectory()) {
        await copyDir(srcPath, dstPath)
        return
      }
      await cp(srcPath, dstPath)
    }),
  )
}

await copyDir(srcFontsDir, outFontsDir)

await mkdir(path.dirname(outBichatCss), { recursive: true })
await cp(srcBichatCss, outBichatCss)
