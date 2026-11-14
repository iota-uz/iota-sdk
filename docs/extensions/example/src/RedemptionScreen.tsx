import { href, type Infer } from "@iota-uz/sdk/runtime"
import type { PageProps } from "@iota-uz/sdk/runtime/client"
import * as c from "./contracts"

export default function RedemptionScreen(props: PageProps<Infer<typeof c.RedemptionRoute.params>>) {
  return <a href={href(c.RedemptionRoute, { id: props.params.id })}>Redemption {props.params.id}</a>
}
