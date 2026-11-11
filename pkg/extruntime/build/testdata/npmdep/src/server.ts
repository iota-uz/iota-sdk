import { implement } from "@iota-uz/sdk/runtime/server"
import { addVat } from "pure-lib"
import * as c from "./contracts"

export const priceWithVat = implement(c.PriceWithVat, async (_ctx, input) => ({ total: addVat(input.amount, input.percent) }))
