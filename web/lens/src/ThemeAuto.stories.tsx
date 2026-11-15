import { onCleanup, onMount, type JSX } from 'solid-js'
import { LensDashboard } from './LensDashboard'
import './styles.css'

/**
 * `theme="auto"` follows the application the board sits in: the `dark` class on
 * `<html>` that the host's own theme switch sets. The server cannot know it, so
 * a board mounted with a fixed `light` stayed a white block on a dark page.
 */
function InApplication(props: { dark: boolean; children: JSX.Element }): JSX.Element {
  const root = window.document.documentElement
  onMount(() => {
    root.classList.toggle('dark', props.dark)
    onCleanup(() => root.classList.remove('dark'))
  })
  return (
    <div style={{ background: props.dark ? '#0b1220' : '#f1f5f9', padding: '16px', 'min-height': '100vh' }}>
      {props.children}
    </div>
  )
}

export const AutoInDarkApplication = () => (
  <InApplication dark>
    <LensDashboard locale="en" theme="auto" />
  </InApplication>
)
AutoInDarkApplication.storyName = 'Auto follows a dark application'

export const AutoInLightApplication = () => (
  <InApplication dark={false}>
    <LensDashboard locale="en" theme="auto" />
  </InApplication>
)
AutoInLightApplication.storyName = 'Auto follows a light application'
