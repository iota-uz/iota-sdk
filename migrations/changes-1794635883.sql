-- Universal acceptance form: the signing date of a document and open kinds (a protocol, or a kind an application adds).
-- +migrate Up
ALTER TABLE project_acceptance_documents
    ADD COLUMN signed_on date;

UPDATE
    project_acceptance_documents
SET
    signed_on = document_date
WHERE
    status = 'SIGNED';

ALTER TABLE project_acceptance_documents
    DROP CONSTRAINT IF EXISTS project_acceptance_documents_kind_check;

ALTER TABLE project_acceptance_documents
    ADD CONSTRAINT project_acceptance_documents_kind_check CHECK (kind ~ '^[A-Z][A-Z0-9_]*$');

-- +migrate Down
UPDATE
    project_acceptance_documents
SET
    kind = 'OTHER'
WHERE
    kind NOT IN ('ACT', 'DELIVERY_NOTE', 'OTHER');

ALTER TABLE project_acceptance_documents
    DROP CONSTRAINT IF EXISTS project_acceptance_documents_kind_check;

ALTER TABLE project_acceptance_documents
    ADD CONSTRAINT project_acceptance_documents_kind_check CHECK (kind IN ('ACT', 'DELIVERY_NOTE', 'OTHER'));

ALTER TABLE project_acceptance_documents
    DROP COLUMN IF EXISTS signed_on;
