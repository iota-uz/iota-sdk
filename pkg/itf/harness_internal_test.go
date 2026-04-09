package itf

import (
	"context"
	"embed"
	"errors"
	"sync"
	"testing"

	"github.com/benbjohnson/hashfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

func TestRunMigrationPolicy_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		policy           MigrationPolicy
		pool             schemaReadinessQuerier
		expectErr        bool
		expectErrKind    serrors.Kind
		expectErrOp      serrors.Op
		expectErrContext string
		expectCalled     bool
	}{
		{
			name:         "ApplyOnce_Migrates",
			policy:       MigrationApplyOnce,
			expectCalled: true,
		},
		{
			name:         "Skip_SchemaReady",
			policy:       MigrationSkip,
			pool:         fakeSchemaReadinessPool{row: fakeBoolRow{value: true}},
			expectCalled: false,
		},
		{
			name:             "Skip_MissingMigrationsTable",
			policy:           MigrationSkip,
			pool:             fakeSchemaReadinessPool{row: fakeBoolRow{value: false}},
			expectErr:        true,
			expectErrKind:    serrors.KindValidation,
			expectErrOp:      opSchemaReadiness,
			expectErrContext: "schema not ready",
			expectCalled:     false,
		},
		{
			name:             "Skip_ProbeError",
			policy:           MigrationSkip,
			pool:             fakeSchemaReadinessPool{row: fakeBoolRow{scanErr: errors.New("probe failed")}},
			expectErr:        true,
			expectErrOp:      opSchemaReadiness,
			expectErrContext: "schema readiness probe failed",
			expectCalled:     false,
		},
		{
			name:          "UnsupportedPolicy_IsRejected",
			policy:        MigrationPolicy("unsupported"),
			expectErr:     true,
			expectErrKind: serrors.Invalid,
			expectErrOp:   opRunMigrationPolicy,
			expectCalled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			migrations := &countingMigrationManager{}
			app := &testApp{migrations: migrations}

			err := runMigrationPolicy(context.Background(), tt.pool, app, MigrationConfig{Policy: tt.policy})
			if !tt.expectErr {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				var serr *serrors.Error
				require.ErrorAs(t, err, &serr)
				if tt.expectErrKind != serrors.Other {
					assert.Equal(t, tt.expectErrKind, serr.Kind)
				}
				if tt.expectErrOp != "" {
					assert.Equal(t, tt.expectErrOp, serr.Op)
				}
				if tt.expectErrContext != "" {
					assert.Contains(t, serr.Error(), tt.expectErrContext)
				}
			}
			assert.Equal(t, tt.expectCalled, migrations.RunCalled())
		})
	}
}

type fakeSchemaReadinessPool struct {
	row pgx.Row
}

func (p fakeSchemaReadinessPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return p.row
}

type fakeBoolRow struct {
	value   bool
	scanErr error
}

func (r fakeBoolRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if len(dest) != 1 {
		return errors.New("expected single destination")
	}
	b, ok := dest[0].(*bool)
	if !ok {
		return errors.New("destination must be *bool")
	}
	*b = r.value
	return nil
}

func TestSharedHarnessManagerReferenceCounting(t *testing.T) {
	t.Parallel()

	manager := &harnessManager{
		entries: map[string]*managedHarnessState{},
	}

	const key = "itf-shared-manager-test"
	state := &harnessState{dbName: "itf-test-db"}
	manager.entries[key] = &managedHarnessState{
		state: state,
		refs:  2,
		cond:  sync.NewCond(&manager.mu),
	}

	got, err := manager.getOrCreate(key, normalizeHarnessConfig(t, HarnessConfig{}))
	require.NoError(t, err)
	require.Same(t, state, got)

	entry := manager.entries[key]
	require.NotNil(t, entry)
	require.Equal(t, 3, entry.refs)

	require.NoError(t, manager.close(key, CleanupKeep))
	require.Equal(t, 2, manager.entries[key].refs)

	require.NoError(t, manager.close(key, CleanupKeep))
	require.Equal(t, 1, manager.entries[key].refs)

	require.NoError(t, manager.close(key, CleanupKeep))
	require.Empty(t, manager.entries)
}

type countingMigrationManager struct {
	runCalled bool
	runErr    error
}

func (m *countingMigrationManager) Rollback() error { return nil }

func (m *countingMigrationManager) Run() error {
	m.runCalled = true
	return m.runErr
}

func (m *countingMigrationManager) Status(context.Context) ([]application.MigrationStatus, error) {
	return nil, nil
}

func (m *countingMigrationManager) RunCalled() bool { return m.runCalled }

type testApp struct {
	migrations application.MigrationManager
}

func (a *testApp) DB() *pgxpool.Pool                               { return nil }
func (a *testApp) EventPublisher() eventbus.EventBus               { return nil }
func (a *testApp) Controllers() []application.Controller           { return nil }
func (a *testApp) Middleware() []mux.MiddlewareFunc                { return nil }
func (a *testApp) Assets() []*embed.FS                             { return nil }
func (a *testApp) HashFsAssets() []*hashfs.FS                      { return nil }
func (a *testApp) Websocket() application.Huber                    { return nil }
func (a *testApp) Spotlight() spotlight.Service                    { return nil }
func (a *testApp) QuickLinks() *spotlight.QuickLinks               { return nil }
func (a *testApp) NavItems(*i18n.Localizer) []types.NavigationItem { return nil }
func (a *testApp) GraphSchemas() []application.GraphSchema         { return nil }
func (a *testApp) Bundle() *i18n.Bundle                            { return nil }
func (a *testApp) GetSupportedLanguages() []string                 { return nil }
func (a *testApp) AppletRegistry() application.AppletRegistry      { return nil }
func (a *testApp) CreateAppletControllers(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]application.Controller, error) {
	return nil, nil
}
func (a *testApp) Session() session.Session                 { return nil }
func (a *testApp) SetSession(session session.Session)       {}
func (a *testApp) Migrations() application.MigrationManager { return a.migrations }

func TestTenantIDHelper(t *testing.T) {
	t.Parallel()

	uid := uuid.New()
	require.Equal(t, uid.String(), tenantID(&uid))
	require.Empty(t, tenantID(nil))
}
