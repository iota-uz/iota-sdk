package controllers_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	financeServices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

var (
	ProjectBasePath = "/projects"
)

func TestProjectController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	projectService := env.App.Service(services.ProjectService{}).(*services.ProjectService)
	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	project1 := project.New(
		"Test Project 1",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("Test description 1"),
	)

	project2 := project.New(
		"Test Project 2",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("Test description 2"),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)
	err = projectService.Create(env.Ctx, project2)
	require.NoError(t, err)

	response := suite.GET(ProjectBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("Test Project 1").
		Contains("Test Project 2").
		Contains("Test description 1").
		Contains("Test description 2")
}

func TestProjectController_List_HTMX_Request(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	projectService := env.App.Service(services.ProjectService{}).(*services.ProjectService)
	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"HTMX Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	project1 := project.New(
		"HTMX Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("HTMX test description"),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	suite.GET(ProjectBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX Test Project").
		Contains("HTMX test description")
}

func TestProjectController_GetNewDrawer_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"Drawer Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	_, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	response := suite.GET(ProjectBasePath + "/new/drawer").
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//input[@name='Name']").Exists()
	html.Element("//textarea[@name='Description']").Exists()
	html.Element("//select[@name='CounterpartyID']").Exists()
}

func TestProjectController_Create_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	projectService := env.App.Service(services.ProjectService{}).(*services.ProjectService)
	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"Create Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "Valid Test Project")
	formData.Set("Description", "Valid test description")
	formData.Set("CounterpartyID", createdCounterparty.ID().String())

	response := suite.POST(ProjectBasePath).
		Form(formData).
		HTMX().
		Header("HX-Target", "project-create-drawer").
		Expect(t)

	// Accept either successful redirect (302) or successful creation without redirect
	statusCode := response.Raw().StatusCode
	if statusCode != 302 && statusCode != 200 {
		t.Fatalf("Expected status 200 or 302, got %d", statusCode)
	}

	projects, err := projectService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, projects, 1)

	savedProject := projects[0]
	require.Equal(t, "Valid Test Project", savedProject.Name())
	require.Equal(t, "Valid test description", savedProject.Description())
	require.Equal(t, createdCounterparty.ID(), savedProject.CounterpartyID())
}

func TestProjectController_Create_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	projectService := env.App.Service(services.ProjectService{}).(*services.ProjectService)
	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"Validation Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "") // Invalid: required field
	formData.Set("Description", "Test description")
	formData.Set("CounterpartyID", createdCounterparty.ID().String())

	response := suite.POST(ProjectBasePath).
		Form(formData).
		HTMX().
		Header("HX-Target", "project-create-drawer").
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	projects, err := projectService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, projects)
}

func TestProjectController_GetEditDrawer_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	projectService := env.App.Service(services.ProjectService{}).(*services.ProjectService)
	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"Edit Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	project1 := project.New(
		"Edit Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("Edit test description"),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	response := suite.GET(fmt.Sprintf("%s/%s/drawer", ProjectBasePath, project1.ID().String())).
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//input[@name='Name']").Exists()
	html.Element("//textarea[@name='Description']").Exists()
	html.Element("//select[@name='CounterpartyID']").Exists()
}

func TestProjectController_GetEditDrawer_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s/drawer", ProjectBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestProjectController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	projectService := env.App.Service(services.ProjectService{}).(*services.ProjectService)
	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"Update Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	project1 := project.New(
		"Original Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("Original description"),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "Updated Project Name")
	formData.Set("Description", "Updated description")
	formData.Set("CounterpartyID", createdCounterparty.ID().String())

	suite.POST(fmt.Sprintf("%s/%s", ProjectBasePath, project1.ID().String())).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(ProjectBasePath)

	updatedProject, err := projectService.GetByID(env.Ctx, project1.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Project Name", updatedProject.Name())
	require.Equal(t, "Updated description", updatedProject.Description())
	require.Equal(t, createdCounterparty.ID(), updatedProject.CounterpartyID())
}

func TestProjectController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	projectService := env.App.Service(services.ProjectService{}).(*services.ProjectService)
	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"Validation Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	project1 := project.New(
		"Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("Test description"),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "") // Invalid: required field
	formData.Set("Description", "Updated description")
	formData.Set("CounterpartyID", createdCounterparty.ID().String())

	response := suite.POST(fmt.Sprintf("%s/%s", ProjectBasePath, project1.ID().String())).
		Form(formData).
		Header("HX-Target", "project-edit-drawer").
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedProject, err := projectService.GetByID(env.Ctx, project1.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Project", unchangedProject.Name())
}

func TestProjectController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	projectService := env.App.Service(services.ProjectService{}).(*services.ProjectService)
	counterpartyService := env.App.Service(financeServices.CounterpartyService{}).(*financeServices.CounterpartyService)

	counterparty1 := counterparty.New(
		"Delete Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	project1 := project.New(
		"Project to Delete",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("Delete test description"),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	existingProject, err := projectService.GetByID(env.Ctx, project1.ID())
	require.NoError(t, err)
	require.Equal(t, "Project to Delete", existingProject.Name())

	suite.DELETE(fmt.Sprintf("%s/%s", ProjectBasePath, project1.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(ProjectBasePath)

	_, err = projectService.GetByID(env.Ctx, project1.ID())
	require.Error(t, err)
}

func TestProjectController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", ProjectBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestProjectController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule(), projects.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectController(env.App)
	suite.Register(controller)

	suite.GET(ProjectBasePath + "/invalid-uuid/drawer").
		Expect(t).
		Status(404)
}
