// Lifecycle probe: what a component of this package registers while it is mounted, reported on
// `window.__extE2E` and in localStorage so the browser suite can assert from outside the page, across a
// navigation. A leaked root shows as a live count above zero or as a listener that still answers.
import { onCleanup, onMount } from "solid-js"

interface Hook {
  /** Components mounted right now, by name. */
  live: Record<string, number>
  /** Mount and dispose events in order. */
  log: string[]
  /** Listeners currently registered on window by components of this package. */
  listeners: number
  /** Calls of the `e2e-ping` listeners: a disposed component answers none. */
  pings: Record<string, number>
}

const STORAGE_KEY = "e2e.lifecycle"

function hook(): Hook {
  const w = window as unknown as { __extE2E?: Hook }
  return (w.__extE2E ??= { live: {}, log: [], listeners: 0, pings: {} })
}

function persist(event: string): void {
  try {
    const log = JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "[]") as string[]
    log.push(event)
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(log))
  } catch {
    /* storage unavailable: the in-page log still holds */
  }
}

export function probe(name: string): void {
  const h = hook()
  const onPing = () => {
    h.pings[name] = (h.pings[name] ?? 0) + 1
  }
  onMount(() => {
    h.live[name] = (h.live[name] ?? 0) + 1
    h.log.push(`mount:${name}`)
    persist(`mount:${name}`)
    window.addEventListener("e2e-ping", onPing)
    h.listeners++
  })
  onCleanup(() => {
    h.live[name] = (h.live[name] ?? 1) - 1
    h.log.push(`dispose:${name}`)
    persist(`dispose:${name}`)
    window.removeEventListener("e2e-ping", onPing)
    h.listeners--
  })
}
