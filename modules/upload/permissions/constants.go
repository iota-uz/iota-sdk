package permissions

import "github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"

const (
	ResourceUpload permission.Resource = "upload"
)

var (
	UploadCreate = permission.Permission{
		ID:       50,
		Name:     "Upload.Create",
		Resource: ResourceUpload,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	UploadRead = permission.Permission{
		ID:       51,
		Name:     "Upload.Read",
		Resource: ResourceUpload,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	UploadUpdate = permission.Permission{
		ID:       52,
		Name:     "Upload.Update",
		Resource: ResourceUpload,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	UploadDelete = permission.Permission{
		ID:       53,
		Name:     "Upload.Delete",
		Resource: ResourceUpload,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
)
