package services

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/costcomponent"
	"testing"
)

func TestUhrService_Calculate(t *testing.T) {
	service := NewUhrService()
	props := &UhrProps{
		Entities: []costcomponent.BillableHourEntity{
			{
				Name: "Entity 1",
			},
			{
				Name: "Entity 2",
			},
		},
		ExpenseComponents: []costcomponent.ExpenseComponent{
			{
				Purpose: "Component 1",
				Value:   1000,
			},
			{
				Purpose: "Component 2",
				Value:   2000,
			},
		},
	}

	entities := service.Calculate(props)

	if len(entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(entities))
	}

	if len(entities[0].Components) != 2 {
		t.Errorf("Expected 2 components, got %d", len(entities[0].Components))
	}

	if entities[0].Components[0].Monthly != 500 {
		t.Errorf("Expected 500, got %f", entities[0].Components[0].Monthly)
	}
}
