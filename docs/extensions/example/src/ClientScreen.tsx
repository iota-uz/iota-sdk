import type { PageProps } from "@iota-uz/sdk/runtime/client"

// `id` is typed by the platform's route: a number, decoded from the path before the page renders.
export default function ClientScreen(props: PageProps<{ id: number }>) {
  return (
    <section>
      <h1>Client {props.params.id}</h1>
    </section>
  )
}
