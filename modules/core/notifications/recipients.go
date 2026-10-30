package notifications

import "context"

// Recipient is a person who can be picked individually on a notification rule.
type Recipient struct {
	ID   uint
	Name string
	Hint string
}

// RecipientQuery selects one page of the recipient directory.
type RecipientQuery struct {
	Search string
	Limit  int
	Offset int
}

// RecipientDirectory defines who can be picked individually on a rule. It never
// returns a person who cannot receive notifications, so an ID that GetByIDs
// omits is unavailable.
type RecipientDirectory interface {
	Search(ctx context.Context, query RecipientQuery) ([]Recipient, error)
	GetByIDs(ctx context.Context, ids []uint) ([]Recipient, error)
}
