-- +migrate Up
-- Add code_interpreter_outputs table for BiChat code execution results

CREATE TABLE IF NOT EXISTS bichat_code_interpreter_outputs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES bichat_messages(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    url TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for fast lookup by message
CREATE INDEX idx_bichat_code_outputs_message ON bichat_code_interpreter_outputs(message_id);

-- Index for created_at ordering
CREATE INDEX idx_bichat_code_outputs_created_at ON bichat_code_interpreter_outputs(created_at);

-- +migrate Down
DROP TABLE IF EXISTS bichat_code_interpreter_outputs CASCADE;
