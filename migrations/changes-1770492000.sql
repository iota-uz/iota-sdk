-- +migrate Up
ALTER TABLE bichat.sessions
    ADD COLUMN IF NOT EXISTS llm_previous_response_id varchar(255);

ALTER TABLE bichat.checkpoints
    ADD COLUMN IF NOT EXISTS previous_response_id varchar(255);

-- +migrate Down
ALTER TABLE bichat.checkpoints
    DROP COLUMN IF EXISTS previous_response_id;

ALTER TABLE bichat.sessions
    DROP COLUMN IF EXISTS llm_previous_response_id;
