package backtest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const op = serrors.Op("automation.backtest")

// Config configures the service.
type Config struct {
	DB *pgxpool.Pool
	// Automations reads graphs and versions; Authoring runs them (engine.Runtime provides both).
	Automations engine.Service
	Authoring   engine.Authoring
	// Catalog tells decision nodes (engine.DecisionNode) and the events an author may read.
	Catalog catalog.Registry
	// Users reloads the rights of the person who queued a job when the worker runs it.
	Users host.SubjectLoader
	// Subjects reads the state of an event's subject (engine.Runtime.Subjects); a SnapshotLoader gives it
	// as it was at the event. nil: examples have no subject state.
	Subjects engine.SubjectLoader
	// Connections and Connectors open a connection whose history is imported (nil: none is offered).
	Connections connectors.ConnectionStore
	Connectors  connectors.Directory
	// Sources are the modules' history sources.
	Sources []HistorySource
	// ConnectionAccess tells whether the actor may import the history of a connection (default
	// DefaultConnectionAccess).
	ConnectionAccess func(a Actor, c connectors.Connection) bool
	// PollInterval is how often the worker looks for queued jobs (default 1 s); LeaseTTL is how long a
	// job is held without a heartbeat (default 2 min); Parallel is how many examples a backtest runs at
	// once (default 4).
	PollInterval time.Duration
	LeaseTTL     time.Duration
	Parallel     int
	// StrictFencing refuses a backtest that would pay a decision whose provider cannot deduplicate a
	// repeated call (catalog.SupportsIdempotencyKey): a worker that stalls past LeaseTTL with such a call
	// in flight may have it paid again by the worker that takes the backtest over. Off (the default), the
	// backtest runs and that risk is bounded to the calls in flight when the lease passes (one in a live
	// backtest, which runs one example at a time; see engine.CallFence).
	StrictFencing bool
	Logger       *slog.Logger
	// Now is the clock (default time.Now).
	Now func() time.Time
}

// DefaultConnectionAccess admits the history of a connection to the one who created it and to those who
// manage connections.
func DefaultConnectionAccess(a Actor, c connectors.Connection) bool {
	return a.Subject.Has(PermConnectionsManage) || (c.CreatedBy != 0 && c.CreatedBy == a.UserID)
}

// New builds the service.
func New(cfg Config) (*Svc, error) {
	if cfg.DB == nil || cfg.Automations == nil || cfg.Authoring == nil || cfg.Catalog == nil || cfg.Users == nil {
		return nil, serrors.New(serrors.Invalid, "backtest: DB, Automations, Authoring, Catalog and Users are required").WithOp(op)
	}
	if cfg.ConnectionAccess == nil {
		cfg.ConnectionAccess = DefaultConnectionAccess
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 2 * time.Minute
	}
	if cfg.Parallel <= 0 {
		cfg.Parallel = 4
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	names := map[string]bool{}
	for _, s := range cfg.Sources {
		if s.Name == "" || s.Importer == nil || names[s.Name] {
			return nil, serrors.New(serrors.Invalid, "backtest: a history source needs a unique name and an importer").WithOp(op)
		}
		names[s.Name] = true
	}
	return &Svc{cfg: cfg}, nil
}

// Svc is the Service and its worker.
type Svc struct{ cfg Config }

var _ Service = (*Svc)(nil)

func (s *Svc) authorize(a Actor) error {
	if a.TenantID == uuid.Nil || a.UserID <= 0 {
		return fmt.Errorf("%w: the actor has no tenant or user", engine.ErrForbidden)
	}
	if !a.Subject.Has(engine.PermAuthor) {
		return fmt.Errorf("%w: %s is required", engine.ErrForbidden, engine.PermAuthor)
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{engine.ErrInvalidInput}, args...)...)
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return engine.ErrNotFound
	}
	return serrors.FromDB(op, err)
}

// automation checks the automation is the actor's tenant's and not deleted.
func (s *Svc) automation(ctx context.Context, a Actor, id uuid.UUID) (engine.Automation, error) {
	return s.cfg.Automations.Get(ctx, a, id)
}

// datasetRow reads a dataset of the actor's tenant whose automation the actor sees.
func (s *Svc) datasetRow(ctx context.Context, a Actor, id DatasetID) (Dataset, error) {
	var d Dataset
	var origin string
	err := s.cfg.DB.QueryRow(ctx, `
		SELECT id, automation_id, name, description, created_by, created_origin, created_at,
			(SELECT count(*) FROM automation.dataset_examples e WHERE e.tenant_id = d.tenant_id AND e.dataset_id = d.id AND e.deleted_at IS NULL),
			(SELECT count(DISTINCT l.example_id) FROM automation.dataset_labels l JOIN automation.dataset_examples e
				ON e.tenant_id = l.tenant_id AND e.id = l.example_id WHERE e.tenant_id = d.tenant_id AND e.dataset_id = d.id AND e.deleted_at IS NULL)
		FROM automation.datasets d WHERE tenant_id = $1 AND id = $2`, a.TenantID, id).
		Scan(&d.ID, &d.AutomationID, &d.Name, &d.Description, &d.CreatedBy, &origin, &d.CreatedAt, &d.Examples, &d.Labeled)
	if err != nil {
		return Dataset{}, dbErr(err)
	}
	d.CreatedOrigin = engine.Origin(origin)
	if _, err := s.automation(ctx, a, d.AutomationID); err != nil {
		return Dataset{}, err
	}
	return d, nil
}

func (s *Svc) CreateDataset(ctx context.Context, a Actor, in CreateDatasetInput) (DatasetID, error) {
	if err := s.authorize(a); err != nil {
		return uuid.Nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 200 {
		return uuid.Nil, invalid("a dataset's name is 1..200 characters")
	}
	if len(in.Description) > 2000 {
		return uuid.Nil, invalid("a dataset's description is at most 2000 characters")
	}
	if _, err := s.automation(ctx, a, in.AutomationID); err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	err := pgx.BeginFunc(ctx, s.cfg.DB, func(tx pgx.Tx) error {
		// The automation's row serializes the count of its datasets.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM automation.automations WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, a.TenantID, in.AutomationID); err != nil {
			return dbErr(err)
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.datasets WHERE tenant_id = $1 AND automation_id = $2`, a.TenantID, in.AutomationID).Scan(&n); err != nil {
			return dbErr(err)
		}
		if n >= MaxDatasets {
			return fmt.Errorf("%w: an automation has at most %d datasets", engine.ErrLimitExceeded, MaxDatasets)
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO automation.datasets (tenant_id, id, automation_id, name, description, created_by, created_origin)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, a.TenantID, id, in.AutomationID, name, strings.TrimSpace(in.Description), a.UserID, string(a.Origin))
		return dbErr(err)
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *Svc) ListDatasets(ctx context.Context, a Actor, automationID uuid.UUID) ([]Dataset, error) {
	if err := s.authorize(a); err != nil {
		return nil, err
	}
	if _, err := s.automation(ctx, a, automationID); err != nil {
		return nil, err
	}
	rows, err := s.cfg.DB.Query(ctx, `
		SELECT id, automation_id, name, description, created_by, created_origin, created_at,
			(SELECT count(*) FROM automation.dataset_examples e WHERE e.tenant_id = d.tenant_id AND e.dataset_id = d.id AND e.deleted_at IS NULL),
			(SELECT count(DISTINCT l.example_id) FROM automation.dataset_labels l JOIN automation.dataset_examples e
				ON e.tenant_id = l.tenant_id AND e.id = l.example_id WHERE e.tenant_id = d.tenant_id AND e.dataset_id = d.id AND e.deleted_at IS NULL)
		FROM automation.datasets d WHERE tenant_id = $1 AND automation_id = $2 ORDER BY created_at DESC, id`, a.TenantID, automationID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Dataset, error) {
		var d Dataset
		var origin string
		err := r.Scan(&d.ID, &d.AutomationID, &d.Name, &d.Description, &d.CreatedBy, &origin, &d.CreatedAt, &d.Examples, &d.Labeled)
		d.CreatedOrigin = engine.Origin(origin)
		return d, err
	})
	if err != nil {
		return nil, dbErr(err)
	}
	for i := range out {
		if out[i].Imports, err = s.imports(ctx, a.TenantID, out[i].ID, 3); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Svc) GetDataset(ctx context.Context, a Actor, ds DatasetID) (Dataset, error) {
	if err := s.authorize(a); err != nil {
		return Dataset{}, err
	}
	d, err := s.datasetRow(ctx, a, ds)
	if err != nil {
		return Dataset{}, err
	}
	d.Imports, err = s.imports(ctx, a.TenantID, ds, 10)
	return d, err
}

func (s *Svc) DeleteDataset(ctx context.Context, a Actor, ds DatasetID) error {
	if err := s.authorize(a); err != nil {
		return err
	}
	if _, err := s.datasetRow(ctx, a, ds); err != nil {
		return err
	}
	_, err := s.cfg.DB.Exec(ctx, `DELETE FROM automation.datasets WHERE tenant_id = $1 AND id = $2`, a.TenantID, ds)
	return dbErr(err)
}

const importColumns = `id, dataset_id, source, status, imported, duplicates, overflow, unreadable, error, created_at, finished_at`

func scanImport(r pgx.Row) (ImportJob, error) {
	var j ImportJob
	var src []byte
	err := r.Scan(&j.ID, &j.DatasetID, &src, &j.Status, &j.Imported, &j.Duplicates, &j.Overflow, &j.Unreadable, &j.Error, &j.CreatedAt, &j.FinishedAt)
	if err == nil {
		_ = json.Unmarshal(src, &j.Source)
	}
	return j, err
}

func (s *Svc) imports(ctx context.Context, tenant uuid.UUID, ds DatasetID, limit int) ([]ImportJob, error) {
	rows, err := s.cfg.DB.Query(ctx, `SELECT `+importColumns+` FROM automation.dataset_imports
		WHERE tenant_id = $1 AND dataset_id = $2 ORDER BY created_at DESC, id LIMIT $3`, tenant, ds, limit)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (ImportJob, error) { return scanImport(r) })
	return out, dbErr(err)
}

func (s *Svc) ImportStatus(ctx context.Context, a Actor, id ImportJobID) (ImportJob, error) {
	if err := s.authorize(a); err != nil {
		return ImportJob{}, err
	}
	j, err := scanImport(s.cfg.DB.QueryRow(ctx, `SELECT `+importColumns+` FROM automation.dataset_imports WHERE tenant_id = $1 AND id = $2`, a.TenantID, id))
	if err != nil {
		return ImportJob{}, dbErr(err)
	}
	if _, err := s.datasetRow(ctx, a, j.DatasetID); err != nil {
		return ImportJob{}, err
	}
	return j, nil
}

// checkSource validates an import source for the actor: the window, the connection and its access, the
// module source and its permission.
func (s *Svc) checkSource(ctx context.Context, a Actor, src *ImportSource) error {
	if src.Window.To.IsZero() {
		src.Window.To = s.cfg.Now()
	}
	if src.Window.From.IsZero() || !src.Window.From.Before(src.Window.To) {
		return invalid("the window needs a start before its end")
	}
	if src.Window.To.Sub(src.Window.From) > MaxWindow {
		return invalid("the window is at most %d days", int(MaxWindow.Hours()/24))
	}
	if len(src.Filter) > 16<<10 {
		return invalid("the filter is at most 16 KiB")
	}
	switch src.Kind {
	case ImportJournal:
		if len(src.Events) > 40 {
			return invalid("at most 40 event kinds")
		}
	case ImportConnection:
		_, _, err := s.openConnection(ctx, a, src.ConnectionID)
		return err
	case ImportModule:
		hs, ok := s.source(src.Source)
		if !ok {
			return fmt.Errorf("%w: no history source %q", ErrUnsupported, src.Source)
		}
		if hs.Permission != "" && !a.Subject.Has(hs.Permission) {
			return fmt.Errorf("%w: %s is required", engine.ErrForbidden, hs.Permission)
		}
	default:
		return invalid("an import's kind is journal, connection or module")
	}
	return nil
}

func (s *Svc) source(name string) (HistorySource, bool) {
	for _, hs := range s.cfg.Sources {
		if hs.Name == name {
			return hs, true
		}
	}
	return HistorySource{}, false
}

// openConnection opens a connection whose history the actor may import: its connector must import
// history.
func (s *Svc) openConnection(ctx context.Context, a Actor, id uuid.UUID) (connectors.Connection, HistoryImporter, error) {
	if s.cfg.Connections == nil || s.cfg.Connectors == nil || id == uuid.Nil {
		return connectors.Connection{}, nil, fmt.Errorf("%w: no connection to import from", ErrUnsupported)
	}
	conn, _, err := s.cfg.Connections.Get(ctx, a.TenantID, id)
	if errors.Is(err, connectors.ErrNotFound) {
		return connectors.Connection{}, nil, engine.ErrNotFound
	}
	if err != nil {
		return connectors.Connection{}, nil, serrors.Wrap(op, err)
	}
	if !s.cfg.ConnectionAccess(a, conn) {
		return connectors.Connection{}, nil, fmt.Errorf("%w: importing the history of a connection needs the right to that connection", engine.ErrForbidden)
	}
	c, ok := s.cfg.Connectors.Get(conn.Kind)
	if !ok {
		return connectors.Connection{}, nil, fmt.Errorf("%w: the connection's connector is not installed", ErrUnsupported)
	}
	hi, ok := c.(HistoryImporter)
	if !ok {
		return connectors.Connection{}, nil, fmt.Errorf("%w: the connection's connector does not import history", ErrUnsupported)
	}
	return conn, hi, nil
}

func (s *Svc) Sources(ctx context.Context, a Actor, automationID uuid.UUID) ([]SourceOption, error) {
	if err := s.authorize(a); err != nil {
		return nil, err
	}
	if _, err := s.automation(ctx, a, automationID); err != nil {
		return nil, err
	}
	out := make([]SourceOption, 0, len(s.cfg.Sources))
	if s.cfg.Connections != nil && s.cfg.Connectors != nil {
		for _, c := range s.cfg.Connectors.List() {
			if _, ok := c.(HistoryImporter); !ok {
				continue
			}
			list, err := s.cfg.Connections.List(ctx, a.TenantID, c.Kind())
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			for _, conn := range list {
				if !s.cfg.ConnectionAccess(a, conn) {
					continue
				}
				title := conn.DisplayName
				if title == "" {
					title = conn.Name
				}
				out = append(out, SourceOption{Kind: ImportConnection, ConnectionID: conn.ID, Title: title, ConnectorKind: string(conn.Kind)})
			}
		}
	}
	if len(s.cfg.Sources) == 0 {
		return out, nil
	}
	triggers, err := s.triggerEvents(ctx, a, automationID)
	if err != nil {
		return nil, err
	}
	for _, hs := range s.cfg.Sources {
		if hs.Permission != "" && !a.Subject.Has(hs.Permission) {
			continue
		}
		if !offeredFor(hs, triggers) {
			continue
		}
		out = append(out, SourceOption{Kind: ImportModule, Source: hs.Name, Title: hs.Title})
	}
	return out, nil
}

func (s *Svc) Import(ctx context.Context, a Actor, ds DatasetID, src ImportSource) (ImportJobID, error) {
	if err := s.authorize(a); err != nil {
		return uuid.Nil, err
	}
	if _, err := s.datasetRow(ctx, a, ds); err != nil {
		return uuid.Nil, err
	}
	if err := s.checkSource(ctx, a, &src); err != nil {
		return uuid.Nil, err
	}
	body, err := json.Marshal(src)
	if err != nil {
		return uuid.Nil, invalid("the source does not encode")
	}
	id := uuid.New()
	_, err = s.cfg.DB.Exec(ctx, `
		INSERT INTO automation.dataset_imports (tenant_id, id, dataset_id, source, status, created_by, created_origin)
		VALUES ($1, $2, $3, $4, 'queued', $5, $6)`, a.TenantID, id, ds, body, a.UserID, string(a.Origin))
	if err != nil {
		return uuid.Nil, dbErr(err)
	}
	return id, nil
}

// offeredFor is whether a module's source is offered for an automation triggered by events: a source
// that names its events is offered when one of them starts the automation; one that names none always.
func offeredFor(hs HistorySource, triggers []string) bool {
	if len(hs.Events) == 0 {
		return true
	}
	for _, e := range hs.Events {
		for _, t := range triggers {
			if e == t {
				return true
			}
		}
	}
	return false
}

// validKey is a node id or an outcome: what labels name.
func validKey(s string) bool {
	return s != "" && len(s) <= 100 && strings.TrimSpace(s) == s
}

func (s *Svc) AddSynthetic(ctx context.Context, a Actor, ds DatasetID, ev SyntheticEvent) (ExampleID, error) {
	if err := s.authorize(a); err != nil {
		return uuid.Nil, err
	}
	if _, err := s.datasetRow(ctx, a, ds); err != nil {
		return uuid.Nil, err
	}
	kind := strings.TrimSpace(ev.Kind)
	if kind == "" {
		return uuid.Nil, invalid("a synthetic event names its kind")
	}
	view, err := s.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject))
	if err != nil {
		return uuid.Nil, serrors.Wrap(op, err)
	}
	spec, ok := view.Catalog.Events[kind]
	if !ok {
		return uuid.Nil, invalid("the event %s is not in your catalog", kind)
	}
	if ev.Subject.Entity == "" {
		// The event's own entity, with no record: the graph runs about such a subject without its state.
		ev.Subject.Entity = spec.Subject
	}
	// The members are kept as written: a number beyond float64 precision is not rounded.
	payload := map[string]json.RawMessage{}
	if len(ev.Payload) > 0 && string(ev.Payload) != "null" {
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload == nil {
			return uuid.Nil, invalid("the payload of a synthetic event is a JSON object")
		}
	}
	if len(ev.Expected) > 50 {
		return uuid.Nil, invalid("at most 50 expected outcomes")
	}
	for node, outcome := range ev.Expected {
		if !validKey(node) || !validKey(outcome) {
			return uuid.Nil, invalid("an expected outcome names a node and an outcome")
		}
	}
	at, dated := ev.OccurredAt, !ev.OccurredAt.IsZero()
	if !dated {
		at = s.cfg.Now()
	}
	kindJSON, err := json.Marshal(kind)
	if err != nil {
		return uuid.Nil, invalid("the kind does not encode")
	}
	payload["kind"] = kindJSON
	event, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, invalid("the payload does not encode")
	}
	snap, approx := ev.Snapshot, false
	if len(snap) > 0 && string(snap) != "null" {
		var m map[string]any
		if json.Unmarshal(snap, &m) != nil || m == nil {
			return uuid.Nil, invalid("the snapshot is a JSON object")
		}
	} else {
		snap, approx, err = s.snapshot(ctx, a.Subject, a.TenantID, ev.Subject, at)
		if err != nil {
			return uuid.Nil, err
		}
		// An event the service dates now: the record read now is its state at the event. An event the
		// author dated keeps the loader's word.
		approx = approx && dated
	}
	if len(event) > MaxEventBytes || len(snap) > MaxEventBytes {
		return uuid.Nil, fmt.Errorf("%w: an event and its snapshot are at most %d KiB each", engine.ErrLimitExceeded, MaxEventBytes>>10)
	}
	title := strings.TrimSpace(ev.Title)
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	ex := exampleRow{id: uuid.New(), source: SourceSynthetic, title: title, kind: kind, event: event, subject: ev.Subject,
		snapshot: snap, approximate: approx, readWith: a.Subject.Permissions, at: at}
	err = pgx.BeginFunc(ctx, s.cfg.DB, func(tx pgx.Tx) error {
		added, err := insertExamples(ctx, tx, a.TenantID, ds, a.UserID, []exampleRow{ex})
		if err != nil {
			return err
		}
		if added.overflow > 0 {
			return fmt.Errorf("%w: a dataset has at most %d examples", engine.ErrLimitExceeded, MaxExamples)
		}
		for node, outcome := range ev.Expected {
			if _, err := tx.Exec(ctx, `
				INSERT INTO automation.dataset_labels (tenant_id, example_id, node_id, source, outcome, set_by)
				VALUES ($1, $2, $3, 'expected', $4, $5)`, a.TenantID, ex.id, node, outcome, a.UserID); err != nil {
				return dbErr(err)
			}
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return ex.id, nil
}

// snapshot is the subject's state at the event: SnapshotAt when the loader has it (exact), else the
// record as it is now (approximate). nil: no subject, no loader, or the record is not visible.
func (s *Svc) snapshot(ctx context.Context, as spi.Subject, tenant uuid.UUID, ref engine.SubjectRef, at time.Time) (json.RawMessage, bool, error) {
	if s.cfg.Subjects == nil || ref.Entity == "" || ref.Entity == graph.SubjectNone || ref.Key == "" {
		return nil, false, nil
	}
	as.TenantID = tenant
	if sl, ok := s.cfg.Subjects.(SnapshotLoader); ok {
		raw, err := sl.SnapshotAt(ctx, as, ref, at)
		switch {
		case errors.Is(err, engine.ErrNotFound):
			return nil, false, nil
		case err != nil:
			return nil, false, serrors.Wrap(op, err)
		}
		return raw, false, nil
	}
	raw, err := s.cfg.Subjects.Load(ctx, as, ref)
	switch {
	case errors.Is(err, engine.ErrNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, serrors.Wrap(op, err)
	}
	return raw, true, nil
}

// exampleRow is an example to insert.
type exampleRow struct {
	id       uuid.UUID
	source   string
	origin   string // the source within its kind: "connection:<id>", "module:<name>"
	ref      string
	importID uuid.UUID
	title    string
	kind     string
	event    json.RawMessage
	subject  engine.SubjectRef
	snapshot json.RawMessage
	// readWith are the permissions the snapshot was read with (the author's, then).
	readWith    []string
	approximate bool
	at          time.Time
}

type insertCounts struct{ added, duplicates, overflow int }

// insertExamples adds examples to a dataset under its row's lock: an example whose origin and
// source_ref are in the dataset is a duplicate (unless the same import added it: an import a worker
// resumes counts it once), one past MaxExamples is overflow.
func insertExamples(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, ds DatasetID, by int64, rows []exampleRow) (insertCounts, error) {
	var c insertCounts
	if _, err := tx.Exec(ctx, `SELECT 1 FROM automation.datasets WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenant, ds); err != nil {
		return c, dbErr(err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.dataset_examples WHERE tenant_id = $1 AND dataset_id = $2 AND deleted_at IS NULL`, tenant, ds).Scan(&n); err != nil {
		return c, dbErr(err)
	}
	for _, r := range rows {
		if r.ref != "" {
			var by *uuid.UUID
			err := tx.QueryRow(ctx, `SELECT import_id FROM automation.dataset_examples
				WHERE tenant_id = $1 AND dataset_id = $2 AND source = $3 AND origin = $4 AND source_ref = $5 AND deleted_at IS NULL`,
				tenant, ds, r.source, r.origin, r.ref).Scan(&by)
			switch {
			case err == nil:
				if r.importID == uuid.Nil || by == nil || *by != r.importID {
					c.duplicates++
				}
				continue
			case !errors.Is(err, pgx.ErrNoRows):
				return c, dbErr(err)
			}
		}
		if n >= MaxExamples {
			c.overflow++
			continue
		}
		var snap any
		var readWith []string
		if len(r.snapshot) > 0 && string(r.snapshot) != "null" {
			snap = r.snapshot
			readWith = append([]string{}, r.readWith...) // not NULL: a snapshot always says what it was read with
		}
		var importID *uuid.UUID
		if r.importID != uuid.Nil {
			importID = &r.importID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO automation.dataset_examples (tenant_id, dataset_id, id, source, origin, source_ref, import_id, title, event_kind, event,
				subject_entity, subject_key, snapshot, snapshot_approximate, snapshot_read_with, occurred_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
			tenant, ds, r.id, r.source, r.origin, r.ref, importID, r.title, r.kind, []byte(r.event), r.subject.Entity, r.subject.Key, snap, r.approximate, readWith, r.at, by); err != nil {
			return c, dbErr(err)
		}
		n++
		c.added++
	}
	if c.added > 0 {
		if _, err := tx.Exec(ctx, `UPDATE automation.datasets SET updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenant, ds); err != nil {
			return c, dbErr(err)
		}
	}
	return c, nil
}

// cursor is the position after the last item of a page.
type cursor struct {
	At time.Time `json:"at"`
	ID uuid.UUID `json:"id"`
}

func encodeCursor(at time.Time, id uuid.UUID) string {
	b, err := json.Marshal(cursor{At: at, ID: id})
	if err != nil { // a time and a UUID always encode
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (*cursor, error) {
	if s == "" {
		return nil, nil //nolint:nilnil // no cursor: the first page
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	var c cursor
	if err != nil || json.Unmarshal(b, &c) != nil {
		return nil, invalid("cursor")
	}
	return &c, nil
}

func pageLimit(n, def, maxN int) int {
	if n <= 0 {
		return def
	}
	return min(n, maxN)
}

const exampleColumns = `e.id, e.dataset_id, e.source, e.source_ref, e.title, e.event_kind, e.event, e.subject_entity, e.subject_key,
	e.snapshot, e.snapshot_approximate, e.occurred_at, e.origin, e.created_by, e.snapshot_read_with`

func scanExample(r pgx.Row) (Example, error) {
	var x Example
	var event, snap []byte
	err := r.Scan(&x.ID, &x.DatasetID, &x.Source, &x.SourceRef, &x.Title, &x.EventKind, &event, &x.Subject.Entity, &x.Subject.Key,
		&snap, &x.SnapshotApproximate, &x.OccurredAt, &x.Origin, &x.CreatedBy, &x.SnapshotReadWith)
	x.Event = event
	if len(snap) > 0 {
		x.Snapshot, x.HasSnapshot = snap, true
	}
	return x, err
}

// labelsOf are the labels in force of examples: the manual label wins over the expected one.
func labelsOf(ctx context.Context, q querier, tenant uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]map[string]Label, error) {
	out := map[uuid.UUID]map[string]Label{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT example_id, node_id, source, outcome FROM automation.dataset_labels
		WHERE tenant_id = $1 AND example_id = ANY($2) ORDER BY example_id, node_id, source`, tenant, ids)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var node, source, outcome string
		if err := rows.Scan(&id, &node, &source, &outcome); err != nil {
			return nil, dbErr(err)
		}
		m := out[id]
		if m == nil {
			m = map[string]Label{}
			out[id] = m
		}
		l := m[node]
		switch source {
		case LabelManual:
			l.Outcome, l.Source = outcome, LabelManual
		case LabelExpected:
			l.Expected = outcome
			if l.Source != LabelManual {
				l.Outcome, l.Source = outcome, LabelExpected
			}
		}
		m[node] = l
	}
	if err := rows.Err(); err != nil {
		return nil, dbErr(err)
	}
	for _, m := range out {
		for node, l := range m {
			if l.Source == LabelExpected {
				l.Expected = ""
				m[node] = l
			}
		}
	}
	return out, nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s *Svc) Examples(ctx context.Context, a Actor, ds DatasetID, f ExampleFilter) (Page[Example], error) {
	if err := s.authorize(a); err != nil {
		return Page[Example]{}, err
	}
	if _, err := s.datasetRow(ctx, a, ds); err != nil {
		return Page[Example]{}, err
	}
	c, err := decodeCursor(f.Cursor)
	if err != nil {
		return Page[Example]{}, err
	}
	limit := pageLimit(f.Limit, 50, 200)
	args := []any{a.TenantID, ds, limit + 1}
	where := ""
	if c != nil {
		args = append(args, c.At, c.ID)
		where += ` AND (e.occurred_at, e.id) > ($4, $5)`
	}
	if f.Labeled != nil {
		not := "NOT "
		if *f.Labeled {
			not = ""
		}
		where += ` AND ` + not + `EXISTS (SELECT 1 FROM automation.dataset_labels l WHERE l.tenant_id = e.tenant_id AND l.example_id = e.id)`
	}
	// The total, the page and its labels are read in one snapshot.
	page := Page[Example]{}
	var items []Example
	var labels map[uuid.UUID]map[string]Label
	err = pgx.BeginTxFunc(ctx, s.cfg.DB, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		countWhere := strings.Replace(where, ` AND (e.occurred_at, e.id) > ($4, $5)`, "", 1)
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.dataset_examples e
			WHERE e.tenant_id = $1 AND e.dataset_id = $2 AND e.deleted_at IS NULL`+countWhere, a.TenantID, ds).Scan(&page.Total); err != nil {
			return dbErr(err)
		}
		rows, err := tx.Query(ctx, `SELECT `+exampleColumns+` FROM automation.dataset_examples e
			WHERE e.tenant_id = $1 AND e.dataset_id = $2 AND e.deleted_at IS NULL`+where+` ORDER BY e.occurred_at, e.id LIMIT $3`, args...)
		if err != nil {
			return dbErr(err)
		}
		if items, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Example, error) { return scanExample(r) }); err != nil {
			return dbErr(err)
		}
		if len(items) > limit {
			items = items[:limit]
			last := items[limit-1]
			page.NextCursor = encodeCursor(last.OccurredAt, last.ID)
		}
		ids := make([]uuid.UUID, len(items))
		for i := range items {
			ids[i] = items[i].ID
		}
		labels, err = labelsOf(ctx, tx, a.TenantID, ids)
		return err
	})
	if err != nil {
		return Page[Example]{}, err
	}
	rd, err := s.readerOf(ctx, a)
	if err != nil {
		return Page[Example]{}, err
	}
	for i := range items {
		items[i].Labels = labels[items[i].ID]
		if items[i].Labels == nil {
			items[i].Labels = map[string]Label{}
		}
		rd.example(ctx, &items[i])
	}
	page.Items = items
	return page, nil
}

// exampleOf reads an example and checks the actor sees its dataset.
func (s *Svc) exampleOf(ctx context.Context, a Actor, ex ExampleID) (Example, error) {
	x, err := scanExample(s.cfg.DB.QueryRow(ctx, `SELECT `+exampleColumns+` FROM automation.dataset_examples e
		WHERE e.tenant_id = $1 AND e.id = $2 AND e.deleted_at IS NULL`, a.TenantID, ex))
	if err != nil {
		return Example{}, dbErr(err)
	}
	if _, err := s.datasetRow(ctx, a, x.DatasetID); err != nil {
		return Example{}, err
	}
	return x, nil
}

func (s *Svc) DeleteExample(ctx context.Context, a Actor, ex ExampleID) error {
	if err := s.authorize(a); err != nil {
		return err
	}
	x, err := s.exampleOf(ctx, a, ex)
	if err != nil {
		return err
	}
	// An example a backtest's result refers to, or a queued or running backtest will run, leaves the
	// dataset but stays for that backtest: it keeps its total, its results and its report. The dataset's
	// row orders the delete with Run, which lists the examples under it: a run either committed (the
	// check sees it) or reads the dataset without the example.
	return pgx.BeginFunc(ctx, s.cfg.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM automation.datasets WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, a.TenantID, x.DatasetID); err != nil {
			return dbErr(err)
		}
		_, err := tx.Exec(ctx, `
			WITH used AS (SELECT EXISTS (SELECT 1 FROM automation.backtest_results WHERE tenant_id = $1 AND example_id = $2)
				OR EXISTS (SELECT 1 FROM automation.backtests WHERE tenant_id = $1 AND status IN ('queued', 'running') AND $2 = ANY(example_ids)) AS yes),
			soft AS (UPDATE automation.dataset_examples SET deleted_at = now() WHERE tenant_id = $1 AND id = $2 AND (SELECT yes FROM used))
			DELETE FROM automation.dataset_examples WHERE tenant_id = $1 AND id = $2 AND NOT (SELECT yes FROM used)`, a.TenantID, ex)
		return dbErr(err)
	})
}

func (s *Svc) Label(ctx context.Context, a Actor, ex ExampleID, node string, outcome string) error {
	if err := s.authorize(a); err != nil {
		return err
	}
	node, outcome = strings.TrimSpace(node), strings.TrimSpace(outcome)
	if !validKey(node) || (outcome != "" && !validKey(outcome)) {
		return invalid("a label names a node and an outcome of 1..100 characters")
	}
	if _, err := s.exampleOf(ctx, a, ex); err != nil {
		return err
	}
	if outcome == "" {
		_, err := s.cfg.DB.Exec(ctx, `DELETE FROM automation.dataset_labels WHERE tenant_id = $1 AND example_id = $2 AND node_id = $3 AND source = 'manual'`,
			a.TenantID, ex, node)
		return dbErr(err)
	}
	_, err := s.cfg.DB.Exec(ctx, `
		INSERT INTO automation.dataset_labels (tenant_id, example_id, node_id, source, outcome, set_by)
		VALUES ($1, $2, $3, 'manual', $4, $5)
		ON CONFLICT (tenant_id, example_id, node_id, source) DO UPDATE SET outcome = EXCLUDED.outcome, set_by = EXCLUDED.set_by, set_at = now()`,
		a.TenantID, ex, node, outcome, a.UserID)
	return dbErr(err)
}

// versionOf reads the version a backtest runs (0: the latest).
func (s *Svc) versionOf(ctx context.Context, a Actor, automationID uuid.UUID, version int) (engine.Version, error) {
	if version <= 0 {
		am, err := s.automation(ctx, a, automationID)
		if err != nil {
			return engine.Version{}, err
		}
		version = am.LatestVersion
	}
	return s.cfg.Automations.GetVersion(ctx, a, automationID, version)
}

// MaxDecisionsPerItem bounds the decisions of one decision node in one example of a graph that has a
// foreach: a node inside it decides once per item. Estimate counts it and a backtest enforces it.
const MaxDecisionsPerItem = 20

// callsPerExample bounds the calls to providers of one example: one per decision node, times
// MaxDecisionsPerItem when the graph has a foreach (its items may each decide).
func callsPerExample(raw json.RawMessage, decisions int) int {
	var g struct {
		Nodes []graph.Node `json:"nodes"`
	}
	if json.Unmarshal(raw, &g) == nil {
		for _, n := range g.Nodes {
			if n.Type == "foreach" {
				return decisions * MaxDecisionsPerItem
			}
		}
	}
	return decisions
}

// decisionNodes are the nodes of a graph whose type answers with a model's decision.
func (s *Svc) decisionNodes(raw json.RawMessage) map[string]bool {
	var g struct {
		Nodes []graph.Node `json:"nodes"`
	}
	out := map[string]bool{}
	if json.Unmarshal(raw, &g) != nil {
		return out
	}
	for _, n := range g.Nodes {
		nt, ok := s.cfg.Catalog.Node(n.Type)
		if !ok {
			continue
		}
		if _, decides := nt.(engine.DecisionNode); decides {
			out[n.ID] = true
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// revisionSQL is the digest of a dataset's examples (alias e), an empty set's too: with the version of
// the graph (revisionOf) it is what a run checks against its estimate.
const revisionSQL = `md5(coalesce(string_agg(e.id::text, ',' ORDER BY e.id), ''))`

// revisionOf is the revision of an estimate: the examples counted and the version of the graph whose
// decision nodes were counted. It is never empty (an empty RunInput.Revision names no estimate).
func revisionOf(examples string, version int) string {
	return fmt.Sprintf("%s.v%d", examples, version)
}

func validDecisions(mode string) error {
	switch mode {
	case "", DecisionsCached, DecisionsLive:
		return nil
	}
	return invalid("decisions is cached or live")
}

func (s *Svc) Estimate(ctx context.Context, a Actor, in RunInput) (Estimate, error) {
	var revision string
	if err := s.authorize(a); err != nil {
		return Estimate{}, err
	}
	if err := validDecisions(in.Decisions); err != nil {
		return Estimate{}, err
	}
	d, err := s.datasetRow(ctx, a, in.DatasetID)
	if err != nil {
		return Estimate{}, err
	}
	v, err := s.versionOf(ctx, a, d.AutomationID, in.Version)
	if err != nil {
		return Estimate{}, err
	}
	nodes := s.decisionNodes(v.Graph)
	perExample := callsPerExample(v.Graph, len(nodes))
	// The examples and their revision are read together: a run named with this revision runs exactly them.
	if err := s.cfg.DB.QueryRow(ctx, `SELECT count(*), `+revisionSQL+` FROM automation.dataset_examples e
		WHERE e.tenant_id = $1 AND e.dataset_id = $2 AND e.deleted_at IS NULL`, a.TenantID, d.ID).Scan(&d.Examples, &revision); err != nil {
		return Estimate{}, dbErr(err)
	}
	est := Estimate{Examples: d.Examples, Revision: revisionOf(revision, v.Version), DecisionNodes: sortedKeys(nodes), MaxCalls: d.Examples * perExample,
		PerItem: perExample > len(nodes), AgentCallsBlocked: a.Origin != engine.OriginHuman}
	if len(nodes) == 0 || d.Examples == 0 {
		return est, nil
	}
	// The average cost of each node's decisions in earlier backtests of the automation.
	rows, err := s.cfg.DB.Query(ctx, `
		SELECT d->>'node', avg((d->>'cost_usd')::float8)
		FROM automation.backtest_results r
		JOIN automation.backtests b ON b.tenant_id = r.tenant_id AND b.id = r.backtest_id
		CROSS JOIN LATERAL jsonb_array_elements(r.decisions) d
		WHERE r.tenant_id = $1 AND b.automation_id = $2 AND d ? 'cost_usd' AND jsonb_typeof(d->'cost_usd') = 'number'
		GROUP BY 1`, a.TenantID, d.AutomationID)
	if err != nil {
		return Estimate{}, dbErr(err)
	}
	avg := map[string]float64{}
	for rows.Next() {
		var node string
		var v float64
		if err := rows.Scan(&node, &v); err != nil {
			rows.Close()
			return Estimate{}, dbErr(err)
		}
		avg[node] = v
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Estimate{}, dbErr(err)
	}
	total, known := 0.0, false
	for n := range nodes {
		if v, ok := avg[n]; ok {
			total += v * float64(d.Examples*perExample/max(len(nodes), 1))
			known = true
		}
	}
	if known {
		est.CostUSD = &total
	}
	return est, nil
}

func (s *Svc) Run(ctx context.Context, a Actor, in RunInput) (BacktestID, error) {
	if err := s.authorize(a); err != nil {
		return uuid.Nil, err
	}
	if err := validDecisions(in.Decisions); err != nil {
		return uuid.Nil, err
	}
	mode := in.Decisions
	if mode == "" {
		mode = DecisionsCached
	}
	if mode == DecisionsLive {
		if a.Origin != engine.OriginHuman {
			return uuid.Nil, fmt.Errorf("%w: only a person runs a backtest with live decisions", engine.ErrHumanRequired)
		}
		// Paid calls start only after their estimate was shown.
		if in.Revision == "" {
			return uuid.Nil, invalid("a backtest with live decisions names its estimate (revision)")
		}
	}
	d, err := s.datasetRow(ctx, a, in.DatasetID)
	if err != nil {
		return uuid.Nil, err
	}
	if d.Examples == 0 {
		return uuid.Nil, invalid("the dataset has no examples")
	}
	v, err := s.versionOf(ctx, a, d.AutomationID, in.Version)
	if err != nil {
		return uuid.Nil, err
	}
	// A backtest that pays its decisions (live, or a person's cached one) is refused up front when strict
	// fencing meets a provider that cannot deduplicate.
	if mode == DecisionsLive || a.Origin == engine.OriginHuman {
		if err := s.unfenced(v.Graph); err != nil {
			return uuid.Nil, err
		}
	}
	id := uuid.New()
	err = pgx.BeginFunc(ctx, s.cfg.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM automation.automations WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, a.TenantID, d.AutomationID); err != nil {
			return dbErr(err)
		}
		var active int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.backtests WHERE tenant_id = $1 AND automation_id = $2 AND status IN ('queued', 'running')`,
			a.TenantID, d.AutomationID).Scan(&active); err != nil {
			return dbErr(err)
		}
		if active >= MaxActiveBacktests {
			return fmt.Errorf("%w: at most %d backtests of an automation run at once", engine.ErrLimitExceeded, MaxActiveBacktests)
		}
		// The examples of the dataset now are what the backtest runs and what its progress counts.
		// The dataset's row serializes imports into it: the examples read here are the ones the run gets.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM automation.datasets WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, a.TenantID, d.ID); err != nil {
			return dbErr(err)
		}
		var ids []uuid.UUID
		var revision string
		if err := tx.QueryRow(ctx, `SELECT coalesce(array_agg(e.id ORDER BY e.occurred_at, e.id), '{}'), `+revisionSQL+`
			FROM automation.dataset_examples e WHERE e.tenant_id = $1 AND e.dataset_id = $2 AND e.deleted_at IS NULL`, a.TenantID, d.ID).Scan(&ids, &revision); err != nil {
			return dbErr(err)
		}
		if len(ids) == 0 {
			return invalid("the dataset has no examples")
		}
		if in.Revision != "" && in.Revision != revisionOf(revision, v.Version) {
			return fmt.Errorf("%w: the dataset or the latest version changed since the estimate", ErrStaleEstimate)
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO automation.backtests (tenant_id, id, automation_id, dataset_id, version, decisions, status, example_ids, total, created_by, created_origin)
			VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7, $8, $9, $10)`, a.TenantID, id, d.AutomationID, d.ID, v.Version, mode, ids, len(ids), a.UserID, string(a.Origin))
		return dbErr(err)
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *Svc) Cancel(ctx context.Context, a Actor, id BacktestID) error {
	if err := s.authorize(a); err != nil {
		return err
	}
	if _, err := s.backtest(ctx, a, id); err != nil {
		return err
	}
	tag, err := s.cfg.DB.Exec(ctx, `UPDATE automation.backtests SET status = 'cancelled', finished_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status IN ('queued', 'running')`, a.TenantID, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: the backtest has finished", engine.ErrRunFinished)
	}
	return nil
}

const backtestColumns = `b.id, b.automation_id, b.dataset_id, d.name, b.version, b.decisions, b.status, b.total,
	(SELECT count(*) FROM automation.backtest_results r WHERE r.tenant_id = b.tenant_id AND r.backtest_id = b.id),
	(SELECT coalesce(sum(r.live_calls), 0) FROM automation.backtest_results r WHERE r.tenant_id = b.tenant_id AND r.backtest_id = b.id), b.error, b.created_by, b.created_origin, b.created_at, b.started_at, b.finished_at`

func scanBacktest(r pgx.Row) (Backtest, error) {
	var b Backtest
	var origin string
	err := r.Scan(&b.ID, &b.AutomationID, &b.DatasetID, &b.DatasetName, &b.Version, &b.Decisions, &b.Status, &b.Total, &b.Done,
		&b.LiveCalls, &b.Error, &b.CreatedBy, &origin, &b.CreatedAt, &b.StartedAt, &b.FinishedAt)
	b.CreatedOrigin = engine.Origin(origin)
	return b, err
}

func (s *Svc) backtest(ctx context.Context, a Actor, id BacktestID) (Backtest, error) {
	b, err := scanBacktest(s.cfg.DB.QueryRow(ctx, `SELECT `+backtestColumns+` FROM automation.backtests b
		JOIN automation.datasets d ON d.tenant_id = b.tenant_id AND d.id = b.dataset_id WHERE b.tenant_id = $1 AND b.id = $2`, a.TenantID, id))
	if err != nil {
		return Backtest{}, dbErr(err)
	}
	if _, err := s.automation(ctx, a, b.AutomationID); err != nil {
		return Backtest{}, err
	}
	return b, nil
}

func (s *Svc) ListBacktests(ctx context.Context, a Actor, automationID uuid.UUID) ([]Backtest, error) {
	if err := s.authorize(a); err != nil {
		return nil, err
	}
	if _, err := s.automation(ctx, a, automationID); err != nil {
		return nil, err
	}
	rows, err := s.cfg.DB.Query(ctx, `SELECT `+backtestColumns+` FROM automation.backtests b
		JOIN automation.datasets d ON d.tenant_id = b.tenant_id AND d.id = b.dataset_id
		WHERE b.tenant_id = $1 AND b.automation_id = $2 ORDER BY b.created_at DESC, b.id LIMIT 100`, a.TenantID, automationID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Backtest, error) { return scanBacktest(r) })
	return out, dbErr(err)
}

func (s *Svc) Count(ctx context.Context, a Actor, automationID uuid.UUID) (int, error) {
	if err := s.authorize(a); err != nil {
		return 0, err
	}
	if _, err := s.automation(ctx, a, automationID); err != nil {
		return 0, err
	}
	var n int
	err := s.cfg.DB.QueryRow(ctx, `SELECT count(*) FROM automation.backtests WHERE tenant_id = $1 AND automation_id = $2`, a.TenantID, automationID).Scan(&n)
	return n, dbErr(err)
}

// resultRows reads the results of a backtest with their examples and labels.
func (s *Svc) resultRows(ctx context.Context, a Actor, id BacktestID) ([]scored, error) {
	tenant := a.TenantID
	rd, err := s.readerOf(ctx, a)
	if err != nil {
		return nil, err
	}
	rows, err := s.cfg.DB.Query(ctx, `
		SELECT r.example_id, e.title, e.occurred_at, e.origin, e.event_kind, e.event, e.subject_entity, e.subject_key, e.snapshot,
			e.created_by, e.snapshot_read_with, r.end_kind, r.triggered, r.snapshot_withheld, r.outcomes, r.decisions, r.effects
		FROM automation.backtest_results r
		JOIN automation.dataset_examples e ON e.tenant_id = r.tenant_id AND e.id = r.example_id
		WHERE r.tenant_id = $1 AND r.backtest_id = $2 ORDER BY e.occurred_at, e.id`, tenant, id)
	if err != nil {
		return nil, dbErr(err)
	}
	type row struct {
		scored
		ex Example
	}
	read, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		var outcomes, decisions, effects, event, snap []byte
		if err := r.Scan(&x.example, &x.title, &x.occurredAt, &x.ex.Origin, &x.ex.EventKind, &event, &x.ex.Subject.Entity, &x.ex.Subject.Key, &snap,
			&x.ex.CreatedBy, &x.ex.SnapshotReadWith, &x.end, &x.triggered, &x.snapshotWithheld, &outcomes, &decisions, &effects); err != nil {
			return x, err
		}
		x.ex.Event = event
		if len(snap) > 0 {
			x.ex.Snapshot = snap
		}
		_ = json.Unmarshal(outcomes, &x.outcomes)
		_ = json.Unmarshal(decisions, &x.decisions)
		_ = json.Unmarshal(effects, &x.effects)
		return x, nil
	})
	if err != nil {
		return nil, dbErr(err)
	}
	// The rights are applied once the rows are closed: reading a connection takes a connection of its
	// own from the pool.
	out := make([]scored, 0, len(read))
	for _, x := range read {
		// The title and the effects' params (the report's samples, a comparison) come from the event and
		// the snapshot: a reader who may not read all of them does not see them.
		switch rd.example(ctx, &x.ex) {
		case seeNone:
			x.title = ""
			fallthrough
		case seePartial:
			for i := range x.effects {
				x.effects[i].Params = json.RawMessage("null")
			}
		case seeAll:
		}
		out = append(out, x.scored)
	}
	ids := make([]uuid.UUID, len(out))
	for i := range out {
		ids[i] = out[i].example
	}
	labels, err := labelsOf(ctx, s.cfg.DB, tenant, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].labels = labels[out[i].example]
	}
	return out, nil
}

// reportOf is the report of a backtest the actor reads.
func (s *Svc) reportOf(ctx context.Context, a Actor, b Backtest) (Report, map[uuid.UUID]scored, error) {
	v, err := s.cfg.Automations.GetVersion(ctx, a, b.AutomationID, b.Version)
	if err != nil {
		return Report{}, nil, err
	}
	rows, err := s.resultRows(ctx, a, b.ID)
	if err != nil {
		return Report{}, nil, err
	}
	byID := make(map[uuid.UUID]scored, len(rows))
	for _, r := range rows {
		byID[r.example] = r
	}
	return buildReport(b, s.decisionNodes(v.Graph), rows), byID, nil
}

func (s *Svc) Report(ctx context.Context, a Actor, id BacktestID) (Report, error) {
	if err := s.authorize(a); err != nil {
		return Report{}, err
	}
	b, err := s.backtest(ctx, a, id)
	if err != nil {
		return Report{}, err
	}
	rep, _, err := s.reportOf(ctx, a, b)
	return rep, err
}

func (s *Svc) Results(ctx context.Context, a Actor, id BacktestID, f ResultFilter) (Page[Result], error) {
	if err := s.authorize(a); err != nil {
		return Page[Result]{}, err
	}
	b, err := s.backtest(ctx, a, id)
	if err != nil {
		return Page[Result]{}, err
	}
	v, err := s.cfg.Automations.GetVersion(ctx, a, b.AutomationID, b.Version)
	if err != nil {
		return Page[Result]{}, err
	}
	nodes := s.decisionNodes(v.Graph)
	c, err := decodeCursor(f.Cursor)
	if err != nil {
		return Page[Result]{}, err
	}
	limit := pageLimit(f.Limit, 50, 200)
	// The filters are SQL over the labels in force (a manual label wins over the expected one): the
	// total counts the whole filtered set, the page reads its own rows only, traces included.
	args := []any{a.TenantID, id, sortedKeys(nodes)}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	// A labeled node whose outcome is not its label; a decision node whose decision is missing from the
	// cache is undecided, not a mismatch.
	mismatch := func(node string) string {
		// As mismatchesOf: the node's first decision decides whether it is undecided.
		return `lbl.outcome IS DISTINCT FROM r.outcomes->>lbl.node_id AND NOT (lbl.node_id = ANY($3)
			AND (SELECT d->>'status' FROM jsonb_array_elements(r.decisions) WITH ORDINALITY AS x(d, n)
				WHERE d->>'node' = lbl.node_id ORDER BY n LIMIT 1) IS NOT DISTINCT FROM 'missing')` + node
	}
	filter := ""
	if f.End != "" {
		filter += ` AND r.end_kind = ` + arg(f.End)
	}
	switch {
	case f.Node != "" && f.Mismatch:
		filter += ` AND EXISTS (SELECT 1 FROM labels lbl WHERE lbl.example_id = r.example_id AND lbl.node_id = ` + arg(f.Node) + ` AND ` + mismatch("") + `)`
	case f.Node != "":
		filter += ` AND EXISTS (SELECT 1 FROM labels lbl WHERE lbl.example_id = r.example_id AND lbl.node_id = ` + arg(f.Node) + `)`
	case f.Mismatch:
		filter += ` AND EXISTS (SELECT 1 FROM labels lbl WHERE lbl.example_id = r.example_id AND ` + mismatch("") + `)`
	}
	from := `
		WITH labels AS (
			SELECT DISTINCT ON (l.example_id, l.node_id) l.example_id, l.node_id, l.outcome
			FROM automation.dataset_labels l
			JOIN automation.backtest_results br ON br.tenant_id = l.tenant_id AND br.backtest_id = $2 AND br.example_id = l.example_id
			WHERE l.tenant_id = $1
			ORDER BY l.example_id, l.node_id, (l.source = 'manual') DESC)
		SELECT %s
		FROM automation.backtest_results r
		JOIN automation.dataset_examples e ON e.tenant_id = r.tenant_id AND e.id = r.example_id
		WHERE r.tenant_id = $1 AND r.backtest_id = $2 AND cardinality($3::text[]) >= 0` + filter
	// The total, the page and its labels are read in one snapshot: a label changed meanwhile does not put
	// a row that no longer mismatches on a page of mismatches.
	page := Page[Result]{Items: []Result{}}
	var all []Result
	var labels map[uuid.UUID]map[string]Label
	err = pgx.BeginTxFunc(ctx, s.cfg.DB, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, fmt.Sprintf(from, "count(*)"), args...).Scan(&page.Total); err != nil {
			return dbErr(err)
		}
		where := ""
		if c != nil {
			where = ` AND (e.occurred_at, e.id) > (` + arg(c.At) + `, ` + arg(c.ID) + `)`
		}
		rows, err := tx.Query(ctx, fmt.Sprintf(from, exampleColumns+`, r.end_kind, r.outcomes, r.decisions, r.effects, r.trace, r.error, r.live_calls, r.snapshot_withheld`)+
			where+` ORDER BY e.occurred_at, e.id LIMIT `+arg(limit+1), args...)
		if err != nil {
			return dbErr(err)
		}
		all, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Result, error) {
			var x Result
			var event, snap, outcomes, decisions, effects, trace []byte
			err := r.Scan(&x.Example.ID, &x.Example.DatasetID, &x.Example.Source, &x.Example.SourceRef, &x.Example.Title, &x.Example.EventKind, &event,
				&x.Example.Subject.Entity, &x.Example.Subject.Key, &snap, &x.Example.SnapshotApproximate, &x.Example.OccurredAt,
				&x.Example.Origin, &x.Example.CreatedBy, &x.Example.SnapshotReadWith, &x.End, &outcomes, &decisions, &effects, &trace, &x.Error, &x.LiveCalls, &x.SnapshotWithheld)
			if err != nil {
				return x, err
			}
			x.Example.Event = event
			if len(snap) > 0 {
				x.Example.Snapshot, x.Example.HasSnapshot = snap, true
			}
			_ = json.Unmarshal(outcomes, &x.Outcomes)
			_ = json.Unmarshal(decisions, &x.Decisions)
			_ = json.Unmarshal(effects, &x.Effects)
			var recs []traceRecord
			_ = json.Unmarshal(trace, &recs)
			x.Trace = traceSteps(recs)
			return x, nil
		})
		if err != nil {
			return dbErr(err)
		}
		if len(all) > limit {
			all = all[:limit]
			last := all[limit-1]
			page.NextCursor = encodeCursor(last.Example.OccurredAt, last.Example.ID)
		}
		ids := make([]uuid.UUID, len(all))
		for i := range all {
			ids[i] = all[i].Example.ID
		}
		labels, err = labelsOf(ctx, tx, a.TenantID, ids)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Page[Result]{}, err
	}
	rd, err := s.readerOf(ctx, a)
	if err != nil {
		return Page[Result]{}, err
	}
	for _, x := range all {
		x.Example.Labels = labels[x.Example.ID]
		if x.Example.Labels == nil {
			x.Example.Labels = map[string]Label{}
		}
		x.Mismatches = mismatchesOf(scored{outcomes: x.Outcomes, decisions: x.Decisions, labels: x.Example.Labels}, nodes)
		rd.result(ctx, &x)
		page.Items = append(page.Items, x)
	}
	return page, nil
}

func (s *Svc) Compare(ctx context.Context, a Actor, a1, a2 BacktestID) (Comparison, error) {
	if err := s.authorize(a); err != nil {
		return Comparison{}, err
	}
	b1, err := s.backtest(ctx, a, a1)
	if err != nil {
		return Comparison{}, err
	}
	b2, err := s.backtest(ctx, a, a2)
	if err != nil {
		return Comparison{}, err
	}
	if b1.AutomationID != b2.AutomationID {
		return Comparison{}, invalid("both backtests are of one automation")
	}
	r1, rows1, err := s.reportOf(ctx, a, b1)
	if err != nil {
		return Comparison{}, err
	}
	r2, rows2, err := s.reportOf(ctx, a, b2)
	if err != nil {
		return Comparison{}, err
	}
	cmp := Comparison{A: r1, B: r2}
	var diffs []ExampleDiff
	for id, x := range rows1 {
		y, ok := rows2[id]
		if !ok {
			continue
		}
		cmp.CommonExamples++
		if d, differ := diffOf(x, y); differ {
			diffs = append(diffs, d)
		}
	}
	sort.Slice(diffs, func(i, j int) bool {
		if !diffs[i].OccurredAt.Equal(diffs[j].OccurredAt) {
			return diffs[i].OccurredAt.Before(diffs[j].OccurredAt)
		}
		return diffs[i].ExampleID.String() < diffs[j].ExampleID.String()
	})
	cmp.Changed = len(diffs)
	if len(diffs) > MaxMismatches {
		diffs = diffs[:MaxMismatches]
	}
	cmp.Diffs = diffs
	return cmp, nil
}
