package permissions

import (
	"github.com/google/uuid"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
)

const (
	ResourceUpload permission.Resource = "upload"
)

var (
	UploadCreate = permission.Permission{
		ID:       uuid.MustParse("b9b4668d-8eef-4b23-b5df-e41fbc6aadee"),
		Name:     "Upload.Create",
		Resource: ResourceUpload,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	UploadRead = permission.Permission{
		ID:       uuid.MustParse("d5d9d214-264e-4553-a401-36893f708aa2"),
		Name:     "Upload.Read",
		Resource: ResourceUpload,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	UploadUpdate = permission.Permission{
		ID:       uuid.MustParse("30f5f65c-f952-4435-94c8-08071b3d6f57"),
		Name:     "Upload.Update",
		Resource: ResourceUpload,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	UploadDelete = permission.Permission{
		ID:       uuid.MustParse("954b1671-90c0-4df2-95c6-a66ea7d4efb5"),
		Name:     "Upload.Delete",
		Resource: ResourceUpload,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
)
