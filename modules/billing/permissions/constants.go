// Package permissions provides this package.
package permissions

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceBillingTransaction permission.Resource = "billing_transaction"
)

var (
	// BillingTransactionRead lets a user see billing transactions and the
	// billing.transaction.status_changed.v1 events about them.
	BillingTransactionRead = permission.MustCreate(
		uuid.MustParse("f70cd062-bef1-41b7-9fdf-7a11a11313bd"),
		"BillingTransaction.Read",
		ResourceBillingTransaction,
		permission.ActionRead,
		permission.ModifierAll,
	)
)

var Permissions = []permission.Permission{
	BillingTransactionRead,
}
