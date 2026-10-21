-- Project revenue: the contract amount of a project and the acceptance documents that recognise it.
-- +migrate Up
ALTER TABLE projects
    ADD COLUMN contract_amount bigint,
    ADD COLUMN contract_currency_id varchar(3) REFERENCES currencies (code) ON DELETE SET NULL;

CREATE TABLE project_acceptance_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    kind varchar(32) NOT NULL CHECK (kind IN ('ACT', 'DELIVERY_NOTE', 'OTHER')),
    number varchar(64) NOT NULL,
    document_date date NOT NULL,
    amount bigint NOT NULL,
    currency_id varchar(3) NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
    status varchar(16) NOT NULL CHECK (status IN ('DRAFT', 'SIGNED', 'CANCELLED')),
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE INDEX project_acceptance_documents_project_id_idx ON project_acceptance_documents (project_id);

-- +migrate Down
DROP TABLE IF EXISTS project_acceptance_documents;

ALTER TABLE projects
    DROP COLUMN IF EXISTS contract_currency_id,
    DROP COLUMN IF EXISTS contract_amount;
