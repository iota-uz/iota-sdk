import { createSignal, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { Dialog, type DialogProps } from './Dialog'

export type DrawerDirection = 'ltr' | 'rtl' | 'btt' | 'ttb'

export interface DrawerProps extends DialogProps { direction?: DrawerDirection }

export function Drawer(props: DrawerProps) {
  const [local, dialogProps] = splitProps(props, ['direction', 'class'])
  return <Dialog {...dialogProps} canonicalClass={classes('dialog m-0 bg-transparent', `dialog-${local.direction ?? 'ltr'}`, 'w-full h-full max-w-full max-h-full')} class={local.class} />
}

function CloseIcon() {
  return <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" width="20" height="20" viewBox="0 0 256 256"><rect width="256" height="256" fill="none" /><line x1="160" y1="96" x2="96" y2="160" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><line x1="96" y1="96" x2="160" y2="160" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><circle cx="128" cy="128" r="96" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
}

export interface ViewDrawerProps extends Omit<DrawerProps, 'title'> {
  title: JSX.Element
  closeLabel?: string
}

export function ViewDrawer(props: ViewDrawerProps) {
  const [local, drawerProps] = splitProps(props, ['title', 'closeLabel', 'children', 'open', 'defaultOpen', 'onOpenChange'])
  const [internal, setInternal] = createSignal(local.defaultOpen ?? false)
  const expanded = () => local.open ?? internal()
  const setOpen = (open: boolean) => {
    if (local.open === undefined) setInternal(open)
    local.onOpenChange?.(open)
  }
  return (
    <Drawer {...drawerProps} direction={drawerProps.direction ?? 'rtl'} open={expanded()} onOpenChange={setOpen} class={classes('flex items-stretch', drawerProps.class)}>
      <div class="bg-white dark:bg-gray-900 w-full sm:w-3/4 md:w-2/3 ml-auto">
        <div class="flex flex-col h-full">
          <div class="flex justify-between px-4 py-3 border-b border-subtle">
            <h3 class="font-medium">{local.title}</h3>
            <div><button type="button" class="cursor-pointer" aria-label={local.closeLabel ?? 'Close'} onClick={() => setOpen(false)}><CloseIcon /></button></div>
          </div>
          <div class="flex-1 min-h-0 overflow-y-auto">{local.children}</div>
        </div>
      </div>
    </Drawer>
  )
}
