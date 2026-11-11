// A whole-shell renderer as a package would ship it: wraps the host-owned outlet.
import { For } from 'solid-js'
import type { ShellProps } from '../../../../../../src/client.ts'

function Shell(props: ShellProps) {
  return (
    <div data-testid="package-shell" data-theme-resolved={props.theme.resolved}>
      <header data-testid="package-header">Acme shell · {props.user.name}</header>
      <nav>
        <For each={props.nav.groups}>
          {(group) => (
            <section>
              <h2>{group.label || 'Main'}</h2>
              <For each={group.items}>{(item) => <a href={item.href}>{item.label}</a>}</For>
            </section>
          )}
        </For>
        <ul data-testid="palette">
          <For each={props.nav.palette}>{(entry) => <li>{entry.label}</li>}</For>
        </ul>
      </nav>
      <main data-testid="outlet-slot">{props.outlet}</main>
    </div>
  )
}

export const shell = { component: Shell }
