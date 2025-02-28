CREATE TABLE authentication_logs
(
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER                  NOT NULL
        CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
    ip         VARCHAR(255)             NOT NULL,
    user_agent VARCHAR(255)             NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE action_logs
(
    id         SERIAL PRIMARY KEY,
    method     VARCHAR(255) NOT NULL,
    path       VARCHAR(255) NOT NULL,
    user_id    INT          REFERENCES users (id) ON DELETE SET NULL,
    after      JSON,
    before     JSON,
    user_agent VARCHAR(255) NOT NULL,
    ip         VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE INDEX action_log_user_id_idx ON action_logs (user_id);
CREATE INDEX authentication_logs_user_id_idx ON authentication_logs (user_id);
CREATE INDEX authentication_logs_created_at_idx ON authentication_logs (created_at);

