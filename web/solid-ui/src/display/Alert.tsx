import { splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type AlertVariant = 'error' | 'success'

export interface AlertProps extends JSX.HTMLAttributes<HTMLDivElement> {
  variant?: AlertVariant
}

export function Alert(props: AlertProps) {
  const [local, native] = splitProps(props, ['children', 'class', 'variant'])
  const tone = () => local.variant === 'success' ? 'bg-badge-green text-green' : 'bg-red-100 text-pink'
  return (
    <div {...native} class={classes(tone(), 'py-2.5 px-4 text-sm font-medium rounded-md w-full text-center', local.class)} role={native.role ?? 'alert'}>
      <span>{local.children}</span>
    </div>
  )
}

export type AlertPresetProps = Omit<AlertProps, 'variant'>
export const ErrorAlert = (props: AlertPresetProps) => <Alert {...props} variant="error" />
export const SuccessAlert = (props: AlertPresetProps) => <Alert {...props} variant="success" />

