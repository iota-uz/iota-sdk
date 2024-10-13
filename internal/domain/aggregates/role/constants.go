package role

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"time"
)

var (
	CEO = Role{
		ID:          1,
		Name:        "CEO",
		Description: "Chief Executive Officer",
		Permissions: permission.Permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	Owner = Role{
		ID:          2,
		Name:        "Owner",
		Description: "Owner",
		Permissions: permission.Permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	CCO = Role{
		ID:          3,
		Name:        "CCO",
		Description: "Chief Commercial Officer",
		Permissions: []permission.Permission{
			permission.PaymentCreate,
			permission.PaymentRead,
			permission.PaymentUpdate,
			permission.PaymentDelete,
			permission.UserCreate,
			permission.UserRead,
			permission.UserUpdate,
			permission.UserDelete,
			permission.RoleCreate,
			permission.RoleRead,
			permission.RoleUpdate,
			permission.RoleDelete,
			permission.AccountCreate,
			permission.AccountRead,
			permission.AccountUpdate,
			permission.AccountDelete,
			permission.ProjectCreate,
			permission.ProjectRead,
			permission.ProjectUpdate,
			permission.ProjectDelete,
			permission.StageCreate,
			permission.StageRead,
			permission.StageUpdate,
			permission.StageDelete,
			permission.ExpenseCreate,
			permission.ExpenseRead,
			permission.ExpenseUpdate,
			permission.ExpenseDelete,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	CFO = Role{
		ID:          4,
		Name:        "CFO",
		Description: "Chief Financial Officer",
		Permissions: []permission.Permission{
			permission.PaymentCreate,
			permission.PaymentRead,
			permission.PaymentUpdate,
			permission.PaymentDelete,
			permission.UserCreate,
			permission.UserRead,
			permission.UserUpdate,
			permission.UserDelete,
			permission.RoleCreate,
			permission.RoleRead,
			permission.RoleUpdate,
			permission.RoleDelete,
			permission.AccountCreate,
			permission.AccountRead,
			permission.AccountUpdate,
			permission.AccountDelete,
			permission.ProjectCreate,
			permission.ProjectRead,
			permission.ProjectUpdate,
			permission.ProjectDelete,
			permission.StageCreate,
			permission.StageRead,
			permission.StageUpdate,
			permission.StageDelete,
			permission.ExpenseCreate,
			permission.ExpenseRead,
			permission.ExpenseUpdate,
			permission.ExpenseDelete,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	CPO = Role{
		ID:          5,
		Name:        "CPO",
		Description: "Chief Product Officer",
		Permissions: []permission.Permission{
			permission.PaymentCreate,
			permission.PaymentRead,
			permission.PaymentUpdate,
			permission.PaymentDelete,
			permission.UserCreate,
			permission.UserRead,
			permission.UserUpdate,
			permission.UserDelete,
			permission.RoleCreate,
			permission.RoleRead,
			permission.RoleUpdate,
			permission.RoleDelete,
			permission.AccountCreate,
			permission.AccountRead,
			permission.AccountUpdate,
			permission.AccountDelete,
			permission.ProjectCreate,
			permission.ProjectRead,
			permission.ProjectUpdate,
			permission.ProjectDelete,
			permission.StageCreate,
			permission.StageRead,
			permission.StageUpdate,
			permission.StageDelete,
			permission.ExpenseCreate,
			permission.ExpenseRead,
			permission.ExpenseUpdate,
			permission.ExpenseDelete,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	CMO = Role{
		ID:          6,
		Name:        "CMO",
		Description: "Chief Marketing Officer",
		Permissions: []permission.Permission{
			permission.PaymentCreate,
			permission.PaymentRead,
			permission.PaymentUpdate,
			permission.PaymentDelete,
			permission.UserCreate,
			permission.UserRead,
			permission.UserUpdate,
			permission.UserDelete,
			permission.RoleCreate,
			permission.RoleRead,
			permission.RoleUpdate,
			permission.RoleDelete,
			permission.AccountCreate,
			permission.AccountRead,
			permission.AccountUpdate,
			permission.AccountDelete,
			permission.ProjectCreate,
			permission.ProjectRead,
			permission.ProjectUpdate,
			permission.ProjectDelete,
			permission.StageCreate,
			permission.StageRead,
			permission.StageUpdate,
			permission.StageDelete,
			permission.ExpenseCreate,
			permission.ExpenseRead,
			permission.ExpenseUpdate,
			permission.ExpenseDelete,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	COO = Role{
		ID:          7,
		Name:        "COO",
		Description: "Chief Operating Officer",
		Permissions: []permission.Permission{
			permission.PaymentCreate,
			permission.PaymentRead,
			permission.PaymentUpdate,
			permission.PaymentDelete,
			permission.UserCreate,
			permission.UserRead,
			permission.UserUpdate,
			permission.UserDelete,
			permission.RoleCreate,
			permission.RoleRead,
			permission.RoleUpdate,
			permission.RoleDelete,
			permission.AccountCreate,
			permission.AccountRead,
			permission.AccountUpdate,
			permission.AccountDelete,
			permission.ProjectCreate,
			permission.ProjectRead,
			permission.ProjectUpdate,
			permission.ProjectDelete,
			permission.StageCreate,
			permission.StageRead,
			permission.StageUpdate,
			permission.StageDelete,
			permission.ExpenseCreate,
			permission.ExpenseRead,
			permission.ExpenseUpdate,
			permission.ExpenseDelete,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	CTO = Role{
		ID:          8,
		Name:        "CTO",
		Description: "Chief Technology Officer",
		Permissions: []permission.Permission{
			permission.UserCreate,
			permission.UserRead,
			permission.UserUpdate,
			permission.UserDelete,
			permission.RoleCreate,
			permission.RoleRead,
			permission.RoleUpdate,
			permission.RoleDelete,
			permission.AccountCreate,
			permission.AccountRead,
			permission.AccountUpdate,
			permission.AccountDelete,
			permission.ProjectCreate,
			permission.ProjectRead,
			permission.ProjectUpdate,
			permission.ProjectDelete,
			permission.StageCreate,
			permission.StageRead,
			permission.StageUpdate,
			permission.StageDelete,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	ProjectManager = Role{
		ID:          9,
		Name:        "Project Manager",
		Description: "Project Manager",
		Permissions: []permission.Permission{
			permission.UserCreate,
			permission.UserRead,
			permission.UserUpdate,
			permission.UserDelete,
			permission.RoleCreate,
			permission.RoleRead,
			permission.RoleUpdate,
			permission.RoleDelete,
			permission.AccountCreate,
			permission.AccountRead,
			permission.AccountUpdate,
			permission.AccountDelete,
			permission.ProjectCreate,
			permission.ProjectRead,
			permission.ProjectUpdate,
			permission.ProjectDelete,
			permission.StageCreate,
			permission.StageRead,
			permission.StageUpdate,
			permission.StageDelete,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
)

var (
	Roles = []Role{
		CEO,
		Owner,
		CCO,
		CFO,
		CPO,
		CMO,
		COO,
		CTO,
		ProjectManager,
	}
)