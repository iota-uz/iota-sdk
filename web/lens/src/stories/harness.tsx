/**
 * Solid story harness (the Ladle replacement).
 *
 * Reproduces the URL and markup contract the visual-regression lane depends on:
 * a story renders at `/?story=<id>&mode=preview&lens-vr=1`, `meta.json` lists
 * every id, and `.lens-root` plus the canvas-readiness markers are present. In
 * VR mode the harness waits for web fonts before mounting, because ECharts
 * measures label text once at mount — a font landing later leaves the chart
 * laid out with fallback metrics.
 */
import { render } from 'solid-js/web'
import type { JSX } from 'solid-js'
import '../styles.css'
import './fonts.css'
import './harness.css'

type Story = () => JSX.Element

const modules = import.meta.glob('../**/*.stories.tsx', { eager: true })

/** PascalCase export name -> kebab-case. */
const kebabExport = (name: string): string =>
  name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()

/** Ladle's story-name idiom: " - " becomes "--", remaining spaces become "-". */
const kebabName = (name: string): string =>
  name.replace(/,/g, '').trim().toLowerCase().replace(/\s+-\s+/g, '--').replace(/\s+/g, '-')

const stories = new Map<string, Story>()
const filePrefixes = new Map<string, string>()

for (const [path, module] of Object.entries(modules)) {
  const fileBase = path.split('/').pop()!.replace('.stories.tsx', '')
  const prefix = fileBase.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()
  filePrefixes.set(path, prefix)
  for (const [exportName, value] of Object.entries(module as Record<string, unknown>)) {
    if (typeof value !== 'function' || exportName === 'default') continue
    const story = value as Story & { storyName?: string }
    const name = typeof story.storyName === 'string' && story.storyName
      ? kebabName(story.storyName)
      : kebabExport(exportName)
    stories.set(`${prefix}--${name}`, story)
  }
}

const params = new URLSearchParams(window.location.search)
const storyId = params.get('story') ?? ''
const isVr = params.get('lens-vr') === '1'

if (isVr) {
  document.documentElement.dataset.lensVr = 'true'
  // Disable View Transitions and ECharts animation through the runtime's own
  // flag; Playwright additionally disables CSS animations.
}

async function boot(): Promise<void> {
  if (isVr) await document.fonts.ready
  const mount = document.getElementById('story-root')
  if (!mount) return
  const story = stories.get(storyId)
  if (!story) {
    const available = [...stories.keys()].sort().map((id) => `  ${id}`).join('\n')
    render(() => (
      <pre data-story-missing="true">{`Unknown story: ${storyId}\n\nKnown stories:\n${available}`}</pre>
    ), mount)
    return
  }
  render(story, mount)
}

void boot()

export const storyIds = [...stories.keys()].sort()
