import { createEffect, createMemo, type JSX } from 'solid-js'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/common'
import { useI18n } from '../../i18n/i18n'

let stylesInjected = false

function injectCodeStyles(): void {
  if (stylesInjected || typeof document === 'undefined') return
  stylesInjected = true
  const style = document.createElement('style')
  style.textContent = `
.bichat-md pre { background: #0f172a; color: #e2e8f0; border-radius: 0.5rem; padding: 0.75rem 1rem; overflow-x: auto; font-size: 0.8125rem; line-height: 1.5; }
.bichat-md code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.bichat-md :not(pre) > code { background: #f5f5f4; border-radius: 0.25rem; padding: 0.1rem 0.3rem; font-size: 0.85em; color: #44403c; }
.bichat-md .hljs-comment, .bichat-md .hljs-quote { color: #64748b; font-style: italic; }
.bichat-md .hljs-keyword, .bichat-md .hljs-selector-tag, .bichat-md .hljs-literal, .bichat-md .hljs-doctag { color: #c084fc; }
.bichat-md .hljs-string, .bichat-md .hljs-regexp, .bichat-md .hljs-addition { color: #86efac; }
.bichat-md .hljs-number, .bichat-md .hljs-built_in, .bichat-md .hljs-attr, .bichat-md .hljs-variable, .bichat-md .hljs-template-variable { color: #fbbf24; }
.bichat-md .hljs-title, .bichat-md .hljs-title.function_, .bichat-md .hljs-section, .bichat-md .hljs-name { color: #7dd3fc; }
.bichat-md .hljs-type, .bichat-md .hljs-class .hljs-title, .bichat-md .hljs-tag { color: #fda4af; }
.bichat-md .hljs-params, .bichat-md .hljs-symbol, .bichat-md .hljs-bullet, .bichat-md .hljs-meta { color: #e2e8f0; }
.bichat-md .hljs-deletion { color: #fca5a5; }
.bichat-md .hljs-emphasis { font-style: italic; }
.bichat-md .hljs-strong { font-weight: 600; }
`
  document.head.appendChild(style)
}

marked.setOptions({ gfm: true, breaks: true })

export function MarkdownRenderer(props: { content: string }): JSX.Element {
  const i18n = useI18n()
  let container: HTMLDivElement | undefined

  // sync parse: marked returns string unless async option is enabled
  const html = createMemo(() => DOMPurify.sanitize(marked.parse(props.content) as string))

  createEffect(() => {
    html()
    if (!container) return
    container.querySelectorAll('pre code').forEach((node) => {
      hljs.highlightElement(node as HTMLElement)
    })
    container.querySelectorAll('pre').forEach((pre) => {
      if (pre.parentElement?.classList.contains('bichat-code-wrap')) return
      const wrapper = document.createElement('div')
      wrapper.className = 'bichat-code-wrap group relative'
      pre.replaceWith(wrapper)
      wrapper.appendChild(pre)
      const button = document.createElement('button')
      button.type = 'button'
      button.setAttribute('aria-live', 'polite')
      button.className =
        'absolute right-2 top-2 hidden rounded-md border border-white/20 bg-white/10 px-2 py-1 text-xs text-white/80 transition hover:bg-white/20 group-hover:block'
      button.textContent = i18n.t('chat.copy')
      button.addEventListener('click', () => {
        void navigator.clipboard?.writeText(pre.textContent ?? '').then(() => {
          button.textContent = i18n.t('chat.copied')
          window.setTimeout(() => {
            button.textContent = i18n.t('chat.copy')
          }, 1500)
        })
      })
      wrapper.appendChild(button)
    })
  })

  injectCodeStyles()

  return (
    <div
      ref={container}
      class="bichat-md text-sm leading-relaxed text-neutral-800"
      innerHTML={html()}
    />
  )
}
