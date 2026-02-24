import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const distDir = path.resolve(__dirname, '../../assets/dist')
const gitignorePath = path.join(distDir, '.gitignore')
const gitkeepPath = path.join(distDir, '.gitkeep')

fs.mkdirSync(distDir, { recursive: true })
fs.writeFileSync(gitignorePath, '*\n!.gitignore\n!.gitkeep\n', 'utf8')
if (!fs.existsSync(gitkeepPath)) {
  fs.writeFileSync(gitkeepPath, '', 'utf8')
}
