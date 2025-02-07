-- +migrate Up
CREATE TABLE authentication_logs
(
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER                  NOT NULL
        CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
    ip         VARCHAR(255)             NOT NULL,
    user_agent VARCHAR(255)             NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- +migrate Down
DROP TABLE IF EXISTS authentication_logs CASCADE;
