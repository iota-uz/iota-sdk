import { probe } from "./probe"

export default function Fragile() {
  probe("fragile")
  return <p data-testid="fragile">Fragile page loaded</p>
}
