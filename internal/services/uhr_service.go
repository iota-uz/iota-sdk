package services

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/costcomponent"
	"github.com/iota-agency/iota-erp/internal/domain/entities/employee"
	"github.com/iota-agency/iota-erp/internal/domain/entities/settings"
)

type UhrService struct {
}

type UhrProps struct {
	Entities          []costcomponent.BillableHourEntity
	ExpenseComponents []costcomponent.ExpenseComponent
}

func NewUhrService() *UhrService {
	return &UhrService{}
}

func (s *UhrService) CalculateStaffExpenses(employees []*employee.Employee, settings *settings.Settings) {

}

func (s *UhrService) Calculate(props *UhrProps) []costcomponent.UnifiedHourlyRateResult {
	// Calculate
	entities := make([]costcomponent.UnifiedHourlyRateResult, 0, len(props.Entities))
	for _, entity := range props.Entities {
		components := make([]costcomponent.CostComponent, 0, len(props.ExpenseComponents))
		for _, expenseComponent := range props.ExpenseComponents {
			monthly := expenseComponent.Value / float64(len(props.Entities))
			components = append(components, costcomponent.CostComponent{
				Purpose: expenseComponent.Purpose,
				Monthly: monthly,
				Hourly:  monthly / costcomponent.HoursInMonth,
			})
		}
		entities = append(entities, costcomponent.UnifiedHourlyRateResult{
			Entity:     entity,
			Components: components,
		})
	}
	return entities
}
