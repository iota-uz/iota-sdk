import type { PageProps } from "@iota-uz/sdk/runtime/client"

export default function PolicyScreen(props: PageProps<{ id: string }>) {
  return <article>Policy {props.params.id}</article>
}
