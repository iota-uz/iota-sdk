package permission

const (
	ResourcePayment Resource = "payment"
	ResourceUser    Resource = "user"
	ResourceRole    Resource = "role"
	ResourceAccount Resource = "account"
	ResourceStage   Resource = "stage"
	ResourceProject Resource = "project"
)

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

const (
	ModifierAll Modifier = "all"
	ModifierOwn Modifier = "own"
)

var (
	PaymentCreate = Permission{
		ID:       1,
		Name:     "Payment.Create",
		Resource: ResourcePayment,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	PaymentRead = Permission{
		ID:       2,
		Name:     "Payment.Read",
		Resource: ResourcePayment,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	PaymentUpdate = Permission{
		ID:       3,
		Name:     "Payment.Update",
		Resource: ResourcePayment,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	PaymentDelete = Permission{
		ID:       4,
		Name:     "Payment.Delete",
		Resource: ResourcePayment,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	UserCreate = Permission{
		ID:       5,
		Name:     "User.Create",
		Resource: ResourceUser,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	UserRead = Permission{
		ID:       6,
		Name:     "User.Read",
		Resource: ResourceUser,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	UserUpdate = Permission{
		ID:       7,
		Name:     "User.Update",
		Resource: ResourceUser,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	UserDelete = Permission{
		ID:       8,
		Name:     "User.Delete",
		Resource: ResourceUser,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	RoleCreate = Permission{
		ID:       9,
		Name:     "Role.Create",
		Resource: ResourceRole,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	RoleRead = Permission{
		ID:       10,
		Name:     "Role.Read",
		Resource: ResourceRole,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	RoleUpdate = Permission{
		ID:       11,
		Name:     "Role.Update",
		Resource: ResourceRole,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	RoleDelete = Permission{
		ID:       12,
		Name:     "Role.Delete",
		Resource: ResourceRole,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	AccountCreate = Permission{
		ID:       13,
		Name:     "Account.Create",
		Resource: ResourceAccount,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	AccountRead = Permission{
		ID:       14,
		Name:     "Account.Read",
		Resource: ResourceAccount,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	AccountUpdate = Permission{
		ID:       15,
		Name:     "Account.Update",
		Resource: ResourceAccount,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	AccountDelete = Permission{
		ID:       16,
		Name:     "Account.Delete",
		Resource: ResourceAccount,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	StageCreate = Permission{
		ID:       17,
		Name:     "Stage.Create",
		Resource: ResourceStage,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	StageRead = Permission{
		ID:       18,
		Name:     "Stage.Read",
		Resource: ResourceStage,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	StageUpdate = Permission{
		ID:       19,
		Name:     "Stage.Update",
		Resource: ResourceStage,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	StageDelete = Permission{
		ID:       20,
		Name:     "Stage.Delete",
		Resource: ResourceStage,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	ProjectCreate = Permission{
		ID:       21,
		Name:     "Project.Create",
		Resource: ResourceProject,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	ProjectRead = Permission{
		ID:       22,
		Name:     "Project.Read",
		Resource: ResourceProject,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	ProjectUpdate = Permission{
		ID:       23,
		Name:     "Project.Update",
		Resource: ResourceProject,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	ProjectDelete = Permission{
		ID:       24,
		Name:     "Project.Delete",
		Resource: ResourceProject,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
)

var Permissions = []Permission{
	PaymentCreate,
	PaymentRead,
	PaymentUpdate,
	PaymentDelete,
	UserCreate,
	UserRead,
	UserUpdate,
	UserDelete,
	RoleCreate,
	RoleRead,
	RoleUpdate,
	RoleDelete,
	AccountCreate,
	AccountRead,
	AccountUpdate,
	AccountDelete,
	ProjectCreate,
	ProjectRead,
	ProjectUpdate,
	ProjectDelete,
	StageCreate,
	StageRead,
	StageUpdate,
	StageDelete,
}
