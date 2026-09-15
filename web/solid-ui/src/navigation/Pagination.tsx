import { For, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export interface PaginationPage {
  number?: number
  href?: string
  filler?: boolean
}

export interface PaginationProps extends JSX.HTMLAttributes<HTMLUListElement> {
  current: number
  totalPages: number
  href: (page: number) => string
  siblingCount?: number
  previousLabel?: string
  nextLabel?: string
}

function pageWindow(current: number, total: number, siblingCount: number): PaginationPage[] {
  if (total <= 0) return []
  if (total < 10) return Array.from({ length: total }, (_, index) => ({ number: index + 1 }))
  if (current < 5) return [...Array.from({ length: 10 }, (_, index) => ({ number: index + 1 })), { filler: true }, { number: total }]
  if (current > total - 4) return [{ number: 1 }, { filler: true }, ...Array.from({ length: 10 }, (_, index) => ({ number: total - 9 + index }))]
  const radius = Math.max(1, siblingCount)
  const start = Math.max(2, current - radius)
  const end = Math.min(total - 1, start + radius * 2 - 1)
  return [{ number: 1 }, { filler: true }, ...Array.from({ length: end - start + 1 }, (_, index) => ({ number: start + index })), { filler: true }, { number: total }]
}

function Caret(props: { direction: 'left' | 'right' }) {
  return (
    <svg aria-hidden="true" width="16" height="16" viewBox="0 0 256 256">
      <polyline points={props.direction === 'left' ? '160 208 80 128 160 48' : '96 48 176 128 96 208'} fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
    </svg>
  )
}

export function Pagination(props: PaginationProps) {
  const [local, native] = splitProps(props, ['current', 'totalPages', 'href', 'siblingCount', 'previousLabel', 'nextLabel', 'class'])
  const current = () => Math.min(Math.max(1, local.current), Math.max(1, local.totalPages))
  const previous = () => current() > 1 ? local.href(current() - 1) : undefined
  const next = () => current() < local.totalPages ? local.href(current() + 1) : undefined
  return (
    <ul {...native} class={classes('inline-flex -space-x-px text-sm p-4', local.class)}>
      <li><a aria-label={local.previousLabel ?? 'Previous page'} aria-disabled={!previous()} tabindex={previous() ? undefined : -1} class={classes('btn btn-secondary border-none rounded-r-none h-11', !previous() && 'opacity-70 pointer-events-none')} href={previous()}><Caret direction="left" /></a></li>
      <For each={pageWindow(current(), local.totalPages, local.siblingCount ?? 5)}>{(page) => (
        <li>
          {page.filler
            ? <a aria-hidden="true" class="btn btn-secondary border-none rounded-none h-11 opacity-70 pointer-events-none">...</a>
            : <a aria-current={page.number === current() ? 'page' : undefined} class={classes('btn btn-secondary border-none rounded-none h-11 px-5', page.number === current() && 'bg-brand-500 text-white')} href={local.href(page.number!)}>{page.number}</a>}
        </li>
      )}</For>
      <li><a aria-label={local.nextLabel ?? 'Next page'} aria-disabled={!next()} tabindex={next() ? undefined : -1} class={classes('btn btn-secondary border-none rounded-l-none h-11', !next() && 'opacity-70 pointer-events-none')} href={next()}><Caret direction="right" /></a></li>
    </ul>
  )
}
