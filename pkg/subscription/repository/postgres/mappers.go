package postgres

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/subscription/repository"
)

func toModel(entitlement *repository.Entitlement) (*entitlementModel, error) {
	features, err := json.Marshal(entitlement.Features)
	if err != nil {
		return nil, err
	}
	limits, err := json.Marshal(entitlement.EntityLimits)
	if err != nil {
		return nil, err
	}

	return &entitlementModel{
		TenantID:              entitlement.TenantID.String(),
		Tier:                  entitlement.Tier,
		StripeSubscriptionID:  entitlement.StripeSubscriptionID,
		StripeCustomerID:      entitlement.StripeCustomerID,
		Features:              features,
		EntityLimits:          limits,
		SeatLimit:             entitlement.SeatLimit,
		CurrentSeats:          entitlement.CurrentSeats,
		InGracePeriod:         entitlement.InGracePeriod,
		GracePeriodEndsAt:     entitlement.GracePeriodEndsAt,
		LastSyncedAt:          entitlement.LastSyncedAt,
		StripeSubscriptionEnd: entitlement.StripeSubscriptionEnd,
		CreatedAt:             entitlement.CreatedAt,
		UpdatedAt:             entitlement.UpdatedAt,
	}, nil
}

func toDomain(model *entitlementModel) (*repository.Entitlement, error) {
	tenantID, err := uuid.Parse(model.TenantID)
	if err != nil {
		return nil, err
	}
	features := make([]string, 0)
	if len(model.Features) > 0 {
		if err := json.Unmarshal(model.Features, &features); err != nil {
			return nil, err
		}
	}
	limits := make(map[string]int)
	if len(model.EntityLimits) > 0 {
		if err := json.Unmarshal(model.EntityLimits, &limits); err != nil {
			return nil, err
		}
	}

	return &repository.Entitlement{
		TenantID:              tenantID,
		Tier:                  model.Tier,
		StripeSubscriptionID:  model.StripeSubscriptionID,
		StripeCustomerID:      model.StripeCustomerID,
		Features:              features,
		EntityLimits:          limits,
		SeatLimit:             model.SeatLimit,
		CurrentSeats:          model.CurrentSeats,
		InGracePeriod:         model.InGracePeriod,
		GracePeriodEndsAt:     model.GracePeriodEndsAt,
		LastSyncedAt:          model.LastSyncedAt,
		StripeSubscriptionEnd: model.StripeSubscriptionEnd,
		CreatedAt:             model.CreatedAt,
		UpdatedAt:             model.UpdatedAt,
	}, nil
}

func planToModel(plan repository.Plan) (*planModel, error) {
	features, err := json.Marshal(plan.Features)
	if err != nil {
		return nil, err
	}
	limits, err := json.Marshal(plan.EntityLimits)
	if err != nil {
		return nil, err
	}
	return &planModel{
		Tier:         plan.Tier,
		Name:         plan.DisplayName,
		Description:  plan.Description,
		PriceCents:   plan.PriceCents,
		Interval:     plan.Interval,
		Features:     features,
		EntityLimits: limits,
		SeatLimit:    plan.SeatLimit,
		DisplayOrder: plan.DisplayOrder,
	}, nil
}
