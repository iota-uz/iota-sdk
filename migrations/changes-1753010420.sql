-- +migrate Up
-- Create projects module tables

CREATE TABLE projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    counterparty_id uuid NOT NULL REFERENCES counterparty(id) ON DELETE RESTRICT,
    name varchar(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE(tenant_id, name)
);

CREATE TABLE project_stages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    stage_number int NOT NULL,
    description text,
    total_amount bigint NOT NULL,
    start_date date,
    planned_end_date date,
    factual_end_date date,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE(project_id, stage_number)
);

CREATE TABLE project_stage_payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_stage_id uuid NOT NULL REFERENCES project_stages(id) ON DELETE CASCADE,
    payment_id uuid NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    created_at timestamp with time zone DEFAULT now(),
    UNIQUE(project_stage_id, payment_id)
);

-- Indexes for performance
CREATE INDEX projects_tenant_id_idx ON projects(tenant_id);
CREATE INDEX projects_counterparty_id_idx ON projects(counterparty_id);
CREATE INDEX projects_name_idx ON projects(name);

CREATE INDEX project_stages_project_id_idx ON project_stages(project_id);
CREATE INDEX project_stages_stage_number_idx ON project_stages(stage_number);
CREATE INDEX project_stages_start_date_idx ON project_stages(start_date);
CREATE INDEX project_stages_planned_end_date_idx ON project_stages(planned_end_date);
CREATE INDEX project_stages_factual_end_date_idx ON project_stages(factual_end_date);

CREATE INDEX project_stage_payments_project_stage_id_idx ON project_stage_payments(project_stage_id);
CREATE INDEX project_stage_payments_payment_id_idx ON project_stage_payments(payment_id);

-- +migrate Down
-- Drop projects module tables

DROP TABLE IF EXISTS project_stage_payments;
DROP TABLE IF EXISTS project_stages;
DROP TABLE IF EXISTS projects;