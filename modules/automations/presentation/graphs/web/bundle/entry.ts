// Entry of the bundle the automations module embeds and serves itself (/assets/automation-graphs/):
// the automations list and editor with the standard Solid client host. The editor's canvas is a lazy
// chunk of this bundle, loaded on the editor page only.
import { mountSolidClientRouteFromDocument } from '@iota-uz/sdk/solid'
import Screen from '../Automations.solid.generated'
import './bundle.generated.css'

mountSolidClientRouteFromDocument(Screen, {})
