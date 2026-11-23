import { splitProps, type JSX } from 'solid-js'

export interface IconProps extends JSX.SvgSVGAttributes<SVGSVGElement> {
  size?: number | string
}

function Icon(props: IconProps & { children: JSX.Element }) {
  const [local, native] = splitProps(props, ['children', 'size', 'class'])
  return (
    <svg
      {...native}
      aria-hidden="true"
      class={local.class}
      height={local.size ?? 16}
      viewBox="0 0 256 256"
      width={local.size ?? 16}
      xmlns="http://www.w3.org/2000/svg"
    >
      {local.children}
    </svg>
  )
}

// Geometry mirrors github.com/iota-uz/icons/phosphor Check (regular variant),
// which templ primitives render; templ-parity VR compares them pixel-for-pixel.
export function CheckIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <polyline points="40 144 96 200 224 72" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
    </Icon>
  )
}

export function MinusIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <line x1="40" y1="128" x2="216" y2="128" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
    </Icon>
  )
}

export function CaretDownIcon(props: IconProps) {
  const [local, native] = splitProps(props, ['size', 'class'])
  return (
    <svg {...native} aria-hidden="true" class={local.class} height={local.size ?? 16} viewBox="0 0 16 16" width={local.size ?? 16} xmlns="http://www.w3.org/2000/svg">
      <polyline points="3 6 8 10 13 6" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1" />
    </svg>
  )
}

export function EyeIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M128 56C48 56 16 128 16 128s32 72 112 72 112-72 112-72-32-72-112-72Z" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
      <circle cx="128" cy="128" r="32" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
    </Icon>
  )
}

export function EyeSlashIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M52.4 36.5 219.5 203.6M167.8 172.8A68 68 0 0 1 128 184c-80 0-112-56-112-56a134.8 134.8 0 0 1 39.6-41.4M104.5 73.7A75.6 75.6 0 0 1 128 72c80 0 112 56 112 56a132.6 132.6 0 0 1-18.7 25.4M148.7 148.7a29.3 29.3 0 0 1-41.4-41.4" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
    </Icon>
  )
}

const stroke = { fill: 'none', stroke: 'currentColor', 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '16' } as const

// Regular-variant geometry of github.com/iota-uz/icons/phosphor, which the templ
// Metrics band renders.
export function CalendarBlankIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <rect x="40" y="40" width="176" height="176" rx="8" {...stroke} />
      <line x1="176" y1="24" x2="176" y2="56" {...stroke} />
      <line x1="80" y1="24" x2="80" y2="56" {...stroke} />
      <line x1="40" y1="88" x2="216" y2="88" {...stroke} />
    </Icon>
  )
}

export function FunnelIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M34.1,61.38A8,8,0,0,1,40,48H216a8,8,0,0,1,5.92,13.38L152,136v58.65a8,8,0,0,1-3.56,6.66l-32,21.33A8,8,0,0,1,104,216V136Z" {...stroke} />
    </Icon>
  )
}

export function InfoIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="128" cy="128" r="96" {...stroke} />
      <path d="M120,120a8,8,0,0,1,8,8v40a8,8,0,0,0,8,8" {...stroke} />
      <circle cx="124" cy="84" r="12" fill="currentColor" />
    </Icon>
  )
}

export function ArrowDownLeftIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <line x1="192" y1="64" x2="64" y2="192" {...stroke} />
      <polyline points="168 192 64 192 64 88" {...stroke} />
    </Icon>
  )
}

export function ArrowUpRightIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <line x1="64" y1="192" x2="192" y2="64" {...stroke} />
      <polyline points="88 64 192 64 192 168" {...stroke} />
    </Icon>
  )
}

export function ScalesIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <line x1="128" y1="40" x2="128" y2="216" {...stroke} />
      <line x1="104" y1="216" x2="152" y2="216" {...stroke} />
      <line x1="56" y1="88" x2="200" y2="56" {...stroke} />
      <path d="M24,168c0,17.67,20,24,32,24s32-6.33,32-24L56,88Z" {...stroke} />
      <path d="M168,136c0,17.67,20,24,32,24s32-6.33,32-24L200,56Z" {...stroke} />
    </Icon>
  )
}
