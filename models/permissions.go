package models

type Permission struct {
	Id          int64
	Description string
	Resource    string
	Action      string
	Modifier    string
	CreatedAt   string
	UpdatedAt   string
}

type RolePermissions struct {
	PermissionId int64
	RoleId       int64
}
