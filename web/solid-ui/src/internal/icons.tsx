import type { JSX } from 'solid-js'

export interface IconProps extends JSX.SvgSVGAttributes<SVGSVGElement> {
  size?: number | string
}

function Icon(props: IconProps & { children: JSX.Element }) {
  return (
    <svg
      aria-hidden="true"
      class={props.class}
      height={props.size ?? 16}
      viewBox="0 0 256 256"
      width={props.size ?? 16}
      xmlns="http://www.w3.org/2000/svg"
    >
      {props.children}
    </svg>
  )
}

export function CheckIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <polyline points="216 72 104 184 48 128" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
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
  return (
    <Icon {...props}>
      <polyline points="208 96 128 176 48 96" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
    </Icon>
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
