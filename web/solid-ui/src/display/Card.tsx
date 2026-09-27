import { Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export interface CardHeaderProps extends JSX.HTMLAttributes<HTMLDivElement> {}

export function CardHeader(props: CardHeaderProps) {
  return <div {...props} class={classes('border-b border-subtle p-4', props.class)}><p>{props.children}</p></div>
}

export interface CardProps extends JSX.HTMLAttributes<HTMLDivElement> {
  header?: JSX.Element
  contentClass?: string
}

export function Card(props: CardProps) {
  const [local, native] = splitProps(props, ['children', 'class', 'header', 'contentClass'])
  return (
    <div {...native} class={classes('bg-surface-300 rounded-lg border border-subtle', local.class)}>
      <Show when={local.header}>{local.header}</Show>
      <div class={classes('p-4', local.contentClass)}>{local.children}</div>
    </div>
  )
}

