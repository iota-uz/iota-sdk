package models

type Permission struct {
	Id          int64  `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	CreatedAt   string `json:"created_at" db:"created_at"`
	UpdatedAt   string `json:"updated_at" db:"updated_at"`
}

type RolePermissions struct {
	PermissionId int64  `json:"permission_id" db:"permission_id"`
	RoleId       int64  `json:"role_id" db:"role_id"`
	CreatedAt    string `json:"created_at" db:"created_at"`
}
