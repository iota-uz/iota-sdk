import { For, lazy } from "solid-js"
import { navigation, page, shellRenderer } from "@iota-uz/sdk/runtime/client"

// A page of this package, rendered inside whatever shell is active.
export const home = page("e2e.shell.home", {
  path: "/shell-home",
  nav: { label: { en: "Shell home" }, icon: "home" },
  component: lazy(() => import("./Home")),
})

export const nav = navigation({
  workspaces: [{ id: "e2e.shell.main", label: { en: "Main", ru: "Главная" } }],
  palette: [{ id: "e2e.shell.openHome", label: { en: "Open shell home" }, target: "e2e.shell.home" }],
})

// The whole authenticated shell: navigation, user and the host-owned outlet
// holding the current page (legacy or Solid).
export const shell = shellRenderer((p) => (
  <div class="e2e-shell">
    <nav>
      <For each={p.nav.groups}>
        {(group) => <For each={group.items}>{(item) => <a href={item.href}>{item.label}</a>}</For>}
      </For>
    </nav>
    <header>{p.user.name}</header>
    <main>{p.outlet}</main>
  </div>
))
