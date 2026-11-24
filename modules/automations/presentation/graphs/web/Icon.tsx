import type { JSX } from 'solid-js'
import type { IconName } from './model'

// Stroke icons of one set and size (V3): 24×24 paths, currentColor, never the only carrier of meaning.
const PATHS: Record<IconName, string> = {
  check: 'M5 12.5l4.5 4.5L19 7.5',
  x: 'M6 6l12 12M18 6L6 18',
  clock: 'M12 7v5l3 2M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z',
  play: 'M8 5.5v13l10.5-6.5z',
  pause: 'M8 5v14M16 5v14',
  alert: 'M12 8v5M12 16.5v.5M10.3 3.9L2.6 17.5A2 2 0 0 0 4.3 20.5h15.4a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z',
  dot: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  bolt: 'M13 2.5L4.5 13.5H12l-1 8 8.5-11H12z',
  calendar: 'M4 6.5h16v13H4zM4 10.5h16M8 3.5v5M16 3.5v5',
  hand: 'M8 11V5.5a1.5 1.5 0 0 1 3 0V11m0-1V4a1.5 1.5 0 0 1 3 0v7m0-5.5a1.5 1.5 0 0 1 3 0V14a7 7 0 0 1-7 7h-.5a6 6 0 0 1-5-2.7L3.6 15a1.6 1.6 0 0 1 2.6-1.9L8 15V8.5',
  plug: 'M9 3v5M15 3v5M6 8h12v3a6 6 0 0 1-12 0zM12 17v4',
  filter: 'M4 5h16l-6 7.5V19l-4 2v-8.5z',
  plus: 'M12 5v14M5 12h14',
  search: 'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14zM20 20l-4-4',
  robot: 'M12 3v3M6 8h12v10H6zM9.5 12.5v.5M14.5 12.5v.5M3 12v3M21 12v3',
  code: 'M9 7l-5 5 5 5M15 7l5 5-5 5',
  key: 'M14.5 13a4.5 4.5 0 1 0-4.2-2.9L3.5 17v3.5H7v-2h2v-2h2l2.3-2.6A4.5 4.5 0 0 0 14.5 13z',
  globe: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM3 12h18M12 3c2.5 2.7 3.5 5.7 3.5 9s-1 6.3-3.5 9c-2.5-2.7-3.5-5.7-3.5-9s1-6.3 3.5-9z',
  lock: 'M6 11h12v9H6zM8.5 11V8a3.5 3.5 0 0 1 7 0v3',
  copy: 'M9 9h10v11H9zM5 15V4h10',
  trash: 'M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3',
  refresh: 'M20 11a8 8 0 0 0-14.3-4.6L4 8M4 4v4h4M4 13a8 8 0 0 0 14.3 4.6L20 16M20 20v-4h-4',
  arrow: 'M5 12h14M13 6l6 6-6 6',
}

export function Icon(props: { name: IconName; size?: number; class?: string; label?: string }): JSX.Element {
  const size = () => props.size ?? 16
  return (
    <svg
      width={size()} height={size()} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
      class={`inline-block shrink-0 align-[-0.125em] ${props.class ?? ''}`}
      aria-hidden={props.label ? undefined : 'true'} role={props.label ? 'img' : undefined} aria-label={props.label}
    >
      <path d={PATHS[props.name]} fill={props.name === 'play' || props.name === 'dot' ? 'currentColor' : 'none'} />
    </svg>
  )
}
