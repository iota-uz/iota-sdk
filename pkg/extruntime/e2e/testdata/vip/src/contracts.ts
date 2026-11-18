import { command, t } from "@iota-uz/sdk/runtime"
import * as crm from "@iota-kit/crm"

export const Sneak = command("e2e.vip.sneak", {
  input: t.object({ client: t.ref(crm.Client) }),
  output: t.object({ ok: t.bool() }),
})
