import { Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type ButtonVariant = 'primary' | 'secondary' | 'primary-outline' | 'sidebar' | 'danger' | 'ghost'
export type ButtonSize = 'normal' | 'md' | 'sm' | 'xs'

interface ButtonCommonProps {
  variant?: ButtonVariant
  size?: ButtonSize
  fixed?: boolean
  rounded?: boolean
  loading?: boolean
  icon?: JSX.Element
  children?: JSX.Element
  class?: string
}

export type ButtonProps = ButtonCommonProps & (
  | ({ href: string } & Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, keyof ButtonCommonProps | 'href'>)
  | ({ href?: undefined } & Omit<JSX.ButtonHTMLAttributes<HTMLButtonElement>, keyof ButtonCommonProps>)
)

const variantClasses: Record<ButtonVariant, string> = {
  primary: 'btn-primary',
  secondary: 'btn-secondary',
  'primary-outline': 'btn-primary-outline',
  sidebar: 'btn-sidebar',
  danger: 'btn-danger',
  ghost: 'btn-ghost',
}

const sizeClasses: Record<ButtonSize, string> = {
  normal: 'btn-normal',
  md: 'btn-md',
  sm: 'btn-sm',
  xs: 'btn-xs',
}

export function Button(props: ButtonProps) {
  const [local, native] = splitProps(props as ButtonProps & { disabled?: boolean }, [
    'children', 'class', 'variant', 'size', 'fixed', 'rounded', 'loading', 'icon', 'href', 'disabled',
  ])
  const className = () => classes(
    'shrink-0 btn cursor-pointer',
    variantClasses[local.variant ?? 'primary'],
    sizeClasses[local.size ?? 'normal'],
    local.fixed && 'btn-fixed',
    local.rounded && 'btn-rounded',
    local.icon !== undefined && 'btn-with-icon',
    local.loading && 'btn-loading',
    local.disabled && 'btn-disabled',
    local.class,
  )
  const content = () => <>{local.icon}{local.children}<div class="btn-loading-indicator" /></>

  return (
    <Show
      when={local.href}
      fallback={
        <button
          {...native as JSX.ButtonHTMLAttributes<HTMLButtonElement>}
          class={className()}
          disabled={local.disabled}
          aria-busy={local.loading || undefined}
        >
          {content()}
        </button>
      }
    >
      {(href) => (
        <a
          {...native as JSX.AnchorHTMLAttributes<HTMLAnchorElement>}
          href={href()}
          class={className()}
          aria-busy={local.loading || undefined}
          aria-disabled={local.disabled || undefined}
        >
          {content()}
        </a>
      )}
    </Show>
  )
}

export type ButtonPresetProps = Omit<ButtonProps, 'variant'>
export const PrimaryButton = (props: ButtonPresetProps) => <Button {...props as ButtonProps} variant="primary" />
export const SecondaryButton = (props: ButtonPresetProps) => <Button {...props as ButtonProps} variant="secondary" />
export const PrimaryOutlineButton = (props: ButtonPresetProps) => <Button {...props as ButtonProps} variant="primary-outline" />
export const DangerButton = (props: ButtonPresetProps) => <Button {...props as ButtonProps} variant="danger" />
export const SidebarButton = (props: ButtonPresetProps) => <Button {...props as ButtonProps} variant="sidebar" />
export const GhostButton = (props: ButtonPresetProps) => <Button {...props as ButtonProps} variant="ghost" />

