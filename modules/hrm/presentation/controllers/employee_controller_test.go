package controllers_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/hrm"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/hrm/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const employeesPath = "/hrm/employees"

func setupEmployees(t *testing.T) (*itf.Suite, *services.EmployeeService) {
	t.Helper()

	suite := itf.NewSuiteBuilder(t).
		WithComponents(core.NewComponent(&core.ModuleOptions{
			PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
		}), hrm.NewComponent()).
		AsAdmin().
		Build()
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(suite.Env().Ctx, currency.USD))

	service := itf.GetService[services.EmployeeService](suite.Env())
	suite.Register(controllers.NewEmployeeController(service))
	return suite, service
}

func createEmployee(t *testing.T, suite *itf.Suite, service *services.EmployeeService, firstName string, resignation string) employee.Employee {
	t.Helper()

	dto := &employee.CreateDTO{
		FirstName: firstName,
		LastName:  "Doe",
		Email:     fmt.Sprintf("%s@example.com", firstName),
		Phone:     fmt.Sprintf("+99890%07d", len(firstName)),
		Salary:    1000,
		HireDate:  shared.DateOnly(time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)),
	}
	if resignation != "" {
		date, err := time.Parse(time.DateOnly, resignation)
		require.NoError(t, err)
		dto.ResignationDate = shared.DateOnly(date)
	}
	require.NoError(t, service.Create(suite.Env().Ctx, dto))

	all, err := service.GetAll(suite.Env().Ctx)
	require.NoError(t, err)
	for _, e := range all {
		if e.FirstName() == firstName {
			return e
		}
	}
	t.Fatalf("employee %s not created", firstName)
	return nil
}

func TestEmployeeController_List_FiltersByStatus(t *testing.T) {
	t.Parallel()
	suite, service := setupEmployees(t)

	createEmployee(t, suite, service, "Alice", "")
	createEmployee(t, suite, service, "Bob", "2026-09-01")

	suite.GET(employeesPath).
		Expect(t).
		Status(200).
		Contains("Alice").
		Contains("Bob").
		Contains("Active").
		Contains("Former")

	suite.GET(employeesPath + "?status=former").HTMX().
		Expect(t).
		Status(200).
		Contains("Bob").
		NotContains("Alice")

	suite.GET(employeesPath + "?status=active").HTMX().
		Expect(t).
		Status(200).
		Contains("Alice").
		NotContains("Bob")

	suite.GET(employeesPath + "?status=unknown").HTMX().
		Expect(t).
		Status(200).
		Contains("Alice").
		Contains("Bob")
}

func TestEmployeeController_Edit_ShowsStatusAndSubmitsDates(t *testing.T) {
	t.Parallel()
	suite, service := setupEmployees(t)

	resigned := createEmployee(t, suite, service, "Bob", "2026-09-01")

	html := suite.GET(fmt.Sprintf("%s/%d", employeesPath, resigned.ID())).
		Expect(t).
		Status(200).
		HTML()

	assert.Contains(t, html.Element("//*[@data-testid='employee-status']").Text(), "Former")
	html.Element("//input[@name='ResignationDate'][@value='2026-09-01'][@form='save-form']").Exists()
	html.Element("//input[@name='HireDate'][@value='2024-01-10'][@form='save-form']").Exists()
}

func TestEmployeeController_Update_ResignationDateChangesStatus(t *testing.T) {
	t.Parallel()
	suite, service := setupEmployees(t)

	alice := createEmployee(t, suite, service, "Alice", "")
	path := fmt.Sprintf("%s/%d", employeesPath, alice.ID())
	form := map[string]interface{}{
		"FirstName": "Alice",
		"LastName":  "Doe",
		"Email":     "Alice@example.com",
		"Phone":     alice.Phone(),
		"Salary":    "1000",
		"HireDate":  "2024-01-10",
	}

	form["ResignationDate"] = "2026-09-01"
	suite.POST(path).FormFields(form).Expect(t)
	got, err := service.GetByID(suite.Env().Ctx, alice.ID())
	require.NoError(t, err)
	assert.Equal(t, employee.StatusFormer, got.Status())
	assert.Equal(t, "2024-01-10", got.HireDate().Format(time.DateOnly))

	suite.GET(employeesPath + "?status=former").HTMX().Expect(t).Status(200).Contains("Alice")

	form["ResignationDate"] = ""
	suite.POST(path).FormFields(form).Expect(t)
	got, err = service.GetByID(suite.Env().Ctx, alice.ID())
	require.NoError(t, err)
	assert.Equal(t, employee.StatusActive, got.Status())
}
