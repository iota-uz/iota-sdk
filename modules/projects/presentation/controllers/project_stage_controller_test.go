package controllers_test

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	financeServices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	ProjectStageBasePath = "/project-stages"
)

func newProjectStageSuiteBuilder(tb testing.TB) *itf.SuiteBuilder {
	tb.Helper()
	return itf.NewSuiteBuilder(tb).WithComponents(
		core.NewComponent(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}),
		finance.NewComponent(),
		projects.NewComponent(),
	)
}

func TestProjectStageController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test project
	project1 := project.New(
		"Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("Test project description"),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	// Create test project stages
	stage1 := projectstage.New(
		project1.ID(),
		1,
		100000, // $1,000.00
		projectstage.WithDescription("Test stage 1"),
	)

	stage2 := projectstage.New(
		project1.ID(),
		2,
		200000, // $2,000.00
		projectstage.WithDescription("Test stage 2"),
	)

	err = projectStageService.Create(env.Ctx, stage1)
	require.NoError(t, err)
	err = projectStageService.Create(env.Ctx, stage2)
	require.NoError(t, err)

	response := suite.GET(ProjectStageBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("Test stage 1").
		Contains("Test stage 2")
}

func TestProjectStageController_List_FilterChips(t *testing.T) {
	t.Parallel()
	suite := newProjectStageSuiteBuilder(t).Build().AsUser(itf.User())
	env := suite.Environment()
	suite.Register(controllers.NewProjectStageController())

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)
	customer, err := counterpartyService.Create(env.Ctx, counterparty.New("Chip Customer", counterparty.Customer,
		counterparty.Individual, counterparty.WithTenantID(env.Tenant.ID)))
	require.NoError(t, err)

	alpha := project.New("Alpha", customer.ID(), project.WithTenantID(env.Tenant.ID))
	beta := project.New("Beta", customer.ID(), project.WithTenantID(env.Tenant.ID))
	require.NoError(t, projectService.Create(env.Ctx, alpha))
	require.NoError(t, projectService.Create(env.Ctx, beta))

	done := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, projectStageService.Create(env.Ctx, projectstage.New(alpha.ID(), 1, 100000,
		projectstage.WithDescription("Alpha finished stage"), projectstage.WithFactualEndDate(&done))))
	require.NoError(t, projectStageService.Create(env.Ctx, projectstage.New(alpha.ID(), 2, 100000,
		projectstage.WithDescription("Alpha open stage"))))
	require.NoError(t, projectStageService.Create(env.Ctx, projectstage.New(beta.ID(), 1, 100000,
		projectstage.WithDescription("Beta open stage"))))

	cases := []struct {
		name    string
		query   string
		present []string
		absent  []string
	}{
		{"project", "?f=project:is:" + alpha.ID().String(), []string{"Alpha finished stage", "Alpha open stage"}, []string{"Beta open stage"}},
		{"not finished", "?f=factual_end:empty:", []string{"Alpha open stage", "Beta open stage"}, []string{"Alpha finished stage"}},
		{"project and not finished", "?f=project:is:" + alpha.ID().String() + "&f=factual_end:empty:", []string{"Alpha open stage"}, []string{"Alpha finished stage", "Beta open stage"}},
		{"search keeps working with chips", "?Search=finished&f=project:is:" + alpha.ID().String(), []string{"Alpha finished stage"}, []string{"Alpha open stage"}},
	}
	for _, tc := range cases {
		response := suite.GET(ProjectStageBasePath + tc.query).HTMX().Expect(t).Status(200)
		for _, text := range tc.present {
			response.Contains(text)
		}
		for _, text := range tc.absent {
			response.NotContains(text)
		}
		response.Contains(`hx-swap-oob="outerHTML"`)
	}

	// Paging keeps the search and the chips: with one stage per page the
	// second page of "Alpha" stages is still Alpha's, and its links carry the
	// conditions on.
	query := "?Search=stage&f=project:is:" + alpha.ID().String() + "&fb=1&limit=1"
	first := suite.GET(ProjectStageBasePath + query).Expect(t).Status(200)
	match := regexp.MustCompile(`href="([^"]*[?&](?:amp;)?page=2[^"]*)"`).FindStringSubmatch(first.Body())
	require.NotNil(t, match, "the first page links to page 2")
	link, err := url.Parse(html.UnescapeString(match[1]))
	require.NoError(t, err)
	got := link.Query()
	assert.Equal(t, "stage", got.Get("Search"))
	assert.Equal(t, []string{"project:is:" + alpha.ID().String()}, got["f"])
	assert.Equal(t, "1", got.Get("fb"))
	assert.Equal(t, "1", got.Get("limit"))

	// Follow the generated link itself, not a hand-built one.
	second := suite.GET(link.Path + "?" + link.RawQuery).HTMX().Expect(t).Status(200)
	body := second.Body()
	assert.True(t, strings.Contains(body, "Alpha finished stage") || strings.Contains(body, "Alpha open stage"),
		"page 2 shows one of Alpha's stages")
	second.NotContains("Beta open stage")

	// Opened as a full page (reload, or the link in a new tab), the form the
	// next chip change serializes holds the same search and page size.
	full := suite.GET(link.Path + "?" + link.RawQuery).Expect(t).Status(200)
	full.Contains(`name="Search"`)
	assert.Regexp(t, `name="Search"[^>]*value="stage"|value="stage"[^>]*name="Search"`, full.Body(), "the search box keeps the search")
	assert.Regexp(t, `<option value="1" selected`, full.Body(), "the page size keeps limit=1")
	// A chip change sends that form: page 1 with the search and size kept.
	changed := suite.GET(ProjectStageBasePath + "?Search=stage&limit=1&f=project:is:" + alpha.ID().String() + "&f=factual_end:empty:&fb=1").HTMX().Expect(t).Status(200)
	changed.Contains("Alpha open stage").NotContains("Alpha finished stage").NotContains("Beta open stage")
}

func TestProjectStageController_List_HTMX_Request(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"HTMX Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test project
	project1 := project.New(
		"HTMX Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithDescription("HTMX test project"),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	// Create test project stage
	stage1 := projectstage.New(
		project1.ID(),
		1,
		150000, // $1,500.00
		projectstage.WithDescription("HTMX test stage"),
	)

	err = projectStageService.Create(env.Ctx, stage1)
	require.NoError(t, err)

	suite.GET(ProjectStageBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX test stage")
}

func TestProjectStageController_ListByProject_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Project Filter Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test projects
	project1 := project.New(
		"Project Filter Test 1",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
	)

	project2 := project.New(
		"Project Filter Test 2",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)
	err = projectService.Create(env.Ctx, project2)
	require.NoError(t, err)

	// Create stages for project1
	stage1 := projectstage.New(
		project1.ID(),
		1,
		100000,
		projectstage.WithDescription("Project 1 Stage 1"),
	)

	stage2 := projectstage.New(
		project1.ID(),
		2,
		200000,
		projectstage.WithDescription("Project 1 Stage 2"),
	)

	// Create stage for project2
	stage3 := projectstage.New(
		project2.ID(),
		1,
		300000,
		projectstage.WithDescription("Project 2 Stage 1"),
	)

	err = projectStageService.Create(env.Ctx, stage1)
	require.NoError(t, err)
	err = projectStageService.Create(env.Ctx, stage2)
	require.NoError(t, err)
	err = projectStageService.Create(env.Ctx, stage3)
	require.NoError(t, err)

	// Test filtering by project1
	response := suite.GET(fmt.Sprintf("%s/by-project/%s", ProjectStageBasePath, project1.ID().String())).
		Expect(t).
		Status(200)

	// Should contain project1 stages but not project2 stages
	response.Contains("Project 1 Stage 1").
		Contains("Project 1 Stage 2").
		NotContains("Project 2 Stage 1")
}

func TestProjectStageController_ListByProject_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	suite.GET(ProjectStageBasePath + "/by-project/invalid-uuid").
		Expect(t).
		Status(404) // 404 because regex pattern doesn't match
}

func TestProjectStageController_GetNewDrawer_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	response := suite.GET(ProjectStageBasePath + "/new/drawer").
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//select[@name='ProjectID']").Exists()
	html.Element("//input[@name='StageNumber']").Exists()
	html.Element("//textarea[@name='Desc']").Exists()
	html.Element("//input[@name='TotalAmount']").Exists()
}

func TestProjectStageController_Create_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Create Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test project
	project1 := project.New(
		"Create Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("ProjectID", project1.ID().String())
	formData.Set("StageNumber", "1")
	formData.Set("Desc", "Valid test stage")
	formData.Set("TotalAmount", "250000") // $2,500.00

	suite.POST(ProjectStageBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(ProjectStageBasePath)

	stages, err := projectStageService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, stages, 1)

	savedStage := stages[0]
	require.Equal(t, project1.ID(), savedStage.ProjectID())
	require.Equal(t, 1, savedStage.StageNumber())
	require.Equal(t, "Valid test stage", savedStage.Description())
	require.Equal(t, int64(250000), savedStage.TotalAmount())
}

func TestProjectStageController_Create_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)

	formData := url.Values{}
	formData.Set("ProjectID", "")    // Invalid: required field
	formData.Set("StageNumber", "0") // Invalid: must be min=1
	formData.Set("Desc", "Test description")
	formData.Set("TotalAmount", "0") // Invalid: must be min=1

	response := suite.POST(ProjectStageBasePath).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	stages, err := projectStageService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, stages)
}

func TestProjectStageController_GetEditDrawer_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Edit Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test project
	project1 := project.New(
		"Edit Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	// Create test project stage
	stage1 := projectstage.New(
		project1.ID(),
		1,
		300000,
		projectstage.WithDescription("Edit test stage"),
	)

	err = projectStageService.Create(env.Ctx, stage1)
	require.NoError(t, err)

	response := suite.GET(fmt.Sprintf("%s/%s/drawer", ProjectStageBasePath, stage1.ID().String())).
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//input[@name='StageNumber']").Exists()
	html.Element("//textarea[@name='Desc']").Exists()
	html.Element("//input[@name='TotalAmount']").Exists()

	response.Contains("Edit test stage")
}

func TestProjectStageController_GetEditDrawer_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s/drawer", ProjectStageBasePath, nonExistentID.String())).
		Expect(t).
		Status(404)
}

func TestProjectStageController_GetEditDrawer_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	suite.GET(ProjectStageBasePath + "/invalid-uuid/drawer").
		Expect(t).
		Status(404) // 404 because regex pattern doesn't match
}

func TestProjectStageController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Update Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test project
	project1 := project.New(
		"Update Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	// Create test project stage
	stage1 := projectstage.New(
		project1.ID(),
		1,
		100000,
		projectstage.WithDescription("Original stage description"),
	)

	err = projectStageService.Create(env.Ctx, stage1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("StageNumber", "2")
	formData.Set("Desc", "Updated stage description")
	formData.Set("TotalAmount", "350000")

	suite.POST(fmt.Sprintf("%s/%s", ProjectStageBasePath, stage1.ID().String())).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(ProjectStageBasePath)

	updatedStage, err := projectStageService.GetByID(env.Ctx, stage1.ID())
	require.NoError(t, err)

	require.Equal(t, 2, updatedStage.StageNumber())
	require.Equal(t, "Updated stage description", updatedStage.Description())
	require.Equal(t, int64(350000), updatedStage.TotalAmount())
}

func TestProjectStageController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Validation Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test project
	project1 := project.New(
		"Validation Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	// Create test project stage
	stage1 := projectstage.New(
		project1.ID(),
		1,
		100000,
		projectstage.WithDescription("Test stage"),
	)

	err = projectStageService.Create(env.Ctx, stage1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("StageNumber", "0") // Invalid: must be min=1
	formData.Set("Desc", "Updated description")
	formData.Set("TotalAmount", "0") // Invalid: must be min=1

	response := suite.POST(fmt.Sprintf("%s/%s", ProjectStageBasePath, stage1.ID().String())).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedStage, err := projectStageService.GetByID(env.Ctx, stage1.ID())
	require.NoError(t, err)
	require.Equal(t, "Test stage", unchangedStage.Description())
	require.Equal(t, int64(100000), unchangedStage.TotalAmount())
}

func TestProjectStageController_Update_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	nonExistentID := uuid.New()

	formData := url.Values{}
	formData.Set("StageNumber", "1")
	formData.Set("Desc", "Test description")
	formData.Set("TotalAmount", "100000")

	suite.POST(fmt.Sprintf("%s/%s", ProjectStageBasePath, nonExistentID.String())).
		Form(formData).
		Expect(t).
		Status(404)
}

func TestProjectStageController_Update_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	formData := url.Values{}
	formData.Set("StageNumber", "1")
	formData.Set("Desc", "Test description")
	formData.Set("TotalAmount", "100000")

	suite.POST(ProjectStageBasePath + "/invalid-uuid").
		Form(formData).
		Expect(t).
		Status(404) // 404 because regex pattern doesn't match
}

func TestProjectStageController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Delete Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test project
	project1 := project.New(
		"Delete Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	// Create test project stage
	stage1 := projectstage.New(
		project1.ID(),
		1,
		100000,
		projectstage.WithDescription("Stage to delete"),
	)

	err = projectStageService.Create(env.Ctx, stage1)
	require.NoError(t, err)

	// Verify stage exists
	existingStage, err := projectStageService.GetByID(env.Ctx, stage1.ID())
	require.NoError(t, err)
	require.Equal(t, "Stage to delete", existingStage.Description())

	suite.DELETE(fmt.Sprintf("%s/%s", ProjectStageBasePath, stage1.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(ProjectStageBasePath)

	// Verify stage is deleted
	_, err = projectStageService.GetByID(env.Ctx, stage1.ID())
	require.Error(t, err)
}

func TestProjectStageController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", ProjectStageBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestProjectStageController_Delete_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	suite.DELETE(ProjectStageBasePath + "/invalid-uuid").
		Expect(t).
		Status(404) // 404 because regex pattern doesn't match
}

func TestProjectStageController_Create_WithDates(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := newProjectStageSuiteBuilder(t).Build().AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewProjectStageController()
	suite.Register(controller)

	projectStageService := itf.GetService[services.ProjectStageService](env)
	projectService := itf.GetService[services.ProjectService](env)
	counterpartyService := itf.GetService[financeServices.CounterpartyService](env)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Date Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test project
	project1 := project.New(
		"Date Test Project",
		createdCounterparty.ID(),
		project.WithTenantID(env.Tenant.ID),
	)

	err = projectService.Create(env.Ctx, project1)
	require.NoError(t, err)

	startDate := time.Now().AddDate(0, 0, 1)      // Tomorrow
	plannedEndDate := time.Now().AddDate(0, 1, 0) // Next month

	formData := url.Values{}
	formData.Set("ProjectID", project1.ID().String())
	formData.Set("StageNumber", "1")
	formData.Set("Desc", "Stage with dates")
	formData.Set("TotalAmount", "100000")
	formData.Set("StartDate", startDate.Format("2006-01-02"))
	formData.Set("PlannedEndDate", plannedEndDate.Format("2006-01-02"))

	suite.POST(ProjectStageBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(ProjectStageBasePath)

	stages, err := projectStageService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, stages, 1)

	savedStage := stages[0]
	require.Equal(t, "Stage with dates", savedStage.Description())
	require.NotNil(t, savedStage.StartDate())
	require.NotNil(t, savedStage.PlannedEndDate())
	require.Nil(t, savedStage.FactualEndDate())
}
