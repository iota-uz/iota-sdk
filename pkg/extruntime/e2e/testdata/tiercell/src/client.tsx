// A custom renderer for a column another package adds to the native clients list.
import { cell } from "@iota-uz/sdk/runtime/client"
import * as extra from "@iota-kit/e2e.clientextra"

export const tierCell = cell(extra.TierColumn, (p) => <b data-testid="tier-cell">{String(p.value).toUpperCase()}</b>)
