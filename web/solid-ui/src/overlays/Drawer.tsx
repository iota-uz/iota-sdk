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
  return <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" width="20" height="20" viewBox="0 0 256 256"><rect width="256" height="256" fill="none" /><line x1="200" y1="56" x2="56" y2="200" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><line x1="200" y1="200" x2="56" y2="56" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
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
      <div class="ml-auto w-full bg-surface-300 sm:w-3/4 md:w-2/3">
        <div class="flex h-full flex-col">
          <div class="shrink-0">
            <div class="flex min-h-14 items-center justify-between gap-2 border-b border-subtle py-1 pl-4 pr-2 md:pl-6">
              <h3 class="min-w-0 flex-1 truncate font-medium" data-dialog-title tabindex="-1">{local.title}</h3>
              <button type="button" class="drawer-close" aria-label={local.closeLabel ?? 'Close'} data-test-id="drawer-close" onClick={() => setOpen(false)}><CloseIcon /></button>
            </div>
          </div>
          <div class="min-h-0 flex-1 overflow-y-auto">{local.children}</div>
        </div>
      </div>
    </Drawer>
  )
}
