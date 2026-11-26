package engine

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// LiveRun is a run that is still live (queued, running, waiting or blocked) about a subject.
type LiveRun struct {
	RunID        uuid.UUID
	AutomationID uuid.UUID
	Version      int
}

// SubjectRuns is what the module that owns a record does with the runs about it, as the system and not
// as a person: it reads which runs are still live about the record (a module that routes the record
// through a graph defers to the run) and cancels them when the record goes away (deleted, withdrawn),
// which cancels their waits and open human tasks. It takes no actor: the caller is the record's module,
// and it must not expose these operations to a person without its own check.
type SubjectRuns interface {
	Live(ctx context.Context, tenant uuid.UUID, subject SubjectRef) ([]LiveRun, error)
	// Cancel cancels every live run about the subject and returns how many it cancelled. A run that
	// finishes meanwhile is not an error.
	Cancel(ctx context.Context, tenant uuid.UUID, subject SubjectRef) (int, error)
}

// SubjectRuns is the runtime's SubjectRuns.
func (r *Runtime) SubjectRuns() SubjectRuns { return subjectRuns{r: r} }

type subjectRuns struct{ r *Runtime }

// liveAbout lists the live runs about a subject; a subject of nothing has no runs of its own here.
func (s subjectRuns) liveAbout(ctx context.Context, tenant uuid.UUID, subject SubjectRef) ([]LiveRun, []*uuid.UUID, error) {
	subject = normSubject(subject)
	if tenant == uuid.Nil || subject.Entity == graph.SubjectNone || subject.Key == "" {
		return nil, nil, serrors.New(serrors.Invalid, "automation: a subject has an entity and a key").WithOp(opService)
	}
	rows, err := s.r.reader(ctx).Query(ctx, `SELECT r.id, r.automation_id, r.version, r.instance_id`+runFrom+`
		WHERE r.tenant_id = $1 AND r.subject_entity = $2 AND r.subject_key = $3 AND `+liveSQL+`
		ORDER BY r.created_at, r.id`, tenant, subject.Entity, subject.Key)
	if err != nil {
		return nil, nil, serrors.FromDB(opService, err)
	}
	defer rows.Close()
	var out []LiveRun
	var instances []*uuid.UUID
	for rows.Next() {
		var l LiveRun
		var inst *uuid.UUID
		if err := rows.Scan(&l.RunID, &l.AutomationID, &l.Version, &inst); err != nil {
			return nil, nil, serrors.FromDB(opService, err)
		}
		out = append(out, l)
		instances = append(instances, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, serrors.FromDB(opService, err)
	}
	return out, instances, nil
}

func (s subjectRuns) Live(ctx context.Context, tenant uuid.UUID, subject SubjectRef) ([]LiveRun, error) {
	out, _, err := s.liveAbout(ctx, tenant, subject)
	return out, err
}

func (s subjectRuns) Cancel(ctx context.Context, tenant uuid.UUID, subject SubjectRef) (int, error) {
	list, instances, err := s.liveAbout(ctx, tenant, subject)
	if err != nil {
		return 0, err
	}
	n := 0
	for i, l := range list {
		err := s.r.cancelRun(ctx, tenant, l.RunID, instances[i])
		switch {
		case err == nil:
			n++
		case errors.Is(err, ErrRunFinished):
		default:
			return n, err
		}
	}
	return n, nil
}
