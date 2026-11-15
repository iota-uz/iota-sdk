import './styles.css'
import { registerLensDashboardElement } from './element'
import { registerLensFormulaElement } from './formula/element'

registerLensDashboardElement()
registerLensFormulaElement()

if (import.meta.env.DEV && import.meta.env.LENS_FIXTURE_MODE !== 'true') {
  document.querySelector('lens-dashboard')?.setAttribute('src', '/lens/document')
}
