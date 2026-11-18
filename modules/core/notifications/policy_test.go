package notifications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
)

func verdict(name string, allow bool, reason string, calls *[]string) notifications.DeliveryPolicy {
	return notifications.DeliveryPolicyFunc(func(context.Context, notifications.PolicyRequest) (notifications.PolicyDecision, error) {
		*calls = append(*calls, name)
		return notifications.PolicyDecision{Allow: allow, Reason: reason}, nil
	})
}

func TestDeliveryPoliciesComposeDeterministically(t *testing.T) {
	ctx := context.Background()
	var calls []string

	// nothing configured: allow
	d, err := notifications.NewDeliveryPolicies(nil).Evaluate(ctx, notifications.PolicyRequest{})
	require.NoError(t, err)
	require.True(t, d.Allow)

	// the Go-configured policy first, then the named ones in lexical order; the first denial wins and stops the walk
	p := notifications.NewDeliveryPolicies(verdict("go", true, "", &calls))
	require.NoError(t, p.Set("zeta", verdict("zeta", false, "zeta_no", &calls)))
	require.NoError(t, p.Set("alpha", verdict("alpha", true, "", &calls)))
	require.NoError(t, p.Set("mid", verdict("mid", false, "mid_no", &calls)))
	require.Equal(t, []string{"alpha", "mid", "zeta"}, p.Names())
	d, err = p.Evaluate(ctx, notifications.PolicyRequest{})
	require.NoError(t, err)
	require.Equal(t, notifications.PolicyDecision{Allow: false, Reason: "mid_no"}, d)
	require.Equal(t, []string{"go", "alpha", "mid"}, calls)

	// a Go-level denial beats every named policy
	calls = nil
	p = notifications.NewDeliveryPolicies(verdict("go", false, "go_no", &calls))
	require.NoError(t, p.Set("a", verdict("a", true, "", &calls)))
	d, err = p.Evaluate(ctx, notifications.PolicyRequest{})
	require.NoError(t, err)
	require.Equal(t, "go_no", d.Reason)
	require.Equal(t, []string{"go"}, calls)

	// a failing policy is an error, never an implicit allow; Set replaces and nil removes
	boom := errors.New("boom")
	require.NoError(t, p.Set("a", notifications.DeliveryPolicyFunc(func(context.Context, notifications.PolicyRequest) (notifications.PolicyDecision, error) {
		return notifications.PolicyDecision{}, boom
	})))
	p = notifications.NewDeliveryPolicies(nil)
	require.NoError(t, p.Set("a", notifications.DeliveryPolicyFunc(func(context.Context, notifications.PolicyRequest) (notifications.PolicyDecision, error) {
		return notifications.PolicyDecision{}, boom
	})))
	_, err = p.Evaluate(ctx, notifications.PolicyRequest{})
	require.ErrorIs(t, err, boom)
	require.NoError(t, p.Set("a", nil))
	d, err = p.Evaluate(ctx, notifications.PolicyRequest{})
	require.NoError(t, err)
	require.True(t, d.Allow)
	require.Error(t, p.Set(" ", verdict("x", true, "", &calls)))
}
