package persistence_test

import (
	"github.com/gabriel-vasile/mimetype"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	core "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
)

func TestGormPositionRepository_CRUD(t *testing.T) {
	f := setupTest(t)

	unitRepository := persistence.NewUnitRepository()
	positionRepository := persistence.NewPositionRepository()
	uploadRepository := core.NewUploadRepository()

	if err := unitRepository.Create(
		f.ctx,
		&unit.Unit{
			ID:         1,
			Title:      "Unit 1",
			ShortTitle: "U1",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	); err != nil {
		t.Fatal(err)
	}

	if err := uploadRepository.Create(
		f.ctx, &upload.Upload{
			ID:        1,
			Hash:      "hash",
			Path:      "url",
			Size:      1,
			Mimetype:  *mimetype.Lookup("image/png"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
		t.Fatal(err)
	}

	if err := positionRepository.Create(
		f.ctx, &position.Position{
			ID:        1,
			Title:     "Position 1",
			Barcode:   "3141592653589",
			UnitID:    1,
			Images:    []upload.Upload{{ID: 1}},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
		t.Fatal(err)
	}

	t.Run(
		"GetByID", func(t *testing.T) {
			positionEntity, err := positionRepository.GetByID(f.ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if positionEntity.Title != "Position 1" {
				t.Errorf("expected %s, got %s", "Position 1", positionEntity.Title)
			}
			if positionEntity.Barcode != "3141592653589" {
				t.Errorf("expected %s, got %s", "3141592653589", positionEntity.Barcode)
			}
		},
	)

	t.Run(
		"Update", func(t *testing.T) {
			if err := positionRepository.Update(
				f.ctx, &position.Position{
					ID:      1,
					Title:   "Updated Position 1",
					Barcode: "3141592653589",
				},
			); err != nil {
				t.Fatal(err)
			}
			positionEntity, err := positionRepository.GetByID(f.ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if positionEntity.Title != "Updated Position 1" {
				t.Errorf("expected %s, got %s", "Updated Position 1", positionEntity.Title)
			}
		},
	)

	t.Run(
		"Delete", func(t *testing.T) {
			if err := positionRepository.Delete(f.ctx, 1); err != nil {
				t.Fatal(err)
			}
			_, err := positionRepository.GetByID(f.ctx, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		},
	)
}
