import { splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

function CaretDown() {
  return <svg aria-hidden="true" width="16" height="16" viewBox="0 0 256 256" class="duration-200 group-open:rotate-180"><polyline points="208 96 128 176 48 96" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
}

export interface DescriptionListProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'> {
  title?: JSX.Element
  subtitle?: JSX.Element
  header?: JSX.Element
  collapsible?: boolean
  defaultOpen?: boolean
  contentClass?: string
}

function ListHeader(props: Pick<DescriptionListProps, 'title' | 'subtitle' | 'header'>) {
  if (props.header !== undefined) return <>{props.header}</>
  return <div class="flex flex-col gap-2 p-4">{props.title !== undefined && <span class="text-sm font-medium">{props.title}</span>}{props.subtitle !== undefined && <span class="text-brand-500 text-sm">{props.subtitle}</span>}</div>
}

export function DescriptionList(props: DescriptionListProps) {
  const [local, native] = splitProps(props, ['title', 'subtitle', 'header', 'collapsible', 'defaultOpen', 'contentClass', 'class', 'children'])
  const content = () => <div class={classes('border border-subtle rounded-xl divide-y divide-subtle overflow-hidden', local.contentClass)}>{local.children}</div>
  if (local.collapsible) return (
    <details {...native as JSX.HTMLAttributes<HTMLDetailsElement>} open={local.defaultOpen} class={classes('bg-surface-300 rounded-lg border border-subtle group', local.class)}>
      <summary class="flex items-center justify-between cursor-pointer"><ListHeader title={local.title} subtitle={local.subtitle} header={local.header} /><div class="pr-4"><CaretDown /></div></summary>
      <div class="px-4 pb-4">{content()}</div>
    </details>
  )
  return (
    <div {...native} class={classes('bg-surface-300 rounded-lg border border-subtle', local.class)}>
      <ListHeader title={local.title} subtitle={local.subtitle} header={local.header} />
      <div class="px-4 pb-4 pt-0">{content()}</div>
    </div>
  )
}

export function DescriptionListItem(props: JSX.HTMLAttributes<HTMLDivElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <div {...native} class={classes('flex p-3 justify-between items-center bg-gray-100 gap-2', local.class)}>{local.children}</div>
}

export function DescriptionListLabel(props: JSX.HTMLAttributes<HTMLSpanElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <span {...native} class={classes('text-200 text-sm', local.class)}>{local.children}</span>
}

export function DescriptionListValue(props: JSX.HTMLAttributes<HTMLSpanElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <span {...native} class={classes('font-medium text-sm text-right', local.class)}>{local.children}</span>
}

export function DescriptionListLink(props: JSX.AnchorHTMLAttributes<HTMLAnchorElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <a {...native} class={classes('font-medium text-sm text-brand-500 underline text-right', local.class)}>{local.children}</a>
}

export interface DescriptionListDetailsProps extends JSX.DetailsHtmlAttributes<HTMLDetailsElement> { label: JSX.Element; contentClass?: string }

export function DescriptionListDetails(props: DescriptionListDetailsProps) {
  const [local, native] = splitProps(props, ['label', 'contentClass', 'class', 'children'])
  return (
    <details {...native} class={classes('[&[open]>summary>.caret]:rotate-180', local.class)}>
      <summary class="flex p-3 justify-between items-center bg-surface-100 gap-2 text-200 text-sm cursor-pointer">{local.label}<span class="caret duration-200"><CaretDown /></span></summary>
      <div class={classes('flex flex-col gap-3 p-4 bg-surface-100', local.contentClass)}>{local.children}</div>
    </details>
  )
}
