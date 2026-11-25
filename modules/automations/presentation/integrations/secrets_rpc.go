package integrations

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
)

// The «Secret» connector's methods. Every one requires Secrets.Manage (not Connections.Manage): a secret
// administrator may be someone else than the connections' one. A value goes in (create, rotate) and never
// comes out: not in a result, an error or a log line.

var manageSecrets = permissions.SecretsManage.Name()

// secretsCaller is the caller of a secrets method and the manager it calls: forbidden without
// Secrets.Manage, unavailable when the application manages no secrets.
func (c *Controller) secretsCaller(ctx context.Context) (secrets.Manager, services.Actor, error) {
	sub, err := caller(ctx, manageSecrets)
	if err != nil {
		return nil, services.Actor{}, err
	}
	if c.cfg.Secrets == nil {
		return nil, services.Actor{}, secretsUnavailable()
	}
	return c.cfg.Secrets, actor(sub), nil
}

func secretsUnavailable() error {
	return solid.NewRPCError(string(ErrUnavailable), "secrets are not available in this application", map[string]string{"code": string(ErrUnavailable)})
}

func secretNotFound() error {
	return solid.NewRPCError(solid.CodeNotFound, "secret not found", map[string]string{"code": solid.CodeNotFound})
}

func secretView(s secrets.Secret) SecretView {
	v := SecretView{
		ID: s.ID.String(), Name: s.Name, Description: s.Description, Version: s.Version,
		CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: s.UpdatedAt.UTC().Format(time.RFC3339),
		RotatedAt: s.RotatedAt.UTC().Format(time.RFC3339),
	}
	if s.RotatedAt.IsZero() {
		v.RotatedAt = v.CreatedAt // never rotated
	}
	if s.LastUsedAt != nil {
		used := s.LastUsedAt.UTC().Format(time.RFC3339)
		v.LastUsedAt = &used
	}
	return v
}

// secretSaveResult turns a refusal of create, update or rotate into the screen's result; other errors are
// RPC errors. A refusal names a field, never a value.
func secretSaveResult(s secrets.Secret, err error) (SecretSaveResponse, error) {
	switch {
	case err == nil:
		v := secretView(s)
		return SecretSaveResponse{OK: true, Secret: &v}, nil
	case errors.Is(err, secrets.ErrNameTaken):
		return SecretSaveResponse{Error: ErrNameTaken, Fields: map[string]string{"name": "taken"}}, nil
	case errors.Is(err, secrets.ErrVersionConflict):
		return SecretSaveResponse{Error: ErrVersionConflict}, nil
	case errors.Is(err, secrets.ErrInvalid):
		return SecretSaveResponse{Error: ErrInvalidInput, Fields: invalidSecretField(err)}, nil
	case errors.Is(err, secrets.ErrUnavailable):
		return SecretSaveResponse{Error: ErrUnavailable}, nil
	}
	return SecretSaveResponse{}, mapSecretErr(err)
}

// invalidSecretField is the field an ErrInvalid names ("…: value is empty"); none when it names no
// field the form has.
func invalidSecretField(err error) map[string]string {
	msg := err.Error()
	for _, f := range []string{"name", "description", "value"} {
		if strings.Contains(msg, ": "+f+" ") {
			return map[string]string{f: "invalid"}
		}
	}
	return nil
}

// mapSecretErr turns the manager's errors into RPC errors the screen explains; anything else is internal.
// The manager's errors never carry a value.
func mapSecretErr(err error) error {
	switch {
	case errors.Is(err, secrets.ErrNotFound):
		return secretNotFound()
	case errors.Is(err, secrets.ErrUnavailable):
		return secretsUnavailable()
	case errors.Is(err, secrets.ErrForbidden):
		return solid.NewRPCError(solid.CodeForbidden, "forbidden", map[string]string{"code": solid.CodeForbidden})
	}
	return err
}

// validateSecret checks the fields the form has before the manager does, so the screen names them.
func validateSecret(name *string, value *string) map[string]string {
	out := map[string]string{}
	if name != nil {
		switch {
		case strings.TrimSpace(*name) == "":
			out["name"] = "required"
		case !secrets.IsName(*name):
			out["name"] = "secret_name"
		}
	}
	if value != nil && *value == "" {
		out["value"] = "required"
	}
	return out
}

func (c *Controller) listSecrets(ctx context.Context, _ Empty) (SecretListResponse, error) {
	m, a, err := c.secretsCaller(ctx)
	if err != nil {
		return SecretListResponse{}, err
	}
	list, err := m.List(ctx, a.TenantID)
	if err != nil {
		return SecretListResponse{}, mapSecretErr(err)
	}
	out := SecretListResponse{Secrets: make([]SecretView, 0, len(list))}
	for _, s := range list {
		out.Secrets = append(out.Secrets, secretView(s))
	}
	return out, nil
}

func (c *Controller) createSecret(ctx context.Context, req SecretCreateRequest) (SecretSaveResponse, error) {
	m, a, err := c.secretsCaller(ctx)
	if err != nil {
		return SecretSaveResponse{}, err
	}
	name := strings.TrimSpace(req.Name)
	if fields := validateSecret(&name, &req.Value); len(fields) > 0 {
		return SecretSaveResponse{Error: ErrInvalidInput, Fields: fields}, nil
	}
	return secretSaveResult(m.Create(ctx, a.TenantID, secrets.CreateInput{
		Name: name, Description: strings.TrimSpace(req.Description), Value: req.Value, CreatedBy: a.UserID,
	}))
}

func (c *Controller) updateSecret(ctx context.Context, req SecretUpdateRequest) (SecretSaveResponse, error) {
	m, a, err := c.secretsCaller(ctx)
	if err != nil {
		return SecretSaveResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return SecretSaveResponse{}, secretNotFound()
	}
	in := secrets.UpdateInput{ID: id, Version: req.Version}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if fields := validateSecret(&name, nil); len(fields) > 0 {
			return SecretSaveResponse{Error: ErrInvalidInput, Fields: fields}, nil
		}
		in.Name = &name
	}
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		in.Description = &d
	}
	return secretSaveResult(m.Update(ctx, a.TenantID, in))
}

func (c *Controller) rotateSecret(ctx context.Context, req SecretRotateRequest) (SecretSaveResponse, error) {
	m, a, err := c.secretsCaller(ctx)
	if err != nil {
		return SecretSaveResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return SecretSaveResponse{}, secretNotFound()
	}
	if fields := validateSecret(nil, &req.Value); len(fields) > 0 {
		return SecretSaveResponse{Error: ErrInvalidInput, Fields: fields}, nil
	}
	return secretSaveResult(m.Rotate(ctx, a.TenantID, secrets.RotateInput{ID: id, Version: req.Version, Value: req.Value}))
}

func (c *Controller) deleteSecret(ctx context.Context, req IDRequest) (OKResponse, error) {
	m, a, err := c.secretsCaller(ctx)
	if err != nil {
		return OKResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return OKResponse{}, secretNotFound()
	}
	if err := m.Delete(ctx, a.TenantID, id); err != nil {
		return OKResponse{}, mapSecretErr(err)
	}
	return OKResponse{OK: true}, nil
}
