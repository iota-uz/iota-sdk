import './styles.css'
import { registerLensDashboardElement } from './element'

registerLensDashboardElement()

if (import.meta.env.DEV && import.meta.env.LENS_FIXTURE_MODE !== 'true') {
  document.querySelector('lens-dashboard')?.setAttribute('src', '/lens/document')
}
