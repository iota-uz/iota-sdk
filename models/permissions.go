package models

type Permission struct {
	Id          int64  `gql:"id" db:"id"`
	Name        string `gql:"name" db:"name"`
	Description string `gql:"description" db:"description"`
	CreatedAt   string `gql:"created_at" db:"created_at"`
	UpdatedAt   string `gql:"updated_at" db:"updated_at"`
}

type RolePermissions struct {
	PermissionId int64  `gql:"permission_id" db:"permission_id"`
	RoleId       int64  `gql:"role_id" db:"role_id"`
	CreatedAt    string `gql:"created_at" db:"created_at"`
}
