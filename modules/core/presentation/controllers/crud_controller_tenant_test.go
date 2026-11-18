package controllers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tenantNote struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	Title    string
}

type tenantNoteMapper struct {
	fields crud.Fields
}

func (m *tenantNoteMapper) ToEntities(_ context.Context, values ...[]crud.FieldValue) ([]tenantNote, error) {
	result := make([]tenantNote, len(values))
	for i, fvs := range values {
		for _, fv := range fvs {
			switch fv.Field().Name() {
			case "id":
				id, err := fv.AsUUID()
				if err != nil {
					return nil, err
				}
				result[i].ID = id
			case "tenant_id":
				id, err := fv.AsUUID()
				if err != nil {
					return nil, err
				}
				result[i].TenantID = id
			case "title":
				title, err := fv.AsString()
				if err != nil {
					return nil, err
				}
				result[i].Title = title
			}
		}
	}
	return result, nil
}

func (m *tenantNoteMapper) ToFieldValuesList(_ context.Context, entities ...tenantNote) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))
	for i, e := range entities {
		fvs, err := m.fields.FieldValues(map[string]any{
			"id":        e.ID,
			"tenant_id": e.TenantID,
			"title":     e.Title,
		})
		if err != nil {
			return nil, err
		}
		result[i] = fvs
	}
	return result, nil
}

type tenantNoteBuilder struct {
	schema  crud.Schema[tenantNote]
	repo    crud.Repository[tenantNote]
	service crud.Service[tenantNote]
}

func (b *tenantNoteBuilder) Schema() crud.Schema[tenantNote]         { return b.schema }
func (b *tenantNoteBuilder) Repository() crud.Repository[tenantNote] { return b.repo }
func (b *tenantNoteBuilder) Service() crud.Service[tenantNote]       { return b.service }

// newTenantNoteBuilder declares tenant_id the way most consumers do: hidden,
// not readonly, with an initial value from context.
func newTenantNoteBuilder() *tenantNoteBuilder {
	fields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey(), crud.WithInitialValue(func(context.Context) any { return uuid.New() })),
		crud.NewUUIDField("tenant_id", crud.WithHidden(), crud.WithInitialValue(func(ctx context.Context) any {
			id, _ := composables.UseTenantID(ctx)
			return id
		})),
		crud.NewStringField("title", crud.WithSearchable()),
	})
	schema := crud.NewSchema[tenantNote]("crud_tenant_notes", fields, &tenantNoteMapper{fields: fields}, crud.TenantScoped[tenantNote]())
	repo := crud.DefaultRepository(schema)
	return &tenantNoteBuilder{
		schema:  schema,
		repo:    repo,
		service: crud.DefaultService(schema, repo, eventbus.NewEventPublisher(logrus.New())),
	}
}

func TestCrudController_TenantScoped(t *testing.T) {
	suite := newCoreCrudSuiteBuilder(t).Build().AsUser(itf.User())
	env := suite.Env()
	ctx := env.Ctx
	tx := env.Tx

	_, err := tx.Exec(ctx, `CREATE TABLE crud_tenant_notes (
		id UUID PRIMARY KEY,
		tenant_id UUID NOT NULL,
		title TEXT NOT NULL
	)`)
	require.NoError(t, err)

	ownTenant := env.Tenant.ID
	foreignTenant := uuid.New()
	foreignID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO crud_tenant_notes (id, tenant_id, title) VALUES ($1, $2, 'foreign secret')`, foreignID, foreignTenant)
	require.NoError(t, err)

	builder := newTenantNoteBuilder()
	suite.Register(controllers.NewCrudController[tenantNote]("/notes", builder))

	tenantOf := func(t *testing.T, id uuid.UUID) (uuid.UUID, string) {
		t.Helper()
		var tenantID uuid.UUID
		var title string
		require.NoError(t, tx.QueryRow(ctx, `SELECT tenant_id, title FROM crud_tenant_notes WHERE id = $1`, id).Scan(&tenantID, &title))
		return tenantID, title
	}

	var ownID uuid.UUID
	t.Run("create ignores submitted tenant_id", func(t *testing.T) {
		suite.POST("/notes").
			Form(url.Values{"title": {"mine"}, "tenant_id": {foreignTenant.String()}}).
			Expect(t).
			Status(http.StatusSeeOther)

		require.NoError(t, tx.QueryRow(ctx, `SELECT id FROM crud_tenant_notes WHERE title = 'mine'`).Scan(&ownID))
		tenantID, _ := tenantOf(t, ownID)
		assert.Equal(t, ownTenant, tenantID)
	})

	t.Run("update ignores submitted tenant_id", func(t *testing.T) {
		suite.POST(fmt.Sprintf("/notes/%s", ownID)).
			Form(url.Values{"title": {"mine updated"}, "tenant_id": {foreignTenant.String()}}).
			Expect(t).
			Status(http.StatusSeeOther)

		tenantID, title := tenantOf(t, ownID)
		assert.Equal(t, ownTenant, tenantID)
		assert.Equal(t, "mine updated", title)
	})

	t.Run("read by ID is tenant scoped", func(t *testing.T) {
		suite.GET(fmt.Sprintf("/notes/%s/edit", foreignID)).Expect(t).Status(http.StatusNotFound)
		suite.GET(fmt.Sprintf("/notes/%s/details", foreignID)).Expect(t).Status(http.StatusNotFound)
		suite.GET(fmt.Sprintf("/notes/%s/edit", ownID)).Expect(t).Status(http.StatusOK)
	})

	// The List handler runs List and Count concurrently, which the shared test
	// transaction cannot serve, so list scoping is checked on the repository.
	t.Run("list, count and get all are tenant scoped", func(t *testing.T) {
		scopedCtx := composables.WithTenantID(composables.WithTx(ctx, tx), ownTenant)

		listed, err := builder.repo.List(scopedCtx, &crud.FindParams{})
		require.NoError(t, err)
		require.Len(t, listed, 1)
		assert.Equal(t, ownID, listed[0].ID)

		all, err := builder.repo.GetAll(scopedCtx)
		require.NoError(t, err)
		require.Len(t, all, 1)

		count, err := builder.repo.Count(scopedCtx, &crud.FindParams{})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		_, err = builder.repo.List(composables.WithTx(context.Background(), tx), &crud.FindParams{})
		require.Error(t, err, "tenant-scoped schema must fail closed without a tenant in context")
	})

	t.Run("update of foreign record is rejected", func(t *testing.T) {
		resp := suite.POST(fmt.Sprintf("/notes/%s", foreignID)).
			Form(url.Values{"title": {"hijacked"}}).
			Expect(t)
		assert.NotEqual(t, http.StatusSeeOther, resp.Raw().StatusCode)

		tenantID, title := tenantOf(t, foreignID)
		assert.Equal(t, foreignTenant, tenantID)
		assert.Equal(t, "foreign secret", title)
	})

	t.Run("delete of foreign record is rejected", func(t *testing.T) {
		resp := suite.DELETE(fmt.Sprintf("/notes/%s", foreignID)).Expect(t)
		assert.NotEqual(t, http.StatusSeeOther, resp.Raw().StatusCode)

		tenantID, _ := tenantOf(t, foreignID)
		assert.Equal(t, foreignTenant, tenantID)
	})
}
