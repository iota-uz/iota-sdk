// Entry of the bundle the automations module embeds and serves itself (/assets/automations/): the
// integrations screen with the standard Solid client host, so an application that only lists the
// module gets a working page without merging the feature into its own client bundle.
import { mountSolidClientRouteFromDocument } from '@iota-uz/sdk/solid'
import Screen from '../Integrations.solid.generated'
import './bundle.generated.css'

mountSolidClientRouteFromDocument(Screen, {})
