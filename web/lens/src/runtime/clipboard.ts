/**
 * Put text on the clipboard, from wherever the runtime is embedded.
 *
 * Three paths, in order: the async clipboard API; the legacy selection copy for
 * an insecure context or a rejected permission; and silence. Silence is a real
 * outcome and not a failure to report — every value this copies is also on
 * screen to read, and a thrown error out of a click handler would take the
 * dashboard down for a convenience.
 */
export async function copyText(text: string): Promise<void> {
  const clipboard = globalThis.navigator?.clipboard
  try {
    if (clipboard?.writeText) {
      await clipboard.writeText(text)
      return
    }
  } catch {
    // Fall through to the selection copy.
  }
  try {
    const field = document.createElement('textarea')
    field.value = text
    field.setAttribute('readonly', '')
    field.style.position = 'fixed'
    field.style.opacity = '0'
    document.body.appendChild(field)
    field.select()
    document.execCommand('copy')
    field.remove()
  } catch {
    // Silent no-op: the value is still on screen to read.
  }
}
