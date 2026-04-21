package controllers

import (
	"errors"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/validators"
)

func (c *UsersController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	props, err := c.buildCreateFormProps(r.Context(), roleService, groupQueryService, nil)
	if err != nil {
		logger.WithError(err).Error("error building create form props")
		http.Error(w, "Error retrieving user form options", http.StatusInternalServerError)
		return
	}

	if err := users.New(props).Render(r.Context(), w); err != nil {
		logger.WithError(err).Error("error rendering create page")
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
	}
}

func (c *UsersController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	respondWithForm := func(errors map[string]string, dto *dtos.CreateUserDTO) {
		props, err := c.buildCreateFormProps(r.Context(), roleService, groupQueryService, &userCreateFormState{
			DTO:    dto,
			Errors: errors,
		})
		if err != nil {
			logger.WithError(err).Error("error building create form props")
			http.Error(w, "Error retrieving user form options", http.StatusInternalServerError)
			return
		}

		if err := users.CreateForm(props).Render(r.Context(), w); err != nil {
			logger.WithError(err).Error("error rendering create form")
			http.Error(w, "Error rendering response", http.StatusInternalServerError)
		}
	}

	dto, err := composables.UseForm(&dtos.CreateUserDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("error parsing form")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errs, ok := dto.Ok(r.Context()); !ok {
		respondWithForm(errs, dto)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logger.WithError(err).Error("error getting tenant")
		http.Error(w, "Error getting tenant", http.StatusInternalServerError)
		return
	}

	userEntity, err := dto.ToEntity(tenantID)
	if err != nil {
		logger.WithError(err).Error("error converting dto to entity")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := userService.Create(r.Context(), userEntity); err != nil {
		var errs *validators.ValidationError
		if errors.As(err, &errs) {
			respondWithForm(errs.Fields, dto)
			return
		}

		logger.WithError(err).Error("error creating user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *UsersController) Update(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("error parsing user id")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&dtos.UpdateUserDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("error parsing form")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondWithForm := func(errors map[string]string, dto *dtos.UpdateUserDTO) {
		props, err := c.buildEditFormProps(r.Context(), userService, roleService, groupQueryService, id, &userEditFormState{
			DTO:    dto,
			Errors: errors,
		})
		if err != nil {
			logger.WithError(err).Error("error building edit form props")
			http.Error(w, "Error retrieving user information", http.StatusInternalServerError)
			return
		}

		if err := users.EditForm(props).Render(r.Context(), w); err != nil {
			logger.WithError(err).Error("error rendering edit form")
			http.Error(w, "Error rendering response", http.StatusInternalServerError)
		}
	}

	if errs, ok := dto.Ok(r.Context()); !ok {
		respondWithForm(errs, dto)
		return
	}

	userEntity, err := userService.GetByID(r.Context(), id)
	if err != nil {
		logger.WithError(err).Error("error retrieving user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roles := make([]role.Role, 0, len(dto.RoleIDs))
	for _, roleID := range dto.RoleIDs {
		roleEntity, err := roleService.GetByID(r.Context(), roleID)
		if err != nil {
			logger.WithError(err).Error("error getting role")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		roles = append(roles, roleEntity)
	}

	permissions := c.selectedPermissionsFromIDs(dto.PermissionIDs)

	userEntity, err = dto.Apply(userEntity, roles, permissions)
	if err != nil {
		logger.WithError(err).Error("error applying user update")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := userService.Update(r.Context(), userEntity); err != nil {
		var errs *validators.ValidationError
		if errors.As(err, &errs) {
			respondWithForm(errs.Fields, dto)
			return
		}

		logger.WithError(err).Error("error updating user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *UsersController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("error parsing user id")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := userService.Delete(r.Context(), id); err != nil {
		logger.WithError(err).Error("error deleting user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
