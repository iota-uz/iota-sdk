-- The step's context a call node reads (a Code node's ctx: event, nodes, the subject's variables). It
-- is kept apart from the call's workflow instance input, which the workflow API shows without the
-- automation's field rules, and goes with the call's result or its run.
-- +migrate Up
CREATE TABLE automation.call_contexts (
    tenant_id uuid NOT NULL,
    instance_id uuid NOT NULL,
    run_id uuid NOT NULL,
    context jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, instance_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES automation.runs (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX call_contexts_run_idx ON automation.call_contexts (tenant_id, run_id);

-- The sensitive fields a run's input derives from besides its own event's: those of the run whose step
-- started it (start_automation, run_automation). A restart carries them to the new run.
ALTER TABLE automation.runs
    ADD COLUMN inherited jsonb NOT NULL DEFAULT '[]'::jsonb;

-- +migrate Down
ALTER TABLE automation.runs
    DROP COLUMN IF EXISTS inherited;

DROP TABLE IF EXISTS automation.call_contexts;
