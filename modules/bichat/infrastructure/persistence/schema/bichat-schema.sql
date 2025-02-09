-- +migrate Up
CREATE TABLE prompts
(
    id          VARCHAR(30) PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    description TEXT         NOT NULL,
    prompt      TEXT         NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE dialogues
(
    id         SERIAL PRIMARY KEY,
    user_id    INT          NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label      VARCHAR(255) NOT NULL,
    messages   JSON         NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE INDEX dialogues_user_id_idx ON dialogues (user_id);

-- +migrate Down
DROP TABLE IF EXISTS dialogues CASCADE;
DROP TABLE IF EXISTS prompts CASCADE;
