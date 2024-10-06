package persistence

import (
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/project"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	stage "github.com/iota-agency/iota-erp/internal/domain/entities/project_stages"
	"github.com/iota-agency/iota-erp/internal/test_utils"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	db, err := test_utils.SqlOpen(configuration.Use().DbOpts)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err := test_utils.RunMigrations(db); err != nil {
		panic(err)
	}
	code := m.Run()
	if err := test_utils.RollbackMigrations(db); err != nil {
		panic(err)
	}
	os.Exit(code)
}

func TestGormPaymentRepository_CRUD(t *testing.T) {
	projectRepository := NewProjectRepository()
	stageRepository := NewProjectStageRepository()
	paymentRepository := NewPaymentRepository()
	ctx, tx, err := test_utils.GetTestContext(configuration.Use().DbOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	if err := projectRepository.Create(ctx, &project.Project{
		Id:   1,
		Name: "test",
	}); err != nil {
		t.Fatal(err)
	}
	stageEntity := &stage.ProjectStage{
		Id:        1,
		Name:      "test",
		ProjectID: 1,
	}
	if err := stageRepository.Create(ctx, stageEntity); err != nil {
		t.Fatal(err)
	}
	if err := paymentRepository.Create(ctx, &payment.Payment{
		StageId: 1,
		Amount:  100,
	}); err != nil {
		t.Fatal(err)
	}
	count, err := paymentRepository.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}
