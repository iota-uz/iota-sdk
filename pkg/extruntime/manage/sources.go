package manage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// syncSubscriptions keeps the activation watermarks of the package's
// subscriptions. A subscription that is new at this activation starts at the
// current outbox position (events published before are never delivered to it,
// there is no automatic replay of older events); one that continues from the
// previously active version keeps its position; one the new version dropped
// loses its row.
func (s *Service) syncSubscriptions(ctx context.Context, tx pgx.Tx, ns string, m *manifest.Manifest) (int, error) {
	const op = serrors.Op("extruntime.manage.syncSubscriptions")
	rows, err := tx.Query(ctx, `SELECT subscription FROM extruntime.subscription_watermarks WHERE package = $1`, ns)
	if err != nil {
		return 0, serrors.FromDB(op, err)
	}
	existing := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, serrors.FromDB(op, err)
		}
		existing[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, serrors.FromDB(op, err)
	}
	added := 0
	keep := make([]string, 0, len(m.Subscriptions))
	var mark int64
	haveMark := false
	for _, sub := range m.Subscriptions {
		keep = append(keep, sub.ID)
		if existing[sub.ID] {
			continue
		}
		if !haveMark {
			var werr error
			if mark, werr = events.Watermark(ctx, tx); werr != nil {
				return 0, werr
			}
			haveMark = true
		}
		if _, err := tx.Exec(ctx, `INSERT INTO extruntime.subscription_watermarks (subscription, package, after_seq) VALUES ($1, $2, $3)`, sub.ID, ns, mark); err != nil {
			if isUnique(err) {
				return 0, serrors.New(serrors.FailedPrecondition, "activation blocked: subscription "+sub.ID+" belongs to another package").WithOp(op)
			}
			return 0, serrors.FromDB(op, err)
		}
		added++
	}
	if _, err := tx.Exec(ctx, `DELETE FROM extruntime.subscription_watermarks WHERE package = $1 AND NOT (subscription = ANY($2))`, ns, keep); err != nil {
		return 0, serrors.FromDB(op, err)
	}
	return added, nil
}

// SubscriptionSource lists the subscriptions of the activated versions with
// their activation watermarks; it implements events.SubscriptionSource.
func (s *Service) SubscriptionSource() events.SubscriptionSource { return subscriptionSource{s} }

type subscriptionSource struct{ s *Service }

func (src subscriptionSource) ActiveSubscriptions(ctx context.Context) ([]events.Subscription, error) {
	const op = serrors.Op("extruntime.manage.ActiveSubscriptions")
	s := src.s
	active, err := s.catalog.Active(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT subscription, after_seq FROM extruntime.subscription_watermarks`)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	marks := map[string]int64{}
	for rows.Next() {
		var id string
		var seq int64
		if err := rows.Scan(&id, &seq); err != nil {
			rows.Close()
			return nil, serrors.FromDB(op, err)
		}
		marks[id] = seq
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(op, err)
	}
	var out []events.Subscription
	for _, av := range active {
		if len(av.Manifest.Subscriptions) == 0 {
			continue
		}
		art, secrets, approved, err := s.resolve(ctx, av.Namespace, av.Version)
		if err != nil {
			return nil, err
		}
		if !approved {
			continue // approval revoked after activation: it delivers nothing
		}
		for _, sub := range av.Manifest.Subscriptions {
			after, ok := marks[sub.ID]
			if !ok {
				continue // not watermarked: activation has not committed it
			}
			out = append(out, events.Subscription{
				ID: sub.ID, Package: av.Namespace, Version: av.Version, Export: sub.Export, Event: sub.Event,
				Artifact: art, Secrets: secrets, AfterSeq: after,
			})
		}
	}
	return out, nil
}

// ScheduleSource lists the schedules of the activated versions; it
// implements events.ScheduleSource.
func (s *Service) ScheduleSource() events.ScheduleSource { return scheduleSource{s} }

type scheduleSource struct{ s *Service }

func (src scheduleSource) ActiveSchedules(ctx context.Context) ([]events.ScheduleDef, error) {
	s := src.s
	active, err := s.catalog.Active(ctx)
	if err != nil {
		return nil, err
	}
	// A schedule never fires slots before its version was activated.
	rows, err := s.db.Query(ctx, `SELECT namespace, activated_at FROM extruntime.activations WHERE state = 'active'`)
	if err != nil {
		return nil, serrors.FromDB(serrors.Op("extruntime.manage.ActiveSchedules"), err)
	}
	activated := map[string]time.Time{}
	for rows.Next() {
		var ns string
		var at time.Time
		if err := rows.Scan(&ns, &at); err != nil {
			rows.Close()
			return nil, serrors.FromDB(serrors.Op("extruntime.manage.ActiveSchedules"), err)
		}
		activated[ns] = at
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(serrors.Op("extruntime.manage.ActiveSchedules"), err)
	}
	var out []events.ScheduleDef
	for _, av := range active {
		if len(av.Manifest.Schedules) == 0 {
			continue
		}
		art, secrets, approved, err := s.resolve(ctx, av.Namespace, av.Version)
		if err != nil {
			return nil, err
		}
		if !approved {
			continue // approval revoked after activation: it fires nothing
		}
		for _, sc := range av.Manifest.Schedules {
			out = append(out, events.ScheduleDef{
				ID: sc.ID, Package: av.Namespace, Version: av.Version, Export: sc.Export, Cron: sc.Cron, Timezone: sc.Timezone,
				Artifact: art, Secrets: secrets, ActiveFrom: activated[av.Namespace],
			})
		}
	}
	return out, nil
}

// Implementations resolves workflow implementations from activated and
// stored package versions; it implements workflow.Implementations.
func (s *Service) Implementations() workflow.Implementations { return implementations{s} }

type implementations struct{ s *Service }

func (i implementations) find(m manifest.Manifest, workflowID string) (string, bool) {
	for _, impl := range m.Implementations {
		if impl.Kind == manifest.ImplWorkflow && impl.Implements == workflowID {
			return impl.Export, true
		}
	}
	return "", false
}

func (i implementations) build(ctx context.Context, pkg, version, export string) (workflow.Implementation, error) {
	art, secrets, err := i.s.ResolveArtifact(ctx, pkg, version)
	if err != nil {
		return workflow.Implementation{}, err
	}
	return workflow.Implementation{Package: pkg, Version: version, Export: export, Artifact: art, Secrets: secrets}, nil
}

// Active implements workflow.Implementations.
func (i implementations) Active(ctx context.Context, workflowID string) (workflow.Implementation, error) {
	active, err := i.s.catalog.Active(ctx)
	if err != nil {
		return workflow.Implementation{}, err
	}
	for _, av := range active {
		if export, ok := i.find(av.Manifest, workflowID); ok {
			return i.build(ctx, av.Namespace, av.Version, export)
		}
	}
	return workflow.Implementation{}, serrors.New(serrors.NotFound, "no active package implements workflow "+workflowID)
}

// At implements workflow.Implementations.
func (i implementations) At(ctx context.Context, workflowID, pkg, version string) (workflow.Implementation, error) {
	m, err := i.s.ManifestOf(ctx, pkg, version)
	if err != nil {
		return workflow.Implementation{}, err
	}
	export, ok := i.find(m, workflowID)
	if !ok {
		return workflow.Implementation{}, serrors.New(serrors.NotFound, fmt.Sprintf("%s@%s does not implement workflow %s", pkg, strings.TrimSpace(version), workflowID))
	}
	return i.build(ctx, pkg, version, export)
}
