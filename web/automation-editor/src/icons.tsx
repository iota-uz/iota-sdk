// A small single-stroke icon set (16 px, currentColor) so a row of icons shares one look (V3).
import type { JSX } from 'solid-js'

const paths: Record<string, string> = {
  event: 'M9 1.5 3.5 9H8l-1 5.5L12.5 7H8z',
  schedule: 'M8 2a6 6 0 1 0 0 12A6 6 0 0 0 8 2Zm0 2.5V8l2.5 1.5',
  manual: 'M5.5 8V3.5a1 1 0 0 1 2 0V7m0-1a1 1 0 0 1 2 0v1m0-.5a1 1 0 0 1 2 0V9a5 5 0 0 1-5 5 3.5 3.5 0 0 1-3-1.7L2 10a1 1 0 0 1 1.7-1L5.5 10',
  branch: 'M4 2v12M4 8c0-3 3-4 8-4m-2-2 2 2-2 2',
  switch: 'M2 8h4m0 0 6-5m-6 5h6m-6 0 6 5',
  wait: 'M4 2h8M4 14h8M5 2c0 3 6 3 6 6s-6 3-6 6',
  call: 'M2 8h9m-3-3 3 3-3 3M13 3v10',
  effect: 'M3 13 13 3m-6 0h6v6',
  end: 'M3 3h10v10H3z',
  parallel: 'M4 2v12M12 2v12M4 8h8',
  join: 'M3 3c0 4 5 5 5 5s5-1 5-5M8 8v6',
  foreach: 'M2 8a6 6 0 0 1 10.5-4M14 8a6 6 0 0 1-10.5 4M12.5 1.5V4H10M3.5 14.5V12H6',
  collect: 'M3 3h10l-4 5v5l-2 1V8z',
  node: 'M3 3h10v10H3z',
  error: 'M8 1.5 15 14H1zM8 6v3.5m0 2.2v.1',
  passed: 'M3 8.5 6.5 12 13 4.5',
  running: 'M8 2a6 6 0 1 1-6 6',
  waiting: 'M8 2a6 6 0 1 0 0 12A6 6 0 0 0 8 2Zm0 2.5V8l2.5 1.5',
  failed: 'M4 4l8 8m0-8-8 8',
  skipped: 'M3 3l5 5-5 5m5-10 5 5-5 5',
  cancelled: 'M8 2a6 6 0 1 0 0 12A6 6 0 0 0 8 2ZM4 12l8-8',
  ignore: 'M4 8h8',
  plus: 'M8 3v10M3 8h10',
  minus: 'M3 8h10',
  fit: 'M2 6V2h4M10 2h4v4M14 10v4h-4M6 14H2v-4',
  undo: 'M5 3 2 6l3 3M2 6h8a4 4 0 0 1 0 8H7',
  redo: 'M11 3l3 3-3 3m3-3H6a4 4 0 0 0 0 8h3',
  tidy: 'M2 3h5v4H2zM9 9h5v4H9zM7 5h2.5a1 1 0 0 1 1 1v3',
  broom: 'M10.5 1.5 7 7m-3.2 1.2L7 7l2.5 2.5-1.3 4.5H2.5l1.3-5.8ZM5.2 11.5l-.4 2.4m2.6-3.4-.3 3.2',
  search: 'M7 2a5 5 0 1 0 0 10A5 5 0 0 0 7 2Zm3.5 8.5L14 14',
  settings: 'M8 5.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5ZM8 1v2m0 10v2M1 8h2m10 0h2M3 3l1.5 1.5m7 7L13 13M3 13l1.5-1.5m7-7L13 3',
  check: 'M3 8.5 6.5 12 13 4.5',
  close: 'M4 4l8 8m0-8-8 8',
  keys: 'M1.5 4h13v8h-13zM4 6.5h1m2 0h1m2 0h1M4 9.5h8',
  select: 'M2 2h3M7 2h2m2 0h3v3m0 2v2m0 2v3h-3m-2 0H7m-2 0H2v-3m0-2V7m0-2V2',
  copy: 'M5 5h8v8H5zM3 11V3h8',
  more: 'M3 8h.01M8 8h.01M13 8h.01',
  dot: 'M8 6a2 2 0 1 0 0 4 2 2 0 0 0 0-4Z',
  open: 'M9 2.5h4.5V7M13.5 2.5 7.5 8.5M12 9.5v3a1 1 0 0 1-1 1H3.5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1h3',
  connect: 'M4 2.5v4a3 3 0 0 0 3 3h6.5m-2.5-2.5 2.5 2.5-2.5 2.5M4 2.5h.01',
  unignore: 'M3 8h6.5a3 3 0 0 1 0 6H8M5.5 5.5 3 8l2.5 2.5',
  duplicate: 'M5.5 5.5h8v8h-8zM2.5 10.5v-8h8',
  trash: 'M2.5 4h11M6 4V2.5h4V4m-6 0 .7 9.5h6.6L12 4M6.75 6.5v4.5m2.5-4.5v4.5',
  paste: 'M5.5 2.5h5v2h-5zM5.5 3.5H3.5v10h9v-10h-2',
  grid: 'M2.5 2.5h11v11h-11zM6.17 2.5v11M9.83 2.5v11M2.5 6.17h11M2.5 9.83h11',
}

export function Icon(props: { name: string; class?: string; size?: number; label?: string }): JSX.Element {
  const size = () => props.size ?? 16
  return (
    <svg
      class={`ae-icon ${props.class ?? ''}`}
      width={size()}
      height={size()}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden={props.label ? undefined : 'true'}
      role={props.label ? 'img' : undefined}
      aria-label={props.label}
    >
      <path d={paths[props.name] ?? paths.node} />
    </svg>
  )
}
