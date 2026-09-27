import { render } from 'solid-js/web'
import { App } from './App'
import { isSpecimenID, isTheme } from './catalog'
import './styles.css'

const params = new URLSearchParams(window.location.search)
const requestedSpecimen = params.get('specimen')
const requestedTheme = params.get('theme')
const specimenID = isSpecimenID(requestedSpecimen) ? requestedSpecimen : 'gallery-shell'
const theme = isTheme(requestedTheme) ? requestedTheme : 'light'
const root = document.getElementById('root')

if (!root) throw new Error('Solid UI gallery root is missing')

document.documentElement.dataset.theme = theme
document.documentElement.style.colorScheme = theme
render(() => <App specimenID={specimenID} theme={theme} />, root)
document.documentElement.dataset.galleryReady = 'true'
