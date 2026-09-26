package services

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Balance is the money of one currency: what is on the accounts, what open
// payable debts have reserved of it, and what is left to spend. A reserve does
// not move money, so OnAccounts stays the actual bank balance.
type Balance struct {
	OnAccounts *money.Money
	Reserved   *money.Money
	Available  *money.Money
}

type BalanceService struct {
	accountRepo moneyaccount.Repository
	debtRepo    debt.Repository
}

func NewBalanceService(accountRepo moneyaccount.Repository, debtRepo debt.Repository) *BalanceService {
	return &BalanceService{
		accountRepo: accountRepo,
		debtRepo:    debtRepo,
	}
}

// Balances returns one balance per currency across all accounts, including
// reserves of open payable debts that name no account.
func (s *BalanceService) Balances(ctx context.Context) ([]Balance, error) {
	const op serrors.Op = "BalanceService.Balances"
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		return nil, serrors.E(op, err)
	}

	accounts, err := s.accountRepo.GetAll(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	reserved, err := s.debtRepo.OpenPayableTotals(ctx, nil)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	onAccounts := make([]*money.Money, 0, len(accounts))
	for _, account := range accounts {
		onAccounts = append(onAccounts, account.Balance())
	}
	byCurrency, err := combine(onAccounts, reserved)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	codes := make([]string, 0, len(byCurrency))
	for code := range byCurrency {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	balances := make([]Balance, 0, len(codes))
	for _, code := range codes {
		balances = append(balances, byCurrency[code])
	}
	return balances, nil
}

// AccountBalance returns the balance of one account in its currency.
func (s *BalanceService) AccountBalance(ctx context.Context, accountID uuid.UUID) (Balance, error) {
	const op serrors.Op = "BalanceService.AccountBalance"
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		return Balance{}, serrors.E(op, err)
	}

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return Balance{}, serrors.E(op, err)
	}
	reserved, err := s.debtRepo.OpenPayableTotals(ctx, &accountID)
	if err != nil {
		return Balance{}, serrors.E(op, err)
	}

	byCurrency, err := combine([]*money.Money{account.Balance()}, reserved)
	if err != nil {
		return Balance{}, serrors.E(op, err)
	}
	return byCurrency[account.Balance().Currency().Code], nil
}

// combine groups amounts by currency and subtracts reserves from what is on
// the accounts.
func combine(onAccounts, reserved []*money.Money) (map[string]Balance, error) {
	byCurrency := map[string]Balance{}
	add := func(amount *money.Money, isReserve bool) error {
		code := amount.Currency().Code
		b, ok := byCurrency[code]
		if !ok {
			b = Balance{OnAccounts: money.New(0, code), Reserved: money.New(0, code)}
		}
		var err error
		if isReserve {
			b.Reserved, err = b.Reserved.Add(amount)
		} else {
			b.OnAccounts, err = b.OnAccounts.Add(amount)
		}
		byCurrency[code] = b
		return err
	}

	for _, amount := range onAccounts {
		if err := add(amount, false); err != nil {
			return nil, err
		}
	}
	for _, amount := range reserved {
		if err := add(amount, true); err != nil {
			return nil, err
		}
	}

	for code, b := range byCurrency {
		available, err := b.OnAccounts.Subtract(b.Reserved)
		if err != nil {
			return nil, err
		}
		b.Available = available
		byCurrency[code] = b
	}
	return byCurrency, nil
}
