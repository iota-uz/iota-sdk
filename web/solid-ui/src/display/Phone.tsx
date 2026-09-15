import { Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type PhoneDisplayStyle = 'parentheses' | 'dashes' | 'spaces'

export interface PhoneDisplayProps extends JSX.HTMLAttributes<HTMLSpanElement> {
  phone: string
  style?: PhoneDisplayStyle
  showIcon?: boolean
}

export interface PhoneLinkProps extends Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> {
  phone: string
  style?: PhoneDisplayStyle
  showIcon?: boolean
}

export function stripPhone(value: string): string {
  return Array.from(value).filter((character) => /\p{Nd}/u.test(character)).join('')
}

function grouped(value: string, separator: string): string {
  return Array.from({ length: Math.ceil(value.length / 3) }, (_, index) => value.slice(index * 3, index * 3 + 3)).join(separator)
}

function formatParts(prefix: string, parts: readonly string[], style: PhoneDisplayStyle): string {
  if (style === 'spaces') return `+${[prefix, ...parts].join(' ')}`
  if (style === 'dashes') return `+${[prefix, ...parts].join('-')}`
  return `+${prefix}(${parts[0]})${parts.slice(1).join('-')}`
}

export function formatPhoneDisplay(value: string, style: PhoneDisplayStyle = 'parentheses'): string {
  if (!value) return ''
  const phone = stripPhone(value)
  if (!phone) return value
  if (phone.startsWith('998') && phone.length === 12) return formatParts('998', [phone.slice(3, 5), phone.slice(5, 8), phone.slice(8, 10), phone.slice(10)], style)
  if (phone.startsWith('1') && phone.length === 11) return formatParts('1', [phone.slice(1, 4), phone.slice(4, 7), phone.slice(7)], style)
  if (phone.startsWith('44') && phone.length >= 10) return formatParts('44', [phone.slice(2, 4), phone.slice(4, 8), phone.slice(8)], style)
  if (phone.startsWith('49') && phone.length >= 10) return formatParts('49', [phone.slice(2, 5), phone.slice(5, 8), phone.slice(8)], style)
  if (phone.startsWith('33') && phone.length >= 10) return formatParts('33', [phone.slice(2, 4), phone.slice(4, 6), phone.slice(6, 8), phone.slice(8)], style)
  if (phone.startsWith('7') && phone.length >= 10) return formatParts('7', [phone.slice(1, 4), phone.slice(4, 7), phone.slice(7, 9), phone.slice(9)], style)
  if (phone.length < 7) return `+${phone}`
  return `+${grouped(phone, style === 'dashes' ? '-' : ' ')}`
}

function PhoneIcon() {
  return (
    <svg aria-hidden="true" width="16" height="16" viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg">
      <path d="M92.5 124.8a60.7 60.7 0 0 0 39 39l20.3-20.3a8 8 0 0 1 8.2-1.9 91.3 91.3 0 0 0 28.6 4.6 8 8 0 0 1 8 8v32.1a8 8 0 0 1-8 8A126.9 126.9 0 0 1 61.7 67.4a8 8 0 0 1 8-8h32.1a8 8 0 0 1 8 8 91.3 91.3 0 0 0 4.6 28.6 8 8 0 0 1-1.9 8.2Z" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
    </svg>
  )
}

export function PhoneLink(props: PhoneLinkProps) {
  const [local, native] = splitProps(props, ['phone', 'style', 'showIcon', 'class'])
  return (
    <Show when={local.phone}>
      <a {...native} href={`tel:${stripPhone(local.phone)}`} class={classes('inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 hover:underline', local.class)}>
        <Show when={local.showIcon}><PhoneIcon /></Show>
        {formatPhoneDisplay(local.phone, local.style)}
      </a>
    </Show>
  )
}

export function PhoneText(props: PhoneDisplayProps) {
  const [local, native] = splitProps(props, ['phone', 'style', 'showIcon', 'class'])
  return (
    <Show when={local.phone}>
      <span {...native} class={classes('inline-flex items-center gap-1', local.class)}>
        <Show when={local.showIcon}><PhoneIcon /></Show>
        {formatPhoneDisplay(local.phone, local.style)}
      </span>
    </Show>
  )
}
