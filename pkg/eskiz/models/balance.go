package models

import eskizapi "github.com/iota-uz/eskiz"

// Balance is the user's available credit on the Eskiz account.
// Units are Eskiz SMS credits (roughly 1 credit ≈ 1 SMS part at standard
// tariff). Consumers display it as "Balance: N credits" or convert to UZS via
// a configured rate.
type Balance interface {
	// Amount returns the raw credit balance. Zero is a valid value (empty
	// account); negative indicates a billing problem — treat as blocked.
	Amount() float64
}

// NewBalance wraps an Eskiz UserLimitResponse into a domain Balance.
// Returns nil if resp or resp.Data is nil.
func NewBalance(resp *eskizapi.UserLimitResponse) Balance {
	if resp == nil || resp.Data == nil || resp.Data.Balance == nil {
		return nil
	}
	return &balance{amount: *resp.Data.Balance}
}

type balance struct {
	amount float64
}

func (b *balance) Amount() float64 { return b.amount }
