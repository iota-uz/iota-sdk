package persistence_test

import (
	"testing"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	core "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/utils/random"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
)

type testUploadWrapper struct {
	u upload.Upload
}

func (w *testUploadWrapper) ID() uint {
	return w.u.ID()
}

func (w *testUploadWrapper) URL() string {
	if w.u.URL() == nil {
		return ""
	}
	return w.u.URL().String()
}

func (w *testUploadWrapper) Mimetype() string {
	if w.u.Mimetype() == nil {
		return ""
	}
	return w.u.Mimetype().String()
}

func (w *testUploadWrapper) Size() string {
	if w.u.Size() == nil {
		return ""
	}
	return w.u.Size().String()
}

func (w *testUploadWrapper) Hash() string {
	return w.u.Hash()
}

func (w *testUploadWrapper) Slug() string {
	return w.u.Slug()
}

func BenchmarkGormPositionRepository_Create(b *testing.B) {
	f := setupBenchmark(b)

	unitRepository := persistence.NewUnitRepository()
	positionRepository := persistence.NewPositionRepository()
	uploadRepository := core.NewUploadRepository()

	if err := unitRepository.Create(
		f.Ctx,
		&unit.Unit{
			ID:         1,
			Title:      "Unit 1",
			ShortTitle: "U1",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	); err != nil {
		b.Fatal(err)
	}

	uploads := make([]position.Upload, 0, 1000)
	for i := 0; i < 1000; i++ {
		entity, err := uploadRepository.Create(
			f.Ctx,
			upload.NewWithID(
				0,
				uuid.Nil, // tenant_id will be set correctly in repository
				random.String(32, random.LowerCharSet),
				"image.png",
				"image.png",
				"",
				1,
				mimetype.Lookup("image/png"),
				upload.UploadTypeImage,
				time.Now(),
				time.Now(),
			),
		)
		if err != nil {
			b.Fatal(err)
		}
		uploads = append(uploads, &testUploadWrapper{u: entity})
	}

	for range b.N {
		b.StartTimer()
		if _, err := positionRepository.Create(
			f.Ctx,
			position.New("Position 1", random.String(13, random.LowerCharSet),
				position.WithID(1),
				position.WithUnitID(1),
				position.WithImages(uploads),
				position.WithCreatedAt(time.Now()),
				position.WithUpdatedAt(time.Now())),
		); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
	}
}

func TestGormPositionRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	unitRepository := persistence.NewUnitRepository()
	positionRepository := persistence.NewPositionRepository()
	uploadRepository := core.NewUploadRepository()

	if err := unitRepository.Create(
		f.Ctx,
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
	createdUpload, err := uploadRepository.Create(
		f.Ctx,
		upload.NewWithID(
			1,
			uuid.Nil, // tenant_id will be set correctly in repository
			"hash",
			"url",
			"image.png",
			"",
			1,
			mimetype.Lookup("image/png"),
			upload.UploadTypeImage,
			time.Now(),
			time.Now(),
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := positionRepository.Create(
		f.Ctx, position.New("Position 1", "3141592653589",
			position.WithID(1),
			position.WithUnitID(1),
			position.WithImages([]position.Upload{&testUploadWrapper{u: createdUpload}}),
			position.WithCreatedAt(time.Now()),
			position.WithUpdatedAt(time.Now()))); err != nil {
		t.Fatal(err)
	}

	t.Run(
		"GetByID", func(t *testing.T) {
			positionEntity, err := positionRepository.GetByID(f.Ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if positionEntity.Title() != "Position 1" {
				t.Errorf("expected %s, got %s", "Position 1", positionEntity.Title())
			}
			if positionEntity.Barcode() != "3141592653589" {
				t.Errorf("expected %s, got %s", "3141592653589", positionEntity.Barcode())
			}
		},
	)

	t.Run(
		"Update", func(t *testing.T) {
			if _, err := positionRepository.Update(
				f.Ctx,
				position.New("Updated Position 1", "3141592653589",
					position.WithID(1),
					position.WithUnitID(1),
					position.WithImages([]position.Upload{})),
			); err != nil {
				t.Fatal(err)
			}
			positionEntity, err := positionRepository.GetByID(f.Ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if positionEntity.Title() != "Updated Position 1" {
				t.Errorf("expected %s, got %s", "Updated Position 1", positionEntity.Title())
			}
		},
	)

	t.Run(
		"Delete", func(t *testing.T) {
			if err := positionRepository.Delete(f.Ctx, 1); err != nil {
				t.Fatal(err)
			}
			_, err := positionRepository.GetByID(f.Ctx, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, persistence.ErrPositionNotFound) {
				t.Errorf("expected %v, got %v", persistence.ErrPositionNotFound, err)
			}
		},
	)
}
