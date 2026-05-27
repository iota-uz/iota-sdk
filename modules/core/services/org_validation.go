// Package services hosts organizational-model write-path validation shared by
// the department/user-position services and their seeders (tenant isolation,
// hierarchy integrity, multilingual completeness).
package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// orgRequiredLocales is the set of locale codes that every multilingual
// organizational name/title must populate. These mirror the EAI-supported UI
// languages (see pkg/intl: en, ru, uz, uz-Cyrl).
var orgRequiredLocales = []string{"en", "ru", "uz", "uz-Cyrl"}

// SubtreeFunc returns the department subtree rooted at the given department id
// (the department itself plus all descendants). It is satisfied by
// query.OrgQueryRepository.DepartmentSubtree and the OrgQuery service, and is
// injected so this package stays decoupled from a concrete query backend.
type SubtreeFunc func(ctx context.Context, deptID uuid.UUID) ([]uuid.UUID, error)

// validateOrgMultiLang ensures a multilingual value is present and carries a
// non-empty translation for every required locale. field is used to build a
// human-readable error context (e.g. "name", "title").
func validateOrgMultiLang(op serrors.Op, field string, ml models.MultiLang) error {
	if ml == nil || ml.IsEmpty() {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("%s must be provided in all required locales", field))
	}
	var missing []string
	for _, locale := range orgRequiredLocales {
		if !ml.HasLocale(locale) {
			missing = append(missing, locale)
		}
	}
	if len(missing) > 0 {
		return serrors.E(
			op,
			serrors.KindValidation,
			fmt.Errorf("%s is missing required locales: %v", field, missing),
		)
	}
	return nil
}

// ValidateDepartment enforces the department write-path invariants: a complete
// multilingual name and, when a parent is set, a structurally sound and
// tenant-consistent hierarchy. It is the single validation entrypoint shared by
// DepartmentService (Create/Update) and the department seeder.
//
// tenantID is the authoritative caller tenant (resolved from context in
// services, or the seeded row's own tenant in seeds). repo loads the parent and
// subtree resolves the department's descendant set for cycle detection. All
// lookups run against the context's transaction, so this must be called inside
// the same unit of work as the subsequent save.
func ValidateDepartment(
	ctx context.Context,
	op serrors.Op,
	tenantID uuid.UUID,
	d department.Department,
	repo department.Repository,
	subtree SubtreeFunc,
) error {
	if d.TenantID() != tenantID {
		return serrors.E(
			op,
			serrors.KindValidation,
			fmt.Errorf("department %s belongs to a different tenant than the caller", d.ID()),
		)
	}
	if err := validateOrgMultiLang(op, "name", d.NameI18n()); err != nil {
		return err
	}
	return validateDepartmentParent(ctx, op, tenantID, d.ID(), d.ParentID(), repo, subtree)
}

// validateDepartmentParent rejects parent assignments that would corrupt the
// tenant boundary or the hierarchy: a missing parent, a cross-tenant parent, a
// self-reference, or a parent that already sits within the department's own
// subtree (which would create a cycle). A nil parentID is always valid.
func validateDepartmentParent(
	ctx context.Context,
	op serrors.Op,
	tenantID uuid.UUID,
	deptID uuid.UUID,
	parentID *uuid.UUID,
	repo department.Repository,
	subtree SubtreeFunc,
) error {
	if parentID == nil {
		return nil
	}

	if *parentID == deptID {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("department %s cannot be its own parent", deptID))
	}

	parent, err := repo.GetByID(ctx, *parentID)
	if err != nil {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("parent department %s not found: %w", *parentID, err))
	}
	if parent.TenantID() != tenantID {
		return serrors.E(
			op,
			serrors.KindValidation,
			fmt.Errorf("parent department %s belongs to a different tenant", *parentID),
		)
	}

	// Cycle detection: the new parent must not be the department itself or any
	// of its descendants. On create the department row does not yet exist, so
	// the subtree is empty and this check is a no-op.
	descendants, err := subtree(ctx, deptID)
	if err != nil {
		return serrors.E(op, fmt.Errorf("failed to resolve department subtree for cycle check: %w", err))
	}
	for _, id := range descendants {
		if id == *parentID {
			return serrors.E(
				op,
				serrors.KindValidation,
				fmt.Errorf("parent department %s is a descendant of %s (would create a cycle)", *parentID, deptID),
			)
		}
	}

	return nil
}

// ValidateUserPosition enforces the user-position write-path invariants: a
// complete multilingual title plus tenant-consistent department and user
// references. It is the single validation entrypoint shared by
// UserPositionService (Create/Update) and the user-position seeder.
//
// tenantID is the authoritative caller tenant. deptRepo/userRepo load the
// referenced rows; both lookups run against the context's transaction and must
// be called inside the same unit of work as the subsequent save.
func ValidateUserPosition(
	ctx context.Context,
	op serrors.Op,
	tenantID uuid.UUID,
	p userposition.UserPosition,
	deptRepo department.Repository,
	userRepo user.Repository,
) error {
	if p.TenantID() != tenantID {
		return serrors.E(
			op,
			serrors.KindValidation,
			fmt.Errorf("user position %s belongs to a different tenant than the caller", p.ID()),
		)
	}
	if err := validateOrgMultiLang(op, "title", p.TitleI18n()); err != nil {
		return err
	}
	return validatePositionRefs(ctx, op, tenantID, p.UserID(), p.DepartmentID(), deptRepo, userRepo)
}

// validatePositionRefs rejects a position whose department or user is missing
// or belongs to a different tenant than the caller. The repositories are
// tenant-scoped, so a cross-tenant reference surfaces as "not found"; the
// explicit tenant comparison documents the invariant and guards any future
// unscoped lookup path.
func validatePositionRefs(
	ctx context.Context,
	op serrors.Op,
	tenantID uuid.UUID,
	userID uint,
	departmentID uuid.UUID,
	deptRepo department.Repository,
	userRepo user.Repository,
) error {
	dept, err := deptRepo.GetByID(ctx, departmentID)
	if err != nil {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("department %s not found: %w", departmentID, err))
	}
	if dept.TenantID() != tenantID {
		return serrors.E(
			op,
			serrors.KindValidation,
			fmt.Errorf("department %s belongs to a different tenant", departmentID),
		)
	}

	targetUser, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("user %d not found: %w", userID, err))
	}
	if targetUser.TenantID() != tenantID {
		return serrors.E(
			op,
			serrors.KindValidation,
			fmt.Errorf("user %d belongs to a different tenant", userID),
		)
	}

	return nil
}
