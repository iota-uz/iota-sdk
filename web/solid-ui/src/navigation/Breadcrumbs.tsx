import { splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export function Breadcrumbs(props: JSX.OlHTMLAttributes<HTMLOListElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <ol {...native} class={classes('flex items-center gap-1 text-sm', local.class)}>{local.children}</ol>
}

export function BreadcrumbItem(props: JSX.LiHTMLAttributes<HTMLLIElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <li {...native} class={classes('text-100', local.class)}>{local.children}</li>
}

export function BreadcrumbLink(props: JSX.AnchorHTMLAttributes<HTMLAnchorElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <a {...native} class={classes('text-300', local.class)}>{local.children}</a>
}

export function BreadcrumbSeparator(props: JSX.LiHTMLAttributes<HTMLLIElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <li {...native} aria-hidden="true" class={classes('text-300', local.class)}>{local.children ?? '/'}</li>
}
