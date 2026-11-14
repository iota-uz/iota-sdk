import type { PageProps } from "@iota-uz/sdk/runtime/client"

// `id` is typed by the platform route: a number, decoded from the path.
export default function ClientView(props: PageProps<{ id: number }>) {
  return (
    <main data-testid="package-client-view">
      Package view of client {props.params.id} ({typeof props.params.id})
    </main>
  )
}
