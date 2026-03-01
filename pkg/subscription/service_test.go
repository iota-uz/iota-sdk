package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_PlanInheritanceAndEvaluation(t *testing.T) {
	t.Parallel()

	engine, err := NewService(Config{
		DefaultPlan: "FREE",
		Plans: []PlanDefinition{
			{
				PlanID:       "FREE",
				Features:     []string{"core_access"},
				EntityLimits: map[string]int{"drivers": 1},
			},
			{
				PlanID:       "PRO",
				ParentPlanID: "FREE",
				Features:     []string{"analytics"},
				EntityLimits: map[string]int{"drivers": 5},
			},
		},
	})
	require.NoError(t, err)

	subject := Subject{Scope: ScopeTenant, ID: uuid.New()}
	require.NoError(t, engine.AssignPlan(context.Background(), subject.Ref(), "PRO"))

	coreDecision, err := engine.EvaluateFeature(context.Background(), subject, FeatureKey("core_access"))
	require.NoError(t, err)
	assert.True(t, coreDecision.Allowed)

	proDecision, err := engine.EvaluateFeature(context.Background(), subject, FeatureKey("analytics"))
	require.NoError(t, err)
	assert.True(t, proDecision.Allowed)

	limitDecision, err := engine.EvaluateLimit(context.Background(), subject, QuotaKey{Resource: "drivers", Window: WindowNone})
	require.NoError(t, err)
	assert.Equal(t, 5, limitDecision.Limit)
	assert.True(t, limitDecision.Allowed)
}

func TestEngine_GrantPrecedence_DenyWins(t *testing.T) {
	t.Parallel()

	engine, err := NewService(Config{DefaultPlan: "FREE"})
	require.NoError(t, err)

	subject := Subject{Scope: ScopeTenant, ID: uuid.New()}

	err = engine.UpsertGrant(context.Background(), Grant{
		ID:      "allow-export",
		Kind:    GrantKindOverride,
		Subject: subject.Ref(),
		Features: map[FeatureKey]GrantEffect{
			"export_pdf": GrantEffectAllow,
		},
	})
	require.NoError(t, err)

	allowDecision, err := engine.EvaluateFeature(context.Background(), subject, "export_pdf")
	require.NoError(t, err)
	assert.True(t, allowDecision.Allowed)

	err = engine.UpsertGrant(context.Background(), Grant{
		ID:      "deny-export",
		Kind:    GrantKindDeny,
		Subject: subject.Ref(),
		Features: map[FeatureKey]GrantEffect{
			"export_pdf": GrantEffectDeny,
		},
	})
	require.NoError(t, err)

	denyDecision, err := engine.EvaluateFeature(context.Background(), subject, "export_pdf")
	require.NoError(t, err)
	assert.False(t, denyDecision.Allowed)
	assert.Equal(t, "explicitly denied by grant", denyDecision.Reason)
}

func TestEngine_AddOnQuotaAndReservationLifecycle(t *testing.T) {
	t.Parallel()

	engine, err := NewService(Config{
		DefaultPlan: "FREE",
		Plans: []PlanDefinition{
			{
				PlanID:       "FREE",
				EntityLimits: map[string]int{"drivers": 5},
			},
		},
	})
	require.NoError(t, err)

	subject := Subject{Scope: ScopeTenant, ID: uuid.New()}
	quota := QuotaKey{Resource: "drivers", Window: WindowNone}

	require.NoError(t, engine.UpsertGrant(context.Background(), Grant{
		ID:      "addon-drivers",
		Kind:    GrantKindAddOn,
		Subject: subject.Ref(),
		Quotas: map[string]QuotaRule{
			"drivers": {
				Effect: GrantEffectAllow,
				Limit:  2,
				Mode:   QuotaModeAdditive,
			},
		},
	}))

	decision, err := engine.EvaluateLimit(context.Background(), subject, quota)
	require.NoError(t, err)
	assert.Equal(t, 7, decision.Limit)

	reservation, err := engine.Reserve(context.Background(), subject, quota, 3, "token-1")
	require.NoError(t, err)
	assert.Equal(t, ReservationPending, reservation.Status)

	afterReserve, err := engine.EvaluateLimit(context.Background(), subject, quota)
	require.NoError(t, err)
	assert.Equal(t, 3, afterReserve.Current)

	require.NoError(t, engine.Commit(context.Background(), reservation.ID))

	afterCommit, err := engine.EvaluateLimit(context.Background(), subject, quota)
	require.NoError(t, err)
	assert.Equal(t, 3, afterCommit.Current)
	assert.Equal(t, 4, afterCommit.Remaining)

	_, err = engine.Reserve(context.Background(), subject, quota, 5, "token-2")
	require.Error(t, err)
	var limitErr ErrLimitExceeded
	require.ErrorAs(t, err, &limitErr)

	require.NoError(t, engine.Release(context.Background(), reservation.ID))
	afterRelease, err := engine.EvaluateLimit(context.Background(), subject, quota)
	require.NoError(t, err)
	assert.Equal(t, 0, afterRelease.Current)
}

func TestEngine_GlobalGrantInheritedByTenant(t *testing.T) {
	t.Parallel()

	engine, err := NewService(Config{DefaultPlan: "FREE"})
	require.NoError(t, err)

	require.NoError(t, engine.UpsertGrant(context.Background(), Grant{
		ID:      "global-feature",
		Kind:    GrantKindDefault,
		Subject: SubjectRef{Scope: ScopeGlobal, ID: uuid.Nil},
		Features: map[FeatureKey]GrantEffect{
			"beta_ui": GrantEffectAllow,
		},
	}))

	subject := Subject{Scope: ScopeTenant, ID: uuid.New()}
	decision, err := engine.EvaluateFeature(context.Background(), subject, "beta_ui")
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
}

func TestEngine_EvaluateLimit_NormalizesWindow(t *testing.T) {
	t.Parallel()

	engine, err := NewService(Config{
		DefaultPlan: "FREE",
		Plans: []PlanDefinition{
			{
				PlanID: "FREE",
				EntityLimits: map[string]int{
					QuotaKey{Resource: "drivers", Window: WindowNone}.String(): 2,
				},
			},
		},
	})
	require.NoError(t, err)

	subject := Subject{Scope: ScopeTenant, ID: uuid.New()}
	decision, err := engine.EvaluateLimit(context.Background(), subject, QuotaKey{Resource: "drivers"})
	require.NoError(t, err)
	assert.Equal(t, 2, decision.Limit)
	assert.Equal(t, WindowNone, decision.Quota.Window)
}

func TestEngine_Reserve_ReusesTokenOnlyForActiveReservation(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	engine, err := NewService(
		Config{
			DefaultPlan:    "FREE",
			ReservationTTL: time.Second,
			Plans: []PlanDefinition{
				{
					PlanID:       "FREE",
					EntityLimits: map[string]int{"drivers": 5},
				},
			},
		},
		WithClock(func() time.Time { return now }),
	)
	require.NoError(t, err)

	subject := Subject{Scope: ScopeTenant, ID: uuid.New()}
	quota := QuotaKey{Resource: "drivers"}

	first, err := engine.Reserve(context.Background(), subject, quota, 1, "token-stable")
	require.NoError(t, err)

	now = now.Add(2 * time.Second)
	second, err := engine.Reserve(context.Background(), subject, quota, 1, "token-stable")
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
}
