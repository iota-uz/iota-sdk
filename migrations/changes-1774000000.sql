-- +migrate Up
-- Hard reset legacy BiChat artifact rows and move attachments to core upload linkage.
DELETE FROM bichat.artifacts;

ALTER TABLE IF EXISTS bichat.artifacts
    ADD COLUMN IF NOT EXISTS upload_id bigint REFERENCES public.uploads (id) ON DELETE RESTRICT;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_attachment_requires_upload;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_attachment_requires_upload_chk;

ALTER TABLE IF EXISTS bichat.artifacts
    ADD CONSTRAINT artifacts_attachment_requires_upload
    CHECK (type <> 'attachment' OR upload_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS idx_artifacts_upload ON bichat.artifacts (upload_id)
    WHERE upload_id IS NOT NULL;

DROP TABLE IF EXISTS bichat.attachments;

-- +migrate Down
CREATE TABLE IF NOT EXISTS bichat.attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    message_id uuid NOT NULL REFERENCES bichat.messages (id) ON DELETE CASCADE,
    file_name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    size_bytes bigint NOT NULL,
    storage_path text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_attachments_message ON bichat.attachments (message_id);
CREATE INDEX IF NOT EXISTS idx_attachments_created_at ON bichat.attachments (created_at);

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_attachment_requires_upload;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_attachment_requires_upload_chk;

DROP INDEX IF EXISTS bichat.idx_artifacts_upload;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP COLUMN IF EXISTS upload_id;
