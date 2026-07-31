package share

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/periodics"
)

const (
	scheduleClaimLease    = 15 * time.Minute
	scheduleFinishTimeout = 5 * time.Second
)

type Workbook struct {
	Filename string
	Content  []byte
}

type PeriodicExportTask struct {
	runner *ScheduleRunner
	now    func() time.Time
}

func NewPeriodicExportTask(runner *ScheduleRunner) (*PeriodicExportTask, error) {
	if runner == nil {
		return nil, fmt.Errorf("schedule runner is required")
	}
	return &PeriodicExportTask{runner: runner, now: time.Now}, nil
}

func (t *PeriodicExportTask) Name() string     { return "lens-scheduled-exports" }
func (t *PeriodicExportTask) Schedule() string { return "* * * * *" }
func (t *PeriodicExportTask) RunOnStart() bool { return false }
func (t *PeriodicExportTask) Config() periodics.TaskConfig {
	return periodics.TaskConfig{Timeout: 10 * time.Minute, MaxRetries: periodics.IntPtr(1)}
}
func (t *PeriodicExportTask) Execute(ctx context.Context) error {
	return t.runner.RunDue(ctx, t.now().UTC())
}

var _ periodics.PeriodicTask = (*PeriodicExportTask)(nil)

type WorkbookExporter interface {
	ExportSavedView(context.Context, ExportSchedule) (Workbook, error)
}

type Mail struct {
	Recipients []string
	Subject    string
	Body       string
	Attachment Workbook
}

type Mailer interface {
	Send(context.Context, Mail) error
}

type ScheduleRunner struct {
	store    Store
	exporter WorkbookExporter
	mailer   Mailer
	batch    int
	mu       sync.Mutex
}

func NewScheduleRunner(store Store, exporter WorkbookExporter, mailer Mailer, batch int) (*ScheduleRunner, error) {
	if store == nil || exporter == nil || mailer == nil {
		return nil, fmt.Errorf("store, exporter, and mailer are required")
	}
	if batch <= 0 {
		batch = 25
	}
	return &ScheduleRunner{store: store, exporter: exporter, mailer: mailer, batch: batch}, nil
}

// RunDue renders and mails each due workbook independently. A failed delivery
// is recorded and advanced to the next occurrence so one bad address cannot
// hot-loop or block another tenant's schedule.
func (r *ScheduleRunner) RunDue(ctx context.Context, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	due, err := r.store.DueSchedules(ctx, now.UTC(), r.batch)
	if err != nil {
		return err
	}
	var failures []error
	for _, schedule := range due {
		runErr := r.deliver(ctx, schedule)
		finishedAt := time.Now().UTC()
		if finishedAt.Before(now) {
			finishedAt = now
		}
		next, nextErr := schedule.Next(finishedAt)
		if nextErr != nil {
			runErr = errors.Join(runErr, nextErr)
			next = finishedAt.Add(24 * time.Hour).UTC()
		}
		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scheduleFinishTimeout)
		if finishErr := r.store.FinishSchedule(
			finishCtx,
			schedule.TenantID,
			schedule.ID,
			schedule.NextRunAt,
			finishedAt,
			next,
			runErr,
		); finishErr != nil {
			runErr = errors.Join(runErr, finishErr)
		}
		cancel()
		if runErr != nil {
			failures = append(failures, fmt.Errorf("schedule %s: %w", schedule.ID, runErr))
		}
	}
	return errors.Join(failures...)
}

func (r *ScheduleRunner) deliver(ctx context.Context, schedule ExportSchedule) error {
	workbook, err := r.exporter.ExportSavedView(ctx, schedule)
	if err != nil {
		return fmt.Errorf("export workbook: %w", err)
	}
	if len(workbook.Content) == 0 || workbook.Filename == "" {
		return fmt.Errorf("exporter returned an empty workbook")
	}
	return r.mailer.Send(ctx, Mail{
		Recipients: append([]string(nil), schedule.Recipients...),
		Subject:    schedule.Name,
		Body:       "Scheduled Lens dashboard export",
		Attachment: workbook,
	})
}
