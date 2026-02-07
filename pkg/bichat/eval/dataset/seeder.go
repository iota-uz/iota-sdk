package dataset

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type closeFn interface {
	Close()
}

type Seeder struct {
	beginner TxBeginner
	resolver Resolver
	closer   closeFn
}

func NewSeeder(ctx context.Context, dsn string, resolver Resolver) (*Seeder, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	return NewSeederWithPool(pool, resolver), nil
}

func NewSeederWithPool(pool *pgxpool.Pool, resolver Resolver) *Seeder {
	if resolver == nil {
		resolver = DefaultRegistry()
	}

	return &Seeder{
		beginner: pool,
		resolver: resolver,
		closer:   pool,
	}
}

func NewSeederWithBeginner(beginner TxBeginner, resolver Resolver) (*Seeder, error) {
	if beginner == nil {
		return nil, fmt.Errorf("tx beginner is nil")
	}
	if resolver == nil {
		resolver = DefaultRegistry()
	}

	return &Seeder{
		beginner: beginner,
		resolver: resolver,
	}, nil
}

func (s *Seeder) Close() {
	if s == nil || s.closer == nil {
		return
	}
	s.closer.Close()
}

func (s *Seeder) SeedAndBuildOracle(ctx context.Context, datasetID string, tenantID uuid.UUID) (map[string]Fact, error) {
	if s == nil {
		return nil, fmt.Errorf("seeder is nil")
	}
	if s.beginner == nil {
		return nil, fmt.Errorf("tx beginner is nil")
	}

	ds, err := s.resolver.Get(datasetID)
	if err != nil {
		return nil, err
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ds.Seed(ctx, tx, tenantID); err != nil {
		return nil, fmt.Errorf("seed dataset %q: %w", datasetID, err)
	}

	facts, err := ds.BuildOracle(ctx, tx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("build oracle for %q: %w", datasetID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return facts, nil
}

func (s *Seeder) BuildOracle(ctx context.Context, datasetID string, tenantID uuid.UUID) (map[string]Fact, error) {
	if s == nil {
		return nil, fmt.Errorf("seeder is nil")
	}
	if s.beginner == nil {
		return nil, fmt.Errorf("tx beginner is nil")
	}

	ds, err := s.resolver.Get(datasetID)
	if err != nil {
		return nil, err
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	facts, err := ds.BuildOracle(ctx, tx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("build oracle for %q: %w", datasetID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return facts, nil
}

func (s *Seeder) SeedAndBuildOracleMany(ctx context.Context, datasetIDs []string, tenantID uuid.UUID) (map[string]Fact, error) {
	return s.buildMany(ctx, datasetIDs, tenantID, true)
}

func (s *Seeder) BuildOracleMany(ctx context.Context, datasetIDs []string, tenantID uuid.UUID) (map[string]Fact, error) {
	return s.buildMany(ctx, datasetIDs, tenantID, false)
}

func (s *Seeder) buildMany(ctx context.Context, datasetIDs []string, tenantID uuid.UUID, withSeed bool) (map[string]Fact, error) {
	if len(datasetIDs) == 0 {
		return map[string]Fact{}, nil
	}

	unique := make(map[string]struct{}, len(datasetIDs))
	for _, id := range datasetIDs {
		if id == "" {
			continue
		}
		unique[id] = struct{}{}
	}

	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	allFacts := make(map[string]Fact)
	for _, id := range ids {
		var (
			facts map[string]Fact
			err   error
		)
		if withSeed {
			facts, err = s.SeedAndBuildOracle(ctx, id, tenantID)
		} else {
			facts, err = s.BuildOracle(ctx, id, tenantID)
		}
		if err != nil {
			return nil, err
		}
		for k, v := range facts {
			allFacts[k] = v
		}
	}

	return allFacts, nil
}
