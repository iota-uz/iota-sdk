package mcptools

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

type workflowListIn struct {
	Status   string `json:"status,omitempty" jsonschema:"filter: running, waiting, blocked, completed, failed, terminated"`
	Workflow string `json:"workflow,omitempty" jsonschema:"filter by workflow contract ID"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

type workflowList struct {
	Instances []workflow.Instance `json:"instances"`
}

type instanceIn struct {
	ID string `json:"id" jsonschema:"workflow instance ID"`
}

type signalIn struct {
	ID      string          `json:"id"`
	Name    string          `json:"name" jsonschema:"signal name declared by the workflow contract"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Key     string          `json:"key,omitempty" jsonschema:"idempotency key: a signal with the same key is delivered once"`
}

type decideIn struct {
	TaskID   string          `json:"task_id"`
	Decision json.RawMessage `json:"decision" jsonschema:"decision value matching the task's declared decision type"`
}

type startV2In struct {
	ID      string `json:"id" jsonschema:"finished, terminated or blocked predecessor instance"`
	Version string `json:"version,omitempty" jsonschema:"package version to start on; the active one when omitted"`
}

type startV2Out struct {
	Instance string `json:"instance"`
}

type occurrenceIn struct {
	ID     string          `json:"id"`
	Name   string          `json:"name" jsonschema:"occurrence name"`
	N      int             `json:"n" jsonschema:"occurrence counter (0-based repeat count of name)"`
	Result json.RawMessage `json:"result,omitempty" jsonschema:"result to record (resolve only)"`
}

func workflowTools(r *registrar, d Deps) {
	svc := d.Manage

	add(r, mcp.Spec{
		Name:        "ext_workflow_list",
		Description: describe("List workflow instances of your tenant, newest first.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "List workflow instances", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in workflowListIn) (workflowList, error) {
		out, err := svc.ListWorkflows(ctx, actor, workflow.ListFilter{Status: in.Status, Workflow: in.Workflow, Limit: in.Limit, Offset: in.Offset})
		return workflowList{Instances: nonNilSlice(out)}, err
	})

	add(r, mcp.Spec{
		Name:        "ext_workflow_get",
		Description: describe("Read an instance with its history: occurrences, human tasks, signals and the invocations that ran it.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Get workflow instance", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in instanceIn) (workflow.History, error) {
		id, err := parseUUID("id", in.ID)
		if err != nil {
			return workflow.History{}, err
		}
		return svc.GetWorkflow(ctx, actor, id)
	})

	add(r, mcp.Spec{
		Name:        "ext_workflow_signal",
		Description: describe("Deliver a named signal to a waiting instance.", manage.PermOperate),
		Annotations: mcp.Annotations{Title: "Signal workflow", Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermOperate),
	}, func(ctx context.Context, actor spi.Subject, in signalIn) (ack, error) {
		id, err := parseUUID("id", in.ID)
		if err != nil {
			return ack{}, err
		}
		return done, svc.SignalWorkflow(ctx, actor, id, in.Name, in.Payload, in.Key)
	})

	add(r, mcp.Spec{
		Name: "ext_workflow_decide_task",
		Description: "Record the decision of a human task. The assignee may decide it (for a task assigned to a permission, " +
			"anyone who holds that permission); anyone else needs " + manage.PermOperate + ".",
		Annotations: mcp.Annotations{Title: "Decide task", Idempotent: true},
		Policy:      managePolicy(), Meta: meta(),
	}, func(ctx context.Context, actor spi.Subject, in decideIn) (ack, error) {
		id, err := parseUUID("task_id", in.TaskID)
		if err != nil {
			return ack{}, err
		}
		return done, svc.DecideTask(ctx, actor, id, in.Decision)
	})

	add(r, mcp.Spec{
		Name:        "ext_workflow_terminate",
		Description: describe("Stop an instance: open tasks and timers are cancelled, effects already done stay. Children are not terminated.", manage.PermOperate),
		Annotations: mcp.Annotations{Title: "Terminate workflow", Destructive: true, Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermOperate),
	}, func(ctx context.Context, actor spi.Subject, in instanceIn) (ack, error) {
		id, err := parseUUID("id", in.ID)
		if err != nil {
			return ack{}, err
		}
		return done, svc.TerminateWorkflow(ctx, actor, id)
	})

	add(r, mcp.Spec{
		Name:        "ext_workflow_start_v2",
		Description: describe("Start a new instance from a finished, terminated or blocked one on another package version.", manage.PermOperate),
		Annotations: mcp.Annotations{Title: "Start workflow v2"},
		Policy:      managePolicy(), Meta: meta(manage.PermOperate),
	}, func(ctx context.Context, actor spi.Subject, in startV2In) (startV2Out, error) {
		id, err := parseUUID("id", in.ID)
		if err != nil {
			return startV2Out{}, err
		}
		out, err := svc.StartWorkflowV2(ctx, actor, id, in.Version)
		return startV2Out{Instance: uuidString(out)}, err
	})

	add(r, mcp.Spec{
		Name:        "ext_workflow_resolve",
		Description: describe("Complete a blocked call or step with a result you supply.", manage.PermOperate),
		Annotations: mcp.Annotations{Title: "Resolve occurrence", Destructive: true},
		Policy:      managePolicy(), Meta: meta(manage.PermOperate),
	}, func(ctx context.Context, actor spi.Subject, in occurrenceIn) (ack, error) {
		id, err := parseUUID("id", in.ID)
		if err != nil {
			return ack{}, err
		}
		return done, svc.ResolveWorkflowOccurrence(ctx, actor, id, in.Name, in.N, in.Result)
	})

	add(r, mcp.Spec{
		Name:        "ext_workflow_retry",
		Description: describe("Forget an unresolved occurrence so the next run executes it again; call ext_workflow_resume afterwards.", manage.PermOperate),
		Annotations: mcp.Annotations{Title: "Retry occurrence"},
		Policy:      managePolicy(), Meta: meta(manage.PermOperate),
	}, func(ctx context.Context, actor spi.Subject, in occurrenceIn) (ack, error) {
		id, err := parseUUID("id", in.ID)
		if err != nil {
			return ack{}, err
		}
		return done, svc.RetryWorkflowOccurrence(ctx, actor, id, in.Name, in.N)
	})

	add(r, mcp.Spec{
		Name:        "ext_workflow_resume",
		Description: describe("Make a blocked instance runnable again.", manage.PermOperate),
		Annotations: mcp.Annotations{Title: "Resume workflow", Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermOperate),
	}, func(ctx context.Context, actor spi.Subject, in instanceIn) (ack, error) {
		id, err := parseUUID("id", in.ID)
		if err != nil {
			return ack{}, err
		}
		return done, svc.ResumeWorkflow(ctx, actor, id)
	})
}

func uuidString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
