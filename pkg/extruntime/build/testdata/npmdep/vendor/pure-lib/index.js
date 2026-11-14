export function addVat(amount, percent) {
  return Math.round(amount * (100 + percent)) / 100
}
