// Styling a package without inline styles: the SDK's Solid UI kit (the host's instance), Tailwind utility classes
// compiled with the SDK design tokens, and a scoped CSS module. The slot also links to the package's own documentation
// with helpHref and no namespace, wherever it is rendered (a native page, or another package's page). The page uses all three; the slot contribution (mounted
// in a Templ/HTMX page) only the utilities and the module.
import { lazy } from "solid-js"
import { helpHref, page, slot } from "@iota-uz/sdk/runtime/client"
import * as crm from "@iota-kit/crm"
import styles from "./Badge.module.css"

export const dashboard = page("e2e.styled.dashboard", {
  path: "/styled/dashboard",
  component: lazy(() => import("./Dashboard")),
})

export const clientBadge = slot(
  crm.ClientSidebar,
  (p) => (
    <div data-testid="styled-badge" class={`${styles.badge} px-3`}>
      Styled for client {p.clientId}
      <a data-testid="styled-help" href={helpHref("getting-started")}>
        Help
      </a>
    </div>
  ),
  { order: 50 },
)
