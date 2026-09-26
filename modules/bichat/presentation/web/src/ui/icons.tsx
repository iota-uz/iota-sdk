import type { JSX } from 'solid-js'

export interface IconProps {
  class?: string
}

function strokeProps(props: IconProps) {
  return {
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    'stroke-width': 2,
    'stroke-linecap': 'round' as const,
    'stroke-linejoin': 'round' as const,
    'aria-hidden': true as const,
    class: props.class,
  }
}

export function PlusIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <path d="M12 5v14M5 12h14" />
    </svg>
  )
}

export function SearchIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.3-4.3" />
    </svg>
  )
}

export function SendIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <path d="m22 2-7 20-4-9-9-4Z" />
      <path d="M22 2 11 13" />
    </svg>
  )
}

export function PinIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <path d="M12 17v5" />
      <path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1z" />
    </svg>
  )
}

export function DotsVerticalIcon(props: IconProps): JSX.Element {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" stroke="none" aria-hidden={true} class={props.class}>
      <circle cx="12" cy="5" r="1.6" />
      <circle cx="12" cy="12" r="1.6" />
      <circle cx="12" cy="19" r="1.6" />
    </svg>
  )
}

export function ArchiveIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <rect width="20" height="5" x="2" y="3" rx="1" />
      <path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8" />
      <path d="M10 12h4" />
    </svg>
  )
}

export function XIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  )
}

export function ChevronLeftIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <path d="m15 18-6-6 6-6" />
    </svg>
  )
}

export function PanelLeftIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <rect width="18" height="18" x="3" y="3" rx="2" />
      <path d="M9 3v18" />
    </svg>
  )
}

export function PencilIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
      <path d="m15 5 4 4" />
    </svg>
  )
}

export function CheckIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <path d="M20 6 9 17l-5-5" />
    </svg>
  )
}

export function AlertIcon(props: IconProps): JSX.Element {
  return (
    <svg {...strokeProps(props)}>
      <circle cx="12" cy="12" r="10" />
      <path d="M12 8v4" />
      <path d="M12 16h.01" />
    </svg>
  )
}
